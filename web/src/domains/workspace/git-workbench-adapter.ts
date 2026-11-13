import type {
  Branches,
  GitChange,
  GitCommitDetailResponse,
  GitCommitPreview,
  GitCommitResult,
  GitContext,
  GitHistoryPage,
  GitStagingEntry,
  GitStagingFileDiff,
  GitStagingOverview,
  Repository,
  SessionInfo,
  Worktree,
} from "../../contracts/schemas";
import type {
  GitBranchItem,
  GitCommitDetail,
  GitCommitFile,
  GitCommitOutcome,
  GitCommitPreviewModel,
  GitHistoryRow,
  GitWorkbenchViewState,
  GitWorkingChange,
  GitWorktreeContext,
} from "../../contracts/git-workbench-schemas";
import { remoteBranchBases } from "./branch-name";

export function deriveGitWorktreeContext(options: {
  repository: Repository | undefined;
  repositoryId: string;
  worktree: Worktree | undefined;
  gitCtx: GitContext | null;
  authorName?: string;
  authorEmail?: string;
  signEnabled?: boolean;
  executableHooks?: string[];
}): GitWorktreeContext {
  const { repository, repositoryId, worktree, gitCtx } = options;
  const repositoryName = repository?.name ?? (repositoryId || "Git 工作台");

  const writableWorktree =
    worktree &&
    worktree.available &&
    !worktree.prunable &&
    !worktree.locked &&
    worktree.kind !== "bare" &&
    gitCtx?.rootKind !== "repository" &&
    gitCtx?.rootKind !== "bare"
      ? worktree
      : undefined;

  if (!writableWorktree) {
    return {
      repositoryId,
      repositoryName,
      worktreeId: null,
      worktreePath: null,
      worktreeKind: worktree?.kind === "bare" ? "bare" : "none",
      checkedOutBranch: null,
      headOid: worktree?.head ?? null,
      headState: "readonly",
      ahead: 0,
      behind: 0,
      upstream: null,
      authorName: options.authorName ?? "",
      authorEmail: options.authorEmail ?? "",
      signEnabled: options.signEnabled ?? false,
      executableHooks: options.executableHooks ?? [],
      operationInProgress: null,
    };
  }

  const headOid = writableWorktree.head ?? null;
  const branch = writableWorktree.branch ?? null;
  const headState: GitWorktreeContext["headState"] = writableWorktree.detached
    ? "detached"
    : !headOid
      ? "unborn"
      : "branch";

  return {
    repositoryId,
    repositoryName,
    worktreeId: writableWorktree.id,
    worktreePath: writableWorktree.path,
    worktreeKind: writableWorktree.kind,
    checkedOutBranch: writableWorktree.detached ? null : branch,
    headOid,
    headState,
    ahead: writableWorktree.ahead ?? 0,
    behind: writableWorktree.behind ?? 0,
    upstream: writableWorktree.upstream ?? null,
    authorName: options.authorName ?? "Git User",
    authorEmail: options.authorEmail ?? "git@localhost",
    signEnabled: options.signEnabled ?? false,
    executableHooks: options.executableHooks ?? [],
    operationInProgress: writableWorktree.operation ?? null,
  };
}

export function mapGitHistoryPage(page: GitHistoryPage): {
  historyState: GitWorkbenchViewState["historyState"];
  historyReason: string;
  historyUpstream?: string;
  lastFetchedAt?: string;
  nextCursor: string;
  refsChanged: boolean;
  commits: GitHistoryRow[];
} {
  const commits: GitHistoryRow[] = page.rows.map((row) => ({
    ...row,
    committerEmail: row.authorEmail,
    changedPaths: [],
    branches: row.refs.map((r) => r.name),
  }));

  return {
    historyState: page.status,
    historyReason: page.reason ?? "",
    historyUpstream: page.upstream,
    lastFetchedAt: page.lastFetchedAt,
    nextCursor: page.nextCursor,
    refsChanged: page.refsChanged,
    commits,
  };
}

