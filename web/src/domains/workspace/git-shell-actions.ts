import type {
  GitBranchItem,
  GitCommitOutcome,
  GitPrecedingCommit,
  GitPushPreviewModel,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import type { GitSharedBranchActionKind } from "./git-branches-model";
import { selectLogBranchFilter } from "./git-workbench-state";

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
