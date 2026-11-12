import { fetchRepositoryRemotes, repositoryBranches } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import {
  gitCommit,
  gitCommitDetail,
  gitCommitPreview,
  gitConflictReason,
  gitContext,
  gitHistory,
  gitStage,
  gitStaging,
  gitStagingDiff,
  gitUnstage,
} from "../../platform/git-api";
import type {
  GitBranchItem,
  GitCommitDetail,
  GitCommitOutcome,
  GitCommitPreviewModel,
  GitHistoryRow,
  GitHistoryScope,
  GitWorkbenchViewState,
  GitWorkingChange,
  GitWorktreeContext,
} from "../../contracts/git-workbench-schemas";
import type {
  GitStagingEntry,
  GitStagingFileDiff,
  Repository,
  SessionInfo,
  Worktree,
} from "../../contracts/schemas";
import {
  deriveGitWorktreeContext,
  mapGitBranchesResponse,
  mapGitCommitDetailResponse,
  mapGitCommitPreviewResponse,
  mapGitCommitResultResponse,
  mapGitHistoryPage,
  mapGitStagingOverview,
} from "../../domains/workspace/git-workbench-adapter";
import { readFavoriteBranches } from "../../domains/workspace/git-favorites";
import { groupWorkingChanges } from "../../domains/workspace/git-workbench-state";

function resolveUsableWorktree(
  worktreeId: string | undefined,
  worktree: Worktree | undefined,
): Worktree | undefined {
  return worktreeId &&
    worktree &&
    worktree.available &&
    !worktree.prunable &&
    !worktree.locked &&
    worktree.kind !== "bare"
    ? worktree
    : undefined;
}

export function emptyGitViewState(repositoryId: string): GitWorkbenchViewState {
  return {
    mode: "log",
    context: deriveGitWorktreeContext({
      repository: undefined,
      repositoryId,
      worktree: undefined,
      gitCtx: null,
    }),
    historyState: "empty",
    historyReason: "",
    nextCursor: "",
    refsChanged: false,
    historyLoadingMore: false,
    scope: "all",
    branchFilter: null,
    searchQuery: "",
    authorFilter: "",
    sinceFilter: "",
    untilFilter: "",
    pathFilter: "",
    commits: [],
    commitDetails: {},
    selectedCommitOid: null,
    selectedParentByCommit: {},
    selectedCommitFilePath: null,
    selectionNotice: null,
    compareBaseMarkOid: null,
    compareMode: null,
    workingChanges: [],
    activeChangeId: null,
    reviewedPaths: [],
    commitMessage: "",
    branches: [],
    commitPreview: null,
    lastCommitOutcome: null,
    pushPreview: null,
    statusBanner: "",
  };
}

export async function fetchGitHistoryPageMapped(options: {
  repositoryId: string;
  worktreeId?: string;
  scope: GitHistoryScope;
  branchFilter: string | null;
  searchQuery?: string;
  authorFilter?: string;
  sinceFilter?: string;
  untilFilter?: string;
  pathFilter?: string;
  cursor?: string;
  limit?: number;
  signal: AbortSignal;
}) {
  const page = await gitHistory(
    options.repositoryId,
    {
      worktreeId: options.worktreeId,
      scope: options.scope,
      branch: options.branchFilter ?? undefined,
      q: options.searchQuery?.trim() || undefined,
      author: options.authorFilter?.trim() || undefined,
      since: options.sinceFilter?.trim() || undefined,
      until: options.untilFilter?.trim() || undefined,
      path: options.pathFilter?.trim() || undefined,
      cursor: options.cursor || undefined,
      limit: options.limit ?? 100,
    },
    options.signal,
  );
  return mapGitHistoryPage(page);
}

