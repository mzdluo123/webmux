import { m } from "$lib/paraglide/messages.js";
import type { FeatureID } from "../contracts/feature-schemas";
import { featureEnabled } from "../platform/feature-host.svelte";
import type { SettingsContribution } from "./context";

type SettingsEntryFor<Section extends SettingsContribution> = Omit<
  Section,
  "Component" | "label"
> & {
  feature: FeatureID;
  label: () => string;
  load: () => Promise<{ default: Section["Component"] }>;
};
type LaunchSection = Extract<SettingsContribution, { id: "launch-configs" }>;
type SettingsEntry =
  | SettingsEntryFor<LaunchSection>
  | SettingsEntryFor<Exclude<SettingsContribution, LaunchSection>>;

// These fixed imports are runtime feature entries, not core dependencies.
const entries: SettingsEntry[] = [
  {
    feature: "code",
    id: "code-reader",
    label: () => m.s1_settings_nav_code_reader(),
    load: () => import("../components/settings/CodeReaderSettings.svelte"),
  },
  {
    feature: "quick-commands",
    id: "quick-commands",
    label: () => m.s1_settings_nav_quick_commands(),
    load: () => import("../components/settings/QuickCommandsSettings.svelte"),
  },
  {
    feature: "voice",
    id: "voice",
    label: () => m.s1_voice_input(),
    load: () => import("../components/settings/VoiceSettings.svelte"),
  },
  {
    feature: "launch",
    id: "launch-configs",
    label: () => m.s1_settings_nav_launch_configs(),
    load: () => import("../components/settings/LaunchConfigsSettings.svelte"),
  },
  {
    feature: "launch",
    id: "agent-tools",
    label: () => m.agent_tools_title(),
    load: () => import("../components/settings/AgentToolsSettings.svelte"),
  },
  {
    feature: "cloudflare",
    id: "access",
    label: () => m.s1_public_access(),
    load: () => import("../components/settings/TunnelSettings.svelte"),
  },
  {
    feature: "tailscale",
    id: "tailscale",
    label: () => "Tailscale",
    load: () => import("../components/settings/TailscaleSettings.svelte"),
  },
  {
    feature: "notifications",
    id: "push",
    label: () => m.s1_settings_nav_push(),
    load: () => import("../components/settings/PushSettings.svelte"),
  },
];

export async function loadSettingsContributions(
  onerror: (id: FeatureID, error: unknown) => void,
): Promise<SettingsContribution[]> {
  const sections = await Promise.all(
    entries.map(async (entry): Promise<SettingsContribution | null> => {
      if (!featureEnabled(entry.feature)) return null;
      try {
        if (entry.id === "launch-configs") {
          const module = await entry.load();
          return {
            id: entry.id,
            get label() {
              return entry.label();
            },
            Component: module.default,
          };
        }
        const module = await entry.load();
        return {
          id: entry.id,
          get label() {
            return entry.label();
          },
          Component: module.default,
        };
      } catch (error) {
        onerror(entry.feature, error);
        return null;
      }
    }),
  );
  return sections.filter(
    (section): section is SettingsContribution => section !== null,
  );
}
