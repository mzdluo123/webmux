import {
  gitStashApply,
  gitStashApplyPreview,
  gitStashCreate,
  gitStashCreatePreview,
  gitStashDrop,
  gitStashDropPreview,
  gitStashPop,
} from "../../platform/git-stash-api";
import type {
  GitInProgressOperation,
  GitStashApplyPreview,
  GitStashCreatePreview,
  GitStashDropPreview,
  GitStashEntry,
} from "../../contracts/schemas";
import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import { generateGitClientRequestId } from "../../domains/workspace/git-operations-model";
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import { parseGitManageError } from "./git-remotes-controller.svelte";

export { parseGitManageError as parseStashApiError };

export interface GitStashDialogsDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  operations?: GitOperationsController;
  onRefreshList: (preferredOid?: string | null) => Promise<void>;
  onAfterStashMutation: () => Promise<void>;
  onInProgressUpdated: (next: GitInProgressOperation) => void;
  onOpenConflictResolver: (path?: string) => void;
}

export function createGitStashDialogsController(deps: GitStashDialogsDeps) {
  let createOpen = $state(false);
  let createMessage = $state("");
  let createIncludeUntracked = $state(false);
  let createKeepIndex = $state(false);
  let createPreview = $state<GitStashCreatePreview | null>(null);

  let applyOpen = $state(false);
  let applyMode = $state<"apply" | "pop">("pop");
  let applyReinstateIndex = $state(false);
  let applyTarget = $state<GitStashEntry | null>(null);
  let applyPreview = $state<GitStashApplyPreview | null>(null);

  let dropOpen = $state(false);
  let dropTarget = $state<GitStashEntry | null>(null);
  let dropPreview = $state<GitStashDropPreview | null>(null);

  let loadingPreview = $state(false);
  let submitting = $state(false);
  let acknowledgeSessions = $state(false);
  let dialogError = $state("");

  const previewReq = new LatestRequest();
  let disposed = false;

  function dispose() {
    disposed = true;
    previewReq.dispose();
  }

  function resetTarget() {
    previewReq.cancel();
    createOpen = false;
    createPreview = null;
    applyOpen = false;
    applyPreview = null;
    dropOpen = false;
    dropPreview = null;
    loadingPreview = false;
    submitting = false;
    acknowledgeSessions = false;
    dialogError = "";
  }

  async function refreshCreatePreview() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId) {
      dialogError = "当前处于只读仓库视图，请先选择工作树再创建贮藏。";
      return;
    }
    loadingPreview = true;
    dialogError = "";
    const req = previewReq.begin();
    try {
      const res = await gitStashCreatePreview(
        repoId,
        wtId,
        {
          message: createMessage.trim() || undefined,
          includeUntracked: createIncludeUntracked,
          keepIndex: createKeepIndex,
        },
        req.signal,
      );
      if (!req.current() || disposed) return;
      createPreview = res;
    } catch (err) {
      if (req.reportable()) dialogError = parseGitManageError(err);
    } finally {
      if (req.current()) loadingPreview = false;
    }
  }

  function openCreateDialog(initialUntracked = false) {
    createOpen = true;
    createMessage = "";
    createIncludeUntracked = initialUntracked;
    createKeepIndex = false;
    createPreview = null;
    acknowledgeSessions = false;
    dialogError = "";
    void refreshCreatePreview();
  }

  async function confirmCreate() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const cur = createPreview;
    if (!repoId || !wtId || !cur?.confirmationId || submitting) return;
    submitting = true;
    dialogError = "";
    const clientRequestId = generateGitClientRequestId("stash_push");
    deps.operations?.setPhase("running", "stash_push", "正在保存贮藏…", {
      clientRequestId,
    });
    try {
      const res = await gitStashCreate(repoId, wtId, {
        confirmationId: cur.confirmationId,
        clientRequestId,
        acknowledgeSessions,
      });
      createOpen = false;
      createPreview = null;
      deps.patchView({
        statusBanner: `已保存贮藏 ${res.entry.stashRef} (${res.entry.shortOid})：${res.entry.summary}`,
      });
      await Promise.all([
        deps.onRefreshList(res.entry.oid),
        deps.onAfterStashMutation(),
      ]);
      await deps.operations?.refreshOperations(repoId, wtId);
    } catch (err) {
      dialogError = parseGitManageError(err);
    } finally {
      deps.operations?.clearActive();
      submitting = false;
    }
  }

  async function refreshApplyPreview() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const target = applyTarget;
    if (!repoId || !wtId || !target) {
      dialogError = "请先选择可写工作树再应用或弹出贮藏。";
      return;
    }
    loadingPreview = true;
    dialogError = "";
    const req = previewReq.begin();
    try {
      const res = await gitStashApplyPreview(
        repoId,
        wtId,
        {
          stashOid: target.oid,
          stashRef: target.stashRef,
          mode: applyMode,
          reinstateIndex: applyReinstateIndex,
        },
        req.signal,
      );
      if (!req.current() || disposed) return;
      applyPreview = res;
    } catch (err) {
      if (req.reportable()) dialogError = parseGitManageError(err);
    } finally {
      if (req.current()) loadingPreview = false;
    }
  }

  function openApplyDialog(entry: GitStashEntry, mode: "apply" | "pop") {
    applyTarget = entry;
    applyMode = mode;
    applyReinstateIndex = false;
    applyPreview = null;
    acknowledgeSessions = false;
    dialogError = "";
    applyOpen = true;
    void refreshApplyPreview();
  }

  async function confirmApply() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const cur = applyPreview;
    if (!repoId || !wtId || !cur?.confirmationId || submitting) return;
    submitting = true;
    dialogError = "";
    const opKind = applyMode === "pop" ? "stash_pop" : "stash_apply";
    const clientRequestId = generateGitClientRequestId(opKind);
    deps.operations?.setPhase(
      "running",
      opKind,
      applyMode === "pop" ? "正在安全弹出贮藏…" : "正在应用贮藏…",
      { clientRequestId },
    );
    try {
      const exec = applyMode === "pop" ? gitStashPop : gitStashApply;
      const res = await exec(repoId, wtId, {
        confirmationId: cur.confirmationId,
        clientRequestId,
        acknowledgeSessions,
      });
      applyOpen = false;
      applyPreview = null;
      deps.onInProgressUpdated(res.inProgress);
      deps.patchView({ statusBanner: res.summary });
      await Promise.all([deps.onRefreshList(), deps.onAfterStashMutation()]);
      await deps.operations?.refreshOperations(repoId, wtId);
      if (res.outcome === "conflicted" || res.inProgress.conflictedCount > 0) {
        deps.onOpenConflictResolver(res.inProgress.conflictedFiles[0]?.path);
      }
    } catch (err) {
      dialogError = parseGitManageError(err);
    } finally {
      deps.operations?.clearActive();
      submitting = false;
    }
  }

  async function openDropDialog(entry: GitStashEntry) {
    const repoId = deps.getRepoId();
    if (!repoId) return;
    dropTarget = entry;
    dropPreview = null;
    dialogError = "";
    dropOpen = true;
    loadingPreview = true;
    const req = previewReq.begin();
    try {
      const res = await gitStashDropPreview(
        repoId,
        { stashOid: entry.oid, stashRef: entry.stashRef },
        req.signal,
      );
      if (!req.current() || disposed) return;
      dropPreview = res;
    } catch (err) {
      if (req.reportable()) dialogError = parseGitManageError(err);
    } finally {
      if (req.current()) loadingPreview = false;
    }
  }

  async function confirmDrop() {
    const repoId = deps.getRepoId();
    const cur = dropPreview;
    if (!repoId || !cur?.confirmationId || submitting) return;
    submitting = true;
    dialogError = "";
    const clientRequestId = generateGitClientRequestId("stash_drop");
    deps.operations?.setPhase("running", "stash_drop", "正在丢弃贮藏条目…", {
      clientRequestId,
    });
    try {
      const res = await gitStashDrop(repoId, {
        confirmationId: cur.confirmationId,
        clientRequestId,
      });
      dropOpen = false;
      dropPreview = null;
      deps.patchView({
        stashes: res.stashes,
        statusBanner: `已丢弃贮藏 ${res.stashRef}（恢复命令：${res.recoveryCommand}）`,
      });
      await deps.onRefreshList();
      await deps.operations?.refreshOperations(repoId, deps.getWorktreeId());
    } catch (err) {
      dialogError = parseGitManageError(err);
    } finally {
      deps.operations?.clearActive();
      submitting = false;
    }
  }

  function closeDialogs() {
    if (submitting) return;
    previewReq.cancel();
    createOpen = false;
    applyOpen = false;
    dropOpen = false;
    dialogError = "";
  }

  return {
    get createOpen() {
      return createOpen;
    },
    get createMessage() {
      return createMessage;
    },
    setCreateMessage(val: string) {
      createMessage = val;
      void refreshCreatePreview();
    },
    get createIncludeUntracked() {
      return createIncludeUntracked;
    },
    setCreateIncludeUntracked(val: boolean) {
      createIncludeUntracked = val;
      void refreshCreatePreview();
    },
    get createKeepIndex() {
      return createKeepIndex;
    },
    setCreateKeepIndex(val: boolean) {
      createKeepIndex = val;
      void refreshCreatePreview();
    },
    get createPreview() {
      return createPreview;
    },
    get applyOpen() {
      return applyOpen;
    },
    get applyMode() {
      return applyMode;
    },
    setApplyMode(mode: "apply" | "pop") {
      applyMode = mode;
      void refreshApplyPreview();
    },
    get applyReinstateIndex() {
      return applyReinstateIndex;
    },
    setApplyReinstateIndex(val: boolean) {
      applyReinstateIndex = val;
      void refreshApplyPreview();
    },
    get applyTarget() {
      return applyTarget;
    },
    get applyPreview() {
      return applyPreview;
    },
    get dropOpen() {
      return dropOpen;
    },
    get dropTarget() {
      return dropTarget;
    },
    get dropPreview() {
      return dropPreview;
    },
    get loadingPreview() {
      return loadingPreview;
    },
    get submitting() {
      return submitting;
    },
    get acknowledgeSessions() {
      return acknowledgeSessions;
    },
    setAcknowledgeSessions(val: boolean) {
      acknowledgeSessions = val;
    },
    get dialogError() {
      return dialogError;
    },
    dispose,
    resetTarget,
    openCreateDialog,
    refreshCreatePreview,
    confirmCreate,
    openApplyDialog,
    refreshApplyPreview,
    confirmApply,
    openDropDialog,
    confirmDrop,
    closeDialogs,
  };
}
