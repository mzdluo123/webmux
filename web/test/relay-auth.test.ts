import { afterEach, expect, test, vi } from "vite-plus/test";
import { api } from "../src/platform/api";
import { NoContentSchema } from "../src/contracts/schemas";
import {
  applyRelayAuth,
  authenticationHeaders,
  clearRelayAuth,
  relayAuthRevision,
} from "../src/platform/relay-auth";
import {
  relayDiagnostic,
  relayEndpoint,
} from "../src/domains/settings/relay-diagnostics";
import { prepareHostAuthentication } from "../src/hooks/shared/host-authentication";

const ownerSession = () => ({
  authenticated: true,
  csrfToken: "owner-csrf",
  expiresAt: Math.floor(Date.now() / 1000) + 3600,
});
afterEach(() => {
  clearRelayAuth();
  vi.unstubAllGlobals();
  vi.useRealTimers();
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

test("absolute owner expiry revokes optimization and returns to the existing login flow", () => {
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
      csrfToken: "expiring-owner",
      expiresAt: Math.floor(Date.now() / 1000) + 1,
    },
    relayAuthRevision(),
  );
  vi.advanceTimersByTime(1000);
  expect(authenticationHeaders()).not.toHaveProperty("X-WebMux-CSRF");
  expect(events).toEqual(["revoked", "auth"]);
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
