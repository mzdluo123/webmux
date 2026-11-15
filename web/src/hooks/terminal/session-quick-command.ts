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
import { m } from "$lib/paraglide/messages.js";

function quickMessage(state: SubmissionProgress): string | undefined {
  if (state === "failed") return m.s4_quick_msg_failed();
  if (state === "missing") return m.s4_quick_msg_missing();
  if (state === "partial") return m.s4_quick_msg_partial();
  if (state === "unknown") return m.s4_quick_msg_unknown();
  return undefined;
}

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
        ctx.setError(
          m.s4_artifacts_dir_unavailable({ error: errorMessage(err) }),
        );
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
        ctx.setError(
          m.s4_webmux_brief_unavailable({ error: errorMessage(err) }),
        );
      return;
    }
    if (!ctx.isMounted()) return;
    text = renderWebmuxBrief(text, brief);
  }
  if (!chat && cmd.enter && ctx.drafts.destination(ctx.draft) === "terminal") {
    // Text and Enter form one acknowledged submission.
    let unknownMessage = "";
    const sent = ctx.submitTerminal(newSubmissionId(), text, true, (state) => {
      if (!ctx.isMounted()) return;
      const msg = quickMessage(state);
      if (msg) {
        if (state === "unknown") unknownMessage = msg;
        ctx.setError(msg);
      } else if (
        state === "written" &&
        (ctx.getError() === m.s4_quick_msg_unknown() ||
          (unknownMessage && ctx.getError() === unknownMessage))
      ) {
        ctx.setError("");
      }
    });
    ctx.setError(sent ? "" : m.s4_quick_terminal_not_ready());
    return;
  }
  // The chat side only inserts: sending stays with the message box.
  const result = ctx.insertText(text, operation ?? ctx.beginInput());
  if (result === "failed")
    ctx.setError(
      chat || ctx.drafts.destination(ctx.draft, operation) === "draft"
        ? m.s4_quick_draft_submitting()
        : m.s4_quick_terminal_not_ready(),
    );
  else ctx.setError("");
}
