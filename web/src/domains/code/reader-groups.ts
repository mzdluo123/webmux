import type { CodeWorkspaceSnapshot } from "../../contracts/schemas";

/** Keep the selected document in each legacy group's original dock slot. */
export function restoreReaderPanels(seed?: CodeWorkspaceSnapshot) {
  const active = seed?.active ?? "";
  const groups = new Map<number, string[]>();
  for (const { key } of seed?.tabs ?? []) {
    const id = seed?.readerGroups?.[key] ?? 0;
    const group = groups.get(id);
    if (group) group.push(key);
    else groups.set(id, [key]);
  }
  let next = Math.max(0, ...groups.keys());
  const slots = new Map<string, number>();
  const anchors = new Map<number, number>();
  for (const [id, keys] of groups) {
    const preferred = keys.includes(active)
      ? active
      : seed?.readerSelections?.[id];
    const selected =
      preferred && keys.includes(preferred) ? preferred : keys[0];
    for (const key of keys) {
      const slot = key === selected ? id : ++next;
      slots.set(key, slot);
      if (slot !== id) anchors.set(slot, id);
    }
  }
  return { slots, anchors, next };
}
