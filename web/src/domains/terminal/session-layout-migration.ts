import {
  dockGeometry,
  dockGroup,
  dockIds,
  restoreDock,
  type DockNode,
} from "$lib/dock-layout";

/** Collapse legacy view leaves without changing surviving split ratios. */
export function migrateSessionLayout(
  raw: string,
  sessions: readonly string[],
  chat: (id: string) => boolean,
) {
  const old = restoreDock(raw);
  if (!old) return undefined;
  const all = dockIds(old.node);
  const present = new Set(all);
  const hidden = new Set(old.hidden);
  const visible = dockGeometry(old.node, present, hidden).groups;
  const replacement = new Map<string, string>();
  const minimized: string[] = [];
  for (const id of sessions) {
    const views = [id, `chat:${id}`].filter((key) => present.has(key));
    const preferred = chat(id) ? `chat:${id}` : id;
    const keep =
      (visible.has(preferred) && preferred) ||
      views.find((key) => visible.has(key)) ||
      views.find((key) => !hidden.has(key)) ||
      views[0];
    if (keep) {
      replacement.set(keep, id);
      if (views.every((key) => hidden.has(key))) minimized.push(id);
    }
    for (const key of [`files:${id}`, `reader:${id}`]) {
      replacement.set(key, key);
      if (hidden.has(key)) minimized.push(key);
    }
  }
  function prune(node: DockNode): DockNode | undefined {
    if (node.kind === "group") {
      const group = dockGroup(
        ...node.ids.flatMap((key) => replacement.get(key) ?? []),
      );
      if (!group.ids.length) return undefined;
      group.active = replacement.get(node.active) ?? group.active;
      return group;
    }
    const first = prune(node.first),
      second = prune(node.second);
    return first && second ? { ...node, first, second } : (first ?? second);
  }
  const node = prune(old.node);
  return node ? { node, hidden: minimized } : undefined;
}
