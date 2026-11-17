import { subscribeRelayAuth, type RelayAuthState } from "./relay-auth";

// Process-lifetime render facade; transport/authentication stay rune-free.
export const relayAuth = $state<RelayAuthState>({
  authenticated: false,
  csrfToken: "",
  expiresAt: 0,
  relay: false,
  reason: undefined,
});

subscribeRelayAuth((state) => Object.assign(relayAuth, state));

// A relay address without a live login blocks access; the presentation differs
// by reason: only an explicit login reason gates, reconnecting only indicates.
export function relayBlocked() {
  return relayAuth.relay && !relayAuth.authenticated;
}
export function relayGate() {
  return relayBlocked() && relayAuth.reason !== "reconnecting";
}
export function relayReconnecting() {
  return relayBlocked() && relayAuth.reason === "reconnecting";
}

/** The props every reconnect surface (pill, sidebar footer) renders. */
export interface RelayReconnection {
  checking: boolean;
  returnTo: string;
  onretry: () => void;
}
