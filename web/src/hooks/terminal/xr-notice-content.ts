import type { HybridNotice } from "../../domains/sessions/hybrid";
import type {
  XrTextLine,
  XrTextPanel,
} from "../../domains/terminal/xr-workspace";

export function xrNoticePanel(options: {
  error: string;
  notices: readonly HybridNotice[];
  awaitingChoice: boolean;
  forceSwitchNote: string;
  openChoice(this: void): void;
}): XrTextPanel | undefined {
  const lines: XrTextLine[] = [];
  if (options.awaitingChoice) {
    lines.push({
      text: "TUI 中断后仍未停止。请返回 2D 选择继续等待、强制切换或取消；不会自动强制切换。",
      tone: "muted",
    });
    if (options.forceSwitchNote)
      lines.push({ text: options.forceSwitchNote, tone: "muted" });
  }
  // Match TerminalNotices: hybrid strips retain their order, followed by errors.
  for (const notice of options.notices)
    for (const text of notice.text.split("\n"))
      lines.push({
        text,
        tone: notice.tone === "danger" ? "error" : "muted",
      });
  for (const text of options.error ? options.error.split("\n") : [])
    lines.push({ text, tone: "error" });
  if (!lines.length) return undefined;
  return {
    id: "notice",
    kind: "chat",
    title: "会话提示",
    lines,
    actions: options.awaitingChoice
      ? [
          {
            id: "switch-choice-2d",
            label: "返回 2D 处理切换",
            run: options.openChoice,
          },
        ]
      : [],
  };
}
