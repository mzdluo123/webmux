import { SvelteSet } from "svelte/reactivity";
import { errorMessage } from "../../platform/error";
import type {
  GitHistoryRow,
  GitHistoryScope,
  GitSelectionNotice,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import {
  readFavoriteBranches,
  toggleFavoriteBranch,
} from "../../domains/workspace/git-favorites";
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitRouteQuery } from "../shared/route";
import type { GitLogDiffController } from "./git-log-diff.svelte";
import {
  fetchGitCommitDetailMapped,
  fetchGitHistoryPageMapped,
} from "./git-workbench-actions";

export interface GitLogHistoryControllerDeps {
  getRepoId: () => string;
  getEffectiveWorktreeId: () => string | undefined;
  getDefaultBranch: () => string | null;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  persist: (patch: Record<string, unknown>) => void;
  updateUrl: (
    mode: "push" | "replace",
    overrides?: Partial<GitRouteQuery>,
  ) => void;
  logDiff: GitLogDiffController;
}

function mergeCommitPages(
  existing: readonly GitHistoryRow[],
  incoming: readonly GitHistoryRow[],
): GitHistoryRow[] {
  const seen = new SvelteSet(existing.map((item) => item.oid));
  const out = [...existing];
  for (const row of incoming) {
    if (seen.has(row.oid)) continue;
    seen.add(row.oid);
    out.push(row);
  }
  return out;
}

