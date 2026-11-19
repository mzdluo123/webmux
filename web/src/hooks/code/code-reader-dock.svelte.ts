import { SvelteMap, SvelteSet } from "svelte/reactivity";
import type { DockEdge } from "$lib/dock-layout";
import type { CodeWorkspaceSnapshot } from "../../contracts/schemas";
import { restoreReaderPanels } from "../../domains/code/reader-groups";

type ReaderDock = {
  ids: () => string[];
  add: (id: string, target: string, edge: DockEdge, activate?: boolean) => void;
  remove: (id: string) => void;
  show: (id: string) => void;
};

export function useCodeReaderDock(
  seed: CodeWorkspaceSnapshot | undefined,
  session: () => string,
  panels: { files: string; reader: string } | undefined,
  dock: ReaderDock | undefined,
) {
  const restored = restoreReaderPanels(seed);
  const slots = new SvelteMap(restored.slots);
  let next = restored.next;
  let current = seed?.active ?? "";
  if (!slots.has(current)) current = slots.keys().next().value ?? "";

  function id(slot: number) {
    return slot === 0 ? panels!.reader : `${panels!.reader}:${slot}`;
  }

  function panel(key: string) {
    const slot = slots.get(key);
    return panels && slot !== undefined ? id(slot) : undefined;
  }

  function restore(focus: boolean) {
    if (!dock || !panels) return;
    const existing = new SvelteSet(dock.ids());
    const wanted = new SvelteSet(Array.from(slots.values(), id));
    // Dynamic slots can outlive their documents in the saved layout, including
    // tabs closed in the standalone window while the dock was unmounted.
    for (const placed of existing) {
      if (
        !wanted.has(placed) &&
        (placed.startsWith(`${panels.reader}:`) ||
          (seed && placed === panels.reader))
      ) {
        dock.remove(placed);
        existing.delete(placed);
      }
    }
    const preferred = panel(current);
    let target =
      (preferred && existing.has(preferred) && preferred) ||
      [...wanted].find((placed) => existing.has(placed));
    const fallback = existing.has(panels.files) ? panels.files : session();
    for (const slot of slots.values()) {
      const placed = id(slot);
      if (existing.has(placed) || restored.anchors.has(slot)) continue;
      dock.add(placed, target ?? fallback, target ? "center" : "right", false);
      existing.add(placed);
      target ??= placed;
    }
    for (const [slot, anchor] of restored.anchors) {
      const placed = id(slot);
      if (!existing.has(placed)) dock.add(placed, id(anchor), "center", false);
    }
    // Page navigation should not override saved rail/layout state. Returning
    // from the window is an explicit request to show its selected document.
    if (focus) dock.show(preferred ?? panels.files);
  }

  function select(key: string) {
    current = key;
  }

  function open(
    key: string,
    added: boolean,
    activate: boolean,
    rightOf?: string,
  ) {
    if (added) {
      const anchor = panel(current);
      const slot = !slots.size && !rightOf ? 0 : ++next;
      slots.set(key, slot);
      if (dock && panels) {
        const placed = id(slot);
        if (rightOf) {
          const target = dock.ids().includes(rightOf) ? rightOf : session();
          dock.add(placed, target, "right", false);
        } else if (anchor || !dock.ids().includes(placed)) {
          dock.add(
            placed,
            anchor ?? panels.files,
            anchor ? "center" : "right",
            false,
          );
        }
      }
    }
    if (!added && activate && rightOf && dock) {
      const placed = panel(key);
      if (placed)
        dock.add(
          placed,
          rightOf === placed ? session() : rightOf,
          "right",
          false,
        );
    }
    if (activate) {
      select(key);
      const placed = panel(key);
      if (placed) dock?.show(placed);
    }
  }

  return {
    panel,
    restore,
    select,
    open,
    rightAnchor: () => panel(current) ?? session(),
    close(key: string, fallback: string) {
      const placed = panel(key);
      slots.delete(key);
      if (current === key) current = fallback;
      if (placed) dock?.remove(placed);
    },
    capture: () => Object.fromEntries(slots),
  };
}
