import type { SessionInfo } from "../../contracts/schemas";
import { returnQueuedDraft } from "../sessions/hybrid-types";
import { quickStartMemory } from "../sessions/quick-start";
import {
  openPage,
  openSession,
  shellUi,
  type Shell,
} from "../shared/shell.svelte";

export function deliverTextToWorktreeSession(options: {
  shell: Shell;
  session: SessionInfo;
  text: string;
}) {
  const payload = options.text.trim();
  if (!payload) return;
  const draft = options.shell.drafts.get(options.session.id);
  returnQueuedDraft(options.shell.drafts, draft, payload);
  openSession(options.session);
}

export function startNewWorktreeSessionWithText(options: {
  shell: Shell;
  repositoryId: string;
  worktreeId?: string;
  text: string;
}) {
  const payload = options.text.trim();
  if (shellUi.mode === "chat" && options.repositoryId && options.worktreeId) {
    const existing = quickStartMemory.loadDraft();
    const mergedText = existing.text.trim()
      ? `${existing.text}\n\n${payload}`
      : payload;
    quickStartMemory.saveDraftState({
      ...existing,
      text: mergedText,
      repositoryId: options.repositoryId,
      targetMode: "existing",
      worktreeChoice: {
        repositoryId: options.repositoryId,
        worktreeId: options.worktreeId,
      },
    });
    openPage("/new");
    return;
  }

  if (options.repositoryId && options.worktreeId) {
    options.shell.start({
      kind: "worktree",
      repositoryId: options.repositoryId,
      worktreeId: options.worktreeId,
      prompt: payload,
    });
  } else {
    options.shell.start();
  }
}
