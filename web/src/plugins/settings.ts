import type { PluginID } from "../contracts/plugin-schemas";
import { pluginEnabled } from "../platform/plugin-host.svelte";
import type { SettingsContribution } from "./context";

type SettingsEntryFor<Section extends SettingsContribution> = Omit<
  Section,
  "Component"
> & {
  plugin: PluginID;
  load: () => Promise<{ default: Section["Component"] }>;
};
type LaunchSection = Extract<SettingsContribution, { id: "launch-configs" }>;
type SettingsEntry =
  | SettingsEntryFor<LaunchSection>
  | SettingsEntryFor<Exclude<SettingsContribution, LaunchSection>>;

// These fixed imports are runtime plugin entries, not core dependencies.
const entries: SettingsEntry[] = [
  {
    plugin: "code",
    id: "code-reader",
    label: "代码阅读",
    load: () => import("../components/settings/CodeReaderSettings.svelte"),
  },
  {
    plugin: "quick-commands",
    id: "quick-commands",
    label: "快捷指令",
    load: () => import("../components/settings/QuickCommandsSettings.svelte"),
  },
  {
    plugin: "voice",
    id: "voice",
    label: "语音输入",
    load: () => import("../components/settings/VoiceSettings.svelte"),
  },
  {
    plugin: "launch",
    id: "launch-configs",
    label: "启动配置",
    load: () => import("../components/settings/LaunchConfigsSettings.svelte"),
  },
  {
    plugin: "cloudflare",
    id: "access",
    label: "公网访问",
    load: () => import("../components/settings/TunnelSettings.svelte"),
  },
  {
    plugin: "tailscale",
    id: "tailscale",
    label: "Tailscale",
    load: () => import("../components/settings/TailscaleSettings.svelte"),
  },
  {
    plugin: "notifications",
    id: "push",
    label: "通知推送",
    load: () => import("../components/settings/PushSettings.svelte"),
  },
];

export async function loadSettingsContributions(
  onerror: (id: PluginID, error: unknown) => void,
): Promise<SettingsContribution[]> {
  const sections = await Promise.all(
    entries.map(async (entry): Promise<SettingsContribution | null> => {
      if (!pluginEnabled(entry.plugin)) return null;
      try {
        if (entry.id === "launch-configs") {
          const module = await entry.load();
          return {
            id: entry.id,
            label: entry.label,
            Component: module.default,
          };
        }
        const module = await entry.load();
        return { id: entry.id, label: entry.label, Component: module.default };
      } catch (error) {
        onerror(entry.plugin, error);
        return null;
      }
    }),
  );
  return sections.filter(
    (section): section is SettingsContribution => section !== null,
  );
}
