import {
  PluginIDSchema,
  type PluginID,
} from "../../src/contracts/plugin-schemas";
import {
  resetPluginCapabilities,
  setPluginCapabilities,
} from "../../src/platform/plugin-host.svelte";
import {
  loadCreateSettingsContributions,
  resetCreateSettingsContributions,
} from "../../src/plugins/create";
import {
  disposePluginContributions,
  loadPluginContributions,
} from "../../src/plugins/registry.svelte";

const primaryCapabilities: Record<PluginID, string> = {
  launch: "launch.configs",
  workspace: "workspace.manage",
  code: "code.browse",
  files: "files.upload",
  chat: "chat.acp",
  notifications: "notifications.push",
  "quick-commands": "quick-commands.send",
  voice: "voice.recognize",
  forward: "forward.ports",
  cloudflare: "cloudflare.manage",
  tailscale: "tailscale.manage",
  p2p: "p2p.direct",
  mcp: "mcp.control",
  immersive: "immersive.xr",
};

export async function enableCreationPlugins(
  enabled: readonly PluginID[] = [
    "launch",
    "workspace",
    "files",
    "chat",
    "notifications",
  ],
): Promise<void> {
  resetCreateSettingsContributions();
  setPluginCapabilities({
    version: 1,
    plugins: PluginIDSchema.options.map((id) => ({
      id,
      enabled: enabled.includes(id),
      status: enabled.includes(id) ? "ready" : "disabled",
      ...(enabled.includes(id) ? {} : { reason: "plugin_disabled" as const }),
    })),
    capabilities: [
      { id: "terminal.pty", available: true },
      ...enabled.map((id) => ({
        id: primaryCapabilities[id],
        available: true,
      })),
      ...(enabled.includes("workspace") && enabled.includes("code")
        ? [{ id: "workspace.git-workbench", available: true }]
        : []),
    ],
  });
  await loadCreateSettingsContributions();
}

export function resetCreationPlugins(): void {
  resetCreateSettingsContributions();
  resetPluginCapabilities();
}

// Component suites opt into an explicit server profile instead of weakening
// the production host's fail-closed bootstrap. Network starts are not part of
// these mounts; transport behavior is exercised by the corresponding suites.
export async function enableHostPlugins(
  enabled: readonly PluginID[] = PluginIDSchema.options,
): Promise<void> {
  disposePluginContributions();
  await enableCreationPlugins(enabled);
  await loadPluginContributions({ forwardAuth: true, codePopout: true });
}

export function resetHostPlugins(): void {
  disposePluginContributions();
  resetCreationPlugins();
}
