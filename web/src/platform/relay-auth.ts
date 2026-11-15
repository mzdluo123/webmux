import { RelayAuthSessionSchema } from "../contracts/relay-schemas";
import { stored } from "./storage";

export interface RelayAuthState {
  authenticated: boolean;
  csrfToken: string;
  expiresAt: number;
}

// One active host per app. Owner authorization never enters browser storage.
const state: RelayAuthState = {
  authenticated: false,
  csrfToken: "",
  expiresAt: 0,
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

export function clearRelayAuth() {
  revision++;
  clearTimeout(expiryTimer);
  state.authenticated = false;
  state.csrfToken = "";
  state.expiresAt = 0;
  publish();
  if (typeof window !== "undefined")
    window.dispatchEvent(new Event("webmux:relay-revoked"));
}

export function relayAuthRevision() {
  return revision;
}

export function applyRelayAuth(value: unknown, expectedRevision: number) {
  if (revision !== expectedRevision) return;
  const session = RelayAuthSessionSchema.parse(value);
  clearTimeout(expiryTimer);
  if (!session.authenticated || session.expiresAt * 1000 <= Date.now()) {
    clearRelayAuth();
    return;
  }
  Object.assign(state, session);
  publish();
  expiryTimer = setTimeout(
    () => {
      clearRelayAuth();
      if (typeof window !== "undefined")
        window.dispatchEvent(new Event("webmux:auth"));
    },
    Math.min(session.expiresAt * 1000 - Date.now(), 2_147_483_647),
  );
}

export function authenticationHeaders(): Record<string, string> {
  if (state.authenticated) return { "X-WebMux-CSRF": state.csrfToken };
  return { Authorization: "Bearer " + stored("token") };
}

export function authenticationToken() {
  return state.authenticated ? "" : stored("token");
}

export function relayOwnerAuthenticated() {
  return state.authenticated;
}