export function createGitLogHistoryController(
  deps: GitLogHistoryControllerDeps,
) {
  let detailLoading = $state(false);
  let filterTimer: ReturnType<typeof setTimeout> | null = null;

  const historyReq = new LatestRequest();
  const moreReq = new LatestRequest();
  const detailReq = new LatestRequest();

  function clearFilterTimer() {
    if (filterTimer !== null) {
      clearTimeout(filterTimer);
      filterTimer = null;
    }
  }

  function dispose() {
    clearFilterTimer();
    historyReq.dispose();
    moreReq.dispose();
    detailReq.dispose();
  }

  function resetTarget() {
    clearFilterTimer();
    historyReq.cancel();
    moreReq.cancel();
    detailReq.cancel();
    detailLoading = false;
  }

  async function loadCommitDetail(
    repoId: string,
    oid: string,
    parentOid?: string,
  ) {
    if (!repoId || !oid) return;
    const req = detailReq.begin();
    detailLoading = true;
    try {
      const previous = deps.getView().commitDetails[oid];
      const mapped = await fetchGitCommitDetailMapped({
        repositoryId: repoId,
        oid,
        parentOid,
        previous,
        signal: req.signal,
      });
      if (!req.current() || repoId !== deps.getRepoId()) return;
      const latest = deps.getView();
      deps.patchView({
        commitDetails: { ...latest.commitDetails, [oid]: mapped },
      });
      if (latest.selectedCommitOid === oid && !latest.compareMode) {
        deps.logDiff.syncForCommitDetail(mapped);
      }
    } catch (err) {
      if (req.reportable()) {
        deps.patchView({
          statusBanner: `提交详情加载失败：${errorMessage(err)}`,
        });
      }
    } finally {
      if (req.current()) detailLoading = false;
    }
  }

  function ensureCommitLoaded(oid: string | null) {
    const repoId = deps.getRepoId();
    const view = deps.getView();
    if (!repoId || !oid) {
      if (!view.compareMode) deps.logDiff.clearDiff();
      return;
    }
    const desiredParent = view.selectedParentByCommit[oid];
    const cached = view.commitDetails[oid];
    if (
      cached &&
      (!desiredParent || Boolean(cached.filesByParent[desiredParent]))
    ) {
      if (!view.compareMode) deps.logDiff.syncForCommitDetail(cached);
      return;
    }
    void loadCommitDetail(repoId, oid, desiredParent);
  }

  async function reloadHistory(
    reason: "filter" | "refresh" = "filter",
    preferredSelectedOid?: string | null,
  ) {
    clearFilterTimer();
    moreReq.cancel();
    const repoId = deps.getRepoId();
    if (!repoId) return;

    const viewBefore = deps.getView();
    const prevSelected =
      preferredSelectedOid !== undefined
        ? preferredSelectedOid
        : viewBefore.selectedCommitOid;
    const hadPreviousSelection =
      Boolean(prevSelected) && viewBefore.commits.length > 0;

    const req = historyReq.begin();
    deps.patchView({ historyLoadingMore: false });

    try {
      const hist = await fetchGitHistoryPageMapped({
        repositoryId: repoId,
        worktreeId: deps.getEffectiveWorktreeId(),
        scope: viewBefore.scope,
        branchFilter: viewBefore.branchFilter,
        searchQuery: viewBefore.searchQuery,
        authorFilter: viewBefore.authorFilter,
        sinceFilter: viewBefore.sinceFilter,
        untilFilter: viewBefore.untilFilter,
        pathFilter: viewBefore.pathFilter,
        limit: 100,
        signal: req.signal,
      });
      if (!req.current() || repoId !== deps.getRepoId()) return;

      const latest = deps.getView();
      const retained =
        prevSelected && hist.commits.some((c) => c.oid === prevSelected)
          ? prevSelected
          : null;
      const nextSelected = retained ?? hist.commits[0]?.oid ?? null;

      let selectionNotice: GitSelectionNotice | null = null;
      if (hadPreviousSelection && prevSelected && !retained) {
        selectionNotice = {
          message:
            reason === "filter"
              ? `原选中提交 ${prevSelected.slice(0, 7)} 不在当前筛选结果中。`
              : `原选中提交 ${prevSelected.slice(0, 7)} 已不在最新引用历史首屏（可能因历史改写或筛选变化）。`,
          canJumpHead: Boolean(latest.context.headOid),
        };
      }

      deps.patchView({
        historyState: hist.historyState,
        historyReason: hist.historyReason,
        historyUpstream: hist.historyUpstream,
        ...(hist.taskBaseline !== undefined
          ? { taskBaseline: hist.taskBaseline }
          : {}),
        lastFetchedAt: hist.lastFetchedAt,
        nextCursor: hist.nextCursor,
        refsChanged: hist.refsChanged,
        historyLoadingMore: false,
        commits: hist.commits,
        selectedCommitOid: nextSelected,
        selectionNotice,
      });
      deps.persist({ selectedCommitOid: nextSelected });
      deps.updateUrl("replace", { commit: nextSelected ?? undefined });
      ensureCommitLoaded(nextSelected);
    } catch (err) {
      if (!req.reportable()) return;
      deps.patchView({
        statusBanner: `提交历史加载失败：${errorMessage(err)}`,
      });
    }
  }

  function scheduleFilterReload(delayMs = 200) {
    clearFilterTimer();
    filterTimer = setTimeout(() => {
      filterTimer = null;
      void reloadHistory("filter");
    }, delayMs);
  }

  async function loadMoreHistory() {
    const repoId = deps.getRepoId();
    const view = deps.getView();
    if (!repoId || !view.nextCursor || view.historyLoadingMore) return;

    const cursor = view.nextCursor;
    const req = moreReq.begin();
    deps.patchView({ historyLoadingMore: true });
    try {
      const page = await fetchGitHistoryPageMapped({
        repositoryId: repoId,
        worktreeId: deps.getEffectiveWorktreeId(),
        scope: view.scope,
        branchFilter: view.branchFilter,
        searchQuery: view.searchQuery,
        authorFilter: view.authorFilter,
        sinceFilter: view.sinceFilter,
        untilFilter: view.untilFilter,
        pathFilter: view.pathFilter,
        cursor,
        limit: 100,
        signal: req.signal,
      });
      if (!req.current() || repoId !== deps.getRepoId()) return;
      const latest = deps.getView();
      deps.patchView({
        commits: mergeCommitPages(latest.commits, page.commits),
        nextCursor: page.nextCursor,
        refsChanged: Boolean(latest.refsChanged || page.refsChanged),
        historyLoadingMore: false,
      });
    } catch (err) {
      if (!req.reportable()) return;
      deps.patchView({
        historyLoadingMore: false,
        statusBanner: `加载更多提交失败：${errorMessage(err)}`,
      });
    }
  }

  function jumpToHead() {
    const view = deps.getView();
    const headOid = view.context.headOid;
    if (!headOid) return;

    const alreadyVisible = view.commits.some((row) => row.oid === headOid);
    if (alreadyVisible) {
      deps.patchView({
        selectedCommitOid: headOid,
        selectedCommitFilePath: null,
        selectionNotice: null,
      });
      deps.persist({
        selectedCommitOid: headOid,
        selectedCommitFilePath: null,
      });
      deps.updateUrl("replace", { commit: headOid });
      ensureCommitLoaded(headOid);
      return;
    }

    deps.patchView({
      scope: "all",
      branchFilter: null,
      searchQuery: "",
      authorFilter: "",
      sinceFilter: "",
      untilFilter: "",
      pathFilter: "",
      selectedCommitOid: headOid,
      selectedCommitFilePath: null,
      selectionNotice: null,
    });
    deps.persist({
      scope: "all",
      branchFilter: null,
      searchQuery: "",
      authorFilter: "",
      sinceFilter: "",
      untilFilter: "",
      pathFilter: "",
      selectedCommitOid: headOid,
      selectedCommitFilePath: null,
    });
    deps.updateUrl("push", {
      scope: "all",
      branch: undefined,
      q: undefined,
      author: undefined,
      since: undefined,
      until: undefined,
      path: undefined,
      commit: headOid,
    });
    void reloadHistory("refresh", headOid);
  }

  return {
    get detailLoading() {
      return detailLoading;
    },
    dispose,
    resetTarget,
    loadCommitDetail,
    ensureCommitLoaded,
    reloadHistory,
    scheduleFilterReload,
    loadMoreHistory,
    jumpToHead,
    setScope: (scope: GitHistoryScope) => {
      const view = deps.getView();
      if (scope === view.scope) return;
      deps.patchView({ scope, selectionNotice: null });
      deps.persist({ scope });
      deps.updateUrl("push", { scope });
      void reloadHistory("filter");
    },
    setBranchFilter: (
      branch: string | null,
      _repo?: unknown,
      _wt?: unknown,
      _sessions?: unknown,
      switchToLog = false,
    ) => {
      const view = deps.getView();
      const nextMode = switchToLog ? "log" : view.mode;
      if (branch === view.branchFilter && nextMode === view.mode) return;
      deps.patchView({
        mode: nextMode,
        branchFilter: branch,
        selectionNotice: null,
      });
      deps.persist({ mode: nextMode, branchFilter: branch });
      deps.updateUrl("push", { view: nextMode, branch: branch ?? undefined });
      void reloadHistory("filter");
    },
    setSearchQuery: (searchQuery: string) => {
      deps.patchView({ searchQuery, selectionNotice: null });
      deps.persist({ searchQuery });
      deps.updateUrl("replace", { q: searchQuery || undefined });
      scheduleFilterReload();
    },
    setAuthorFilter: (authorFilter: string) => {
      deps.patchView({ authorFilter, selectionNotice: null });
      deps.persist({ authorFilter });
      deps.updateUrl("replace", { author: authorFilter || undefined });
      scheduleFilterReload();
    },
    setSinceFilter: (sinceFilter: string) => {
      deps.patchView({ sinceFilter, selectionNotice: null });
      deps.persist({ sinceFilter });
      deps.updateUrl("replace", { since: sinceFilter || undefined });
      void reloadHistory("filter");
    },
    setUntilFilter: (untilFilter: string) => {
      deps.patchView({ untilFilter, selectionNotice: null });
      deps.persist({ untilFilter });
      deps.updateUrl("replace", { until: untilFilter || undefined });
      void reloadHistory("filter");
    },
    setPathFilter: (pathFilter: string) => {
      deps.patchView({ pathFilter, selectionNotice: null });
      deps.persist({ pathFilter });
      deps.updateUrl("replace", { path: pathFilter || undefined });
      scheduleFilterReload();
    },
    clearLogFilters: () => {
      const reset = {
        scope: "all" as const,
        branchFilter: null,
        searchQuery: "",
        authorFilter: "",
        sinceFilter: "",
        untilFilter: "",
        pathFilter: "",
      };
      deps.patchView({ ...reset, selectionNotice: null });
      deps.persist(reset);
      deps.updateUrl("push", {
        scope: "all",
        branch: undefined,
        q: undefined,
        author: undefined,
        since: undefined,
        until: undefined,
        path: undefined,
      });
      void reloadHistory("filter");
    },
    toggleFavorite: (branchName: string) => {
      const repoId = deps.getRepoId();
      if (!repoId || !branchName) return;
      const view = deps.getView();
      const localSet = new SvelteSet(
        view.branches.filter((b) => b.kind === "local").map((b) => b.name),
      );
      const currentFavs = readFavoriteBranches(
        repoId,
        deps.getDefaultBranch(),
        localSet,
      );
      const nextFavs = toggleFavoriteBranch(repoId, branchName, currentFavs);
      deps.patchView({
        branches: view.branches.map((item) =>
          item.kind === "tag"
            ? item
            : { ...item, favorite: nextFavs.includes(item.name) },
        ),
      });
    },
  };
}
