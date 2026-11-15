import { APIError } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import {
  gitAbortOperation,
  gitAbortOperationPreview,
  gitCherryPickPreview,
  gitContinueOperation,
  gitExecuteDivergence,
  gitMergePreview,
  gitOperationState,
  gitRebasePreview,
  gitRevertPreview,
} from "../../platform/git-conflict-api";
import {
  GitConflictSchema,
  type GitAbortOperationPreview,
  type GitDivergenceKind,
  type GitDivergencePreview,
  type GitDivergenceResult,
  type GitInProgressOperation,
} from "../../contracts/schemas";
import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import {
  generateGitClientRequestId,
  gitOperationKindLabel,
  isRecoverableNetworkFailure,
} from "../../domains/workspace/git-operations-model";
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";

export interface GitDivergenceControllerDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  operations: GitOperationsController;
  onAfterMutation: () => Promise<void>;
  onOpenConflictResolver?: (path?: string) => void;
  onCloseConflictResolver?: () => void;
}

function extractConflictReason(err: unknown): {
  reason?: string;
  message: string;
} {
  if (err instanceof APIError && err.status === 409) {
    const parsed = GitConflictSchema.safeParse(err.body);
    if (parsed.success) {
      return { reason: parsed.data.reason, message: parsed.data.error };
    }
  }
  return { message: errorMessage(err) };
}

