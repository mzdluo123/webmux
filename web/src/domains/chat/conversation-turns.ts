import type { TimelineEntry } from "./chat-model";

export type ConversationTurnItem = {
  id: string;
  /** Every visible message represented by this tick, in transcript order. */
  messageIds: string[];
  role: "user" | "agent";
  /** Zero identifies an agent reply whose user prompt fell out of the cache. */
  turn: number;
  summary: string;
};

/** One tick per user prompt and one for its agent replies, across work steps. */
export function conversationTurns(
  entries: readonly TimelineEntry[],
): ConversationTurnItem[] {
  const items: ConversationTurnItem[] = [];
  let turn = 0;
  for (const entry of entries) {
    if (entry.type !== "message" || entry.role === "thought") continue;
    if (entry.role === "user") turn++;
    const compact = entry.text.replace(/\s+/g, " ").trim();
    if (entry.role === "agent" && !compact && !entry.truncated) continue;
    const previous = items.at(-1);
    if (entry.role === "agent" && previous?.role === "agent") {
      previous.messageIds.push(entry.id);
      continue;
    }
    let end = 80;
    if (
      compact.charCodeAt(end - 1) >= 0xd800 &&
      compact.charCodeAt(end - 1) <= 0xdbff
    )
      end--;
    const summary =
      compact.length > 80 ? `${compact.slice(0, end).trimEnd()}…` : compact;
    items.push({
      id: entry.id,
      messageIds: [entry.id],
      role: entry.role,
      turn,
      summary:
        summary ||
        (entry.role === "user" && entry.attachments?.length
          ? "用户附件"
          : entry.role === "user"
            ? "空消息"
            : "内容已截断"),
    });
  }
  return items;
}
