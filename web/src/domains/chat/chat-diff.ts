import type { ChatContent, ToolStatus } from "../../contracts/schemas";

export type TurnDiffState = {
  id: string;
  tools: ReadonlyMap<string, { status: ToolStatus; diffs: ChatContent[] }>;
};

export function summarizeTurnDiff(turn: TurnDiffState) {
  const changes = new Map<string, ChatContent[]>();
  for (const tool of turn.tools.values()) {
    if (tool.status !== "completed") continue;
    for (const block of tool.diffs) {
      // Go omits empty newText and nil oldText (ACP's new-file origin).
      // Only snapshot content identifies an absent side; patch-only/unknown
      // blocks must not acquire a fabricated whole-file baseline.
      const snapshot =
        !block.text &&
        !block.truncated &&
        (block.oldText !== undefined || block.newText !== undefined);
      const patch = snapshot
        ? {
            ...block,
            oldText: block.oldText ?? "",
            newText: block.newText ?? "",
          }
        : block;
      const path = block.path!;
      const patches = changes.get(path) ?? [];
      const prior = patches.at(-1);
      if (
        prior &&
        prior.oldText !== undefined &&
        !prior.truncated &&
        !patch.truncated &&
        prior.newText === patch.oldText
      )
        patches[patches.length - 1] = { ...patch, oldText: prior.oldText };
      else patches.push(patch);
      changes.set(path, patches);
    }
  }
  return [...changes.values()].flat();
}

const DIFF_CELL_LIMIT = 4_000_000;

/** Added and removed line counts of a diff content block. */
export function diffStats(content: ChatContent) {
  const before = content.oldText ? content.oldText.split("\n") : [];
  const after = content.newText ? content.newText.split("\n") : [];
  let start = 0;
  while (
    start < before.length &&
    start < after.length &&
    before[start] === after[start]
  )
    start++;
  let endBefore = before.length;
  let endAfter = after.length;
  while (
    endBefore > start &&
    endAfter > start &&
    before[endBefore - 1] === after[endAfter - 1]
  ) {
    endBefore--;
    endAfter--;
  }
  const a = before.slice(start, endBefore);
  const b = after.slice(start, endAfter);
  if (a.length * b.length > DIFF_CELL_LIMIT)
    return { added: b.length, removed: a.length };
  const common = lcsLength(a, b);
  return { added: b.length - common, removed: a.length - common };
}

function lcsLength(a: readonly string[], b: readonly string[]) {
  let prev = new Uint32Array(b.length + 1);
  for (const line of a) {
    const row = new Uint32Array(b.length + 1);
    for (let j = 1; j <= b.length; j++)
      row[j] =
        line === b[j - 1] ? prev[j - 1]! + 1 : Math.max(prev[j]!, row[j - 1]!);
    prev = row;
  }
  return prev[b.length]!;
}