function normalizeCommitFileStatus(raw: string): GitCommitFile["status"] {
  const first = raw.trim().charAt(0).toUpperCase();
  if (
    first === "A" ||
    first === "M" ||
    first === "D" ||
    first === "R" ||
    first === "C" ||
    first === "T"
  ) {
    return first;
  }
  return "M";
}

export function mapGitCommitDetailResponse(
  res: GitCommitDetailResponse,
  previous?: GitCommitDetail,
): GitCommitDetail {
  const parentKey = res.comparedParentOid || res.parents[0]?.oid || "";
  const mappedFiles: GitCommitFile[] = res.files.map((file) => ({
    oldPath: file.oldPath,
    newPath: file.newPath || file.oldPath,
    status: normalizeCommitFileStatus(file.status),
    binary: file.binary,
    additions: 0,
    deletions: 0,
    hunks: [],
  }));

  return {
    oid: res.oid,
    shortOid: res.shortOid,
    subject: res.subject,
    body: res.body,
    authorName: res.authorName,
    authorEmail: res.authorEmail,
    authorAt: res.authorAt,
    committerName: res.committerName,
    committerEmail: res.committerEmail,
    committerAt: res.committerAt,
    distinctCommitter: res.distinctCommitter,
    parents: res.parents,
    selectedParent: parentKey,
    refs: res.refs,
    filesByParent: {
      ...previous?.filesByParent,
      [parentKey]: mappedFiles,
    },
  };
}

export function mapGitCompareChanges(
  entries: readonly GitChange[],
): GitCommitFile[] {
  return entries.map((entry) => ({
    changeId: entry.changeId,
    oldPath: entry.oldPath,
    newPath: entry.newPath || entry.oldPath,
    status: normalizeCommitFileStatus(entry.status),
    binary: false,
    additions: 0,
    deletions: 0,
    hunks: [],
  }));
}

export function mapGitBranchesResponse(
  repository: Repository | undefined,
  currentWorktreePath: string | null,
  sessions: readonly SessionInfo[],
  data: Branches,
  favoriteNames?: readonly string[],
): GitBranchItem[] {
  const localNames = new Set(data.branches.map((b) => b.name));
  const favSet = favoriteNames ? new Set(favoriteNames) : null;
  const remotes = remoteBranchBases(data.bases, localNames);
  const items: GitBranchItem[] = [];

  for (const b of data.branches) {
    const otherPath = b.worktreePaths.find(
      (p) => !currentWorktreePath || p !== currentWorktreePath,
    );
    const occupyingTree = otherPath
      ? repository?.worktrees.find((w) => w.path === otherPath)
      : undefined;
    const activeSessions = occupyingTree
      ? sessions.filter(
          (s) =>
            s.state === "active" &&
            s.status !== "exited" &&
            s.workspace?.worktreeId === occupyingTree.id,
        ).length
      : 0;

    items.push({
      name: b.name,
      kind: "local",
      head: b.head,
      shortHead: b.head.slice(0, 7),
      subject: b.subject,
      committedAt: b.committedAt,
      favorite: favSet
        ? favSet.has(b.name)
        : b.name === "main" || b.name === "master",
      upstream: b.upstream ?? undefined,
      ahead: b.ahead ?? undefined,
      behind: b.behind ?? undefined,
      upstreamGone: b.upstreamGone,
      occupiedBy:
        occupyingTree && occupyingTree.path !== currentWorktreePath
          ? {
              worktreeId: occupyingTree.id,
              worktreePath: occupyingTree.path,
              activeSessions,
            }
          : undefined,
    });
  }

  for (const r of remotes) {
    const rawBase = data.bases.find(
      (b) => b.kind === "remote" && b.name === r.name,
    );
    items.push({
      name: r.name,
      kind: "remote",
      head: r.head,
      shortHead: r.head.slice(0, 7),
      subject: r.subject,
      committedAt: rawBase?.committedAt ?? "",
      favorite: favSet ? favSet.has(r.name) : false,
      upstreamGone: false,
    });
  }

  for (const tag of data.tags) {
    items.push({
      name: tag.name,
      kind: "tag",
      head: tag.head,
      shortHead: tag.head.slice(0, 7),
      subject: tag.subject,
      committedAt: tag.committedAt,
      favorite: false,
      upstreamGone: false,
    });
  }

  return items;
}

