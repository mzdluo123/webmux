import {
  FeatureIDSchema,
  type FeatureID,
} from "../../src/contracts/feature-schemas";
import {
  resetFeatureCapabilities,
  setFeatureCapabilities,
} from "../../src/platform/feature-host.svelte";
import {
  loadCreateSettingsContributions,
  resetCreateSettingsContributions,
} from "../../src/features/create";
import {
  disposeFeatureContributions,
  loadFeatureContributions,
} from "../../src/features/registry.svelte";
import { loadTerminalFeatures } from "../../src/features/terminal";

const primaryCapabilities: Record<FeatureID, string> = {
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
  messaging: "messaging.messages",
};

export async function enableCreationFeatures(
  enabled: readonly FeatureID[] = [
    "launch",
    "workspace",
    "files",
    "chat",
    "notifications",
  ],
  projects = false,
): Promise<void> {
  resetCreateSettingsContributions();
  setFeatureCapabilities({
    version: 2,
    features: FeatureIDSchema.options.map((id) => ({
      id,
      enabled: enabled.includes(id),
      status: enabled.includes(id) ? "ready" : "disabled",
      ...(enabled.includes(id) ? {} : { reason: "feature_disabled" as const }),
    })),
    capabilities: [
      { id: "terminal.pty", available: true },
      ...(projects && enabled.includes("workspace")
        ? [
            { id: "workspace.projects", available: true },
            { id: "workspace.clone", available: true },
            { id: "workspace.github", available: true },
          ]
        : []),
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

export function resetCreationFeatures(): void {
  resetCreateSettingsContributions();
  resetFeatureCapabilities();
}

// Component suites opt into an explicit server profile instead of weakening
// the production host's fail-closed bootstrap. Network starts are not part of
// these mounts; transport behavior is exercised by the corresponding suites.
export async function enableHostFeatures(
  enabled: readonly FeatureID[] = FeatureIDSchema.options,
): Promise<void> {
  disposeFeatureContributions();
  await enableCreationFeatures(enabled);
  await loadFeatureContributions({ forwardAuth: true, codePopout: true });
  await loadTerminalFeatures();
}

export function resetHostFeatures(): void {
  disposeFeatureContributions();
  resetCreationFeatures();
}