export async function fetchGitTargetBundle(options: {
  repositoryId: string;
  worktreeId?: string;
  repository: Repository | undefined;
  worktree: Worktree | undefined;
  sessions: readonly SessionInfo[];
  scope: GitHistoryScope;
  branchFilter: string | null;
  searchQuery?: string;
  authorFilter?: string;
  sinceFilter?: string;
  untilFilter?: string;
  pathFilter?: string;
  signal: AbortSignal;
}): Promise<{
  effectiveWorktreeId?: string;
  gitContextId: string;
  defaultBranch: string | null;
  context: GitWorktreeContext;
  historyState: GitWorkbenchViewState["historyState"];
  historyReason: string;
  historyUpstream?: string;
  lastFetchedAt?: string;
  nextCursor: string;
  refsChanged: boolean;
  commits: GitHistoryRow[];
  branches: GitBranchItem[];
}> {
  const validWt = resolveUsableWorktree(options.worktreeId, options.worktree);
  const effectiveWorktreeId = validWt?.id;

  const [ctxRes, hist, branchesRes] = await Promise.all([
    gitContext(options.repositoryId, effectiveWorktreeId, options.signal),
    fetchGitHistoryPageMapped({
      repositoryId: options.repositoryId,
      worktreeId: effectiveWorktreeId,
      scope: options.scope,
      branchFilter: options.branchFilter,
      searchQuery: options.searchQuery,
      authorFilter: options.authorFilter,
      sinceFilter: options.sinceFilter,
      untilFilter: options.untilFilter,
      pathFilter: options.pathFilter,
      limit: 100,
      signal: options.signal,
    }),
    repositoryBranches(options.repositoryId, options.signal).catch(() => ({
      defaultBranch: null,
      defaultBase: null,
      defaultBaseSubject: "",
      defaultParent: "",
      branches: [],
      bases: [],
      tags: [],
    })),
  ]);

  const context = deriveGitWorktreeContext({
    repository: options.repository,
    repositoryId: options.repositoryId,
    worktree: validWt ?? options.worktree,
    gitCtx: ctxRes,
  });
  const localNames = new Set(branchesRes.branches.map((b) => b.name));
  const favoriteNames = readFavoriteBranches(
    options.repositoryId,
    branchesRes.defaultBranch,
    localNames,
  );
  const branches = mapGitBranchesResponse(
    options.repository,
    context.worktreePath,
    options.sessions,
    branchesRes,
    favoriteNames,
  );

  return {
    effectiveWorktreeId,
    gitContextId: ctxRes.contextId,
    defaultBranch: branchesRes.defaultBranch,
    context,
    historyState: hist.historyState,
    historyReason: hist.historyReason,
    historyUpstream: hist.historyUpstream,
    lastFetchedAt: hist.lastFetchedAt,
    nextCursor: hist.nextCursor,
    refsChanged: hist.refsChanged,
    commits: hist.commits,
    branches,
  };
}

export async function fetchGitCommitDetailMapped(options: {
  repositoryId: string;
  oid: string;
  parentOid?: string;
  previous?: GitCommitDetail;
  signal: AbortSignal;
}): Promise<GitCommitDetail> {
  const res = await gitCommitDetail(
    options.repositoryId,
    options.oid,
    options.parentOid,
    options.signal,
  );
  return mapGitCommitDetailResponse(res, options.previous);
}

export async function fetchGitStagingState(options: {
  repositoryId: string;
  worktreeId: string;
  diffCache: ReadonlyMap<string, GitStagingFileDiff>;
  webmuxStagedPaths?: ReadonlySet<string>;
  signal?: AbortSignal;
}): Promise<{
  entryById: Map<string, GitStagingEntry>;
  changes: GitWorkingChange[];
}> {
  const overview = await gitStaging(
    options.repositoryId,
    options.worktreeId,
    options.signal,
  );
  const mapped = mapGitStagingOverview(
    overview,
    options.diffCache,
    options.webmuxStagedPaths,
  );
  return {
    entryById: new Map(mapped.map((m) => [m.change.id, m.entry])),
    changes: mapped.map((m) => m.change),
  };
}

export async function fetchGitStagingDiffForEntry(options: {
  repositoryId: string;
  worktreeId: string;
  entry: GitStagingEntry;
  signal: AbortSignal;
}): Promise<{ diffKey: string; diff: GitStagingFileDiff } | null> {
  if (options.entry.area !== "staged" && options.entry.area !== "unstaged") {
    return null;
  }
  const diffKey = `${options.entry.area}:${options.entry.path}`;
  const diff = await gitStagingDiff(
    options.repositoryId,
    options.worktreeId,
    {
      area: options.entry.area,
      path: options.entry.path,
      oldPath: options.entry.oldPath,
    },
    options.signal,
  );
  return { diffKey, diff };
}

