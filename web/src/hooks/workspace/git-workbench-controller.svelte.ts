import { errorMessage } from "../../platform/error";
import type {
  GitWorkbenchMode,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import type {
  Repository,
  SessionInfo,
  Worktree,
} from "../../contracts/schemas";
import { deriveGitWorktreeContext } from "../../domains/workspace/git-workbench-adapter";
import { writeGitTargetMemory } from "../../domains/workspace/git-workbench-memory";
import { toggleReviewedPath } from "../../domains/workspace/git-workbench-state";
import { LatestRequest } from "../shared/remote-request.svelte";
import { buildGitRouteHref, parseGitRouteQuery } from "../shared/route";
import { createGitLogCompareController } from "./git-log-compare.svelte";
import { createGitLogDiffController } from "./git-log-diff.svelte";
import { createGitLogHistoryController } from "./git-log-history.svelte";
import {
  emptyGitViewState,
  fetchGitTargetBundle,
  performRepositoryFetch,
} from "./git-workbench-actions";
import {
  applyPopstateQueryToView,
  buildTargetInitialState,
  computeGitTargetLoadKey,
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
  let fetching = $state(false);
  let scrollOffsets = $state({
    logList: 0,
    logDiff: 0,
    commitList: 0,
    commitDiff: 0,
  });

  const patchView = (patch: Partial<GitWorkbenchViewState>) => {
    view = { ...view, ...patch };
  };

  function persist(patch: Parameters<typeof writeGitTargetMemory>[2]) {
    writeGitTargetMemory(currentRepoId, currentWorktreeId, patch);
  }

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
  const staging = createGitStagingController({
    getRepoId: () => currentRepoId,
    getWorktreeId: () => effectiveWorktreeId ?? currentWorktreeId,
    getView: () => view,
    patchView,
    onRefreshRepositories: options.onRefreshRepositories,
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

  function dispose() {
    targetReq.dispose();
    staging.dispose();
    logHistory.dispose();
    logCompare.dispose();
    logDiff.dispose();
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
      logDiff.seedContextId(
        repoId,
        bundle.effectiveWorktreeId,
        bundle.gitContextId,
      );

      const headCandidate = bundle.context.headOid ?? null;
      const selectedOid =
        (view.selectedCommitOid &&
        bundle.commits.some((c) => c.oid === view.selectedCommitOid)
          ? view.selectedCommitOid
          : headCandidate && bundle.commits.some((c) => c.oid === headCandidate)
            ? headCandidate
            : bundle.commits[0]?.oid) ?? null;

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
      if (bundle.effectiveWorktreeId) {
        await staging.refresh(repoId, bundle.effectiveWorktreeId, req.signal);
      }
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
      return fetching;
    },
    get stagingBusy() {
      return staging.stagingBusy;
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
      const nextMap = {
        ...view.selectedParentByCommit,
        [commitOid]: parentOid,
      };
      patchView({
        selectedParentByCommit: nextMap,
        selectedCommitFilePath: null,
      });
      persist({
        selectedParentByCommit: nextMap,
        selectedCommitFilePath: null,
      });
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
    toggleReviewed(path: string) {
      const reviewedPaths = toggleReviewedPath(view.reviewedPaths, path);
      patchView({ reviewedPaths });
      persist({ reviewedPaths });
    },
    setCommitMessage(commitMessage: string) {
      patchView({ commitMessage });
      persist({ commitMessage });
    },
    setScroll(
      kind: "logList" | "logDiff" | "commitList" | "commitDiff",
      top: number,
    ) {
      scrollOffsets = { ...scrollOffsets, [kind]: top };
      persist({ [`${kind}ScrollTop`]: top });
    },
    stageOrUnstage: staging.stageOrUnstage,
    openCommitPreview: staging.openCommitPreview,
    closeCommitPreview: staging.closeCommitPreview,
    confirmCommit: staging.confirmCommit,
    async runFetch(wt?: Worktree, sessions: readonly SessionInfo[] = []) {
      if (!currentRepoId || fetching) return;
      fetching = true;
      try {
        const out = await performRepositoryFetch(currentRepoId);
        options.onRepositoryUpdated?.(out.repository);
        patchView({ statusBanner: out.banner });
        const nextTree = currentWorktreeId
          ? out.repository.worktrees.find((w) => w.id === currentWorktreeId)
          : wt;
        await loadTarget(
          currentRepoId,
          currentWorktreeId,
          out.repository,
          nextTree,
          sessions,
        );
      } catch (err) {
        patchView({ statusBanner: `获取远程失败：${errorMessage(err)}` });
      } finally {
        fetching = false;
      }
    },
  };
}
