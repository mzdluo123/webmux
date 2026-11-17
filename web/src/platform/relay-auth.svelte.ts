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
