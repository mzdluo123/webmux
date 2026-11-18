import { dockGroup, dockSplit, type DockNode } from "$lib/dock-layout";
import type { PaneLayout } from "./terminal-layout";

export function sessionPanelIds(id: string) {
  return {
    terminal: id,
    chat: `chat:${id}`,
    files: `files:${id}`,
    reader: `reader:${id}`,
  };
}

export function sessionDockTree(
  layout: PaneLayout,
  hasChat: (id: string) => boolean,
  preferredChat: (id: string) => boolean,
  codeRatio: number,
): DockNode {
  function content(id: string) {
    const { terminal, chat } = sessionPanelIds(id);
    if (!hasChat(id)) return dockGroup(terminal);
    return preferredChat(id)
      ? dockGroup(chat, terminal)
      : dockGroup(terminal, chat);
  }

  function code(id: string) {
    const { files, reader } = sessionPanelIds(id);
    return dockSplit("horizontal", dockGroup(files), dockGroup(reader), 0.25);
  }

  function visit(node: PaneLayout): DockNode {
    if (node.kind === "pane") {
      return dockSplit(
        "horizontal",
        content(node.sessionId),
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

  if (
    layout.kind === "split" &&
    layout.first.kind === "pane" &&
    preferredChat(layout.first.sessionId)
  ) {
    const { sessionId } = layout.first;
    return dockSplit(
      "horizontal",
      content(sessionId),
      dockSplit("vertical", code(sessionId), visit(layout.second), 0.68),
      1 - codeRatio,
    );
  }
  return visit(layout);
}
