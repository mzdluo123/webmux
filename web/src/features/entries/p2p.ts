import { startP2P, stopP2P } from "../../platform/p2p.svelte";
import {
  markFirstScreenReady,
  p2pState,
} from "../../platform/p2p-state.svelte";
import {
  featureAvailable,
  featureEnabled,
} from "../../platform/feature-host.svelte";
import { relayAuth } from "../../platform/relay-auth.svelte";
export { p2pState };
export function start(context: { forwardAuth: boolean }) {
  markFirstScreenReady();
  if (
    !context.forwardAuth &&
    featureEnabled("p2p") &&
    (featureAvailable("p2p.direct") ||
      (relayAuth.authenticated && featureAvailable("relay.hybrid")))
  )
    startP2P();
  return stopP2P;
}
