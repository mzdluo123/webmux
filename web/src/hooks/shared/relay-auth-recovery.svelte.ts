import { createRelayAuthRecovery } from "./relay-auth-recovery";
import { useHostSettingsRecovery } from "./settings-reconnect.svelte";

/** Bounded host/session probe after a relay disconnect; recovers via bootstrap. */
export function useRelayAuthRecovery(
  ready: () => boolean,
  bootstrap: () => void,
) {
  let checking = $state(false);
  const recovery = createRelayAuthRecovery({
    onChecking: (value) => (checking = value),
    onRecovered: useHostSettingsRecovery(ready, bootstrap),
  });
  return {
    get checking() {
      return checking;
    },
    start: () => recovery.start(),
    cancel: () => recovery.cancel(),
  };
}