export function createGitDivergenceController(
  deps: GitDivergenceControllerDeps,
) {
  let inProgress = $state<GitInProgressOperation | null>(null);
  let previewOpen = $state(false);
  let previewKind = $state<GitDivergenceKind>("merge");
  let preview = $state<GitDivergencePreview | null>(null);
  let abortOpen = $state(false);
  let abortPreview = $state<GitAbortOperationPreview | null>(null);
  let loadingPreview = $state(false);
  let submitting = $state(false);
  let dialogError = $state("");
  let acknowledgeSessions = $state(false);

  const stateReq = new LatestRequest();
  const previewReq = new LatestRequest();
  let disposed = false;

  function syncContextOperation(next: GitInProgressOperation | null) {
    inProgress = next;
    const cur = deps.getView();
    const nextOp = next?.active ? (next.kind ?? "merge") : null;
    if (cur.context.operationInProgress !== nextOp) {
      deps.patchView({
        context: {
          ...cur.context,
          operationInProgress: nextOp,
        },
      });
    }
  }

  function dispose() {
    disposed = true;
    stateReq.dispose();
    previewReq.dispose();
  }

  function resetTarget() {
    stateReq.cancel();
    previewReq.cancel();
    inProgress = null;
    previewOpen = false;
    preview = null;
    abortOpen = false;
    abortPreview = null;
    loadingPreview = false;
    submitting = false;
    dialogError = "";
    acknowledgeSessions = false;
  }

  async function refreshState(
    repoId = deps.getRepoId(),
    wtId = deps.getWorktreeId(),
    signal?: AbortSignal,
  ): Promise<GitInProgressOperation | null> {
    if (!repoId || !wtId || disposed) {
      syncContextOperation(null);
      return null;
    }
    const req = stateReq.begin();
    const effectiveSignal = signal ?? req.signal;
    try {
      const res = await gitOperationState(repoId, wtId, effectiveSignal);
      if (
        !req.current() ||
        disposed ||
        effectiveSignal.aborted ||
        repoId !== deps.getRepoId() ||
        wtId !== deps.getWorktreeId()
      ) {
        return res;
      }
      syncContextOperation(res);
      return res;
    } catch {
      return inProgress;
    }
  }

  async function runOpenPreview(
    kind: GitDivergenceKind,
    loader: (
      repoId: string,
      wtId: string,
      signal: AbortSignal,
    ) => Promise<GitDivergencePreview>,
  ) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId) {
      deps.patchView({
        statusBanner: "请先选择可写工作树再执行合并、变基、遴选或还原。",
      });
      return;
    }
    const req = previewReq.begin();
    previewKind = kind;
    previewOpen = true;
    preview = null;
    loadingPreview = true;
    dialogError = "";
    acknowledgeSessions = false;
    try {
      const res = await loader(repoId, wtId, req.signal);
      if (!req.current() || disposed) return;
      preview = res;
    } catch (err) {
      if (req.reportable()) {
        dialogError = extractConflictReason(err).message;
      }
    } finally {
      if (req.current()) loadingPreview = false;
    }
  }

  function openMergePreview(refKind: "local" | "remote", refName: string) {
    return runOpenPreview("merge", (repoId, wtId, signal) =>
      gitMergePreview(repoId, wtId, { refKind, refName }, signal),
    );
  }

  function openRebasePreview(refKind: "local" | "remote", refName: string) {
    return runOpenPreview("rebase", (repoId, wtId, signal) =>
      gitRebasePreview(repoId, wtId, { refKind, refName }, signal),
    );
  }

  function openCherryPickPreview(commitOid: string) {
    return runOpenPreview("cherry_pick", (repoId, wtId, signal) =>
      gitCherryPickPreview(repoId, wtId, { commitOid }, signal),
    );
  }

  function openRevertPreview(commitOid: string) {
    return runOpenPreview("revert", (repoId, wtId, signal) =>
      gitRevertPreview(repoId, wtId, { commitOid }, signal),
    );
  }

  function closePreview() {
    if (submitting) return;
    previewReq.cancel();
    previewOpen = false;
    preview = null;
    dialogError = "";
    acknowledgeSessions = false;
  }

  async function applyDivergenceResult(res: GitDivergenceResult) {
    syncContextOperation(res.inProgress);
    deps.patchView({ statusBanner: res.summary });
    await deps.onAfterMutation();
    await deps.operations.refreshOperations();
    if (res.outcome === "conflicted" || res.inProgress.conflictedCount > 0) {
      deps.onOpenConflictResolver?.(res.inProgress.conflictedFiles[0]?.path);
    } else if (!res.inProgress.active) {
      deps.onCloseConflictResolver?.();
    }
  }

  function beginConfirmedAction(
    kind: GitDivergenceKind | "abort" | "continue",
    label: string,
  ): { repoId: string; wtId: string; clientRequestId: string } | null {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId || submitting) return null;
    submitting = true;
    dialogError = "";
    const clientRequestId = generateGitClientRequestId(kind);
    deps.operations.setPhase("running", kind, label, { clientRequestId });
    return { repoId, wtId, clientRequestId };
  }

  async function confirmDivergence() {
    const cur = preview;
    if (!cur?.confirmationId) return;
    const ctx = beginConfirmedAction(
      cur.kind,
      `正在执行${gitOperationKindLabel(cur.kind)}…`,
    );
    if (!ctx) return;
    const { repoId, wtId, clientRequestId } = ctx;
    try {
      const res = await gitExecuteDivergence(repoId, wtId, cur.kind, {
        confirmationId: cur.confirmationId,
        acknowledgeSessions,
        clientRequestId,
      });
      previewOpen = false;
      preview = null;
      await applyDivergenceResult(res);
    } catch (err) {
      if (isRecoverableNetworkFailure(err)) {
        const recovered = await deps.operations.recoverByClientRequest({
          repositoryId: repoId,
          worktreeId: wtId,
          kind: cur.kind,
          clientRequestId,
        });
        const latestState = await refreshState(repoId, wtId);
        if (recovered || latestState?.active) {
          previewOpen = false;
          preview = null;
          deps.patchView({
            statusBanner:
              recovered?.summary || "已恢复进行中的 Git 操作状态，请核对工作区",
          });
          await deps.onAfterMutation();
          if (latestState && latestState.conflictedCount > 0) {
            deps.onOpenConflictResolver?.(latestState.conflictedFiles[0]?.path);
          }
          return;
        }
      }
      dialogError = extractConflictReason(err).message;
    } finally {
      deps.operations.clearActive();
      submitting = false;
    }
  }

  async function continueOperation() {
    const cur = inProgress;
    if (!cur?.active || !cur.kind) return;
    const ctx = beginConfirmedAction("continue", "正在继续完成 Git 操作…");
    if (!ctx) return;
    const { repoId, wtId, clientRequestId } = ctx;
    try {
      const res = await gitContinueOperation(repoId, wtId, {
        kind: cur.kind,
        clientRequestId,
      });
      await applyDivergenceResult(res);
    } catch (err) {
      const info = extractConflictReason(err);
      deps.patchView({ statusBanner: `继续操作未完成：${info.message}` });
      await refreshState(repoId, wtId);
      await deps.operations.refreshOperations(repoId, wtId);
    } finally {
      deps.operations.clearActive();
      submitting = false;
    }
  }

  async function openAbortPreview() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId) return;
    const req = previewReq.begin();
    abortOpen = true;
    abortPreview = null;
    loadingPreview = true;
    dialogError = "";
    acknowledgeSessions = false;
    try {
      const res = await gitAbortOperationPreview(repoId, wtId, req.signal);
      if (!req.current() || disposed) return;
      abortPreview = res;
    } catch (err) {
      if (req.reportable()) {
        dialogError = extractConflictReason(err).message;
      }
    } finally {
      if (req.current()) loadingPreview = false;
    }
  }

  function closeAbortPreview() {
    if (submitting) return;
    previewReq.cancel();
    abortOpen = false;
    abortPreview = null;
    dialogError = "";
    acknowledgeSessions = false;
  }

  async function confirmAbort() {
    const cur = abortPreview;
    if (!cur?.confirmationId) return;
    const ctx = beginConfirmedAction("abort", "正在中止操作并回退工作树…");
    if (!ctx) return;
    const { repoId, wtId, clientRequestId } = ctx;
    try {
      const res = await gitAbortOperation(repoId, wtId, {
        confirmationId: cur.confirmationId,
        acknowledgeSessions,
        clientRequestId,
      });
      abortOpen = false;
      abortPreview = null;
      await applyDivergenceResult(res);
    } catch (err) {
      dialogError = extractConflictReason(err).message;
      await refreshState(repoId, wtId);
    } finally {
      deps.operations.clearActive();
      submitting = false;
    }
  }

  return {
    get inProgress() {
      return inProgress;
    },
    get previewOpen() {
      return previewOpen;
    },
    get previewKind() {
      return previewKind;
    },
    get preview() {
      return preview;
    },
    get abortOpen() {
      return abortOpen;
    },
    get abortPreview() {
      return abortPreview;
    },
    get loadingPreview() {
      return loadingPreview;
    },
    get submitting() {
      return submitting;
    },
    get dialogError() {
      return dialogError;
    },
    get acknowledgeSessions() {
      return acknowledgeSessions;
    },
    setAcknowledgeSessions(next: boolean) {
      acknowledgeSessions = next;
    },
    setInProgressExternal: syncContextOperation,
    dispose,
    resetTarget,
    refreshState,
    openMergePreview,
    openRebasePreview,
    openCherryPickPreview,
    openRevertPreview,
    closePreview,
    confirmDivergence,
    continueOperation,
    openAbortPreview,
    closeAbortPreview,
    confirmAbort,
  };
}
