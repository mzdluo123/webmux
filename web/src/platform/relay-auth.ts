import {
  RelayAuthSessionSchema,
  type RelayAuthReason,
} from "../contracts/relay-schemas";
import { stored } from "./storage";

export interface RelayAuthState {
  authenticated: boolean;
  csrfToken: string;
  expiresAt: number;
  relay: boolean;
  reason: RelayAuthReason | undefined;
}

// One active host per app. Owner authorization never enters browser storage.
const state: RelayAuthState = {
  authenticated: false,
  csrfToken: "",
  expiresAt: 0,
  relay: false,
  reason: undefined,
};
const listeners = new Set<(state: RelayAuthState) => void>();
let expiryTimer: ReturnType<typeof setTimeout> | undefined;
let revision = 0;

export function subscribeRelayAuth(listener: (state: RelayAuthState) => void) {
  listeners.add(listener);
  listener(state);
  return () => listeners.delete(listener);
}

function publish() {
  for (const listener of listeners) listener(state);
}

export function clearRelayAuth(reason: RelayAuthReason = "revoked") {
  revision++;
  clearTimeout(expiryTimer);
  state.authenticated = false;
  state.csrfToken = "";
  state.expiresAt = reason === "reconnecting" ? state.expiresAt : 0;
  state.reason = reason;
  scheduleExpiry();
  publish();
  if (typeof window !== "undefined")
    window.dispatchEvent(new Event("webmux:relay-revoked"));
}

/** Only an explicit host switch or Token login forgets trusted relay mode. */
export function resetRelayAuth() {
  state.relay = false;
  clearRelayAuth("unauthenticated");
}

export function relayAuthStatus(): Readonly<RelayAuthState> {
  return state;
}

export function relayAuthRevision() {
  return revision;
}

function scheduleExpiry() {
  if (!state.expiresAt) return;
  expiryTimer = setTimeout(
    () => {
      clearRelayAuth("expired");
      if (typeof window !== "undefined")
        window.dispatchEvent(new Event("webmux:auth"));
    },
    Math.min(Math.max(0, state.expiresAt * 1000 - Date.now()), 2_147_483_647),
  );
}

export function applyRelayAuth(value: unknown, expectedRevision: number) {
  if (revision !== expectedRevision) return;
  const session = RelayAuthSessionSchema.parse(value);
  clearTimeout(expiryTimer);
  state.relay = session.relay ?? state.relay;
  if (!session.authenticated || session.expiresAt * 1000 <= Date.now()) {
    clearRelayAuth(
      session.authenticated ? "expired" : (session.reason ?? "unauthenticated"),
    );
    return;
  }
  const restored = !state.authenticated;
  revision++;
  Object.assign(state, session, { reason: undefined });
  scheduleExpiry();
  publish();
  if (restored && typeof window !== "undefined")
    window.dispatchEvent(new Event("webmux:relay-restored"));
}

export function authenticationHeaders(): Record<string, string> {
  if (state.authenticated) return { "X-WebMux-CSRF": state.csrfToken };
  if (state.relay) return {};
  return { Authorization: "Bearer " + stored("token") };
}

export function authenticationToken() {
  return state.authenticated || state.relay ? "" : stored("token");
}

export function relayOwnerAuthenticated() {
  return state.authenticated;
}
