import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { api } from "../src/platform/api";
import { NoContentSchema } from "../src/contracts/schemas";
import { RelayAuthSessionSchema } from "../src/contracts/relay-schemas";
import {
  applyRelayAuth,
  authenticationHeaders,
  clearRelayAuth,
  relayAuthRevision,
  relayAuthStatus,
  resetRelayAuth,
} from "../src/platform/relay-auth";
import { createRelayAuthRecovery } from "../src/hooks/shared/relay-auth-recovery";
import { prepareHostAuthentication } from "../src/hooks/shared/host-authentication";
import { relayLoginHref } from "../src/hooks/shared/relay-login";

const authenticated = () => ({
  authenticated: true,
  relay: true,
  csrfToken: "memory-only",
  expiresAt: Math.floor(Date.now() / 1000) + 3600,
});
const suspended = {
  authenticated: false,
  relay: true,
  csrfToken: "",
  expiresAt: 0,
  reason: "reconnecting",
};
let stop: (() => void) | undefined;

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-11-01T00:00:00Z"));
  vi.stubGlobal("window", new EventTarget());
  vi.stubGlobal("localStorage", {
    getItem: () => "old-token",
    setItem: vi.fn(),
  });
  vi.stubGlobal("location", { protocol: "https:", host: "arbitrary.example" });
  resetRelayAuth();
});
afterEach(() => {
  stop?.();
  stop = undefined;
  resetRelayAuth();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function recovery() {
  applyRelayAuth(authenticated(), relayAuthRevision());
  clearRelayAuth("reconnecting");
  const onRecovered = vi.fn();
  const onChecking = vi.fn();
  const controller = createRelayAuthRecovery({ onRecovered, onChecking });
  stop = controller.cancel;
  return { ...controller, onRecovered, onChecking };
}

test("transient suspension restores the existing session without storing credentials", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockResolvedValueOnce(Response.json(suspended))
    .mockResolvedValueOnce(Response.json(authenticated()));
  vi.stubGlobal("fetch", fetcher);
  const controller = recovery();
  const expiry = relayAuthStatus().expiresAt;
  controller.start();
  controller.start();
  await vi.advanceTimersByTimeAsync(0);
  expect(fetcher).toHaveBeenCalledTimes(1);
  expect(authenticationHeaders()).toEqual({});
  expect(relayAuthStatus()).toMatchObject({
    relay: true,
    reason: "reconnecting",
    expiresAt: expiry,
  });
  await vi.advanceTimersByTimeAsync(500);
  expect(controller.onRecovered).toHaveBeenCalledOnce();
  expect(authenticationHeaders()).toEqual({ "X-WebMux-CSRF": "memory-only" });
  expect(localStorage.setItem).not.toHaveBeenCalled();
});

test("session probes have a finite backoff budget and an explicit retry starts a new budget", async () => {
  const fetcher = vi.fn<typeof fetch>(async () => Response.json(suspended));
  vi.stubGlobal("fetch", fetcher);
  const controller = recovery();
  controller.start();
  await vi.advanceTimersByTimeAsync(60_000);
  expect(fetcher).toHaveBeenCalledTimes(7);
  expect(controller.onChecking).toHaveBeenLastCalledWith(false);
  expect(controller.onRecovered).not.toHaveBeenCalled();
  await vi.advanceTimersByTimeAsync(60_000);
  expect(fetcher).toHaveBeenCalledTimes(7);
  controller.start();
  await vi.advanceTimersByTimeAsync(0);
  expect(fetcher).toHaveBeenCalledTimes(8);
});

test("hung probes are aborted and cannot exceed the retry budget", async () => {
  const signals: AbortSignal[] = [];
  const fetcher = vi.fn<typeof fetch>((_input, init) => {
    const { promise, reject } = Promise.withResolvers<Response>();
    const signal = init!.signal!;
    signals.push(signal);
    signal.addEventListener(
      "abort",
      () => reject(new DOMException("aborted", "AbortError")),
      { once: true },
    );
    return promise;
  });
  vi.stubGlobal("fetch", fetcher);
  const controller = recovery();
  controller.start();
  await vi.advanceTimersByTimeAsync(60_000);
  expect(fetcher).toHaveBeenCalledTimes(7);
  expect(signals.every((signal) => signal.aborted)).toBe(true);
  expect(controller.onChecking).toHaveBeenLastCalledWith(false);
});

test("cleanup aborts the in-flight probe and ignores even a late successful response", async () => {
  const pending = Promise.withResolvers<Response>();
  const fetcher = vi.fn<typeof fetch>(() => pending.promise);
  vi.stubGlobal("fetch", fetcher);
  const controller = recovery();
  controller.start();
  controller.cancel();
  expect(fetcher.mock.calls[0]![1]!.signal!.aborted).toBe(true);
  pending.resolve(Response.json(authenticated()));
  await vi.advanceTimersByTimeAsync(60_000);
  expect(fetcher).toHaveBeenCalledTimes(1);
  expect(controller.onRecovered).not.toHaveBeenCalled();
  expect(relayAuthStatus().authenticated).toBe(false);
});

test.each(["expired", "revoked", "unauthenticated"] as const)(
  "%s stops recovery and never automatically starts another login",
  async (reason) => {
    const fetcher = vi.fn<typeof fetch>(async () =>
      Response.json({ ...suspended, reason }),
    );
    vi.stubGlobal("fetch", fetcher);
    const controller = recovery();
    controller.start();
    await vi.advanceTimersByTimeAsync(60_000);
    controller.start();
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(relayAuthStatus()).toMatchObject({ relay: true, reason });
    expect(controller.onRecovered).not.toHaveBeenCalled();
  },
);

test("suspension cannot extend the original absolute expiry", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async () => Response.json(suspended)),
  );
  const controller = recovery();
  applyRelayAuth(
    { ...authenticated(), expiresAt: Math.floor(Date.now() / 1000) + 1 },
    relayAuthRevision(),
  );
  clearRelayAuth("reconnecting");
  controller.start();
  await vi.advanceTimersByTimeAsync(5000);
  expect(relayAuthStatus()).toMatchObject({
    relay: true,
    authenticated: false,
    reason: "expired",
  });
  expect(controller.onRecovered).not.toHaveBeenCalled();
});

