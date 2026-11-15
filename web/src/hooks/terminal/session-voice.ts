import type {
  DraftOperation,
  TerminalDraft,
  TerminalDrafts,
  TextInsertion,
} from "../../domains/terminal/terminal-draft.svelte";
import type { TerminalController } from "../../domains/terminal/terminal-search";
import type { SessionHybridView } from "../../domains/terminal/session-terminal-io";
export function applySessionVoiceResult(
  text: string,
  ctx: {
    isMounted: () => boolean;
    isChatView: () => boolean;
    draft: TerminalDraft;
    drafts: TerminalDrafts;
    beginInput: () => DraftOperation;
    insertText: (text: string, operation?: DraftOperation) => TextInsertion;
    setError: (msg: string) => void;
    onInserted: () => void;
  },
) {
  if (!text || !ctx.isMounted()) return;
  const chat = ctx.isChatView();
  const operation = ctx.beginInput();
  const result = ctx.insertText(text, operation);
  if (result === "failed") {
    ctx.setError(
      chat || ctx.drafts.destination(ctx.draft, operation) === "draft"
        ? "草稿正在提交到终端，语音识别内容未插入；请确认结果后重试。"
        : "终端未就绪，语音识别内容未发送；请恢复连接后重试。",
    );
  } else {
    ctx.setError("");
    ctx.onInserted();
  }
}
export function isVoiceDisabled(
  hybrid: SessionHybridView,
  t: TerminalController,
  draft: TerminalDraft,
): boolean {
  return hybrid.view === "chat"
    ? hybrid.missing || !!draft.submission
    : t.connection === "missing" ||
        (draft.enabled
          ? !!draft.submission
          : t.exitCode !== null || t.connection === "exited" || !t.ready);
}
