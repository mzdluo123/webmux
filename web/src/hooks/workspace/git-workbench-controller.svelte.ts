import { errorMessage } from "../../platform/error";
import type {
  GitHistoryScope,
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
import {
  filterLogCommits,
  selectLogBranchFilter,
  toggleReviewedPath,
} from "../../domains/workspace/git-workbench-state";
import { LatestRequest } from "../shared/remote-request.svelte";
import { buildGitRouteHref, parseGitRouteQuery } from "../shared/route";
import {
  emptyGitViewState,
  fetchGitCommitDetailMapped,
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

  const targetReq = new LatestRequest();
  const detailReq = new LatestRequest();
  const staging = createGitStagingController({
    getRepoId: () => currentRepoId,
    getWorktreeId: () => currentWorktreeId,
    getView: () => view,
    patchView: (patch) => {
      view = { ...view, ...patch };
    },
    onRefreshRepositories: options.onRefreshRepositories,
  });

  function dispose() {
    targetReq.dispose();
    detailReq.dispose();
    staging.dispose();
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

  async function loadCommitDetail(
    repoId: string,
    oid: string,
    parentOid?: string,
  ) {
    if (!repoId || !oid) return;
    const req = detailReq.begin();
    try {
      const mapped = await fetchGitCommitDetailMapped({
        repositoryId: repoId,
        oid,
        parentOid,
        previous: view.commitDetails[oid],
        signal: req.signal,
      });
      if (!req.current() || repoId !== currentRepoId) return;
      view = {
        ...view,
        commitDetails: { ...view.commitDetails, [oid]: mapped },
      };
    } catch (err) {
      if (req.reportable()) {
        view = {
          ...view,
          statusBanner: `提交详情加载失败：${errorMessage(err)}`,
        };
      }
    }
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
      view = {
        ...view,
        context: deriveGitWorktreeContext({
          repository,
          repositoryId: repoId,
          worktree,
          gitCtx: null,
        }),
      };
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
        signal: req.signal,
      });
      if (
        !req.current() ||
        repoId !== currentRepoId ||
        wtId !== currentWorktreeId
      ) {
        return;
      }
      const visible = filterLogCommits(
        bundle.commits,
        view.branchFilter,
        view.searchQuery,
        view.pathFilter,
      );
      const selectedOid =
        (view.selectedCommitOid &&
        bundle.commits.some((c) => c.oid === view.selectedCommitOid)
          ? view.selectedCommitOid
          : visible[0]?.oid) ?? null;

      view = {
        ...view,
        context: bundle.context,
        historyState: bundle.historyState,
        historyReason: bundle.historyReason,
        commits: bundle.commits,
        branches: bundle.branches,
        selectedCommitOid: selectedOid,
      };
      if (selectedOid && !view.commitDetails[selectedOid]) {
        void loadCommitDetail(
          repoId,
          selectedOid,
          view.selectedParentByCommit[selectedOid],
        );
      }
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
      lastAppliedSearch = params.search;
      lastLoadedKey = desiredLoadKey;
      staging.resetTarget();
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
        lastLoadedKey = desiredLoadKey;
        void loadTarget(
          currentRepoId,
          currentWorktreeId,
          params.repository,
          params.worktree,
          params.sessions,
        );
      } else if (popped.nextCommit && !view.commitDetails[popped.nextCommit]) {
        void loadCommitDetail(currentRepoId, popped.nextCommit);
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
        view = {
          ...view,
          context: deriveGitWorktreeContext({
            repository: params.repository,
            repositoryId: params.repositoryId,
            worktree: params.worktree,
            gitCtx: null,
          }),
        };
      }
    }
  }

  function persist(patch: Parameters<typeof writeGitTargetMemory>[2]) {
    writeGitTargetMemory(currentRepoId, currentWorktreeId, patch);
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
      view = { ...view, mode };
      persist({ mode });
      updateUrl("push", { view: mode });
    },
    setScope(
      scope: GitHistoryScope,
      repo?: Repository,
      wt?: Worktree,
      sessions: readonly SessionInfo[] = [],
    ) {
      if (scope === view.scope) return;
      view = { ...view, scope };
      persist({ scope });
      updateUrl("push", { scope });
      void loadTarget(currentRepoId, currentWorktreeId, repo, wt, sessions);
    },
    setBranchFilter(
      branch: string | null,
      repo?: Repository,
      wt?: Worktree,
      sessions: readonly SessionInfo[] = [],
      switchToLog = false,
    ) {
      const next = selectLogBranchFilter(view, branch);
      const nextMode = switchToLog ? "log" : view.mode;
      view = { ...next, mode: nextMode };
      persist({
        mode: nextMode,
        branchFilter: branch,
        selectedCommitOid: next.selectedCommitOid,
      });
      updateUrl("push", {
        view: nextMode,
        branch: branch ?? undefined,
        commit: next.selectedCommitOid ?? undefined,
      });
      void loadTarget(currentRepoId, currentWorktreeId, repo, wt, sessions);
    },
    setSearchQuery(searchQuery: string) {
      view = { ...view, searchQuery };
      persist({ searchQuery });
      updateUrl("replace", { q: searchQuery || undefined });
    },
    setPathFilter(pathFilter: string) {
      view = { ...view, pathFilter };
      persist({ pathFilter });
    },
    selectCommit(oid: string) {
      view = { ...view, selectedCommitOid: oid, selectedCommitFilePath: null };
      persist({ selectedCommitOid: oid, selectedCommitFilePath: null });
      updateUrl("replace", { commit: oid });
      if (!view.commitDetails[oid]) {
        void loadCommitDetail(
          currentRepoId,
          oid,
          view.selectedParentByCommit[oid],
        );
      }
    },
    selectParent(commitOid: string, parentOid: string) {
      const nextMap = {
        ...view.selectedParentByCommit,
        [commitOid]: parentOid,
      };
      view = {
        ...view,
        selectedParentByCommit: nextMap,
        selectedCommitFilePath: null,
      };
      persist({
        selectedParentByCommit: nextMap,
        selectedCommitFilePath: null,
      });
      void loadCommitDetail(currentRepoId, commitOid, parentOid);
    },
    selectCommitFile(path: string) {
      view = { ...view, selectedCommitFilePath: path };
      persist({ selectedCommitFilePath: path });
    },
    selectChange(changeId: string) {
      view = { ...view, activeChangeId: changeId };
      persist({ activeChangeId: changeId });
      if (currentWorktreeId) {
        void staging.loadActiveDiff(currentRepoId, currentWorktreeId, changeId);
      }
    },
    toggleReviewed(path: string) {
      const reviewedPaths = toggleReviewedPath(view.reviewedPaths, path);
      view = { ...view, reviewedPaths };
      persist({ reviewedPaths });
    },
    setCommitMessage(commitMessage: string) {
      view = { ...view, commitMessage };
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
        view = { ...view, statusBanner: out.banner };
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
        view = { ...view, statusBanner: `获取远程失败：${errorMessage(err)}` };
      } finally {
        fetching = false;
      }
    },
  };
}
