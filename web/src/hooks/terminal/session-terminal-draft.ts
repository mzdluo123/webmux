import {
  createDraftReporter,
  createSessionDraftSender,
  savedStorageLabel,
} from "../../domains/terminal/session-terminal-io";
import type {
  DraftOperation,
  TerminalDraft,
  TerminalDrafts,
  TextInsertion,
} from "../../domains/terminal/terminal-draft.svelte";
import type { TerminalController } from "../../domains/terminal/terminal-search";

interface SessionTerminalDraftOptions {
  draft: TerminalDraft;
  drafts: TerminalDrafts;
  terminal: () => TerminalController;
  isMounted: () => boolean;
  isChatView: () => boolean;
  editComposerKey: (input: string) => void;
  restoreInputFocus: () => void;
  setError: (message: string) => void;
  setNotice: (message: string) => void;
}

export function createSessionTerminalDraft(
  options: SessionTerminalDraftOptions,
) {
  const { draft, drafts } = options;

  function beginInput(): DraftOperation {
    const operation = drafts.begin(draft);
    return options.isChatView()
      ? { ...operation, startedInDraft: true }
      : operation;
  }

  function insertText(
    text: string,
    operation = drafts.begin(draft),
  ): TextInsertion {
    if (!options.isMounted()) return "failed";
    return drafts.insert(draft, text, operation, (value) =>
      options.terminal().paste(value),
    );
  }

  function controlNotice() {
    options.setError(
      "终端控制需先退出草稿模式；草稿框内可正常复制、选择和编辑。",
    );
  }

  function softInput(input: string) {
    if (draft.enabled) options.editComposerKey(input);
    else options.terminal().send(input);
  }

  function toggleDraft() {
    drafts.setEnabled(draft, !draft.enabled);
    options.terminal().resetModifiers();
    if (draft.enabled) options.restoreInputFocus();
  }

  const reportDraft = createDraftReporter(
    draft,
    drafts,
    options.isMounted,
    options.setError,
    options.setNotice,
  );
  const sendDraft = createSessionDraftSender(
    draft,
    drafts,
    options.terminal,
    reportDraft,
    options.setError,
    options.setNotice,
  );

  function releaseDraftSubmission() {
    if (draft.submission) options.terminal().untrack(draft.submission.id);
    drafts.releaseSubmission(draft);
    options.setError("");
  }

  function resumeDraftSubmission() {
    if (draft.submission)
      options
        .terminal()
        .track(draft.submission.id, reportDraft(draft.submission.id, true));
  }

  return {
    get draftSaved() {
      return savedStorageLabel(draft.persisted, "草稿过大，仅保存在当前页面");
    },
    get scratchSaved() {
      return savedStorageLabel(
        draft.scratchPersisted,
        "暂存超出本机保存限制，仅当前页面",
      );
    },
    beginInput,
    insertText,
    controlNotice,
    softInput,
    toggleDraft,
    sendDraft,
    releaseDraftSubmission,
    resumeDraftSubmission,
  };
}
