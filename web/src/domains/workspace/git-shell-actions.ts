import type {
  GitBranchItem,
  GitCommitOutcome,
  GitPrecedingCommit,
  GitPushPreviewModel,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import {
  type GitCommitDraftConflictState,
  joinCommitMessageDraft,
  mergeCommitMessageDrafts,
} from "./git-commit-draft-model";
import type { GitSharedBranchActionKind } from "./git-branches-model";
import {
  applyStageHunk,
  applyUnstageHunk,
  selectLogBranchFilter,
} from "./git-workbench-state";

export function applyShellStageGroup(
  changes: GitWorkbenchViewState["workingChanges"],
  op: "stage" | "unstage",
  group: "unstaged" | "staged" | "untracked",
): GitWorkbenchViewState["workingChanges"] {
  let next = changes;
  for (const it of changes.filter((c) => c.group === group)) {
    next =
      op === "stage"
        ? applyStageHunk(next, it.id)
        : applyUnstageHunk(next, it.id);
  }
  return next;
}

export function applyShellStartCompare(
  view: GitWorkbenchViewState,
  baseOid: string,
  targetOid: string,
): GitWorkbenchViewState {
  const d = view.commitDetails[targetOid];
  const files = d?.filesByParent[d.selectedParent ?? ""] ?? [];
  return {
    ...view,
    compareBaseMarkOid: null,
    selectedCommitFilePath: files[0]?.newPath ?? null,
    compareMode: {
      baseOid,
      targetOid,
      files,
      nextCursor: "",
      loading: false,
      loadingMore: false,
      error: "",
    },
  };
}

export function applyShellClearFilters(
  view: GitWorkbenchViewState,
): GitWorkbenchViewState {
  return {
    ...view,
    scope: "all",
    branchFilter: null,
    searchQuery: "",
    authorFilter: "",
    sinceFilter: "",
    untilFilter: "",
    pathFilter: "",
  };
}

export function applyShellDraftResolution(
  view: GitWorkbenchViewState,
  conflict: GitCommitDraftConflictState | null,
  strategy: "keep-local" | "adopt-remote" | "merge-both",
): {
  nextView: GitWorkbenchViewState;
  nextRevision: number;
} | null {
  if (!conflict) return null;
  if (strategy === "keep-local") {
    return {
      nextRevision: conflict.remoteDraft.revision + 1,
      nextView: {
        ...view,
        statusBanner: "已保留本机输入并覆盖保存提交草稿",
      },
    };
  }
  if (strategy === "adopt-remote") {
    return {
      nextRevision: conflict.remoteDraft.revision,
      nextView: {
        ...view,
        commitMessage: joinCommitMessageDraft(
          conflict.remoteDraft.subject,
          conflict.remoteDraft.body,
        ),
        statusBanner: "已采用另一设备保存的提交草稿版本",
      },
    };
  }
  const merged = mergeCommitMessageDrafts(
    { subject: conflict.localSubject, body: conflict.localBody },
    {
      subject: conflict.remoteDraft.subject,
      body: conflict.remoteDraft.body,
    },
  );
  return {
    nextRevision: conflict.remoteDraft.revision + 1,
    nextView: {
      ...view,
      commitMessage: joinCommitMessageDraft(merged.subject, merged.body),
      statusBanner: "已合并两份提交草稿并保存",
    },
  };
}

export function buildShellPushPreview(
  view: GitWorkbenchViewState,
  readonlyMode: boolean,
  defaultPushPreview: GitPushPreviewModel | null,
  branchOverride?: string,
  precedingCommit?: GitPrecedingCommit,
): GitPushPreviewModel {
  const sourceBranch =
    branchOverride ?? view.context.checkedOutBranch ?? "main";
  const baseModel: GitPushPreviewModel = defaultPushPreview ?? {
    repositoryName: view.context.repositoryName,
    sourceBranch,
    remote: "origin",
    targetBranch: sourceBranch,
    pushUrl: `https://example.invalid/${view.context.repositoryName}.git`,
    publish: !view.context.upstream,
    commits: Object.values(view.commitDetails),
    blockers: readonlyMode
      ? [
          {
            code: "no_worktree",
            message: "无工作树只读模式下无法执行分支推送。",
          },
        ]
      : [],
    warnings: [],
  };
  return precedingCommit ? { ...baseModel, precedingCommit } : baseModel;
}

export function applyShellCommitConfirm(
  view: GitWorkbenchViewState,
  simulatedCommitOutcome: GitCommitOutcome | null,
): {
  nextView: GitWorkbenchViewState;
  followUpPush?: { branch: string; preceding: GitPrecedingCommit };
} | null {
  const preview = view.commitPreview;
  if (!preview) return null;
  const outcome: GitCommitOutcome = simulatedCommitOutcome ?? {
    status: "matched",
    commitOid: "f00dbabe1234567890abcdef1234567890abcdef",
    expectedTreeOid: preview.expectedTreeOid,
    actualTreeOid: preview.expectedTreeOid,
    summary: `已从真实 Git index 创建提交（${preview.stagedFiles.length} 个文件）`,
    reasons: [],
  };
  const remainingChanges =
    outcome.status === "matched"
      ? view.workingChanges.filter((c) => c.group !== "staged")
      : view.workingChanges;
  const shouldFollowUpPush =
    preview.followUpPush && outcome.status === "matched";
  const subject = view.commitMessage.trim().split(/\r?\n/, 1)[0] || "本地提交";
  const commitOid =
    outcome.commitOid || "f00dbabe1234567890abcdef1234567890abcdef";

  return {
    nextView: {
      ...view,
      workingChanges: remainingChanges,
      commitMessage: outcome.status === "matched" ? "" : view.commitMessage,
      commitPreview: null,
      lastCommitOutcome: outcome,
      statusBanner: outcome.summary,
    },
    followUpPush: shouldFollowUpPush
      ? {
          branch: preview.branch,
          preceding: { commitOid, shortOid: commitOid.slice(0, 7), subject },
        }
      : undefined,
  };
}

export function applyShellBranchAction(options: {
  view: GitWorkbenchViewState;
  kind: GitSharedBranchActionKind;
  branch: GitBranchItem;
  onOpenWorktree?: (worktreeId: string, path: string) => void;
  onOpenPush: (branchName: string) => void;
}): GitWorkbenchViewState | null {
  const { view, kind, branch, onOpenWorktree, onOpenPush } = options;

  switch (kind) {
    case "filter-log":
      return { ...selectLogBranchFilter(view, branch.name), mode: "log" };
    case "clear-filter":
      return selectLogBranchFilter(view, null);
    case "toggle-favorite":
      return {
        ...view,
        branches: view.branches.map((b) =>
          b.name === branch.name ? { ...b, favorite: !b.favorite } : b,
        ),
      };
    case "compare": {
      const base =
        view.selectedCommitOid ??
        view.context.headOid ??
        view.commits[0]?.oid ??
        "";
      if (!base || base === branch.head) return view;
      const targetDetail = view.commitDetails[base];
      const pKey = targetDetail?.selectedParent ?? "";
      const files = targetDetail?.filesByParent[pKey] ?? [];
      return {
        ...view,
        mode: "log",
        compareBaseMarkOid: null,
        selectedCommitFilePath: files[0]?.newPath ?? null,
        compareMode: {
          baseOid: branch.head,
          targetOid: base,
          files,
          nextCursor: "",
          loading: false,
          loadingMore: false,
          error: "",
        },
      };
    }
    case "open-occupied-worktree":
      if (!branch.occupiedBy) return view;
      onOpenWorktree?.(
        branch.occupiedBy.worktreeId,
        branch.occupiedBy.worktreePath,
      );
      return {
        ...view,
        statusBanner: `已定位到占用该分支的工作树：${branch.occupiedBy.worktreePath} (${branch.occupiedBy.worktreeId})`,
      };
    case "checkout":
      return {
        ...view,
        statusBanner: `已打开分支检出预览：${branch.name}（不会静默覆盖未提交改动）`,
      };
    case "create-branch-from":
      return {
        ...view,
        statusBanner: `已请求从 ${branch.name} 新建分支…`,
      };
    case "open-new-worktree":
      return {
        ...view,
        statusBanner: `已请求在新工作树打开 ${branch.name}…`,
      };
    case "fetch":
      return { ...view, statusBanner: "已触发远程获取 (Fetch)" };
    case "fast-forward":
      return { ...view, statusBanner: `已打开快进更新预览：${branch.name}` };
    case "push":
      onOpenPush(branch.name);
      return null;
    case "delete":
      return { ...view, statusBanner: `已打开删除分支确认：${branch.name}` };
  }
}

export function applyShellSetTaskBaseline(
  view: GitWorkbenchViewState,
  baseOidOverride?: string,
): GitWorkbenchViewState {
  const unborn = !baseOidOverride && view.context.headState === "unborn";
  const baseOid = unborn ? "" : (baseOidOverride ?? view.context.headOid ?? "");
  const cleanAtStart = view.workingChanges.length === 0;
  const baseline = {
    baselineId: `bl-${view.context.worktreeId ?? "wt-main"}`,
    repositoryId: view.context.repositoryId,
    worktreeId: view.context.worktreeId ?? "wt-main",
    worktreePath: view.context.worktreePath ?? "/work/webmux",
    source: "manual_select" as const,
    unborn,
    ...(baseOid ? { baseOid } : {}),
    capturedAt: new Date().toISOString(),
    cleanAtStart,
    preExistingChanges: {
      staged: view.workingChanges.filter((c) => c.group === "staged").length,
      unstaged: view.workingChanges.filter((c) => c.group === "unstaged")
        .length,
      untracked: view.workingChanges.filter((c) => c.group === "untracked")
        .length,
      conflicted: view.workingChanges.filter((c) => c.group === "conflicted")
        .length,
    },
  };
  const label = unborn ? "空仓起点" : baseOid.slice(0, 7);
  return {
    ...view,
    taskBaseline: baseline,
    historyState:
      view.scope === "task" && view.historyState === "no_task_baseline"
        ? "ready"
        : view.historyState,
    taskReview: {
      status: "ready",
      reason: "",
      baseline,
      headOid: view.context.headOid ?? "",
      headUnborn: view.context.headState === "unborn",
      commitCount: view.taskReview?.commitCount ?? 0,
      uncommittedCount: view.workingChanges.length,
      files: view.taskReview?.files ?? [],
      selectedPath: view.taskReview?.selectedPath ?? null,
      searchQuery: view.taskReview?.searchQuery ?? "",
      nextCursor: "",
      loadingMore: false,
      mutating: false,
    },
    statusBanner: `已将 ${label} 设为当前工作树任务起点`,
  };
}

export function applyShellClearTaskBaseline(
  view: GitWorkbenchViewState,
): GitWorkbenchViewState {
  return {
    ...view,
    taskBaseline: null,
    historyState:
      view.scope === "task" ? "no_task_baseline" : view.historyState,
    taskReview: {
      status: "no_task_baseline",
      reason: "",
      baseline: null,
      headOid: view.context.headOid ?? "",
      headUnborn: view.context.headState === "unborn",
      commitCount: 0,
      uncommittedCount: view.workingChanges.length,
      files: [],
      selectedPath: null,
      searchQuery: "",
      nextCursor: "",
      loadingMore: false,
      mutating: false,
    },
    statusBanner: "已清除当前工作树任务起点基线",
  };
}