function normalizeWorkingStatus(raw: string): GitWorkingChange["status"] {
  const first = raw.trim().charAt(0).toUpperCase();
  if (
    first === "M" ||
    first === "A" ||
    first === "D" ||
    first === "R" ||
    first === "U" ||
    first === "?"
  ) {
    return first;
  }
  return "M";
}

export interface MappedStagingEntry {
  change: GitWorkingChange;
  entry: GitStagingEntry;
}

export function mapGitStagingOverview(
  overview: GitStagingOverview,
  diffCache: ReadonlyMap<string, GitStagingFileDiff>,
): MappedStagingEntry[] {
  const out: MappedStagingEntry[] = [];

  const pushEntry = (
    entry: GitStagingEntry,
    group: GitWorkingChange["group"],
  ) => {
    const id = `${group}:${entry.path}`;
    const diffKey = `${entry.area}:${entry.path}`;
    const cachedDiff = diffCache.get(diffKey);
    out.push({
      entry,
      change: {
        id,
        path: entry.path,
        oldPath: entry.oldPath,
        group,
        status:
          group === "untracked" ? "?" : normalizeWorkingStatus(entry.status),
        binary: cachedDiff?.binary ?? false,
        hunkStageAvailable:
          cachedDiff?.hunkStageAvailable ?? entry.hunkStageAvailable,
        degradeReason: cachedDiff?.degradeReason ?? entry.degradeReason,
        preStagedFromCli: false,
        hunks: cachedDiff?.hunks ?? [],
      },
    });
  };

  for (const entry of overview.conflicted) pushEntry(entry, "conflicted");
  for (const entry of overview.staged) pushEntry(entry, "staged");
  for (const entry of overview.unstaged) {
    pushEntry(entry, entry.status === "?" ? "untracked" : "unstaged");
  }
  return out;
}

export function mapGitCommitPreviewResponse(
  repositoryName: string,
  preview: GitCommitPreview,
  stagedChanges: readonly GitWorkingChange[],
  followUpPush: boolean,
): GitCommitPreviewModel {
  const byPath = new Map(stagedChanges.map((item) => [item.path, item]));
  const stagedFiles: GitWorkingChange[] = preview.stagedFiles.map((file) => {
    const path = file.newPath || file.oldPath;
    return (
      byPath.get(path) ?? {
        id: `staged:${path}`,
        path,
        oldPath: file.oldPath || undefined,
        group: "staged",
        status: normalizeWorkingStatus(file.status),
        binary: file.binary,
        hunkStageAvailable: false,
        preStagedFromCli: true,
        hunks: [],
      }
    );
  });

  return {
    repositoryName,
    worktreePath: preview.worktreePath,
    headState: preview.headState,
    branch: preview.branch ?? "(detached)",
    headOid: preview.headOid ?? "",
    expectedTreeOid: preview.expectedTreeOid ?? "",
    authorName: preview.identity.authorName,
    authorEmail: preview.identity.authorEmail,
    signEnabled: preview.identity.signEnabled,
    executableHooks: preview.executableHooks,
    stagedFiles,
    unstagedCount: preview.unstagedCount,
    untrackedCount: preview.untrackedCount,
    conflictedCount: preview.conflictedCount,
    followUpPush,
    blockers: preview.blockers,
    warnings: preview.warnings,
  };
}

export function mapGitCommitResultResponse(
  result: GitCommitResult,
): GitCommitOutcome {
  return {
    status: result.outcome,
    commitOid: result.commitOid,
    expectedTreeOid: result.expectedTreeOid,
    actualTreeOid: result.treeOid,
    summary:
      result.outcome === "matched"
        ? `已提交 ${result.commitOid?.slice(0, 7) ?? ""}${result.subject ? `：${result.subject}` : ""}`
        : result.outcome === "needs_review"
          ? "提交已完成，但钩子改写了最终树，请复核提交详情。"
          : "提交结果未能在预期时间内确认，请刷新历史与暂存区核实。",
    reasons: result.reviewReasons,
  };
}
