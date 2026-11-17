import { m } from "$lib/paraglide/messages.js";
import type { SessionInfo } from "../../contracts/schemas";
import { hasUrgentAttention } from "../sessions/session-status";

type Attention = NonNullable<SessionInfo["attention"]>;
type AttentionKind = Attention["kind"];

export interface AttentionToast {
  readonly sessionId: string;
  readonly title: string;
  readonly kind: AttentionKind;
  readonly source: Attention["source"];
  readonly since: string;
  readonly label: string;
  readonly detail: string;
  readonly urgent: boolean;
}

function attentionLabel(kind: AttentionKind): string {
  switch (kind) {
    case "question":
      return m.s2_attn_label_question();
    case "permission":
      return m.s2_attn_label_permission();
    case "error":
      return m.s2_attn_label_error();
    case "done":
      return m.s2_attn_label_done();
    case "notify":
      return m.s2_attn_label_notify();
    case "bell":
      return m.s2_attn_label_bell();
  }
}

function attentionDetail(kind: AttentionKind): string {
  switch (kind) {
    case "question":
      return m.s2_attn_detail_question();
    case "permission":
      return m.s2_attn_detail_permission();
    case "error":
      return m.s2_attn_detail_error();
    case "done":
      return m.s2_attn_detail_done();
    case "notify":
      return m.s2_attn_detail_notify();
    case "bell":
      return m.s2_attn_detail_bell();
  }
}

export const maxVisibleToasts = 3;
export const defaultToastDurationMs = 8_000;

function stripControls(raw: string): string {
  return Array.from(raw)
    .filter((ch) => {
      const code = ch.codePointAt(0) ?? 0;
      return code >= 0x20 && code !== 0x7f && (code < 0x80 || code > 0x9f);
    })
    .join("")
    .trim();
}

export function cleanAttentionTitle(raw: string): string {
  const cleaned = stripControls(raw);
  if (!cleaned) return "WebMux";
  const chars = Array.from(cleaned);
  return chars.length > 80 ? `${chars.slice(0, 80).join("")}…` : cleaned;
}

export function attentionKey(attention: Attention): string {
  return `${attention.kind}\u0000${attention.since}`;
}

export function buildAttentionToast(
  session: Pick<SessionInfo, "id" | "title" | "attention">,
  attention: Attention,
): AttentionToast {
  return {
    sessionId: session.id,
    title: cleanAttentionTitle(session.title),
    kind: attention.kind,
    source: attention.source,
    since: attention.since,
    label: attentionLabel(attention.kind),
    detail:
      stripControls(attention.message ?? "") || attentionDetail(attention.kind),
    urgent: hasUrgentAttention({ attention }),
  };
}
