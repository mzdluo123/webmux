import { createContext, tick } from "svelte";
import type { DockEdge, DockRect } from "$lib/dock-layout";

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
  mount: (node: HTMLElement) => { destroy: () => void };
  rect: (id: string) => DockRect | undefined;
  tabs: (id: string) => DockEntry[];
  targets: (id: string) => DockEntry[];
  compact: () => boolean;
  zoomed: (id: string) => boolean;
  show: (id: string) => void;
  minimize: (id: string) => void;
  toggleZoom: (id: string) => void;
  move: (id: string, target: string, edge: DockEdge) => void;
  drag: (id: string | undefined) => void;
  dragOver: (event: DragEvent, target: string) => void;
  drop: (event: DragEvent, target: string) => void;
}>();
