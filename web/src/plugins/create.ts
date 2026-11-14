import type { PluginID } from "../contracts/plugin-schemas";
import { pluginEnabled } from "../platform/plugin-host.svelte";
import { errorMessage } from "../platform/error";
import {
  setPluginContributions,
  type CreationContributions,
  type PluginContributions,
} from "./context";
import { loadSettingsContributions } from "./settings";

let generation = 0;

// Runtime plugin selection must precede implementation imports: disabled
// providers must not load their modules or initialize their resources.
export async function loadCreateSettingsContributions(): Promise<
  Partial<Record<PluginID, string>>
> {
  const current = generation;
  const creation: CreationContributions = {};
  const network: PluginContributions["network"] = {};
  const errors: Partial<Record<PluginID, string>> = {};
  function failed(id: PluginID, error: unknown) {
    errors[id] = errorMessage(error);
  }
  async function load(id: PluginID, importEntry: () => Promise<void>) {
    if (!pluginEnabled(id)) return;
    try {
      await importEntry();
    } catch (error) {
      failed(id, error);
    }
  }
  const [settings] = await Promise.all([
    loadSettingsContributions(failed),
    load("launch", async () => {
      creation.LaunchPicker = (
        await import("../components/sessions/LaunchPicker.svelte")
      ).default;
    }),
    load("workspace", async () => {
      const [location, fields] = await Promise.all([
        import("../components/workspace/LocationPicker.svelte"),
        import("../components/workspace/WorktreeCreateFields.svelte"),
      ]);
      creation.LocationPicker = location.default;
      creation.WorktreeCreateFields = fields.default;
    }),
    load("files", async () => {
      creation.DirectoryPicker = (
        await import("../components/workspace/DirectoryPicker.svelte")
      ).default;
    }),
    load("chat", async () => {
      creation.ConnectorInstallHint = (
        await import("../components/sessions/ConnectorInstallHint.svelte")
      ).default;
      creation.LaunchChatFields = (
        await import("../components/settings/LaunchChatFields.svelte")
      ).default;
      creation.LaunchStartSideFields = (
        await import("../components/sessions/LaunchStartSideFields.svelte")
      ).default;
    }),
    load("notifications", async () => {
      if (pluginEnabled("launch"))
        creation.LaunchAttentionFields = (
          await import("../components/settings/LaunchAttentionFields.svelte")
        ).default;
    }),
    load("p2p", async () => {
      network.P2PSettings = (
        await import("../components/settings/P2PSettings.svelte")
      ).default;
    }),
    load("forward", async () => {
      network.ForwardSettings = (
        await import("../components/settings/ForwardSettings.svelte")
      ).default;
    }),
  ]);
  if (current === generation)
    setPluginContributions({ creation, network, settings });
  return errors;
}

export function resetCreateSettingsContributions(): void {
  generation++;
  setPluginContributions({ creation: {}, settings: [], network: {} });
}