test("a rejected mutation is not replayed when the relay session recovers", async () => {
  const fetcher = vi.fn<typeof fetch>(async (input) =>
    String(input).endsWith("/relay/auth/session")
      ? Response.json(authenticated())
      : Response.json(
          { error: "host reconnecting", relay: true, reason: "reconnecting" },
          { status: 503 },
        ),
  );
  vi.stubGlobal("fetch", fetcher);
  const controller = recovery();
  applyRelayAuth(authenticated(), relayAuthRevision());
  await expect(
    api("/sessions", NoContentSchema, "POST", {}),
  ).rejects.toMatchObject({ status: 503 });
  expect(relayAuthStatus().reason).toBe("reconnecting");
  controller.start();
  await vi.advanceTimersByTimeAsync(0);
  expect(controller.onRecovered).toHaveBeenCalledOnce();
  expect(
    fetcher.mock.calls.filter(([, init]) => init?.method === "POST"),
  ).toHaveLength(1);
});

test("only server session metadata selects relay login, not the hostname", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async () =>
      Response.json({ ...suspended, reason: "unauthenticated" }),
    ),
  );
  expect(
    await prepareHostAuthentication(
      false,
      new AbortController().signal,
      () => true,
    ),
  ).toBe(false);
  expect(relayAuthStatus().relay).toBe(true);
  resetRelayAuth();
  vi.stubGlobal("location", {
    protocol: "https:",
    host: "h-example.relay.example",
  });
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input) =>
      Response.json(
        String(input).endsWith("/relay/auth/session")
          ? { authenticated: false, csrfToken: "", expiresAt: 0 }
          : { ok: true, sessions: 0 },
      ),
    ),
  );
  expect(
    await prepareHostAuthentication(
      false,
      new AbortController().signal,
      () => true,
    ),
  ).toBe(true);
  expect(relayAuthStatus().relay).toBe(false);
});

test("session contracts accept legacy fields but reject unknown auth reasons", () => {
  expect(
    RelayAuthSessionSchema.safeParse({
      authenticated: false,
      csrfToken: "",
      expiresAt: 0,
    }).success,
  ).toBe(true);
  expect(
    RelayAuthSessionSchema.safeParse({ ...suspended, reason: "guess" }).success,
  ).toBe(false);
});

test.each([
  ["/s/aaaaaaaa?message=m1#history", "/s/aaaaaaaa"],
  ["/settings/relay", "/settings/relay"],
  ["/s/aaaaaaaa#token=private", "/s/aaaaaaaa"],
  ["/settings/relay?token=private#section", "/settings/relay"],
  ["/app?token=private#section", "/app"],
  ["/forward-auth?next=/forward/8080/", "/"],
  [
    "/git/repo/worktrees/work?view=log&scope=branch&branch=topic%2Fbranch&commit=abcd&q=term&author=alice&since=2026-01-01&until=2026-02-01&path=src%2Fmain.ts&returnSession=aaaaaaaa#history",
    "/git/repo/worktrees/work?view=log&scope=branch&branch=topic%2Fbranch&commit=abcd&q=term&author=alice&since=2026-01-01&until=2026-02-01&path=src%2Fmain.ts&returnSession=aaaaaaaa",
  ],
  ["/git?token=private&view=log", "/git?view=log"],
  ["/git?view=log&view=commit&scope=branch", "/git?scope=branch"],
  ["/git?path=%0d%0aLocation%3Aevil&branch=main", "/git?branch=main"],
  ["/git?author=%00&view=log", "/git?view=log"],
  ["/git?#history", "/git"],
  ["https://evil.example/s/aaaaaaaa", "/"],
  ["//evil.example/s/aaaaaaaa", "/"],
  ["/\\evil.example/s/aaaaaaaa", "/"],
  ["/api/sessions", "/"],
  ["/auth/relay/start", "/"],
  ["/s/aaaaaaaa/../../settings", "/"],
  ["/settings\n", "/"],
  ["/settings\t", "/"],
  ["/settings\u007f", "/"],
])("safe relay return path: %s", (input, expected) => {
  const target = new URL(relayLoginHref(input), "https://host.example");
  expect(target.origin).toBe("https://host.example");
  expect(target.pathname).toBe("/auth/relay/start");
  expect(target.searchParams.get("returnTo")).toBe(expected);
});

test("a late pre-recovery auth failure cannot suspend the recovered session again", async () => {
  const pending = Promise.withResolvers<Response>();
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(() => pending.promise),
  );
  const controller = recovery();
  applyRelayAuth(authenticated(), relayAuthRevision());
  const request = api("/sessions", NoContentSchema, "POST");
  clearRelayAuth("reconnecting");
  applyRelayAuth(authenticated(), relayAuthRevision());
  pending.resolve(
    Response.json(
      { error: "old disconnect", relay: true, reason: "reconnecting" },
      { status: 503 },
    ),
  );
  await expect(request).rejects.toMatchObject({ status: 503 });
  expect(relayAuthStatus().authenticated).toBe(true);
  expect(controller.onRecovered).not.toHaveBeenCalled();
});
