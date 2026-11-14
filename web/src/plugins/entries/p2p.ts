import {
  markFirstScreenReady,
  startP2P,
  stopP2P,
  p2pState,
} from "../../platform/p2p.svelte";
import { pluginAvailable } from "../../platform/plugin-host.svelte";
export { p2pState };
export function start(context: { forwardAuth: boolean }) {
  markFirstScreenReady();
  if (!context.forwardAuth && pluginAvailable("p2p.direct")) startP2P();
  return stopP2P;
}
