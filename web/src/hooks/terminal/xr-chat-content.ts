import { marked, type Tokens } from "marked";
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
    lines.push({ text: "更早的聊天内容已超出服务端保留范围。", tone: "muted" });
  for (const entry of options.model.entries) {
    if (entry.type === "message") {
      lines.push({
        text:
          entry.role === "user"
            ? "你"
            : entry.role === "thought"
              ? "思考"
              : "Agent",
        tone: "muted",
      });
      for (const text of prose(entry.text)) lines.push({ text });
      for (const attachment of entry.attachments ?? [])
        lines.push({
          text: `附件：${attachment.name || attachment.path}`,
          tone: "muted",
        });
      if (entry.truncated)
        lines.push({ text: "此条消息已截断。", tone: "muted" });
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
              "请在 2D 查看此内容。",
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
        text: `上下文整理 · ${entry.status} ${entry.summary}`,
        tone: "muted",
      });
    } else if (entry.type === "turnEnd" && entry.error) {
      lines.push({ text: entry.error, tone: "error" });
    }
  }
  for (const request of options.state?.requests ?? []) {
    lines.push({
      text: `等待处理：${request.title || request.form?.message || "Agent 请求"}`,
      tone: "muted",
    });
    if (request.kind === "permission") {
      for (const option of request.options ?? []) {
        actions.push({
          id: `${request.requestId}:${option.optionId}`,
          label: option.name,
          disabled: options.answering,
          confirm: option.kind.startsWith("allow")
            ? `确认授权：${request.title || "Agent 操作"} — ${option.name}`
            : undefined,
          run: () => options.answer(request.requestId, option.optionId),
        });
      }
    } else {
      actions.push({
        id: `form:${request.requestId}`,
        label: "返回 2D 填写表单",
        run: options.openForm,
      });
    }
  }
  lines.push({ text: "草稿", tone: "muted" });
  for (const text of options.draft.split("\n")) lines.push({ text });
  return {
    id: "chat",
    kind: "chat",
    title: options.state?.turn.running ? "Agent 聊天 · 运行中" : "Agent 聊天",
    lines,
    actions,
  };
}
