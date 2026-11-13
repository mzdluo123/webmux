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
import { groupWorkingChanges } from "../../domains/workspace/git-workbench-state";

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
    scope: "all",
    branchFilter: null,
    searchQuery: "",
    pathFilter: "",
    commits: [],
    commitDetails: {},
    selectedCommitOid: null,
    selectedParentByCommit: {},
    selectedCommitFilePath: null,
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

export async function fetchGitTargetBundle(options: {
  repositoryId: string;
  worktreeId?: string;
  repository: Repository | undefined;
  worktree: Worktree | undefined;
  sessions: readonly SessionInfo[];
  scope: GitHistoryScope;
  branchFilter: string | null;
  signal: AbortSignal;
}): Promise<{
  effectiveWorktreeId?: string;
  context: GitWorktreeContext;
  historyState: GitWorkbenchViewState["historyState"];
  historyReason: string;
  commits: GitHistoryRow[];
  branches: GitBranchItem[];
}> {
  const validWt =
    options.worktreeId &&
    options.worktree &&
    options.worktree.available &&
    !options.worktree.prunable &&
    !options.worktree.locked &&
    options.worktree.kind !== "bare"
      ? options.worktree
      : undefined;
  const effectiveWorktreeId = validWt?.id;

  const [ctxRes, histRes, branchesRes] = await Promise.all([
    gitContext(options.repositoryId, effectiveWorktreeId, options.signal),
    gitHistory(
      options.repositoryId,
      {
        worktreeId: effectiveWorktreeId,
        scope: effectiveWorktreeId ? options.scope : "all",
        branch: options.branchFilter ?? undefined,
        limit: 100,
      },
      options.signal,
    ),
    repositoryBranches(options.repositoryId, options.signal).catch(() => ({
      defaultBranch: null,
      defaultBase: null,
      defaultBaseSubject: "",
      defaultParent: "",
      branches: [],
      bases: [],
    })),
  ]);

  const context = deriveGitWorktreeContext({
    repository: options.repository,
    repositoryId: options.repositoryId,
    worktree: validWt ?? options.worktree,
    gitCtx: ctxRes,
  });
  const hist = mapGitHistoryPage(histRes);
  const branches = mapGitBranchesResponse(
    options.repository,
    context.worktreePath,
    options.sessions,
    branchesRes,
  );

  return {
    effectiveWorktreeId,
    context,
    historyState: hist.historyState,
    historyReason: hist.historyReason,
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
  const mapped = mapGitStagingOverview(overview, options.diffCache);
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
  if (
    (options.entry.area !== "staged" && options.entry.area !== "unstaged") ||
    options.entry.status === "?"
  ) {
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
  { ok: true } | { ok: false; busyOutcome?: GitCommitOutcome; banner?: string }
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
    if (gitConflictReason(err) === "busy") {
      return {
        ok: false,
        busyOutcome: {
          status: "busy_locked",
          summary: "检测到外部 Git index.lock，暂存操作已安全中止。",
          reasons: [errorMessage(err)],
        },
      };
    }
    return { ok: false, banner: errorMessage(err) };
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
  signEnabled: boolean;
  executableHooks: string[];
  model: GitCommitPreviewModel;
}> {
  const preview = await gitCommitPreview(
    options.repositoryId,
    options.worktreeId,
    options.signal,
  );
  const grouped = groupWorkingChanges(options.workingChanges);
  return {
    confirmationId: preview.confirmationId,
    authorName: preview.identity.authorName,
    authorEmail: preview.identity.authorEmail,
    signEnabled: preview.identity.signEnabled,
    executableHooks: preview.executableHooks,
    model: mapGitCommitPreviewResponse(
      options.repositoryName,
      preview,
      grouped.staged,
      options.followUpPush,
    ),
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
  | { ok: false; busyOutcome?: GitCommitOutcome; banner?: string }
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
    if (gitConflictReason(err) === "busy") {
      return {
        ok: false,
        busyOutcome: {
          status: "busy_locked",
          summary: "检测到外部 Git index.lock，提交未执行。",
          reasons: [errorMessage(err)],
        },
      };
    }
    return { ok: false, banner: `提交失败：${errorMessage(err)}` };
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