export async function performStageMutation(options: {
  kind: "stage" | "unstage";
  repositoryId: string;
  worktreeId: string;
  entry: GitStagingEntry;
  hunkId?: string;
  signal: AbortSignal;
}): Promise<
  | { ok: true }
  | {
      ok: false;
      reason?: string;
      busyOutcome?: GitCommitOutcome;
      banner?: string;
    }
> {
  try {
    const payload = {
      identity: options.entry.identity,
      ...(options.hunkId ? { hunkIds: [options.hunkId] } : {}),
    };
    if (options.kind === "stage") {
      await gitStage(
        options.repositoryId,
        options.worktreeId,
        payload,
        options.signal,
      );
    } else {
      await gitUnstage(
        options.repositoryId,
        options.worktreeId,
        payload,
        options.signal,
      );
    }
    return { ok: true };
  } catch (err) {
    const reason = gitConflictReason(err);
    if (reason === "busy") {
      return {
        ok: false,
        reason,
        busyOutcome: {
          status: "busy_locked",
          summary: "检测到外部 Git index.lock，暂存操作已安全中止。",
          reasons: [errorMessage(err)],
        },
      };
    }
    return { ok: false, reason, banner: errorMessage(err) };
  }
}

export async function requestCommitPreviewModel(options: {
  repositoryId: string;
  worktreeId: string;
  repositoryName: string;
  workingChanges: readonly GitWorkingChange[];
  followUpPush: boolean;
  signal: AbortSignal;
}): Promise<{
  confirmationId: string;
  authorName: string;
  authorEmail: string;
  committerName: string;
  committerEmail: string;
  signEnabled: boolean;
  signFormat?: string;
  operation?: string;
  executableHooks: string[];
  model: GitCommitPreviewModel;
}> {
  const preview = await gitCommitPreview(
    options.repositoryId,
    options.worktreeId,
    options.signal,
  );
  const grouped = groupWorkingChanges(options.workingChanges);
  const model = mapGitCommitPreviewResponse(
    options.repositoryName,
    preview,
    grouped.staged,
    options.followUpPush,
  );
  return {
    confirmationId: preview.confirmationId,
    authorName: model.authorName,
    authorEmail: model.authorEmail,
    committerName: model.committerName,
    committerEmail: model.committerEmail,
    signEnabled: model.signEnabled,
    signFormat: model.signFormat,
    operation: model.operation,
    executableHooks: model.executableHooks,
    model,
  };
}

export async function performGitCommitExecution(options: {
  repositoryId: string;
  worktreeId: string;
  confirmationId: string;
  message: string;
  signal: AbortSignal;
}): Promise<
  | { ok: true; outcome: GitCommitOutcome }
  | {
      ok: false;
      reason?: string;
      busyOutcome?: GitCommitOutcome;
      banner?: string;
    }
> {
  try {
    const result = await gitCommit(
      options.repositoryId,
      options.worktreeId,
      {
        confirmationId: options.confirmationId,
        message: options.message.trim(),
      },
      options.signal,
    );
    return { ok: true, outcome: mapGitCommitResultResponse(result) };
  } catch (err) {
    const reason = gitConflictReason(err);
    if (reason === "busy") {
      return {
        ok: false,
        reason,
        busyOutcome: {
          status: "busy_locked",
          summary: "检测到外部 Git index.lock，提交未执行。",
          reasons: [errorMessage(err)],
        },
      };
    }
    return { ok: false, reason, banner: errorMessage(err) };
  }
}

export async function performRepositoryFetch(repositoryId: string): Promise<{
  repository: Repository;
  banner: string;
}> {
  const res = await fetchRepositoryRemotes(repositoryId);
  const okCount = res.results.filter((r) => r.status === "ok").length;
  const failCount = res.results.length - okCount;
  return {
    repository: res.repository,
    banner:
      failCount === 0
        ? `已获取全部远程（${okCount} 个）`
        : `远程获取完成：成功 ${okCount} 个，失败 ${failCount} 个`,
  };
}
