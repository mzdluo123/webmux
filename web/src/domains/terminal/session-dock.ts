import { dockGroup, dockSplit, type DockNode } from "$lib/dock-layout";
import type { PaneLayout } from "./terminal-layout";

export function sessionPanelIds(id: string) {
  return {
    terminal: id,
    files: `files:${id}`,
    reader: `reader:${id}`,
  };
}

export function panelSession(ids: readonly string[], panel: string) {
  return ids.find(
    (sid) =>
      Object.values(sessionPanelIds(sid)).includes(panel) ||
      panel.startsWith(`reader:${sid}:`),
  );
}

export function dockSessionLabels(
  ids: readonly string[],
  titleFor: (id: string) => string,
) {
  const labels = new Map<string, { name: string; number?: number }>();
  if (ids.length < 2) return labels;
  const names = ids.map(titleFor);
  const seen = new Set<string>();
  const duplicates = new Set<string>();
  for (const name of names) {
    if (seen.has(name)) duplicates.add(name);
    seen.add(name);
  }
  for (const [index, name] of names.entries()) {
    labels.set(ids[index]!, {
      name,
      number: duplicates.has(name) ? index + 1 : undefined,
    });
  }
  return labels;
}

export function sessionDockTree(
  layout: PaneLayout,
  codeRatio: number,
): DockNode {
  function code(id: string) {
    const { files, reader } = sessionPanelIds(id);
    return dockSplit("horizontal", dockGroup(files), dockGroup(reader), 0.38);
  }

  function visit(node: PaneLayout): DockNode {
    if (node.kind === "pane") {
      return dockSplit(
        "horizontal",
        dockGroup(node.sessionId),
        code(node.sessionId),
        1 - codeRatio,
      );
    }
    return dockSplit(
      node.direction,
      visit(node.first),
      visit(node.second),
      node.ratio,
    );
  }

  return visit(layout);
}
