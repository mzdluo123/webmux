import { errorMessage } from "../../platform/error";
import type {
  GitWorkbenchMode,
  GitWorkbenchViewState,
  GitWorkingChange,
} from "../../contracts/git-workbench-schemas";
import type {
  Repository,
  SessionInfo,
  Worktree,
} from "../../contracts/schemas";
import { deriveGitWorktreeContext } from "../../domains/workspace/git-workbench-adapter";
import { writeGitTargetMemory } from "../../domains/workspace/git-workbench-memory";
import { toggleReviewedChange } from "../../domains/workspace/git-workbench-state";
import { LatestRequest } from "../shared/remote-request.svelte";
import { buildGitRouteHref, parseGitRouteQuery } from "../shared/route";
import { createGitLogCompareController } from "./git-log-compare.svelte";
import { createGitLogDiffController } from "./git-log-diff.svelte";
import { createGitLogHistoryController } from "./git-log-history.svelte";
import { createGitOperationsController } from "./git-operations-controller.svelte";
import { createGitPushController } from "./git-push-controller.svelte";
import {
  emptyGitViewState,
  fetchGitTargetBundle,
} from "./git-workbench-actions";
import {
  applyPopstateQueryToView,
  buildTargetInitialState,
  computeGitTargetLoadKey,
  pickSelectedCommitAfterBundle,
  syncGitWorkbenchUrl,
} from "./git-workbench-route-sync";
import { createGitStagingController } from "./git-workbench-staging.svelte";

export interface GitControllerOptions {
  onRepositoryUpdated?: (repo: Repository) => void;
  onRefreshRepositories?: () => void;
}

