import { createContext, tick } from "svelte";
import type { DockEdge, DockRail, DockRect } from "$lib/dock-layout";

export type DockEntry = { id: string; title: string };

// The source anchors stay in their owning component. Move only the panel's
// single host element, then remove it when the source action is destroyed.
export function mountDockHost(
  node: HTMLElement,
  surface: () => HTMLElement | undefined,
) {
  const target = surface();
  if (target) target.append(node);
  else
    void tick().then(() => {
      if (node.isConnected) surface()?.append(node);
    });
  return { destroy: () => node.remove() };
}
export const [getDock, setDock] = createContext<{
  register: (entry: DockEntry) => () => void;
  canGoBack: () => boolean;
  back: () => void;
  active: (id: string) => boolean;
  select: (id: string) => void;
  add: (id: string, target: string, edge: DockEdge) => void;
  remove: (id: string) => void;
  mount: (node: HTMLElement) => { destroy: () => void };
  rect: (id: string) => DockRect | undefined;
  tabs: (id: string) => DockEntry[];
  targets: (id: string) => DockEntry[];
  compact: () => boolean;
  zoomed: (id: string) => boolean;
  show: (id: string) => void;
  edgeRails: () => boolean;
  peeked: (id: string) => boolean;
  minimize: (id: string, side?: DockRail) => void;
  toggleZoom: (id: string) => void;
  move: (id: string, target: string, edge: DockEdge) => void;
  drag: (id: string | undefined) => void;
}>();
