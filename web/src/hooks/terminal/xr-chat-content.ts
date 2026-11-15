import { marked, type Tokens } from "marked";
import * as m from "$lib/paraglide/messages.js";
import type { ChatModel } from "../../domains/chat/chat-model";
import type { ChatState } from "../../contracts/schemas";
import type {
  XrAction,
  XrTextLine,
  XrTextPanel,
} from "../../domains/terminal/xr-workspace";

function prose(text: string): string[] {
  return marked.lexer(text, { gfm: true }).flatMap((token) => {
    if (token.type === "space") return [""];
    if (token.type === "list")
      return token.items.flatMap((item: Tokens.ListItem) => [`• ${item.text}`]);
    return ("text" in token ? String(token.text) : token.raw).split("\n");
  });
}

export function xrChatPanel(options: {
  model: ChatModel;
  state: ChatState | undefined;
  draft: string;
  answering: boolean;
  answer(requestId: string, optionId: string): Promise<void>;
  openForm(this: void): void | Promise<void>;
}): XrTextPanel {
  const lines: XrTextLine[] = [];
  const actions: XrAction[] = [];
  if (options.model.truncated)
    lines.push({ text: m.s4_xr_chat_history_truncated(), tone: "muted" });
  for (const entry of options.model.entries) {
    if (entry.type === "message") {
      lines.push({
        text:
          entry.role === "user"
            ? m.s4_xr_role_you()
            : entry.role === "thought"
              ? m.s4_xr_role_thought()
              : "Agent",
        tone: "muted",
      });
      for (const text of prose(entry.text)) lines.push({ text });
      for (const attachment of entry.attachments ?? [])
        lines.push({
          text: m.s4_xr_chat_attachment({
            name: attachment.name || attachment.path,
          }),
          tone: "muted",
        });
      if (entry.truncated)
        lines.push({ text: m.s4_xr_chat_msg_truncated(), tone: "muted" });
    } else if (entry.type === "tool") {
      lines.push({
        text: `${entry.title || entry.kind} · ${entry.status}`,
        tone: "muted",
      });
      for (const content of entry.content) {
        if (content.text)
          for (const text of content.text.split("\n")) lines.push({ text });
        if (content.type === "diff") {
          if (content.path) lines.push({ text: content.path, tone: "muted" });
          if (content.oldText !== undefined)
            for (const text of content.oldText.split("\n"))
              lines.push({ text: `- ${text}`, tone: "removed" });
          if (content.newText !== undefined)
            for (const text of content.newText.split("\n"))
              lines.push({ text: `+ ${text}`, tone: "added" });
        }
        if (content.type === "resource_link" || content.type === "unsupported")
          lines.push({
            text:
              content.name ||
              content.uri ||
              content.unsupported ||
              m.s4_xr_view_in_2d(),
            tone: "muted",
          });
      }
    } else if (entry.type === "notice") {
      lines.push({
        text: entry.text,
        tone: entry.level === "error" ? "error" : "muted",
      });
    } else if (entry.type === "compaction") {
      lines.push({
        text: m.s4_xr_chat_compaction({
          status: entry.status,
          summary: entry.summary,
        }),
        tone: "muted",
      });
    } else if (entry.type === "turnEnd" && entry.error) {
      lines.push({ text: entry.error, tone: "error" });
    }
  }
  for (const request of options.state?.requests ?? []) {
    lines.push({
      text: m.s4_xr_pending_request({
        title:
          request.title ||
          request.form?.message ||
          m.s4_xr_agent_request_fallback(),
      }),
      tone: "muted",
    });
    if (request.kind === "permission") {
      for (const option of request.options ?? []) {
        actions.push({
          id: `${request.requestId}:${option.optionId}`,
          label: option.name,
          disabled: options.answering,
          confirm: option.kind.startsWith("allow")
            ? m.s4_xr_confirm_permission({
                title: request.title || m.s4_xr_agent_action_fallback(),
                option: option.name,
              })
            : undefined,
          run: () => options.answer(request.requestId, option.optionId),
        });
      }
    } else {
      actions.push({
        id: `form:${request.requestId}`,
        label: m.s4_xr_return_2d_form(),
        run: options.openForm,
      });
    }
  }
  lines.push({ text: m.s4_xr_draft_heading(), tone: "muted" });
  for (const text of options.draft.split("\n")) lines.push({ text });
  return {
    id: "chat",
    kind: "chat",
    title: options.state?.turn.running
      ? m.s4_xr_agent_chat_running()
      : m.s4_xr_agent_chat(),
    lines,
    actions,
  };
}
