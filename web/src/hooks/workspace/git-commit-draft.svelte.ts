import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import type { GitCommitDraft } from "../../contracts/schemas";
import {
  type GitCommitDraftConflictState,
  joinCommitMessageDraft,
  mergeCommitMessageDrafts,
  splitCommitMessageDraft,
} from "../../domains/workspace/git-commit-draft-model";
import { writeGitTargetMemory } from "../../domains/workspace/git-workbench-memory";
import { errorMessage } from "../../platform/error";
import {
  gitCommitDraft,
  gitPutCommitDraft,
} from "../../platform/git-collab-api";
import { LatestRequest } from "../shared/remote-request.svelte";

export interface GitCommitDraftBindings {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
}

const SAVE_DEBOUNCE_MS = 450;

export function createGitCommitDraftController(
  bindings: GitCommitDraftBindings,
) {
  let baseRevision = $state(0);
  let lastSyncedMessage = $state("");
  let dirty = $state(false);
  let saving = $state(false);
  let lastSavedAt = $state<string | null>(null);
  let conflict = $state<GitCommitDraftConflictState | null>(null);

  let saveTimer: ReturnType<typeof setTimeout> | null = null;
  const loadReq = new LatestRequest();
  const saveReq = new LatestRequest();

  function clearSaveTimer() {
    if (saveTimer !== null) {
      clearTimeout(saveTimer);
      saveTimer = null;
    }
  }

  function dispose() {
    clearSaveTimer();
    loadReq.dispose();
    saveReq.dispose();
  }

  function resetTarget() {
    clearSaveTimer();
    loadReq.cancel();
    saveReq.cancel();
    baseRevision = 0;
    lastSyncedMessage = "";
    dirty = false;
    saving = false;
    lastSavedAt = null;
    conflict = null;
  }

  function applyCleanDraft(
    repoId: string,
    wtId: string,
    draft: GitCommitDraft,
  ) {
    const message = joinCommitMessageDraft(draft.subject, draft.body);
    baseRevision = draft.revision;
    lastSyncedMessage = message;
    dirty = false;
    lastSavedAt = draft.updatedAt;
    conflict = null;
    bindings.patchView({ commitMessage: message });
    writeGitTargetMemory(repoId, wtId, { commitMessage: message });
  }

  async function load(
    repoId = bindings.getRepoId(),
    wtId = bindings.getWorktreeId(),
    parentSignal?: AbortSignal,
  ) {
    if (!repoId || !wtId) return;
    const req = loadReq.begin();
    const signal = parentSignal
      ? AbortSignal.any([req.signal, parentSignal])
      : req.signal;
    try {
      const remote = await gitCommitDraft(repoId, wtId, signal);
      if (
        !req.current() ||
        parentSignal?.aborted ||
        repoId !== bindings.getRepoId() ||
        wtId !== bindings.getWorktreeId()
      ) {
        return;
      }

      const remoteMsg = joinCommitMessageDraft(remote.subject, remote.body);
      const localMsg = bindings.getView().commitMessage;

      if (conflict) {
        if (remote.revision > conflict.remoteDraft.revision) {
          conflict = {
            ...conflict,
            remoteDraft: remote,
          };
        }
        return;
      }

      if (!dirty) {
        if (remote.revision === 0 && !remoteMsg && localMsg.trim()) {
          // Migrate legacy local-only memory to server revision 1 on first open.
          baseRevision = 0;
          lastSyncedMessage = "";
          dirty = true;
          scheduleSave();
          return;
        }
        applyCleanDraft(repoId, wtId, remote);
        return;
      }

      // Local has unsaved edits while window regained focus or reloaded:
      if (remote.revision !== baseRevision && remoteMsg !== localMsg) {
        if (!remoteMsg && localMsg.trim()) {
          baseRevision = remote.revision;
          lastSyncedMessage = "";
          scheduleSave();
          return;
        }
        clearSaveTimer();
        const localParts = splitCommitMessageDraft(localMsg);
        conflict = {
          localSubject: localParts.subject,
          localBody: localParts.body,
          remoteDraft: remote,
          message:
            "检测到另一设备或标签页已保存新的提交说明草稿，请选择保留本机内容、采用远端版本或合并两份内容。",
        };
      } else if (remoteMsg === localMsg) {
        baseRevision = remote.revision;
        lastSyncedMessage = remoteMsg;
        dirty = false;
        lastSavedAt = remote.updatedAt;
      }
    } catch {
      /* Readonly or transient worktree state keeps local input untouched */
    }
  }

  async function flushSave(overrideRevision?: number): Promise<boolean> {
    clearSaveTimer();
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId) return true;
    if (conflict && overrideRevision === undefined) return false;

    const currentMessage = bindings.getView().commitMessage;
    if (!dirty && overrideRevision === undefined) return true;

    const parts = splitCommitMessageDraft(currentMessage);
    const revToSend = overrideRevision ?? baseRevision;
    const req = saveReq.begin();
    saving = true;
    try {
      const out = await gitPutCommitDraft(
        repoId,
        wtId,
        {
          subject: parts.subject,
          body: parts.body,
          baseRevision: revToSend,
        },
        req.signal,
      );
      if (
        !req.current() ||
        repoId !== bindings.getRepoId() ||
        wtId !== bindings.getWorktreeId()
      ) {
        return false;
      }
      if (out.ok) {
        baseRevision = out.draft.revision;
        lastSyncedMessage = joinCommitMessageDraft(
          out.draft.subject,
          out.draft.body,
        );
        const latestMsg = bindings.getView().commitMessage;
        dirty = latestMsg !== lastSyncedMessage;
        lastSavedAt = out.draft.updatedAt;
        conflict = null;
        if (dirty) scheduleSave();
        return true;
      }

      if (
        overrideRevision === undefined &&
        !out.conflict.subject.trim() &&
        !out.conflict.body.trim() &&
        (parts.subject.trim() || parts.body.trim())
      ) {
        baseRevision = out.conflict.revision;
        lastSyncedMessage = "";
        return flushSave(out.conflict.revision);
      }

      const latestParts = splitCommitMessageDraft(
        bindings.getView().commitMessage,
      );
      conflict = {
        localSubject: latestParts.subject,
        localBody: latestParts.body,
        remoteDraft: out.conflict,
        message: out.message,
      };
      bindings.patchView({ statusBanner: out.message });
      return false;
    } catch (err) {
      if (req.reportable()) {
        bindings.patchView({
          statusBanner: `保存提交说明草稿失败：${errorMessage(err)}`,
        });
      }
      return false;
    } finally {
      if (req.current()) saving = false;
    }
  }

  function scheduleSave() {
    clearSaveTimer();
    if (conflict) return;
    saveTimer = setTimeout(() => {
      saveTimer = null;
      void flushSave();
    }, SAVE_DEBOUNCE_MS);
  }

  function updateLocalMessage(nextMessage: string) {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    bindings.patchView({ commitMessage: nextMessage });
    writeGitTargetMemory(repoId, wtId, { commitMessage: nextMessage });

    if (conflict) {
      const parts = splitCommitMessageDraft(nextMessage);
      conflict = {
        ...conflict,
        localSubject: parts.subject,
        localBody: parts.body,
      };
      return;
    }

    dirty = nextMessage !== lastSyncedMessage;
    if (dirty && repoId && wtId) {
      scheduleSave();
    } else {
      clearSaveTimer();
    }
  }

  async function resolveKeepLocal() {
    if (!conflict) return;
    const targetRev = conflict.remoteDraft.revision;
    const localMsg = joinCommitMessageDraft(
      conflict.localSubject,
      conflict.localBody,
    );
    bindings.patchView({ commitMessage: localMsg });
    writeGitTargetMemory(bindings.getRepoId(), bindings.getWorktreeId(), {
      commitMessage: localMsg,
    });
    dirty = true;
    await flushSave(targetRev);
  }

  function resolveAdoptRemote() {
    if (!conflict) return;
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId) return;
    applyCleanDraft(repoId, wtId, conflict.remoteDraft);
    bindings.patchView({
      statusBanner: "已采用另一设备保存的提交说明草稿。",
    });
  }

  async function resolveMergeBoth() {
    if (!conflict) return;
    const targetRev = conflict.remoteDraft.revision;
    const merged = mergeCommitMessageDrafts(
      { subject: conflict.localSubject, body: conflict.localBody },
      {
        subject: conflict.remoteDraft.subject,
        body: conflict.remoteDraft.body,
      },
    );
    const mergedMsg = joinCommitMessageDraft(merged.subject, merged.body);
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    bindings.patchView({ commitMessage: mergedMsg });
    writeGitTargetMemory(repoId, wtId, { commitMessage: mergedMsg });
    dirty = true;
    const ok = await flushSave(targetRev);
    if (ok) {
      bindings.patchView({
        statusBanner: "已合并两台设备的提交说明草稿并同步保存。",
      });
    }
  }

  async function markClearedAfterCommit() {
    clearSaveTimer();
    saveReq.cancel();
    lastSyncedMessage = "";
    dirty = false;
    saving = false;
    lastSavedAt = null;
    conflict = null;
    await load();
  }

  return {
    get revision() {
      return baseRevision;
    },
    get dirty() {
      return dirty;
    },
    get saving() {
      return saving;
    },
    get lastSavedAt() {
      return lastSavedAt;
    },
    get conflict() {
      return conflict;
    },
    dispose,
    resetTarget,
    load,
    flushSave,
    updateLocalMessage,
    resolveKeepLocal,
    resolveAdoptRemote,
    resolveMergeBoth,
    markClearedAfterCommit,
    setSimulatedConflict(next: GitCommitDraftConflictState | null) {
      conflict = next;
    },
  };
}
