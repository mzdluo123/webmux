import type {
  GitCommitDetail,
  GitCommitFile,
  GitCommitPreviewModel,
  GitHistoryRow,
  GitIssue,
  GitWorkbenchViewState,
  GitWorkingChange,
  GroupedWorkingChanges,
} from "../../contracts/git-workbench-schemas";

export function groupWorkingChanges(
  changes: readonly GitWorkingChange[],
): GroupedWorkingChanges {
  const out: GroupedWorkingChanges = {
    conflicted: [],
    unstaged: [],
    staged: [],
    untracked: [],
  };
  for (const item of changes) {
    out[item.group].push(item);
  }
  return out;
}

/**
 * Clicking a branch in the Log pane ONLY updates the history filter and NEVER
 * mutates `context.checkedOutBranch`.
 */
export function selectLogBranchFilter(
  state: GitWorkbenchViewState,
  branchName: string | null,
): GitWorkbenchViewState {
  const filtered = filterLogCommits(
    state.commits,
    branchName,
    state.searchQuery,
    state.pathFilter,
  );
  const nextSelected =
    filtered.find((c) => c.oid === state.selectedCommitOid)?.oid ??
    filtered[0]?.oid ??
    null;
  return {
    ...state,
    branchFilter: branchName,
    selectedCommitOid: nextSelected,
    statusBanner: branchName
      ? `按分支 ${branchName} 过滤历史（当前工作树检出仍为 ${state.context.checkedOutBranch ?? "只读上下文"}）`
      : "显示全部可达历史引用",
  };
}

export function filterLogCommits(
  commits: readonly GitHistoryRow[],
  branchFilter: string | null,
  searchQuery: string,
  pathFilter: string,
): GitHistoryRow[] {
  const q = searchQuery.trim().toLowerCase();
  const p = pathFilter.trim().toLowerCase();
  return commits.filter((row) => {
    if (
      branchFilter &&
      row.branches.length > 0 &&
      !row.branches.includes(branchFilter)
    ) {
      return false;
    }
    if (
      q &&
      !row.subject.toLowerCase().includes(q) &&
      !row.authorName.toLowerCase().includes(q) &&
      !row.shortOid.toLowerCase().includes(q)
    ) {
      return false;
    }
    if (p) {
      const hitPath = row.changedPaths.some((filePath) =>
        filePath.toLowerCase().includes(p),
      );
      if (!hitPath && !row.subject.toLowerCase().includes(p)) {
        return false;
      }
    }
    return true;
  });
}

export function resolveCommitParentFiles(
  detail: GitCommitDetail,
  parentOverride?: string,
): { selectedParent: string; files: GitCommitFile[] } {
  const firstParent = detail.parents[0];
  if (!firstParent) {
    return { selectedParent: "", files: detail.filesByParent[""] ?? [] };
  }
  const valid =
    parentOverride && detail.parents.some((p) => p.oid === parentOverride)
      ? parentOverride
      : detail.selectedParent || firstParent.oid;
  return {
    selectedParent: valid,
    files: detail.filesByParent[valid] ?? [],
  };
}

export function toggleReviewedPath(
  reviewedPaths: readonly string[],
  path: string,
): string[] {
  return reviewedPaths.includes(path)
    ? reviewedPaths.filter((item) => item !== path)
    : [...reviewedPaths, path];
}

export function applyStageHunk(
  changes: readonly GitWorkingChange[],
  changeId: string,
  hunkId?: string,
): GitWorkingChange[] {
  const target = changes.find((c) => c.id === changeId);
  if (
    !target ||
    (target.group !== "unstaged" && target.group !== "untracked")
  ) {
    return [...changes];
  }
  const chosenHunks = hunkId
    ? target.hunks.filter((h) => h.id === hunkId)
    : target.hunks;
  const remainingHunks = hunkId
    ? target.hunks.filter((h) => h.id !== hunkId)
    : [];

  const next: GitWorkingChange[] = [];
  let mergedIntoExistingStaged = false;

  for (const item of changes) {
    if (item.id === changeId) {
      if (hunkId && remainingHunks.length > 0) {
        next.push({
          ...item,
          group: "unstaged",
          status: item.status === "?" ? "M" : item.status,
          hunks: remainingHunks,
        });
      }
      continue;
    }
    if (item.group === "staged" && item.path === target.path) {
      mergedIntoExistingStaged = true;
      next.push({
        ...item,
        hunks: [...item.hunks, ...chosenHunks],
      });
      continue;
    }
    next.push(item);
  }

  if (!mergedIntoExistingStaged) {
    next.push({
      ...target,
      id: `staged:${target.path}`,
      group: "staged",
      status: target.status === "?" ? "A" : target.status,
      preStagedFromCli: false,
      hunks: chosenHunks,
    });
  }
  return next;
}

