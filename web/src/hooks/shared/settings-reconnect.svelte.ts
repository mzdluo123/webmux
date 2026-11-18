import {
  settings,
  startSettings,
} from "../../domains/settings/settings.svelte";
import { requestFeatureHostRefresh } from "../../platform/feature-host.svelte";

/** Settings can reconnect while no shell session feed is mounted. */
export function useHostSettingsRecovery(
  ready: () => boolean,
  bootstrap: () => void,
) {
  let hadConnection = false;
  let wasConnected = false;
  $effect(() => {
    const connected = settings.connected;
    if (ready() && connected && !wasConnected && hadConnection)
      void requestFeatureHostRefresh();
    if (connected) hadConnection = true;
    wasConnected = connected;
  });
  return () => {
    if (!ready()) bootstrap();
    else startSettings();
  };
}
