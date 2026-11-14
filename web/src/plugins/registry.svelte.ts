import type { PluginID } from "../contracts/plugin-schemas";
import { pluginAvailable, pluginEnabled } from "../platform/plugin-host.svelte";
import { errorMessage } from "../platform/error";
import {
  loadCreateSettingsContributions,
  resetCreateSettingsContributions,
} from "./create";
import type * as WorkspaceEntry from "./entries/workspace";
import type * as LaunchEntry from "./entries/launch";
import type * as ChatEntry from "./entries/chat";
import type * as NotificationsEntry from "./entries/notifications";
import type * as ForwardEntry from "./entries/forward";
import type * as P2PEntry from "./entries/p2p";
import type * as CodeEntry from "./entries/code";
import type * as CloudflareEntry from "./entries/cloudflare";
import type * as TailscaleEntry from "./entries/tailscale";

const entries = {
  workspace: () => import("./entries/workspace"),
  launch: () => import("./entries/launch"),
  chat: () => import("./entries/chat"),
  notifications: () => import("./entries/notifications"),
  forward: () => import("./entries/forward"),
  p2p: () => import("./entries/p2p"),
  code: () => import("./entries/code"),
  cloudflare: () => import("./entries/cloudflare"),
  tailscale: () => import("./entries/tailscale"),
} as const;

interface Entries {
  workspace?: typeof WorkspaceEntry;
  launch?: typeof LaunchEntry;
  chat?: typeof ChatEntry;
  notifications?: typeof NotificationsEntry;
  forward?: typeof ForwardEntry;
  p2p?: typeof P2PEntry;
  code?: typeof CodeEntry;
  cloudflare?: typeof CloudflareEntry;
  tailscale?: typeof TailscaleEntry;
}
let loaded = $state.raw<Entries>({});
let errors = $state.raw<Partial<Record<PluginID, string>>>({});
let generation = 0;
let runtimes: Partial<
  Record<PluginID, { available: boolean; dispose?: () => void }>
> = {};

type HostContext = { forwardAuth: boolean; codePopout: boolean };

/** Rebind only the lifecycle whose business availability changed. */
export function refreshPluginRuntimes(context: HostContext): void {
  for (const [id, capability] of [
    ["notifications", "notifications.push"],
    ["p2p", "p2p.direct"],
  ] as const) {
    const entry = loaded[id];
    if (!entry) continue;
    const available = pluginAvailable(capability);
    const previous = runtimes[id];
    if (previous?.available === available) continue;
    previous?.dispose?.();
    runtimes[id] = { available, dispose: entry.start(context) };
  }
}

export const pluginContributions = {
  get entries() {
    return loaded;
  },
  get errors() {
    return errors;
  },
};

export async function loadPluginContributions(
  context: HostContext,
): Promise<void> {
  disposePluginContributions();
  const current = generation;
  await Promise.all(
    Object.entries(entries).map(async ([key, load]) => {
      const id = key as keyof typeof entries;
      if (!pluginEnabled(id)) return;
      try {
        const entry = await load();
        if (generation !== current) return;
        loaded = { ...loaded, [id]: entry };
        refreshPluginRuntimes(context);
      } catch (error) {
        if (generation === current)
          errors = { ...errors, [id]: errorMessage(error) };
      }
    }),
  );
  if (generation !== current) return;
  const contributionErrors = await loadCreateSettingsContributions();
  if (generation === current) errors = { ...errors, ...contributionErrors };
}

export function disposePluginContributions(): void {
  generation++;
  for (const runtime of Object.values(runtimes).reverse()) runtime.dispose?.();
  runtimes = {};
  loaded = {};
  errors = {};
  resetCreateSettingsContributions();
}
