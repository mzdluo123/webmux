import { agentGuidePrompt, ensureArtifactsFolder } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import {
  renderArtifactDir,
  renderWebmuxBrief,
  usesArtifactDir,
  usesWebmuxBrief,
} from "../../domains/settings/quick-commands";
import type { QuickCommand, SubmissionProgress } from "../../contracts/schemas";
import type {
  DraftOperation,
  TerminalDraft,
  TerminalDrafts,
  TextInsertion,
} from "../../domains/terminal/terminal-draft.svelte";
import { newSubmissionId } from "../../domains/terminal/terminal-protocol";
const quickMessages: Partial<Record<SubmissionProgress, string>> = {
  failed: "快捷指令未写入终端；请检查终端后重试。",
  missing: "快捷指令未送达服务端；请检查终端后重试。",
  partial: "快捷指令正文已写入，但回车未写入；请检查终端后手动提交。",
  unknown: "连接中断，快捷指令结果未知；重连后自动查询。",
};

export interface SessionQuickCommandContext {
  id: string;
  isMounted: () => boolean;
  isChatView: () => boolean;
  draft: TerminalDraft;
  drafts: TerminalDrafts;
  beginInput: () => DraftOperation;
  insertText: (text: string, operation?: DraftOperation) => TextInsertion;
  resetModifiers: () => void;
  submitTerminal: (
    id: string,
    text: string,
    enter: boolean,
    onProgress: (state: SubmissionProgress) => void,
  ) => boolean;
  getError: () => string;
  setError: (msg: string) => void;
}

// The operation binds this session's draft and target at selection. A body
// with `{{dir}}` or `{{webmux}}` first waits for the artifacts folder or
// the WebMux brief, then runs as written; the brief goes in last, so
// nothing in it is replaced.
export async function executeSessionQuickCommand(
  cmd: QuickCommand,
  ctx: SessionQuickCommandContext,
): Promise<void> {
  ctx.resetModifiers();
  const chat = ctx.isChatView();
  let text = cmd.command;
  let operation: DraftOperation | undefined;
  if (usesArtifactDir(text)) {
    operation = ctx.beginInput();
    let dir: string;
    try {
      dir = await ensureArtifactsFolder(ctx.id);
    } catch (err) {
      if (ctx.isMounted())
        ctx.setError(`临时文件夹不可用：${errorMessage(err)}`);
      return;
    }
    if (!ctx.isMounted()) return;
    text = renderArtifactDir(text, dir);
  }
  if (usesWebmuxBrief(text)) {
    operation ??= ctx.beginInput();
    let brief: string;
    try {
      brief = await agentGuidePrompt(ctx.id);
    } catch (err) {
      if (ctx.isMounted())
        ctx.setError(`WebMux 工具说明不可用：${errorMessage(err)}`);
      return;
    }
    if (!ctx.isMounted()) return;
    text = renderWebmuxBrief(text, brief);
  }
  if (!chat && cmd.enter && ctx.drafts.destination(ctx.draft) === "terminal") {
    // Text and Enter form one acknowledged submission.
    const sent = ctx.submitTerminal(newSubmissionId(), text, true, (state) => {
      if (!ctx.isMounted()) return;
      if (quickMessages[state]) ctx.setError(quickMessages[state]);
      else if (state === "written" && ctx.getError() === quickMessages.unknown)
        ctx.setError("");
    });
    ctx.setError(sent ? "" : "终端未就绪，快捷指令未发送；请恢复连接后重试。");
    return;
  }
  // The chat side only inserts: sending stays with the message box.
  const result = ctx.insertText(text, operation ?? ctx.beginInput());
  if (result === "failed")
    ctx.setError(
      chat || ctx.drafts.destination(ctx.draft, operation) === "draft"
        ? "草稿正在提交到终端，快捷指令未插入；请确认结果后重试。"
        : "终端未就绪，快捷指令未发送；请恢复连接后重试。",
    );
  else ctx.setError("");
}
