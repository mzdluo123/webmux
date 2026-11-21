import type { ChatModel, TimelineEntry, ToolEntry } from "./chat-types";

function entryIndexKey(entry: TimelineEntry): string | undefined {
  switch (entry.type) {
    case "message":
      return `message:${entry.id}`;
    case "tool":
      return `tool:${entry.id}`;
    case "compaction":
      return `compaction:${entry.id}`;
    default:
      return undefined;
  }
}

function mergeTool(earlier: ToolEntry, later: ToolEntry): ToolEntry {
  return {
    ...earlier,
    kind: later.kind !== "other" ? later.kind : earlier.kind,
    title: later.title || earlier.title,
    status: later.status,
    content: later.content.length ? later.content : earlier.content,
    locations: later.locations.length ? later.locations : earlier.locations,
    diffTurnId: later.diffTurnId ?? earlier.diffTurnId,
    partial: earlier.partial && later.partial,
    permission: later.permission ?? earlier.permission,
  };
}

/**
 * Prepends an earlier reduced backfill slice before `later`, reconciling any
 * message, tool, or compaction entries whose events spanned the chunk boundary.
 */
export function mergePrependedModel(
  earlier: ChatModel,
  later: ChatModel,
  more: boolean,
): ChatModel {
  if (!earlier.entries.length && !earlier.title && !earlier.requestTools.size) {
    return later.backfillPending === more
      ? later
      : { ...later, backfillPending: more };
  }
  const entries: TimelineEntry[] = [];
  const index = new Map<string, number>();

  for (const entry of earlier.entries) {
    const key = entryIndexKey(entry);
    if (key) index.set(key, entries.length);
    entries.push(entry);
  }

  for (const entry of later.entries) {
    const key = entryIndexKey(entry);
    const existing = key ? index.get(key) : undefined;
    if (existing !== undefined) {
      const prev = entries[existing]!;
      if (prev.type === "message" && entry.type === "message") {
        entries[existing] = {
          ...prev,
          text: prev.text + entry.text,
          truncated: prev.truncated || entry.truncated,
          attachments: entry.attachments ?? prev.attachments,
        };
        continue;
      }
      if (prev.type === "tool" && entry.type === "tool") {
        entries[existing] = mergeTool(prev, entry);
        continue;
      }
      if (prev.type === "compaction" && entry.type === "compaction") {
        entries[existing] = {
          ...prev,
          status: entry.status,
          summary: entry.summary || prev.summary,
        };
        continue;
      }
    }
    if (key) index.set(key, entries.length);
    entries.push(entry);
  }

  const requestTools =
    earlier.requestTools.size > 0
      ? new Map([...earlier.requestTools, ...later.requestTools])
      : later.requestTools;

  return {
    ...later,
    title: later.title ?? earlier.title,
    entries,
    index,
    turnDiff: later.turnDiff ?? earlier.turnDiff,
    requestTools,
    backfillPending: more,
  };
}
