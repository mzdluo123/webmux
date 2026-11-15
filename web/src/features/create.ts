import type { FeatureID } from "../contracts/feature-schemas";
import { featureEnabled } from "../platform/feature-host.svelte";
import { errorMessage } from "../platform/error";
import {
  setFeatureContributions,
  type CreationContributions,
  type FeatureContributions,
} from "./context";
import { loadSettingsContributions } from "./settings";

let generation = 0;

// Runtime feature selection must precede implementation imports: disabled
// providers must not load their modules or initialize their resources.
export async function loadCreateSettingsContributions(): Promise<
  Partial<Record<FeatureID, string>>
> {
  const current = generation;
  const creation: CreationContributions = {};
  const network: FeatureContributions["network"] = {};
  const errors: Partial<Record<FeatureID, string>> = {};
  function failed(id: FeatureID, error: unknown) {
    errors[id] = errorMessage(error);
  }
  async function load(id: FeatureID, importEntry: () => Promise<void>) {
    if (!featureEnabled(id)) return;
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
      const [location, fields, projects] = await Promise.all([
        import("../components/workspace/LocationPicker.svelte"),
        import("../components/workspace/WorktreeCreateFields.svelte"),
        import("../components/workspace/ProjectPicker.svelte"),
      ]);
      creation.ProjectPicker = projects.default;
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
      if (featureEnabled("launch"))
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
    setFeatureContributions({ creation, network, settings });
  return errors;
}

export function resetCreateSettingsContributions(): void {
  generation++;
  setFeatureContributions({ creation: {}, settings: [], network: {} });
}
