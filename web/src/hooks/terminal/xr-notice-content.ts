import * as m from "$lib/paraglide/messages.js";
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
  openChoice(this: void): void | Promise<void>;
}): XrTextPanel | undefined {
  const lines: XrTextLine[] = [];
  if (options.awaitingChoice) {
    lines.push({
      text: m.s4_xr_notice_tui_not_stopped(),
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
    title: m.s4_xr_notice_panel_title(),
    lines,
    actions: options.awaitingChoice
      ? [
          {
            id: "switch-choice-2d",
            label: m.s4_xr_notice_handle_switch_2d(),
            run: options.openChoice,
          },
        ]
      : [],
  };
}
