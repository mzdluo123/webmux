import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import type {
  Repository,
  SessionInfo,
  Worktree,
} from "../../contracts/schemas";
import type { writeGitTargetMemory } from "../../domains/workspace/git-workbench-memory";
import type { buildGitRouteHref } from "../shared/route";
import { createGitCommitDraftController } from "./git-commit-draft.svelte";
import { createGitConflictController } from "./git-conflict-controller.svelte";
import { createGitDivergenceController } from "./git-divergence-controller.svelte";
import { createGitLogCompareController } from "./git-log-compare.svelte";
import { createGitLogDiffController } from "./git-log-diff.svelte";
import { createGitLogHistoryController } from "./git-log-history.svelte";
import { createGitOperationsController } from "./git-operations-controller.svelte";
import { createGitPushController } from "./git-push-controller.svelte";
import { createGitReviewSyncController } from "./git-review-sync.svelte";
import { createGitReworkController } from "./git-rework-controller.svelte";
import { createGitTaskReviewController } from "./git-task-review.svelte";
import { createGitStagingController } from "./git-workbench-staging.svelte";

export interface GitSubControllersBindings {
  getRepoId: () => string;
  getEffectiveWorktreeId: () => string | undefined;
  getWorktreeTargetId: () => string | undefined;
  getDefaultBranch: () => string | null;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  persist: (patch: Parameters<typeof writeGitTargetMemory>[2]) => void;
  updateUrl: (
    mode: "push" | "replace",
    overrides?: Partial<Parameters<typeof buildGitRouteHref>[0]>,
  ) => void;
  onSelectChange: (changeId: string) => void;
  onReloadTarget: (
    repo?: Repository,
    wt?: Worktree,
    sessions?: readonly SessionInfo[],
  ) => Promise<void>;
  onRepositoryUpdated?: (repo: Repository) => void;
  onRefreshRepositories?: () => void;
}

export function createGitWorkbenchSubControllers(
  deps: GitSubControllersBindings,
) {
  const commitDraft = createGitCommitDraftController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeTargetId,
    getView: deps.getView,
    patchView: deps.patchView,
  });
  const rework = createGitReworkController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeTargetId,
    getView: deps.getView,
    patchView: deps.patchView,
  });
  const operations = createGitOperationsController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeTargetId,
    onOutcomeRestored: (lastCommitOutcome) => {
      if (lastCommitOutcome) deps.patchView({ lastCommitOutcome });
    },
    onStatusBanner: (statusBanner) => deps.patchView({ statusBanner }),
    onRepositoryUpdated: deps.onRepositoryUpdated,
    onReloadTarget: (repo, wt, sessions = []) =>
      deps.onReloadTarget(repo, wt, sessions),
  });
  const logDiff = createGitLogDiffController({
    getRepoId: deps.getRepoId,
    getEffectiveWorktreeId: deps.getEffectiveWorktreeId,
    getView: deps.getView,
    patchView: deps.patchView,
    persistFileSelection: (path) =>
      deps.persist({ selectedCommitFilePath: path }),
  });
  const logCompare = createGitLogCompareController({
    getRepoId: deps.getRepoId,
    getEffectiveWorktreeId: deps.getEffectiveWorktreeId,
    getView: deps.getView,
    patchView: deps.patchView,
    logDiff,
  });
  const logHistory = createGitLogHistoryController({
    getRepoId: deps.getRepoId,
    getEffectiveWorktreeId: deps.getEffectiveWorktreeId,
    getDefaultBranch: deps.getDefaultBranch,
    getView: deps.getView,
    patchView: deps.patchView,
    persist: deps.persist,
    updateUrl: deps.updateUrl,
    logDiff,
  });
  const reviews = createGitReviewSyncController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeTargetId,
    getView: deps.getView,
    patchView: deps.patchView,
    persistReviewedPaths: (reviewedPaths) => deps.persist({ reviewedPaths }),
    onSelectChange: deps.onSelectChange,
    onSelectTaskFile: (path) => taskReview.selectFile(path),
    onStaleRejectedRefresh: () => {
      void staging.refreshActiveSnapshot();
      void taskReview.refresh();
    },
  });
  const taskReview = createGitTaskReviewController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeTargetId,
    getView: deps.getView,
    patchView: deps.patchView,
    persist: deps.persist,
    onBaselineChanged: () => {
      if (deps.getView().scope === "task") {
        void logHistory.reloadHistory("refresh");
      }
      void reviews.refreshScope("task");
    },
    onTaskRefreshed: (repoId, wtId, sig) => {
      void reviews.refreshScope("task", repoId, wtId, sig);
    },
  });
  const staging = createGitStagingController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeTargetId,
    getView: deps.getView,
    patchView: deps.patchView,
    operations,
    onRefreshRepositories: deps.onRefreshRepositories,
    onAfterCommitRefresh: () => {
      void logHistory.reloadHistory("refresh");
      void taskReview.refresh();
    },
    onStagingRefreshed: (repoId, wtId, sig) => {
      void reviews.refreshScope("uncommitted", repoId, wtId, sig);
    },
    onBeforeOpenPreview: () => commitDraft.flushSave(),
    onCommitDraftCleared: () => commitDraft.markClearedAfterCommit(),
  });
  const push = createGitPushController({
    getRepoId: deps.getRepoId,
    getRepoName: () => deps.getView().context.repositoryName,
    getWorktreeId: deps.getWorktreeTargetId,
    operations,
    onRepositoryUpdated: deps.onRepositoryUpdated,
    onRefreshAfterPush: () => {
      deps.onRefreshRepositories?.();
      void logHistory.reloadHistory("refresh");
      void taskReview.refresh();
    },
    onStatusBanner: (statusBanner) => deps.patchView({ statusBanner }),
  });
  const divergence = createGitDivergenceController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeTargetId,
    getView: deps.getView,
    patchView: deps.patchView,
    operations,
    onAfterMutation: async () => {
      deps.onRefreshRepositories?.();
      await Promise.all([
        staging.refreshActiveSnapshot(),
        logHistory.reloadHistory("refresh"),
        taskReview.refresh(),
      ]);
    },
    onOpenConflictResolver: (path) => {
      void conflict.openResolver(path);
    },
    onCloseConflictResolver: () => {
      conflict.closeResolver();
    },
  });
  const conflict = createGitConflictController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeTargetId,
    getInProgress: () => divergence.inProgress,
    onInProgressUpdated: (next) => divergence.setInProgressExternal(next),
    onRefreshStaging: () => staging.refreshActiveSnapshot(),
    onStatusBanner: (statusBanner) => deps.patchView({ statusBanner }),
  });

  function disposeAll() {
    commitDraft.dispose();
    reviews.dispose();
    rework.dispose();
    operations.dispose();
    staging.dispose();
    taskReview.dispose();
    logHistory.dispose();
    logCompare.dispose();
    logDiff.dispose();
    push.dispose();
    divergence.dispose();
    conflict.dispose();
  }

  function resetAllTargets(repoId: string, wtId?: string) {
    commitDraft.resetTarget();
    reviews.resetTarget();
    rework.loadTarget(repoId, wtId);
    operations.resetTarget();
    staging.resetTarget();
    taskReview.resetTarget();
    logHistory.resetTarget();
    logCompare.resetTarget();
    logDiff.resetTarget();
    divergence.resetTarget();
    conflict.resetTarget();
  }

  return {
    commitDraft,
    reviews,
    rework,
    operations,
    staging,
    taskReview,
    logHistory,
    logCompare,
    logDiff,
    push,
    divergence,
    conflict,
    disposeAll,
    resetAllTargets,
  };
}