export function applyUnstageHunk(
  changes: readonly GitWorkingChange[],
  changeId: string,
  hunkId?: string,
): GitWorkingChange[] {
  const target = changes.find((c) => c.id === changeId);
  if (!target || target.group !== "staged") {
    return [...changes];
  }
  const movedHunks = hunkId
    ? target.hunks.filter((h) => h.id === hunkId)
    : target.hunks;
  const remainingStaged = hunkId
    ? target.hunks.filter((h) => h.id !== hunkId)
    : [];

  const next: GitWorkingChange[] = [];
  let mergedIntoExistingUnstaged = false;

  for (const item of changes) {
    if (item.id === changeId) {
      if (hunkId && remainingStaged.length > 0) {
        next.push({ ...item, hunks: remainingStaged });
      }
      continue;
    }
    if (item.group === "unstaged" && item.path === target.path) {
      mergedIntoExistingUnstaged = true;
      next.push({
        ...item,
        hunks: [...movedHunks, ...item.hunks],
      });
      continue;
    }
    next.push(item);
  }

  if (!mergedIntoExistingUnstaged) {
    next.push({
      ...target,
      id: `unstaged:${target.path}`,
      group: target.status === "A" && !hunkId ? "untracked" : "unstaged",
      status: target.status === "A" && !hunkId ? "?" : "M",
      preStagedFromCli: false,
      hunks: movedHunks,
    });
  }
  return next;
}

export function buildCommitPreviewModel(
  state: GitWorkbenchViewState,
  followUpPush: boolean,
): GitCommitPreviewModel {
  const grouped = groupWorkingChanges(state.workingChanges);
  const blockers: GitIssue[] = [];
  const warnings: GitIssue[] = [];

  if (grouped.conflicted.length > 0) {
    blockers.push({
      code: "conflicted",
      message: "存在未解决的合并冲突，请先处理冲突后再提交",
    });
  }
  if (state.context.operationInProgress) {
    blockers.push({
      code: "operation_in_progress",
      message: `工作树正在进行 ${state.context.operationInProgress} 操作，请先在终端完成或中止`,
    });
  }
  if (state.context.headState === "detached") {
    blockers.push({
      code: "detached_head",
      message: "当前处于游离 HEAD 状态，请先创建或切换到分支后再提交",
    });
  }
  if (state.context.headState === "readonly" || !state.context.worktreePath) {
    blockers.push({
      code: "no_worktree",
      message: "当前为无工作树只读仓库视图，请选择具体工作树后再提交",
    });
  }
  if (grouped.staged.length === 0 && grouped.conflicted.length === 0) {
    blockers.push({
      code: "empty_index",
      message: "真实 Git index 暂存区为空，请先暂存至少一项修改",
    });
  }
  if (!state.context.authorName.trim() || !state.context.authorEmail.trim()) {
    blockers.push({
      code: "missing_identity",
      message: "缺少 Git user.name 或 user.email 配置，请先配置作者身份",
    });
  }

  const cliPreStagedCount = grouped.staged.filter(
    (s) => s.preStagedFromCli,
  ).length;
  if (cliPreStagedCount > 0) {
    warnings.push({
      code: "cli_prestaged",
      message: `完整 index 预览已包含终端预先暂存的 ${cliPreStagedCount} 个文件，请确认无误`,
    });
  }
  if (grouped.unstaged.length > 0 || grouped.untracked.length > 0) {
    warnings.push({
      code: "remaining_unstaged",
      message: `其余 ${grouped.unstaged.length} 项未暂存修改与 ${grouped.untracked.length} 个未跟踪文件不会进入本次提交`,
    });
  }
  if (state.context.executableHooks.length > 0) {
    warnings.push({
      code: "hooks_active",
      message: `提交将执行仓库钩子：${state.context.executableHooks.join(", ")}`,
    });
  }

  const headState =
    state.context.headState === "readonly" ? "branch" : state.context.headState;

  return {
    repositoryName: state.context.repositoryName,
    worktreePath: state.context.worktreePath ?? "(无工作树)",
    headState,
    branch: state.context.checkedOutBranch ?? "(detached)",
    headOid: state.context.headOid ?? "",
    expectedTreeOid: `tree-${grouped.staged.map((f) => f.path).join("+") || "empty"}`,
    authorName: state.context.authorName,
    authorEmail: state.context.authorEmail,
    signEnabled: state.context.signEnabled,
    executableHooks: state.context.executableHooks,
    stagedFiles: grouped.staged,
    unstagedCount: grouped.unstaged.length,
    untrackedCount: grouped.untracked.length,
    conflictedCount: grouped.conflicted.length,
    followUpPush,
    blockers,
    warnings,
  };
}
