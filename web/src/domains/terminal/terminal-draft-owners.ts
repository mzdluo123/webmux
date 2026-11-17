import type { TerminalDrafts } from "./terminal-draft.svelte";

const retained = new WeakMap<TerminalDrafts, Map<string, number>>();
const empty: readonly string[] = [];

/** Open owners outlive transient absence from the live and archive lists. */
export function retainTerminalDrafts(
  drafts: TerminalDrafts,
  ids: readonly string[],
): (() => void) | undefined {
  if (!ids.length) return;
  let owners = retained.get(drafts);
  if (!owners) retained.set(drafts, (owners = new Map()));
  for (const id of ids) owners.set(id, (owners.get(id) ?? 0) + 1);
  const current = owners;
  let released = false;
  return () => {
    if (released) return;
    released = true;
    for (const id of ids) {
      const remaining = current.get(id)! - 1;
      if (remaining) current.set(id, remaining);
      else current.delete(id);
    }
    if (!current.size) retained.delete(drafts);
  };
}

export function retainedTerminalDraftIds(
  drafts: TerminalDrafts,
): Iterable<string> {
  return retained.get(drafts)?.keys() ?? empty;
}

export function terminalDraftRetained(
  drafts: TerminalDrafts,
  id: string,
): boolean {
  return retained.get(drafts)?.has(id) ?? false;
}
