import type {
  GitBranchItem,
  GitWorktreeContext,
} from "../../contracts/git-workbench-schemas";

type GitBranchUpstreamKind =
  | "none"
  | "gone"
  | "uncounted"
  | "synced"
  | "ahead"
  | "behind"
  | "diverged";

export interface GitBranchUpstreamSummary {
  kind: GitBranchUpstreamKind;
  badgeText: string;
  detailText: string;
  ahead?: number;
  behind?: number;
}

/**
 * Classifies a branch's upstream tracking state without ever inventing `0` when
 * upstream is missing, deleted, or uncounted (`UI-05`).
 */
export function resolveGitBranchUpstream(
  item: GitBranchItem,
): GitBranchUpstreamSummary {
  if (item.kind !== "local") {
    return { kind: "none", badgeText: "", detailText: "" };
  }
  if (!item.upstream) {
    return {
      kind: "none",
      badgeText: "无上游",
      detailText: "尚未配置远程上游分支，推送时将执行首次发布（Publish）",
    };
  }
  if (item.upstreamGone) {
    return {
      kind: "gone",
      badgeText: "上游已删",
      detailText: `上游 ${item.upstream} 已从远端移除，再次推送将重新发布`,
    };
  }
  if (item.ahead === undefined || item.behind === undefined) {
    return {
      kind: "uncounted",
      badgeText: "待统计",
      detailText: `跟踪 ${item.upstream}（领先/落后计数待预览计算）`,
    };
  }
  if (item.ahead > 0 && item.behind > 0) {
    return {
      kind: "diverged",
      badgeText: `↑${item.ahead} ↓${item.behind}`,
      detailText: `跟踪 ${item.upstream} · 已分叉（领先 ${item.ahead} · 落后 ${item.behind}）`,
      ahead: item.ahead,
      behind: item.behind,
    };
  }
  if (item.ahead > 0) {
    return {
      kind: "ahead",
      badgeText: `↑${item.ahead}`,
      detailText: `跟踪 ${item.upstream} · 领先 ${item.ahead} 个提交`,
      ahead: item.ahead,
      behind: 0,
    };
  }
  if (item.behind > 0) {
    return {
      kind: "behind",
      badgeText: `↓${item.behind}`,
      detailText: `跟踪 ${item.upstream} · 落后 ${item.behind} 个提交`,
      ahead: 0,
      behind: item.behind,
    };
  }
  return {
    kind: "synced",
    badgeText: "已同步",
    detailText: `跟踪 ${item.upstream} · 与上游完全同步`,
    ahead: 0,
    behind: 0,
  };
}

export interface GroupedGitBranches {
  current: GitBranchItem[];
  favorites: GitBranchItem[];
  local: GitBranchItem[];
  remote: GitBranchItem[];
  tags: GitBranchItem[];
}

export function groupAndSortGitBranches(
  branches: readonly GitBranchItem[],
  checkedOutBranch: string | null,
  query = "",
): GroupedGitBranches {
  const q = query.trim().toLowerCase();
  const filtered = q
    ? branches.filter(
        (b) =>
          b.name.toLowerCase().includes(q) ||
          b.shortHead.toLowerCase().includes(q) ||
          b.subject.toLowerCase().includes(q) ||
          (b.upstream?.toLowerCase().includes(q) ?? false),
      )
    : branches;

  const current: GitBranchItem[] = [];
  const favorites: GitBranchItem[] = [];
  const local: GitBranchItem[] = [];
  const remote: GitBranchItem[] = [];
  const tags: GitBranchItem[] = [];

  for (const item of filtered) {
    if (item.kind === "tag") {
      tags.push(item);
      continue;
    }
    const isCurrent = item.kind === "local" && item.name === checkedOutBranch;
    if (isCurrent) {
      current.push(item);
      continue;
    }
    if (item.favorite) {
      favorites.push(item);
    }
    if (item.kind === "local") {
      local.push(item);
    } else {
      remote.push(item);
    }
  }

  return { current, favorites, local, remote, tags };
}

export type GitSharedBranchActionKind =
  | "filter-log"
  | "clear-filter"
  | "toggle-favorite"
  | "compare"
  | "open-occupied-worktree"
  | "checkout"
  | "create-branch-from"
  | "open-new-worktree"
  | "fetch"
  | "fast-forward"
  | "push"
  | "delete";

export interface GitSharedBranchActionItem {
  kind: GitSharedBranchActionKind;
  label: string;
  disabled?: boolean;
  disabledReason?: string;
  destructive?: boolean;
  separatorBefore?: boolean;
}

