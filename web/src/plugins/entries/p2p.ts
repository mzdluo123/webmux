import { startP2P, stopP2P } from "../../platform/p2p.svelte";
import {
  markFirstScreenReady,
  p2pState,
} from "../../platform/p2p-state.svelte";
import {
  pluginAvailable,
  pluginEnabled,
} from "../../platform/plugin-host.svelte";
import { relayAuth } from "../../platform/relay-auth.svelte";
export { p2pState };
export function start(context: { forwardAuth: boolean }) {
  markFirstScreenReady();
  if (
    !context.forwardAuth &&
    pluginEnabled("p2p") &&
    (pluginAvailable("p2p.direct") ||
      (relayAuth.authenticated && pluginAvailable("relay.hybrid")))
  )
    startP2P();
  return stopP2P;
}
