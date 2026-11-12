import { SvelteSet } from "svelte/reactivity";
import { errorMessage } from "../../platform/error";
import {
  gitDeleteTaskBaseline,
  gitPutTaskBaseline,
  gitTaskChanges,
  gitTaskDiff,
} from "../../platform/git-task-api";
import type {
  GitCommitFile,
  GitReviewScope,
  GitTaskBaseline,
  GitTaskReviewState,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import { mapGitCompareChanges } from "../../domains/workspace/git-workbench-adapter";
import { LatestRequest } from "../shared/remote-request.svelte";
import {
  createGitSnapshotDiffReader,
  type GitDiffPanelReader,
} from "./git-snapshot-diff-reader.svelte";

export function emptyTaskReviewState(): GitTaskReviewState {
  return {
    status: "idle",
    reason: "",
    baseline: null,
    headOid: "",
    headUnborn: false,
    commitCount: 0,
    uncommittedCount: 0,
    files: [],
    selectedPath: null,
    searchQuery: "",
    nextCursor: "",
    loadingMore: false,
    mutating: false,
  };
}

function mergeTaskFiles(
  existing: readonly GitCommitFile[],
  incoming: readonly GitCommitFile[],
): GitCommitFile[] {
  const seen = new SvelteSet(existing.map((f) => f.newPath || f.oldPath));
  const out = [...existing];
  for (const file of incoming) {
    const key = file.newPath || file.oldPath;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(file);
  }
  return out;
}

function formatBaselineEndpoint(baseline: GitTaskBaseline | null): string {
  if (!baseline || baseline.unborn || !baseline.baseOid) return "空仓起点";
  return baseline.baseOid.slice(0, 7);
}

export interface GitTaskReviewControllerDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  persist: (patch: { reviewScope?: GitReviewScope }) => void;
  onBaselineChanged: () => void;
  onTaskRefreshed?: (
    repoId: string,
    wtId: string,
    signal?: AbortSignal,
  ) => void;
}

