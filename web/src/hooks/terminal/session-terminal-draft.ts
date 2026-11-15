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
import * as m from "$lib/paraglide/messages.js";

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
    options.setError(m.s4_draft_control_notice());
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
      return savedStorageLabel(draft.persisted, m.s4_draft_oversized_local());
    },
    get scratchSaved() {
      return savedStorageLabel(
        draft.scratchPersisted,
        m.s4_scratch_oversized_local(),
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
