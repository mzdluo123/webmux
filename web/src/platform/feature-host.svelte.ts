import type { Capabilities, FeatureID } from "../contracts/feature-schemas";

// One authenticated browser instance owns this snapshot. Before discovery and
// after authentication changes, optional features are denied by default.
let snapshot = $state.raw<Capabilities | null>(null);
let capabilityRevision = $state(0);
let refreshHost: (() => Promise<void>) | undefined;

/** Settings and reconnects share the authenticated host's validated discovery. */
export function requestFeatureHostRefresh(): Promise<void> {
  return refreshHost?.() ?? Promise.resolve();
}

export function onFeatureHostRefresh(refresh: () => Promise<void>): () => void {
  refreshHost = refresh;
  return () => {
    if (refreshHost === refresh) refreshHost = undefined;
  };
}

export function featureCapabilityRevision(): number {
  return capabilityRevision;
}

export function featureSelectionChanged(next: Capabilities): boolean {
  const selected = (value: Capabilities | null) =>
    value?.features
      .filter((feature) => feature.enabled)
      .map((feature) => feature.id)
      .sort()
      .join(",") ?? "";
  return selected(snapshot) !== selected(next);
}

export function featureEnabled(id: FeatureID): boolean {
  return (
    snapshot?.features.some(
      (feature) => feature.id === id && feature.enabled,
    ) ?? false
  );
}

export function featureAvailable(capabilityID: string): boolean {
  return (
    snapshot?.capabilities.some(
      (capability) => capability.id === capabilityID && capability.available,
    ) ?? false
  );
}

export function setFeatureCapabilities(next: Capabilities): void {
  const availability = (value: Capabilities | null) =>
    value?.capabilities
      .filter((capability) => capability.available)
      .map((capability) => capability.id)
      .sort()
      .join(",") ?? "";
  const changed =
    featureSelectionChanged(next) ||
    availability(snapshot) !== availability(next);
  snapshot = next;
  if (changed) capabilityRevision++;
}

export function resetFeatureCapabilities(): void {
  if (snapshot) capabilityRevision++;
  snapshot = null;
}
