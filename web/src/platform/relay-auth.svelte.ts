import { subscribeRelayAuth, type RelayAuthState } from "./relay-auth";

// Process-lifetime render facade; transport/authentication stay rune-free.
export const relayAuth = $state<RelayAuthState>({
  authenticated: false,
  csrfToken: "",
  expiresAt: 0,
});

subscribeRelayAuth((state) => Object.assign(relayAuth, state));
