import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { api } from "../src/platform/api";
import { NoContentSchema } from "../src/contracts/schemas";
import {
  applyRelayAuth,
  authenticationHeaders,
  clearRelayAuth,
  relayAuthRevision,
  resetRelayAuth,
  relayAuthStatus,
} from "../src/platform/relay-auth";
import {
  relayDiagnostic,
  relayEndpoint,
} from "../src/domains/settings/relay-diagnostics";
import { prepareHostAuthentication } from "../src/hooks/shared/host-authentication";
import { locale } from "../src/lib/locale";
import { RelayStatusSchema } from "../src/contracts/relay-schemas";
import { m } from "$lib/paraglide/messages.js";
import { relayStatus } from "./fixtures/relay";

beforeEach(() => locale.setPreference("zh-CN"));

const ownerSession = () => ({
  authenticated: true,
  relay: true,
  csrfToken: "owner-csrf",
  expiresAt: Math.floor(Date.now() / 1000) + 3600,
});
afterEach(() => {
  resetRelayAuth();
  vi.unstubAllGlobals();
  vi.useRealTimers();
  locale.setPreference("zh-CN");
});

test("late session discovery cannot restore a revoked owner grant", () => {
  const revision = relayAuthRevision();
  clearRelayAuth();
  applyRelayAuth(ownerSession(), revision);
  expect(authenticationHeaders()).not.toHaveProperty("X-WebMux-CSRF");
});

test("Cookie owner writes use CSRF instead of a remembered recovery Token", async () => {
  vi.stubGlobal("localStorage", { getItem: () => "remembered-recovery-token" });
  const fetcher = vi.fn<typeof fetch>(
    async () => new Response(null, { status: 204 }),
  );
  vi.stubGlobal("fetch", fetcher);
  applyRelayAuth(ownerSession(), relayAuthRevision());
  await api("/relay/auth/logout", NoContentSchema, "POST");
  const headers = new Headers(fetcher.mock.calls[0]?.[1]?.headers);
  expect(headers.get("X-WebMux-CSRF")).toBe("owner-csrf");
  expect(headers.has("Authorization")).toBe(false);
});

test.each([
  "candidate:1 1 UDP 1 192.0.2.1 1234 typ relay",
  "https://node.example.test/?token=secret",
  "-----BEGIN PRIVATE KEY-----",
  "turn password=" + "a".repeat(43),
  "2001:db8::1 unreachable",
  "token=short-secret",
  "password: small",
])("sensitive diagnostic material is not rendered: %s", (value) => {
  expect(relayDiagnostic(value)).not.toContain(value);
});

test("credentialized and IP endpoints are never displayed as trusted config", () => {
  expect(relayEndpoint("https://user:secret@node.example.test")).toBe(
    "不可显示的端点",
  );
  expect(relayEndpoint("https://192.0.2.1")).toBe("不可显示的端点");
  expect(relayEndpoint("wss://node.example.test/v1/connector")).toBe(
    "wss://node.example.test/v1/connector",
  );
});

test("absolute owner expiry retains relay mode and requests a new console login", () => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-01-01T00:00:00Z"));
  const target = new EventTarget();
  vi.stubGlobal("window", target);
  const events: string[] = [];
  target.addEventListener("webmux:relay-revoked", () => events.push("revoked"));
  target.addEventListener("webmux:auth", () => events.push("auth"));
  applyRelayAuth(
    {
      authenticated: true,
      relay: true,
      csrfToken: "expiring-owner",
      expiresAt: Math.floor(Date.now() / 1000) + 1,
    },
    relayAuthRevision(),
  );
  vi.advanceTimersByTime(1000);
  expect(authenticationHeaders()).not.toHaveProperty("X-WebMux-CSRF");
  expect(events).toEqual(["revoked", "auth"]);
  expect(relayAuthStatus()).toMatchObject({
    relay: true,
    reason: "expired",
    authenticated: false,
  });
});

test.each([false, true])(
  "HTTPS owner restoration preserves explicit Token preference (%s)",
  async (preferToken) => {
    vi.stubGlobal("location", { protocol: "https:" });
    vi.stubGlobal("localStorage", { getItem: () => "recovery-token" });
    const requests: { path: string; headers: Headers }[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>(async (input, init) => {
        const path = String(input);
        requests.push({ path, headers: new Headers(init?.headers) });
        return Response.json(
          path.endsWith("/relay/auth/session")
            ? ownerSession()
            : { ok: true, sessions: 0 },
        );
      }),
    );
    await prepareHostAuthentication(
      preferToken,
      new AbortController().signal,
      () => true,
    );
    const health = requests.find((request) =>
      request.path.endsWith("/health"),
    )!;
    expect(health.headers.get("Authorization")).toBe(
      preferToken ? "Bearer recovery-token" : null,
    );
    expect(health.headers.get("X-WebMux-CSRF")).toBe(
      preferToken ? null : "owner-csrf",
    );
    expect(
      requests.some((request) => request.path.endsWith("/relay/auth/session")),
    ).toBe(!preferToken);
  },
);

test("obsolete HTTPS bootstrap cannot restore owner auth or advance to health", async () => {
  vi.stubGlobal("location", { protocol: "https:" });
  const response = Promise.withResolvers<Response>();
  const fetcher = vi.fn<typeof fetch>(() => response.promise);
  vi.stubGlobal("fetch", fetcher);
  let current = true;
  const pending = prepareHostAuthentication(
    false,
    new AbortController().signal,
    () => current,
  );
  current = false;
  response.resolve(Response.json(ownerSession()));
  expect(await pending).toBe(false);
  expect(authenticationHeaders()).not.toHaveProperty("X-WebMux-CSRF");
  expect(fetcher).toHaveBeenCalledTimes(1);
});

test("Relay endpoint validation and safe diagnostics follow the current locale", () => {
  const invalid = "https://192.0.2.1";
  locale.setPreference("zh-CN");
  const chineseIssue =
    RelayStatusSchema.shape.publicUrl.safeParse(invalid).error?.issues[0]
      ?.message;
  const chineseReason = relayDiagnostic("authorization-revoked");
  const chineseEndpoint = relayEndpoint(invalid);
  locale.setPreference("en");
  const englishIssue =
    RelayStatusSchema.shape.publicUrl.safeParse(invalid).error?.issues[0]
      ?.message;
  expect(englishIssue).toBe(m.relay_invalid_public_origin());
  expect(englishIssue).not.toBe(chineseIssue);
  expect(relayDiagnostic("authorization-revoked")).toBe(m.relay_diag_revoked());
  expect(relayDiagnostic("authorization-revoked")).not.toBe(chineseReason);
  expect(relayEndpoint(invalid)).toBe(m.relay_endpoint_hidden());
  expect(relayEndpoint(invalid)).not.toBe(chineseEndpoint);
  expect(relayDiagnostic("token=do-not-display")).toBe(m.relay_diag_hidden());
  expect(relayDiagnostic("token=do-not-display")).not.toContain(
    "do-not-display",
  );
});

test("Relay status without public control pins preserves authenticated WSS assignment", () => {
  const status = {
    ...relayStatus,
    controlEndpoint: "https://webmux-relay.rainbit.net",
    controlReady: true,
    controlReachability: "reachable",
    connectorUrl: "wss://node.us1.webmux-relay.rainbit.net/v1/connector",
    dataOrigin: "https://node.us1.webmux-relay.rainbit.net",
    assignmentSource: "authenticated-control",
    assignmentRevision: "2",
    nodeId: "us1",
    capabilities: ["wss", "turn"],
  };

  expect(RelayStatusSchema.parse(status)).toEqual(status);
});