export function resolveGitBranchActions(options: {
  item: GitBranchItem;
  context: GitWorktreeContext;
  branchFilter?: string | null;
  compareTargetLabel?: string | null;
  compareBaseOid?: string | null;
}): GitSharedBranchActionItem[] {
  const { item, context, branchFilter, compareTargetLabel, compareBaseOid } =
    options;
  const isCurrent =
    item.kind === "local" && item.name === context.checkedOutBranch;
  const hasWritableWorktree =
    context.headState !== "readonly" && Boolean(context.worktreePath);
  const upstream = resolveGitBranchUpstream(item);

  const actions: GitSharedBranchActionItem[] = [
    {
      kind: "filter-log",
      label:
        item.kind === "tag"
          ? "在历史 (Log) 中筛选此标签"
          : "在历史 (Log) 中筛选此分支",
    },
  ];

  if (branchFilter === item.name) {
    actions.push({
      kind: "clear-filter",
      label: "清除历史引用筛选",
    });
  }

  if (item.kind !== "tag") {
    actions.push({
      kind: "toggle-favorite",
      label: item.favorite ? "取消收藏分支" : "收藏分支",
    });
  }

  if (compareBaseOid && compareBaseOid !== item.head) {
    actions.push({
      kind: "compare",
      label: compareTargetLabel || "与当前基准比较",
    });
  }

  // Occupancy-safe primary navigation / checkout / worktree actions
  if (item.occupiedBy) {
    actions.push(
      {
        kind: "open-occupied-worktree",
        label:
          item.occupiedBy.activeSessions > 0
            ? `打开所在工作树（${item.occupiedBy.activeSessions} 个活跃会话）`
            : "打开所在工作树",
        separatorBefore: true,
      },
      {
        kind: "open-new-worktree",
        label: "从该提交新建分支工作树…",
      },
    );
  } else if (item.kind !== "tag") {
    if (isCurrent) {
      actions.push({
        kind: "checkout",
        label: "检出分支…",
        disabled: true,
        disabledReason: "当前工作树已检出此分支",
        separatorBefore: true,
      });
    } else if (!hasWritableWorktree) {
      actions.push({
        kind: "checkout",
        label: "检出分支…",
        disabled: true,
        disabledReason: "当前为只读仓库视图，请先选择工作树或在新工作树打开",
        separatorBefore: true,
      });
    } else {
      actions.push({
        kind: "checkout",
        label: item.kind === "remote" ? "检出并跟踪远程分支…" : "检出分支…",
        separatorBefore: true,
      });
    }

    actions.push(
      {
        kind: "create-branch-from",
        label: hasWritableWorktree
          ? "从此分支新建分支…"
          : "从此分支新建工作树分支…",
      },
      {
        kind: "open-new-worktree",
        label: "在新工作树打开…",
      },
    );
  }

  // Local branch remote sync & safe deletion actions
  if (item.kind === "local") {
    actions.push({
      kind: "fetch",
      label: "获取远程 (Fetch)",
      separatorBefore: true,
    });

    const ffDisabledReason =
      upstream.kind === "none"
        ? "尚未配置远程上游分支"
        : upstream.kind === "gone"
          ? "远程上游分支已删除"
          : upstream.kind === "diverged"
            ? "本地与上游已分叉，请在终端合并或变基"
            : upstream.kind === "synced" || upstream.kind === "ahead"
              ? "上游没有领先本地的新提交"
              : undefined;

    actions.push({
      kind: "fast-forward",
      label: "快进更新… (Update)",
      disabled: Boolean(ffDisabledReason),
      disabledReason: ffDisabledReason,
    });

    const pushLabel =
      upstream.kind === "none" || upstream.kind === "gone"
        ? "发布分支… (Publish)"
        : "推送… (Push)";
    const pushDisabledReason =
      upstream.kind === "diverged"
        ? "本地与上游已分叉，请在终端合并或变基后再推送"
        : upstream.kind === "behind"
          ? "分支落后于上游，请先快进更新"
          : upstream.kind === "synced"
            ? "已与上游同步，没有待推送提交"
            : undefined;

    actions.push({
      kind: "push",
      label: pushLabel,
      disabled: Boolean(pushDisabledReason),
      disabledReason: pushDisabledReason,
    });

    const deleteDisabledReason = isCurrent
      ? "当前工作树已检出此分支，无法直接删除"
      : item.occupiedBy
        ? "已在另一工作树检出，无法直接删除"
        : undefined;

    actions.push({
      kind: "delete",
      label: "删除分支…",
      destructive: true,
      disabled: Boolean(deleteDisabledReason),
      disabledReason: deleteDisabledReason,
      separatorBefore: true,
    });
  }

  return actions;
}
