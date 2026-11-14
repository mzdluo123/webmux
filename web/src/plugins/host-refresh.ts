import { tick } from "svelte";
import { CapabilitiesSchema } from "../contracts/plugin-schemas";
import { observeSettings, settings } from "../domains/settings/settings.svelte";
import { clearHomeCache } from "../domains/workspace/home-cache";
import { clearSessionListSnapshot } from "../hooks/sessions/session-list.svelte";
import { api } from "../platform/api";
import { errorMessage } from "../platform/error";
import {
  onPluginHostRefresh,
  pluginSelectionChanged,
  requestPluginHostRefresh,
  setPluginCapabilities,
} from "../platform/plugin-host.svelte";
import {
  disposePluginContributions,
  loadPluginContributions,
  refreshPluginRuntimes,
} from "./registry.svelte";

interface HostRefreshOptions {
  ready(): boolean;
  epoch(): number;
  context(): { forwardAuth: boolean; codePopout: boolean };
  onRebuilding(value: boolean): void;
  onError(message: string): void;
}

/** Own the coalesced discovery request and its settings subscription. */
export function watchPluginHostRefresh(options: HostRefreshOptions): {
  cancel(): void;
  dispose(): void;
} {
  let settingsRevision = settings.revision;
  let refreshAbort: AbortController | undefined;
  let refreshing: Promise<void> | undefined;
  let refreshAgain = false;
  const refreshCapabilities = (): Promise<void> => {
    if (!options.ready()) return Promise.resolve();
    if (refreshing) {
      refreshAgain = true;
      return refreshing;
    }
    const epoch = options.epoch();
    const request = new AbortController();
    refreshAbort = request;
    const stale = () => epoch !== options.epoch() || request.signal.aborted;
    refreshing = (async () => {
      do {
        refreshAgain = false;
        try {
          const snapshot = await api(
            "/capabilities",
            CapabilitiesSchema,
            "GET",
            undefined,
            request.signal,
          );
          if (stale()) return;
          if (pluginSelectionChanged(snapshot)) {
            // Unmount consumers before disposing their captured controllers.
            // App-owned drafts survive this server cutover.
            options.onRebuilding(true);
            await tick();
            if (stale()) return;
            disposePluginContributions();
            clearHomeCache();
            clearSessionListSnapshot();
            setPluginCapabilities(snapshot);
            await loadPluginContributions(options.context());
            if (stale()) return;
            options.onRebuilding(false);
          } else {
            // Business settings must not remount an existing terminal.
            setPluginCapabilities(snapshot);
            refreshPluginRuntimes(options.context());
          }
          options.onError("");
        } catch (failure) {
          if (stale()) return;
          options.onRebuilding(false);
          options.onError(`功能状态刷新失败：${errorMessage(failure)}`);
        }
      } while (refreshAgain);
    })().finally(() => {
      refreshing = undefined;
    });
    return refreshing;
  };
  const disconnectRefresh = onPluginHostRefresh(refreshCapabilities);
  const disconnectSettings = observeSettings(() => {
    if (!options.ready() || settings.revision === settingsRevision) return;
    settingsRevision = settings.revision;
    void requestPluginHostRefresh();
  });
  return {
    cancel: () => refreshAbort?.abort(),
    dispose() {
      disconnectSettings();
      disconnectRefresh();
      refreshAbort?.abort();
    },
  };
}
