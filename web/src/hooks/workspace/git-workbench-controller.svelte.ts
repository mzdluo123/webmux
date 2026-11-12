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
import { LatestRequest } from "../shared/remote-request.svelte";
import { buildGitRouteHref, parseGitRouteQuery } from "../shared/route";
import {
  emptyGitViewState,
  fetchGitTargetBundle,
} from "./git-workbench-actions";
import {
  applyPopstateQueryToView,
  buildBundleViewPatch,
  buildTargetInitialState,
  computeGitTargetLoadKey,
  syncGitWorkbenchUrl,
} from "./git-workbench-route-sync";
import { createGitWorkbenchSubControllers } from "./git-workbench-subcontrollers.svelte";

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

  const getWorktreeTargetId = () => effectiveWorktreeId ?? currentWorktreeId;
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

  function selectChange(changeId: string) {
    patchView({ activeChangeId: changeId });
    persist({ activeChangeId: changeId });
    const wtId = getWorktreeTargetId();
    if (wtId) void sub.staging.loadActiveDiff(currentRepoId, wtId, changeId);
  }

  const targetReq = new LatestRequest();
  const sub = createGitWorkbenchSubControllers({
    getRepoId: () => currentRepoId,
    getEffectiveWorktreeId: () => effectiveWorktreeId,
    getWorktreeTargetId,
    getDefaultBranch: () => defaultBranchName,
    getView: () => view,
    patchView,
    persist,
    updateUrl,
    onSelectChange: selectChange,
    onReloadTarget: (repo, wt, sessions = []) =>
      loadTarget(currentRepoId, currentWorktreeId, repo, wt, sessions),
    onRepositoryUpdated: options.onRepositoryUpdated,
    onRefreshRepositories: options.onRefreshRepositories,
  });

  const patchUnavailableContext = (
    repoId: string,
    repository?: Repository,
    worktree?: Worktree,
  ) =>
    patchView({
      context: deriveGitWorktreeContext({
        repository,
        repositoryId: repoId,
        worktree,
        gitCtx: null,
      }),
    });

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
      patchUnavailableContext(repoId, repository, worktree);
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
      const effId = bundle.effectiveWorktreeId;
      effectiveWorktreeId = effId;
      defaultBranchName = bundle.defaultBranch;
      sub.rework.loadTarget(repoId, effId ?? wtId);
      sub.logDiff.seedContextId(repoId, effId, bundle.gitContextId);
      sub.staging.seedContextId(repoId, effId, bundle.gitContextId);
      sub.taskReview.seedContextId(repoId, effId, bundle.gitContextId);

      const { selectedOid, patch } = buildBundleViewPatch(
        view.selectedCommitOid,
        bundle,
      );
      patchView(patch);
      sub.logHistory.ensureCommitLoaded(selectedOid);
      if (effId) {
        await Promise.all([
          sub.staging.refresh(repoId, effId, req.signal),
          sub.taskReview.refresh(repoId, effId, req.signal),
          sub.commitDraft.load(repoId, effId, req.signal),
        ]);
      }
      await sub.operations.refreshOperations(repoId, effId ?? wtId, req.signal);
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
      sub.resetAllTargets(params.repositoryId, params.worktreeId);
      error = "";

      const init = buildTargetInitialState({
        repositoryId: params.repositoryId,
        worktreeId: params.worktreeId,
        query,
        repository: params.repository,
        worktree: params.worktree,
      });
      scrollOffsets = init.scrollOffsets;
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
        void sub.logHistory.reloadHistory("filter", popped.nextCommit);
      } else {
        sub.logHistory.ensureCommitLoaded(popped.nextCommit);
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
        patchUnavailableContext(
          params.repositoryId,
          params.repository,
          params.worktree,
        );
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
      return sub.operations.fetching;
    },
    get stagingBusy() {
      return sub.staging.stagingBusy;
    },
    get detailLoading() {
      return sub.logHistory.detailLoading;
    },
    get returnSessionId() {
      return returnSessionId;
    },
    get scrollOffsets() {
      return scrollOffsets;
    },
    commitDraft: sub.commitDraft,
    reviews: sub.reviews,
    rework: sub.rework,
    operations: sub.operations,
    staging: sub.staging,
    taskReview: sub.taskReview,
    push: sub.push,
    logDiff: sub.logDiff,
    dispose() {
      targetReq.dispose();
      sub.disposeAll();
    },
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
    setScope: sub.logHistory.setScope,
    setBranchFilter: sub.logHistory.setBranchFilter,
    setSearchQuery: sub.logHistory.setSearchQuery,
    setAuthorFilter: sub.logHistory.setAuthorFilter,
    setSinceFilter: sub.logHistory.setSinceFilter,
    setUntilFilter: sub.logHistory.setUntilFilter,
    setPathFilter: sub.logHistory.setPathFilter,
    clearLogFilters: sub.logHistory.clearLogFilters,
    refreshHistory: () => sub.logHistory.reloadHistory("refresh"),
    loadMoreHistory: sub.logHistory.loadMoreHistory,
    jumpToHead: sub.logHistory.jumpToHead,
    dismissSelectionNotice: () => patchView({ selectionNotice: null }),
    toggleFavorite: sub.logHistory.toggleFavorite,
    selectCommit(oid: string) {
      sub.logCompare.resetTarget();
      patchView({
        selectedCommitOid: oid,
        selectedCommitFilePath: null,
        selectionNotice: null,
        compareMode: null,
      });
      persist({ selectedCommitOid: oid, selectedCommitFilePath: null });
      updateUrl("replace", { commit: oid });
      sub.logHistory.ensureCommitLoaded(oid);
    },
    selectParent(commitOid: string, parentOid: string) {
      const selectedParentByCommit = {
        ...view.selectedParentByCommit,
        [commitOid]: parentOid,
      };
      const patch = { selectedParentByCommit, selectedCommitFilePath: null };
      patchView(patch);
      persist(patch);
      void sub.logHistory.loadCommitDetail(currentRepoId, commitOid, parentOid);
    },
    selectCommitFile: sub.logDiff.selectActiveFile,
    markCompareBase: sub.logCompare.markCompareBase,
    startCompare: sub.logCompare.startCompare,
    loadMoreCompareFiles: sub.logCompare.loadMoreCompareFiles,
    swapCompareDirection: sub.logCompare.swapCompareDirection,
    exitCompare: sub.logCompare.exitCompare,
    selectChange,
    toggleReviewed(change: GitWorkingChange | string) {
      if (typeof change === "string") {
        void sub.reviews.toggleTaskFileReview(change);
        return;
      }
      void sub.reviews.toggleUncommittedReview(change);
    },
    toggleTaskReviewed: sub.reviews.toggleTaskFileReview,
    jumpNextUnreviewed: sub.reviews.jumpToNextUnreviewed,
    dismissLastCommitOutcome() {
      sub.operations.acknowledgeAttention(view.lastCommitOutcome?.operationId);
      patchView({ lastCommitOutcome: null });
    },
    async reloadAndVerifyUncertain(
      repo?: Repository,
      wt?: Worktree,
      sessions: readonly SessionInfo[] = [],
    ) {
      sub.operations.acknowledgeAttention();
      patchView({ lastCommitOutcome: null });
      await loadTarget(currentRepoId, currentWorktreeId, repo, wt, sessions);
    },
    inspectCommitInLog(oid: string) {
      sub.logCompare.resetTarget();
      const patch = {
        mode: "log" as const,
        selectedCommitOid: oid,
        selectedCommitFilePath: null,
      };
      patchView({ ...patch, compareMode: null });
      persist(patch);
      updateUrl("push", { view: "log", commit: oid });
      void sub.logHistory.reloadHistory("refresh", oid);
    },
    setCommitMessage: sub.commitDraft.updateLocalMessage,
    setStatusBanner: (statusBanner: string) => patchView({ statusBanner }),
    setScroll(
      kind: "logList" | "logDiff" | "commitList" | "commitDiff",
      top: number,
    ) {
      scrollOffsets = { ...scrollOffsets, [kind]: top };
      persist({ [`${kind}ScrollTop`]: top });
    },
    stageOrUnstage: sub.staging.stageOrUnstage,
    stageOrUnstageGroup: sub.staging.stageOrUnstageGroup,
    refreshActiveSnapshot: () => {
      void sub.staging.refreshActiveSnapshot();
      void sub.taskReview.refresh();
    },
    checkExternalUpdates: () => {
      void sub.staging.checkExternalUpdates();
      void sub.taskReview.refresh();
      void sub.commitDraft.load();
    },
    openCommitPreview: sub.staging.openCommitPreview,
    refreshCommitPreview: sub.staging.refreshCommitPreview,
    closeCommitPreview: sub.staging.closeCommitPreview,
    confirmCommit: sub.staging.confirmCommit,
    runFetch: sub.operations.runFetch,
  };
}
