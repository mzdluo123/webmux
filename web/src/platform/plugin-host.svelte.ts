import type { Capabilities, PluginID } from "../contracts/plugin-schemas";

// One authenticated browser instance owns this snapshot. Before discovery and
// after authentication changes, optional features are denied by default.
let snapshot = $state.raw<Capabilities | null>(null);
let capabilityRevision = $state(0);
let refreshHost: (() => Promise<void>) | undefined;

/** Settings and reconnects share the authenticated host's validated discovery. */
export function requestPluginHostRefresh(): Promise<void> {
  return refreshHost?.() ?? Promise.resolve();
}

export function onPluginHostRefresh(refresh: () => Promise<void>): () => void {
  refreshHost = refresh;
  return () => {
    if (refreshHost === refresh) refreshHost = undefined;
  };
}

export function pluginCapabilityRevision(): number {
  return capabilityRevision;
}

export function pluginSelectionChanged(next: Capabilities): boolean {
  const selected = (value: Capabilities | null) =>
    value?.plugins
      .filter((plugin) => plugin.enabled)
      .map((plugin) => plugin.id)
      .sort()
      .join(",") ?? "";
  return selected(snapshot) !== selected(next);
}

export function pluginEnabled(id: PluginID): boolean {
  return (
    snapshot?.plugins.some((plugin) => plugin.id === id && plugin.enabled) ??
    false
  );
}

export function pluginAvailable(capabilityID: string): boolean {
  return (
    snapshot?.capabilities.some(
      (capability) => capability.id === capabilityID && capability.available,
    ) ?? false
  );
}

export function setPluginCapabilities(next: Capabilities): void {
  const availability = (value: Capabilities | null) =>
    value?.capabilities
      .filter((capability) => capability.available)
      .map((capability) => capability.id)
      .sort()
      .join(",") ?? "";
  const changed =
    pluginSelectionChanged(next) ||
    availability(snapshot) !== availability(next);
  snapshot = next;
  if (changed) capabilityRevision++;
}

export function resetPluginCapabilities(): void {
  if (snapshot) capabilityRevision++;
  snapshot = null;
}
