import { navigate, replacePath } from "../../platform/api";
import type {
  GitHistoryRow,
  GitHistoryScope,
  GitWorkbenchMode,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import type { Repository, Worktree } from "../../contracts/schemas";
import { deriveGitWorktreeContext } from "../../domains/workspace/git-workbench-adapter";
import {
  applyGitQueryOverrides,
  readGitTargetMemory,
  writeGitTargetMemory,
  type GitTargetMemory,
} from "../../domains/workspace/git-workbench-memory";
import { buildGitRouteHref, type GitRouteQuery } from "../shared/route";
import {
  emptyGitViewState,
  type fetchGitTargetBundle,
} from "./git-workbench-actions";

export function syncGitWorkbenchUrl(
  mode: "push" | "replace",
  state: {
    repositoryId: string;
    worktreeId?: string;
    view: GitWorkbenchViewState;
    returnSession?: string;
  },
  overrides: Partial<Parameters<typeof buildGitRouteHref>[0]> = {},
): string {
  const href = buildGitRouteHref({
    repositoryId: state.repositoryId,
    worktreeId: state.worktreeId,
    view: state.view.mode,
    scope: state.view.scope,
    branch: state.view.branchFilter ?? undefined,
    commit: state.view.selectedCommitOid ?? undefined,
    q: state.view.searchQuery || undefined,
    author: state.view.authorFilter || undefined,
    since: state.view.sinceFilter || undefined,
    until: state.view.untilFilter || undefined,
    path: state.view.pathFilter || undefined,
    returnSession: state.returnSession,
    ...overrides,
  });
  const parsed = new URL(href, location.origin);
  if (
    location.pathname !== parsed.pathname ||
    location.search !== parsed.search
  ) {
    if (mode === "push") navigate(href);
    else replacePath(href);
  }
  return parsed.search;
}

export function buildTargetInitialState(params: {
  repositoryId: string;
  worktreeId?: string;
  query: GitRouteQuery;
  repository: Repository | undefined;
  worktree: Worktree | undefined;
}): {
  memory: GitTargetMemory;
  scrollOffsets: {
    logList: number;
    logDiff: number;
    commitList: number;
    commitDiff: number;
  };
  view: GitWorkbenchViewState;
} {
  const memory = applyGitQueryOverrides(
    readGitTargetMemory(params.repositoryId, params.worktreeId),
    params.query,
  );
  writeGitTargetMemory(params.repositoryId, params.worktreeId, memory);

  const view: GitWorkbenchViewState = {
    ...emptyGitViewState(params.repositoryId),
    mode: memory.mode,
    scope: memory.scope,
    reviewScope: memory.reviewScope,
    branchFilter: memory.branchFilter,
    searchQuery: memory.searchQuery,
    authorFilter: memory.authorFilter,
    sinceFilter: memory.sinceFilter,
    untilFilter: memory.untilFilter,
    pathFilter: memory.pathFilter,
    selectedCommitOid: memory.selectedCommitOid,
    selectedParentByCommit: memory.selectedParentByCommit,
    selectedCommitFilePath: memory.selectedCommitFilePath,
    activeChangeId: memory.activeChangeId,
    reviewedPaths: memory.reviewedPaths,
    commitMessage: memory.commitMessage,
    context: deriveGitWorktreeContext({
      repository: params.repository,
      repositoryId: params.repositoryId,
      worktree: params.worktree,
      gitCtx: null,
    }),
  };
  return {
    memory,
    scrollOffsets: {
      logList: memory.logListScrollTop,
      logDiff: memory.logDiffScrollTop,
      commitList: memory.commitListScrollTop,
      commitDiff: memory.commitDiffScrollTop,
    },
    view,
  };
}

export function applyPopstateQueryToView(
  current: GitWorkbenchViewState,
  repositoryId: string,
  worktreeId: string | undefined,
  query: GitRouteQuery,
): {
  nextView: GitWorkbenchViewState;
  reloadNeeded: boolean;
  nextCommit: string | null;
} {
  const nextMode: GitWorkbenchMode = query.view ?? "log";
  const nextScope: GitHistoryScope = query.scope ?? "all";
  const nextBranch = query.branch ?? null;
  const nextSearch = query.q ?? "";
  const nextAuthor = query.author ?? "";
  const nextSince = query.since ?? "";
  const nextUntil = query.until ?? "";
  const nextPath = query.path ?? "";
  const nextCommit = query.commit ?? current.selectedCommitOid;
  const reloadNeeded =
    nextScope !== current.scope ||
    nextBranch !== current.branchFilter ||
    nextSearch !== current.searchQuery ||
    nextAuthor !== (current.authorFilter ?? "") ||
    nextSince !== (current.sinceFilter ?? "") ||
    nextUntil !== (current.untilFilter ?? "") ||
    nextPath !== current.pathFilter;

  const nextView: GitWorkbenchViewState = {
    ...current,
    mode: nextMode,
    scope: nextScope,
    branchFilter: nextBranch,
    searchQuery: nextSearch,
    authorFilter: nextAuthor,
    sinceFilter: nextSince,
    untilFilter: nextUntil,
    pathFilter: nextPath,
    selectedCommitOid: nextCommit,
  };
  writeGitTargetMemory(repositoryId, worktreeId, {
    mode: nextMode,
    scope: nextScope,
    branchFilter: nextBranch,
    searchQuery: nextSearch,
    authorFilter: nextAuthor,
    sinceFilter: nextSince,
    untilFilter: nextUntil,
    pathFilter: nextPath,
    selectedCommitOid: nextCommit,
  });
  return { nextView, reloadNeeded, nextCommit };
}

function pickSelectedCommitAfterBundle(
  currentSelected: string | null,
  headOid: string | null | undefined,
  commits: readonly GitHistoryRow[],
): string | null {
  const hasOid = (oid: string | null | undefined) =>
    Boolean(oid && commits.some((c) => c.oid === oid));
  if (hasOid(currentSelected)) return currentSelected;
  if (hasOid(headOid)) return headOid ?? null;
  return commits[0]?.oid ?? null;
}

export function buildBundleViewPatch(
  currentSelectedCommitOid: string | null,
  bundle: Awaited<ReturnType<typeof fetchGitTargetBundle>>,
): {
  selectedOid: string | null;
  patch: Partial<GitWorkbenchViewState>;
} {
  const selectedOid = pickSelectedCommitAfterBundle(
    currentSelectedCommitOid,
    bundle.context.headOid,
    bundle.commits,
  );
  return {
    selectedOid,
    patch: {
      context: bundle.context,
      historyState: bundle.historyState,
      historyReason: bundle.historyReason,
      historyUpstream: bundle.historyUpstream,
      ...(bundle.taskBaseline !== undefined
        ? { taskBaseline: bundle.taskBaseline }
        : {}),
      lastFetchedAt: bundle.lastFetchedAt,
      nextCursor: bundle.nextCursor,
      refsChanged: bundle.refsChanged,
      historyLoadingMore: false,
      commits: bundle.commits,
      branches: bundle.branches,
      selectedCommitOid: selectedOid,
    },
  };
}

export function computeGitTargetLoadKey(
  repositoryId: string,
  worktreeId: string | undefined,
  repository: Repository | undefined,
  worktree: Worktree | undefined,
): string {
  if (!repositoryId || repository?.availability !== "available") return "";
  const usableWt =
    worktreeId &&
    worktree &&
    worktree.available &&
    !worktree.prunable &&
    !worktree.locked &&
    worktree.kind !== "bare"
      ? worktree
      : undefined;
  return `${repositoryId}:${usableWt?.id ?? ""}:${usableWt?.branch ?? ""}:${usableWt?.detached ? "1" : "0"}`;
}
