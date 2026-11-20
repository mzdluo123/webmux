import { m } from "$lib/paraglide/messages.js";
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

/** Returns true as soon as any non-whitespace character is found. */
function hasVisibleText(text: string): boolean {
  for (let i = 0; i < text.length; i++) {
    const code = text.charCodeAt(i);
    if (
      code > 32 &&
      code !== 0xa0 &&
      code !== 0xfeff &&
      !(code >= 0x2000 && code <= 0x200b) &&
      code !== 0x2028 &&
      code !== 0x2029 &&
      code !== 0x202f &&
      code !== 0x205f &&
      code !== 0x3000
    )
      return true;
    if (code > 32 && !/\s/.test(text[i]!)) return true;
  }
  return false;
}

/**
 * Compacts leading whitespace-collapsed text up to `limit` UTF-16 code units
 * without regex-scanning the entire message body.
 */
function summarizePrefix(text: string, limit = 80): string {
  let out = "";
  let inSpace = true;
  let overflow = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]!;
    const code = text.charCodeAt(i);
    const space =
      code <= 32 ||
      (code >= 0xa0 &&
        (code === 0xa0 ||
          code === 0xfeff ||
          (code >= 0x2000 && code <= 0x200b) ||
          code === 0x2028 ||
          code === 0x2029 ||
          code === 0x202f ||
          code === 0x205f ||
          code === 0x3000 ||
          /\s/.test(ch)));
    if (space) {
      inSpace = true;
      continue;
    }
    if (inSpace && out.length > 0) {
      if (out.length >= limit) {
        overflow = true;
        break;
      }
      out += " ";
    }
    inSpace = false;
    if (out.length >= limit) {
      overflow = true;
      break;
    }
    out += ch;
  }
  if (!overflow) return out;
  let end = out.length;
  if (
    end > 0 &&
    out.charCodeAt(end - 1) >= 0xd800 &&
    out.charCodeAt(end - 1) <= 0xdbff
  )
    end--;
  return `${out.slice(0, end).trimEnd()}…`;
}

/** One tick per user prompt and one for its agent replies, across work steps. */
export function conversationTurns(
  entries: readonly TimelineEntry[],
): ConversationTurnItem[] {
  const items: ConversationTurnItem[] = [];
  let turn = 0;
  for (const entry of entries) {
    if (entry.type !== "message" || entry.role === "thought") continue;
    if (entry.role === "user") turn++;
    const previous = items.at(-1);
    if (entry.role === "agent" && previous?.role === "agent") {
      if (!entry.truncated && !hasVisibleText(entry.text)) continue;
      previous.messageIds.push(entry.id);
      continue;
    }
    const summary = summarizePrefix(entry.text, 80);
    if (entry.role === "agent" && !summary && !entry.truncated) continue;
    items.push({
      id: entry.id,
      messageIds: [entry.id],
      role: entry.role,
      turn,
      summary:
        summary ||
        (entry.role === "user" && entry.attachments?.length
          ? m.s3_turn_user_attachment()
          : entry.role === "user"
            ? m.s3_turn_empty_message()
            : m.s3_turn_content_truncated()),
    });
  }
  return items;
}

/** Bound the rail's density without discarding any message anchors. */
export function conversationTurnGroups(
  items: readonly ConversationTurnItem[],
  capacity: number,
): ConversationTurnItem[][] {
  const size = Math.max(1, Math.ceil(items.length / Math.max(1, capacity)));
  const groups: ConversationTurnItem[][] = [];
  for (let index = 0; index < items.length; index += size)
    groups.push(items.slice(index, index + size));
  return groups;
}