export function createGitTaskReviewController(
  deps: GitTaskReviewControllerDeps,
) {
  const listReq = new LatestRequest();
  const moreReq = new LatestRequest();
  const mutateReq = new LatestRequest();
  let searchTimer: ReturnType<typeof setTimeout> | null = null;

  const reader = createGitSnapshotDiffReader({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeId,
    onStatusBanner: (statusBanner) => deps.patchView({ statusBanner }),
  });

  function currentReview(): GitTaskReviewState {
    return deps.getView().taskReview ?? emptyTaskReviewState();
  }

  function patchReview(patch: Partial<GitTaskReviewState>) {
    deps.patchView({
      taskReview: { ...currentReview(), ...patch },
    });
  }

  function clearSearchTimer() {
    if (searchTimer !== null) {
      clearTimeout(searchTimer);
      searchTimer = null;
    }
  }

  function dispose() {
    clearSearchTimer();
    listReq.dispose();
    moreReq.dispose();
    mutateReq.dispose();
    reader.dispose();
  }

  function resetTarget() {
    clearSearchTimer();
    listReq.cancel();
    moreReq.cancel();
    mutateReq.cancel();
    reader.resetTarget();
  }

  async function loadTaskFileDiff(
    file: GitCommitFile | null,
    baselineOverride?: GitTaskBaseline | null,
  ) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const review = currentReview();
    const baseline =
      baselineOverride !== undefined ? baselineOverride : review.baseline;
    if (!repoId || !wtId || !file || !baseline) {
      reader.clearDiff();
      return;
    }

    const filePath = file.newPath || file.oldPath;
    await reader.loadFetchedDiff({
      baseOid: baseline.baseOid ?? "",
      targetOid: "WORKTREE",
      leftLabel: formatBaselineEndpoint(baseline),
      rightLabel: "当前工作区",
      file,
      missingReason: "未找到该路径相对任务起点的差异内容。",
      fetchDiff: (_cid, signal) =>
        gitTaskDiff(repoId, wtId, { path: filePath }, signal),
    });
  }

  async function refresh(
    repoId = deps.getRepoId(),
    wtId = deps.getWorktreeId(),
    parentSignal?: AbortSignal,
  ) {
    clearSearchTimer();
    moreReq.cancel();
    if (!repoId || !wtId) {
      listReq.cancel();
      reader.clearDiff();
      deps.patchView({ taskReview: emptyTaskReviewState() });
      return;
    }

    const prev = currentReview();
    const req = listReq.begin();
    const signal = parentSignal
      ? AbortSignal.any([req.signal, parentSignal])
      : req.signal;
    patchReview({
      status: prev.status === "idle" ? "loading" : prev.status,
      loadingMore: false,
      reason: "",
    });

    try {
      const res = await gitTaskChanges(
        repoId,
        wtId,
        { q: prev.searchQuery.trim() || undefined },
        signal,
      );
      if (
        !req.current() ||
        parentSignal?.aborted ||
        repoId !== deps.getRepoId() ||
        wtId !== deps.getWorktreeId()
      ) {
        return;
      }

      const files = mapGitCompareChanges(res.changes.entries);
      const baseline = res.baseline ?? null;
      const retained =
        prev.selectedPath &&
        files.some((f) => (f.newPath || f.oldPath) === prev.selectedPath)
          ? prev.selectedPath
          : null;
      const chosen =
        (retained
          ? files.find((f) => (f.newPath || f.oldPath) === retained)
          : undefined) ??
        files[0] ??
        null;
      const selectedPath = chosen ? chosen.newPath || chosen.oldPath : null;

      deps.patchView({
        taskBaseline: baseline,
        taskReview: {
          ...currentReview(),
          status: res.status,
          reason: res.reason ?? "",
          baseline,
          headOid: res.headOid ?? "",
          headUnborn: Boolean(res.headUnborn),
          commitCount: res.commitCount ?? 0,
          uncommittedCount: res.uncommittedCount ?? 0,
          files,
          selectedPath,
          nextCursor: res.changes.nextCursor,
          loadingMore: false,
          mutating: false,
        },
      });

      const latestView = deps.getView();
      if (
        res.status === "ready" &&
        chosen &&
        latestView.reviewScope === "task"
      ) {
        void loadTaskFileDiff(chosen, baseline);
      } else if (!chosen || res.status !== "ready") {
        reader.clearDiff();
      }
      deps.onTaskRefreshed?.(repoId, wtId, signal);
    } catch (err) {
      if (!req.reportable()) return;
      patchReview({
        status: "error",
        reason: errorMessage(err),
        loadingMore: false,
      });
    }
  }

  async function loadMore() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const review = currentReview();
    if (!repoId || !wtId || !review.nextCursor || review.loadingMore) return;

    const req = moreReq.begin();
    patchReview({ loadingMore: true });
    try {
      const res = await gitTaskChanges(
        repoId,
        wtId,
        {
          cursor: review.nextCursor,
          q: review.searchQuery.trim() || undefined,
        },
        req.signal,
      );
      if (
        !req.current() ||
        repoId !== deps.getRepoId() ||
        wtId !== deps.getWorktreeId()
      ) {
        return;
      }
      const latest = currentReview();
      patchReview({
        files: mergeTaskFiles(
          latest.files,
          mapGitCompareChanges(res.changes.entries),
        ),
        nextCursor: res.changes.nextCursor,
        loadingMore: false,
      });
    } catch (err) {
      if (!req.reportable()) return;
      patchReview({ loadingMore: false });
      deps.patchView({
        statusBanner: `加载更多任务变更失败：${errorMessage(err)}`,
      });
    }
  }

  async function runBaselineMutation(
    errorPrefix: string,
    execute: (
      repoId: string,
      wtId: string,
      signal: AbortSignal,
    ) => Promise<void>,
  ) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId) return;

    const req = mutateReq.begin();
    patchReview({ mutating: true });
    try {
      await execute(repoId, wtId, req.signal);
      if (!req.current()) return;
      await refresh(repoId, wtId);
      deps.onBaselineChanged();
    } catch (err) {
      if (!req.reportable()) return;
      patchReview({ mutating: false });
      deps.patchView({
        statusBanner: `${errorPrefix}：${errorMessage(err)}`,
      });
    }
  }

  function setBaseline(baseOid?: string) {
    return runBaselineMutation(
      "设置任务起点失败",
      async (repoId, wtId, signal) => {
        const res = await gitPutTaskBaseline(
          repoId,
          wtId,
          baseOid ? { baseOid } : {},
          signal,
        );
        const baseline = res.baseline ?? null;
        const label = formatBaselineEndpoint(baseline);
        const dirtySuffix =
          baseline && !baseline.cleanAtStart
            ? "（设定前工作树已有改动，审阅将展示相对所选提交的全部变化）"
            : "";
        deps.patchView({
          taskBaseline: baseline,
          statusBanner: `已将 ${label} 设为当前工作树任务起点${dirtySuffix}`,
        });
      },
    );
  }

  function clearBaseline() {
    return runBaselineMutation(
      "清除任务起点失败",
      async (repoId, wtId, signal) => {
        await gitDeleteTaskBaseline(repoId, wtId, signal);
        reader.clearDiff();
        deps.patchView({
          taskBaseline: null,
          statusBanner: "已清除当前工作树任务起点基线",
        });
      },
    );
  }

  const diffReader: GitDiffPanelReader = {
    get state() {
      return reader.state;
    },
    openSnapshotSide: reader.openSnapshotSide,
    closeSnapshotSide: reader.closeSnapshotSide,
    downloadSnapshotSide: reader.downloadSnapshotSide,
    retryActiveDiff() {
      const review = currentReview();
      const chosen =
        review.files.find(
          (f) => (f.newPath || f.oldPath) === review.selectedPath,
        ) ?? null;
      void loadTaskFileDiff(chosen);
    },
  };

  return {
    get diffReader() {
      return diffReader;
    },
    dispose,
    resetTarget,
    seedContextId: reader.seedContextId,
    refresh,
    loadMore,
    setBaseline,
    clearBaseline,
    setReviewScope(scope: GitReviewScope) {
      const view = deps.getView();
      if (scope === (view.reviewScope ?? "uncommitted")) return;
      deps.patchView({ reviewScope: scope });
      deps.persist({ reviewScope: scope });
      if (scope === "task") {
        const review = currentReview();
        if (review.status === "idle") {
          void refresh();
        } else if (review.status === "ready") {
          const chosen =
            review.files.find(
              (f) => (f.newPath || f.oldPath) === review.selectedPath,
            ) ??
            review.files[0] ??
            null;
          void loadTaskFileDiff(chosen);
        }
      }
    },
    selectFile(path: string) {
      const review = currentReview();
      patchReview({ selectedPath: path });
      const chosen =
        review.files.find((f) => (f.newPath || f.oldPath) === path) ?? null;
      void loadTaskFileDiff(chosen);
    },
    setSearchQuery(searchQuery: string) {
      patchReview({ searchQuery });
      clearSearchTimer();
      searchTimer = setTimeout(() => {
        searchTimer = null;
        void refresh();
      }, 200);
    },
  };
}