export function createGitWorkbenchController(
  options: GitControllerOptions = {},
) {
  let currentRepoId = $state("");
  let currentWorktreeId = $state<string | undefined>(undefined);
  let effectiveWorktreeId = $state<string | undefined>(undefined);
  let defaultBranchName: string | null = null;
  let lastAppliedSearch = "";
  let lastLoadedKey = "";
  let returnSessionId = $state<string | undefined>(undefined);

  let view = $state<GitWorkbenchViewState>(emptyGitViewState(""));
  let loading = $state(false);
  let error = $state("");
  let scrollOffsets = $state({
    logList: 0,
    logDiff: 0,
    commitList: 0,
    commitDiff: 0,
  });

  const patchView = (patch: Partial<GitWorkbenchViewState>) => {
    view = { ...view, ...patch };
  };
  const persist = (patch: Parameters<typeof writeGitTargetMemory>[2]) =>
    writeGitTargetMemory(currentRepoId, currentWorktreeId, patch);

  function updateUrl(
    mode: "push" | "replace",
    overrides: Partial<Parameters<typeof buildGitRouteHref>[0]> = {},
  ) {
    lastAppliedSearch = syncGitWorkbenchUrl(
      mode,
      {
        repositoryId: currentRepoId,
        worktreeId: currentWorktreeId,
        view,
        returnSession: returnSessionId,
      },
      overrides,
    );
  }

  const targetReq = new LatestRequest();
  const operations = createGitOperationsController({
    getRepoId: () => currentRepoId,
    getWorktreeId: () => effectiveWorktreeId ?? currentWorktreeId,
    onOutcomeRestored: (lastCommitOutcome) => {
      if (lastCommitOutcome) patchView({ lastCommitOutcome });
    },
    onStatusBanner: (statusBanner) => patchView({ statusBanner }),
    onRepositoryUpdated: options.onRepositoryUpdated,
    onReloadTarget: (repo, wt, sessions = []) =>
      loadTarget(currentRepoId, currentWorktreeId, repo, wt, sessions),
  });
  const staging = createGitStagingController({
    getRepoId: () => currentRepoId,
    getWorktreeId: () => effectiveWorktreeId ?? currentWorktreeId,
    getView: () => view,
    patchView,
    operations,
    onRefreshRepositories: options.onRefreshRepositories,
    onAfterCommitRefresh: () => void logHistory.reloadHistory("refresh"),
  });
  const logDiff = createGitLogDiffController({
    getRepoId: () => currentRepoId,
    getEffectiveWorktreeId: () => effectiveWorktreeId,
    getView: () => view,
    patchView,
    persistFileSelection: (path) => persist({ selectedCommitFilePath: path }),
  });
  const logCompare = createGitLogCompareController({
    getRepoId: () => currentRepoId,
    getEffectiveWorktreeId: () => effectiveWorktreeId,
    getView: () => view,
    patchView,
    logDiff,
  });
  const logHistory = createGitLogHistoryController({
    getRepoId: () => currentRepoId,
    getEffectiveWorktreeId: () => effectiveWorktreeId,
    getDefaultBranch: () => defaultBranchName,
    getView: () => view,
    patchView,
    persist,
    updateUrl,
    logDiff,
  });
  const push = createGitPushController({
    getRepoId: () => currentRepoId,
    getRepoName: () => view.context.repositoryName,
    getWorktreeId: () => effectiveWorktreeId ?? currentWorktreeId,
    operations,
    onRepositoryUpdated: options.onRepositoryUpdated,
    onRefreshAfterPush: () => {
      options.onRefreshRepositories?.();
      void logHistory.reloadHistory("refresh");
    },
    onStatusBanner: (statusBanner) => patchView({ statusBanner }),
  });

  function dispose() {
    targetReq.dispose();
    operations.dispose();
    staging.dispose();
    logHistory.dispose();
    logCompare.dispose();
    logDiff.dispose();
    push.dispose();
  }

  async function loadTarget(
    repoId: string,
    wtId: string | undefined,
    repository: Repository | undefined,
    worktree: Worktree | undefined,
    sessions: readonly SessionInfo[],
  ) {
    if (!repoId || (repository && repository.availability !== "available")) {
      loading = false;
      effectiveWorktreeId = undefined;
      patchView({
        context: deriveGitWorktreeContext({
          repository,
          repositoryId: repoId,
          worktree,
          gitCtx: null,
        }),
      });
      return;
    }

    const req = targetReq.begin();
    loading = true;
    error = "";
    try {
      const bundle = await fetchGitTargetBundle({
        repositoryId: repoId,
        worktreeId: wtId,
        repository,
        worktree,
        sessions,
        scope: view.scope,
        branchFilter: view.branchFilter,
        searchQuery: view.searchQuery,
        authorFilter: view.authorFilter,
        sinceFilter: view.sinceFilter,
        untilFilter: view.untilFilter,
        pathFilter: view.pathFilter,
        signal: req.signal,
      });
      if (
        !req.current() ||
        repoId !== currentRepoId ||
        wtId !== currentWorktreeId
      ) {
        return;
      }
      effectiveWorktreeId = bundle.effectiveWorktreeId;
      defaultBranchName = bundle.defaultBranch;
      const effId = bundle.effectiveWorktreeId;
      logDiff.seedContextId(repoId, effId, bundle.gitContextId);
      staging.seedContextId(repoId, effId, bundle.gitContextId);

      const selectedOid = pickSelectedCommitAfterBundle(
        view.selectedCommitOid,
        bundle.context.headOid,
        bundle.commits,
      );

      patchView({
        context: bundle.context,
        historyState: bundle.historyState,
        historyReason: bundle.historyReason,
        historyUpstream: bundle.historyUpstream,
        lastFetchedAt: bundle.lastFetchedAt,
        nextCursor: bundle.nextCursor,
        refsChanged: bundle.refsChanged,
        historyLoadingMore: false,
        commits: bundle.commits,
        branches: bundle.branches,
        selectedCommitOid: selectedOid,
      });
      logHistory.ensureCommitLoaded(selectedOid);
      if (effId) await staging.refresh(repoId, effId, req.signal);
      await operations.refreshOperations(repoId, effId ?? wtId, req.signal);
    } catch (err) {
      if (req.reportable()) error = errorMessage(err);
    } finally {
      if (req.current()) loading = false;
    }
  }

  function syncRoute(params: {
    repositoryId: string;
    worktreeId?: string;
    search: string;
    repository: Repository | undefined;
    worktree: Worktree | undefined;
    sessions: readonly SessionInfo[];
  }) {
    const query = parseGitRouteQuery(params.search);
    returnSessionId = query.returnSession;
    const targetChanged =
      params.repositoryId !== currentRepoId ||
      params.worktreeId !== currentWorktreeId;

    const desiredLoadKey = computeGitTargetLoadKey(
      params.repositoryId,
      params.worktreeId,
      params.repository,
      params.worktree,
    );

    if (targetChanged) {
      currentRepoId = params.repositoryId;
      currentWorktreeId = params.worktreeId;
      effectiveWorktreeId = undefined;
      defaultBranchName = null;
      lastAppliedSearch = params.search;
      lastLoadedKey = desiredLoadKey;
      operations.resetTarget();
      staging.resetTarget();
      logHistory.resetTarget();
      logCompare.resetTarget();
      logDiff.resetTarget();
      error = "";

      const init = buildTargetInitialState({
        repositoryId: params.repositoryId,
        worktreeId: params.worktreeId,
        query,
        repository: params.repository,
        worktree: params.worktree,
      });
      scrollOffsets = {
        logList: init.memory.logListScrollTop,
        logDiff: init.memory.logDiffScrollTop,
        commitList: init.memory.commitListScrollTop,
        commitDiff: init.memory.commitDiffScrollTop,
      };
      view = init.view;
      if (!params.search && params.repositoryId) updateUrl("replace");
      void loadTarget(
        params.repositoryId,
        params.worktreeId,
        params.repository,
        params.worktree,
        params.sessions,
      );
      return;
    }

    if (params.search !== lastAppliedSearch) {
      lastAppliedSearch = params.search;
      const popped = applyPopstateQueryToView(
        view,
        currentRepoId,
        currentWorktreeId,
        query,
      );
      view = popped.nextView;
      if (popped.reloadNeeded) {
        void logHistory.reloadHistory("filter", popped.nextCommit);
      } else {
        logHistory.ensureCommitLoaded(popped.nextCommit);
      }
      return;
    }

    if (desiredLoadKey !== lastLoadedKey) {
      lastLoadedKey = desiredLoadKey;
      if (desiredLoadKey) {
        void loadTarget(
          currentRepoId,
          currentWorktreeId,
          params.repository,
          params.worktree,
          params.sessions,
        );
      } else {
        patchView({
          context: deriveGitWorktreeContext({
            repository: params.repository,
            repositoryId: params.repositoryId,
            worktree: params.worktree,
            gitCtx: null,
          }),
        });
      }
    }
  }

  return {
    get view() {
      return view;
    },
    get loading() {
      return loading;
    },
    get error() {
      return error;
    },
    get fetching() {
      return operations.fetching;
    },
    get operations() {
      return operations;
    },
    get stagingBusy() {
      return staging.stagingBusy;
    },
    get staging() {
      return staging;
    },
    get push() {
      return push;
    },
    get detailLoading() {
      return logHistory.detailLoading;
    },
    get logDiff() {
      return logDiff;
    },
    get returnSessionId() {
      return returnSessionId;
    },
    get scrollOffsets() {
      return scrollOffsets;
    },
    dispose,
    syncRoute,
    reload: (
      repo?: Repository,
      wt?: Worktree,
      sessions: readonly SessionInfo[] = [],
    ) => loadTarget(currentRepoId, currentWorktreeId, repo, wt, sessions),
    setMode(mode: GitWorkbenchMode) {
      if (mode === view.mode) return;
      patchView({ mode });
      persist({ mode });
      updateUrl("push", { view: mode });
    },
    setScope: logHistory.setScope,
    setBranchFilter: logHistory.setBranchFilter,
    setSearchQuery: logHistory.setSearchQuery,
    setAuthorFilter: logHistory.setAuthorFilter,
    setSinceFilter: logHistory.setSinceFilter,
    setUntilFilter: logHistory.setUntilFilter,
    setPathFilter: logHistory.setPathFilter,
    clearLogFilters: logHistory.clearLogFilters,
    refreshHistory: () => logHistory.reloadHistory("refresh"),
    loadMoreHistory: logHistory.loadMoreHistory,
    jumpToHead: logHistory.jumpToHead,
    dismissSelectionNotice() {
      patchView({ selectionNotice: null });
    },
    toggleFavorite: logHistory.toggleFavorite,
    selectCommit(oid: string) {
      logCompare.resetTarget();
      patchView({
        selectedCommitOid: oid,
        selectedCommitFilePath: null,
        selectionNotice: null,
        compareMode: null,
      });
      persist({ selectedCommitOid: oid, selectedCommitFilePath: null });
      updateUrl("replace", { commit: oid });
      logHistory.ensureCommitLoaded(oid);
    },
    selectParent(commitOid: string, parentOid: string) {
      const selectedParentByCommit = {
        ...view.selectedParentByCommit,
        [commitOid]: parentOid,
      };
      const patch = { selectedParentByCommit, selectedCommitFilePath: null };
      patchView(patch);
      persist(patch);
      void logHistory.loadCommitDetail(currentRepoId, commitOid, parentOid);
    },
    selectCommitFile: logDiff.selectActiveFile,
    markCompareBase: logCompare.markCompareBase,
    startCompare: logCompare.startCompare,
    loadMoreCompareFiles: logCompare.loadMoreCompareFiles,
    swapCompareDirection: logCompare.swapCompareDirection,
    exitCompare: logCompare.exitCompare,
    selectChange(changeId: string) {
      patchView({ activeChangeId: changeId });
      persist({ activeChangeId: changeId });
      const wtId = effectiveWorktreeId ?? currentWorktreeId;
      if (wtId) {
        void staging.loadActiveDiff(currentRepoId, wtId, changeId);
      }
    },
    toggleReviewed(change: GitWorkingChange | string) {
      const target =
        typeof change === "string"
          ? (view.workingChanges.find(
              (c) => c.id === change || c.path === change,
            ) ?? { path: change })
          : change;
      const reviewedPaths = toggleReviewedChange(view.reviewedPaths, target);
      patchView({ reviewedPaths });
      persist({ reviewedPaths });
    },
    dismissLastCommitOutcome() {
      operations.acknowledgeAttention(view.lastCommitOutcome?.operationId);
      patchView({ lastCommitOutcome: null });
    },
    async reloadAndVerifyUncertain(
      repo?: Repository,
      wt?: Worktree,
      sessions: readonly SessionInfo[] = [],
    ) {
      operations.acknowledgeAttention();
      patchView({ lastCommitOutcome: null });
      await loadTarget(currentRepoId, currentWorktreeId, repo, wt, sessions);
    },
    inspectCommitInLog(oid: string) {
      logCompare.resetTarget();
      const patch = {
        mode: "log" as const,
        selectedCommitOid: oid,
        selectedCommitFilePath: null,
      };
      patchView({ ...patch, compareMode: null });
      persist(patch);
      updateUrl("push", { view: "log", commit: oid });
      void logHistory.reloadHistory("refresh", oid);
    },
    setCommitMessage(commitMessage: string) {
      patchView({ commitMessage });
      persist({ commitMessage });
    },
    setStatusBanner(statusBanner: string) {
      patchView({ statusBanner });
    },
    setScroll(
      kind: "logList" | "logDiff" | "commitList" | "commitDiff",
      top: number,
    ) {
      scrollOffsets = { ...scrollOffsets, [kind]: top };
      persist({ [`${kind}ScrollTop`]: top });
    },
    stageOrUnstage: staging.stageOrUnstage,
    stageOrUnstageGroup: staging.stageOrUnstageGroup,
    refreshActiveSnapshot: staging.refreshActiveSnapshot,
    checkExternalUpdates: staging.checkExternalUpdates,
    openCommitPreview: staging.openCommitPreview,
    refreshCommitPreview: staging.refreshCommitPreview,
    closeCommitPreview: staging.closeCommitPreview,
    confirmCommit: staging.confirmCommit,
    runFetch: operations.runFetch,
  };
}
