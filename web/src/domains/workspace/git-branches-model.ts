import { m } from "$lib/paraglide/messages.js";
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
      badgeText: m.s7_upstream_none_badge(),
      detailText: m.s7_upstream_none_detail(),
    };
  }
  if (item.upstreamGone) {
    return {
      kind: "gone",
      badgeText: m.s7_upstream_gone_badge(),
      detailText: m.s7_upstream_gone_detail({ upstream: item.upstream }),
    };
  }
  if (item.ahead === undefined || item.behind === undefined) {
    return {
      kind: "uncounted",
      badgeText: m.s7_upstream_uncounted_badge(),
      detailText: m.s7_upstream_uncounted_detail({ upstream: item.upstream }),
    };
  }
  if (item.ahead > 0 && item.behind > 0) {
    return {
      kind: "diverged",
      badgeText: `↑${item.ahead} ↓${item.behind}`,
      detailText: m.s7_upstream_diverged_detail({
        upstream: item.upstream,
        ahead: String(item.ahead),
        behind: String(item.behind),
      }),
      ahead: item.ahead,
      behind: item.behind,
    };
  }
  if (item.ahead > 0) {
    return {
      kind: "ahead",
      badgeText: `↑${item.ahead}`,
      detailText: m.s7_upstream_ahead_detail({
        upstream: item.upstream,
        ahead: String(item.ahead),
      }),
      ahead: item.ahead,
      behind: 0,
    };
  }
  if (item.behind > 0) {
    return {
      kind: "behind",
      badgeText: `↓${item.behind}`,
      detailText: m.s7_upstream_behind_detail({
        upstream: item.upstream,
        behind: String(item.behind),
      }),
      ahead: 0,
      behind: item.behind,
    };
  }
  return {
    kind: "synced",
    badgeText: m.s7_upstream_synced_badge(),
    detailText: m.s7_upstream_synced_detail({ upstream: item.upstream }),
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
  | "merge-into-current"
  | "rebase-onto"
  | "fetch"
  | "fast-forward"
  | "push"
  | "rename-branch"
  | "set-upstream"
  | "create-tag"
  | "delete-tag"
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
          ? m.s7_branch_action_filter_tag()
          : m.s7_branch_action_filter_branch(),
    },
  ];

  if (branchFilter === item.name) {
    actions.push({
      kind: "clear-filter",
      label: m.s7_branch_action_clear_filter(),
    });
  }

  if (item.kind !== "tag") {
    actions.push({
      kind: "toggle-favorite",
      label: item.favorite
        ? m.s7_branch_action_unfavorite()
        : m.s7_branch_action_favorite(),
    });
  }

  if (compareBaseOid && compareBaseOid !== item.head) {
    actions.push({
      kind: "compare",
      label: compareTargetLabel || m.s7_compare_with_base(),
    });
  }

  // Occupancy-safe primary navigation / checkout / worktree actions
  if (item.occupiedBy) {
    actions.push(
      {
        kind: "open-occupied-worktree",
        label:
          item.occupiedBy.activeSessions > 0
            ? m.s7_branch_action_open_occupied_sessions({
                count: String(item.occupiedBy.activeSessions),
              })
            : m.s7_branch_action_open_occupied(),
        separatorBefore: true,
      },
      {
        kind: "open-new-worktree",
        label: m.s7_branch_action_new_wt_from_commit(),
      },
    );
  } else if (item.kind !== "tag") {
    if (isCurrent) {
      actions.push({
        kind: "checkout",
        label: m.s7_branch_action_checkout(),
        disabled: true,
        disabledReason: m.s7_branch_disabled_already_checked_out(),
        separatorBefore: true,
      });
    } else if (!hasWritableWorktree) {
      actions.push({
        kind: "checkout",
        label: m.s7_branch_action_checkout(),
        disabled: true,
        disabledReason: m.s7_branch_disabled_readonly_checkout(),
        separatorBefore: true,
      });
    } else {
      actions.push({
        kind: "checkout",
        label:
          item.kind === "remote"
            ? m.s7_branch_action_checkout_remote()
            : m.s7_branch_action_checkout(),
        separatorBefore: true,
      });
    }

    actions.push(
      {
        kind: "create-branch-from",
        label: hasWritableWorktree
          ? m.s7_branch_action_create_from()
          : m.s7_branch_action_create_wt_from(),
      },
      {
        kind: "open-new-worktree",
        label: m.s7_branch_action_open_new_wt(),
      },
    );
  }

  // V2 Divergence actions (Merge into current branch / Rebase current branch onto target)
  if (item.kind !== "tag") {
    const canDivergeOnBranch =
      hasWritableWorktree &&
      context.headState === "branch" &&
      Boolean(context.checkedOutBranch) &&
      !context.operationInProgress;
    const divergeDisabledReason = !hasWritableWorktree
      ? m.s7_branch_disabled_readonly_wt()
      : context.operationInProgress
        ? m.s7_branch_disabled_op_in_progress({
            op: context.operationInProgress,
          })
        : context.headState !== "branch" || !context.checkedOutBranch
          ? m.s7_branch_disabled_not_named_branch()
          : undefined;

    if (!isCurrent) {
      actions.push(
        {
          kind: "merge-into-current",
          label: context.checkedOutBranch
            ? m.s7_branch_action_merge_into_named({
                branch: context.checkedOutBranch,
              })
            : m.s7_branch_action_merge_into_current(),
          disabled: !canDivergeOnBranch,
          disabledReason: divergeDisabledReason,
          separatorBefore: true,
        },
        {
          kind: "rebase-onto",
          label: context.checkedOutBranch
            ? m.s7_branch_action_rebase_named_onto({
                branch: context.checkedOutBranch,
              })
            : m.s7_branch_action_rebase_current_onto(),
          disabled: !canDivergeOnBranch,
          disabledReason: divergeDisabledReason,
        },
      );
    } else if (item.upstream && !item.upstreamGone) {
      actions.push(
        {
          kind: "merge-into-current",
          label: m.s7_branch_action_merge_upstream({ upstream: item.upstream }),
          disabled: !canDivergeOnBranch,
          disabledReason: divergeDisabledReason,
          separatorBefore: true,
        },
        {
          kind: "rebase-onto",
          label: m.s7_branch_action_rebase_upstream({
            upstream: item.upstream,
          }),
          disabled: !canDivergeOnBranch,
          disabledReason: divergeDisabledReason,
        },
      );
    }
  }

  // Local branch remote sync & safe deletion actions
  if (item.kind === "local") {
    actions.push({
      kind: "fetch",
      label: m.s7_branch_action_fetch(),
      separatorBefore: true,
    });

    const ffDisabledReason =
      upstream.kind === "none"
        ? m.s7_branch_disabled_ff_no_upstream()
        : upstream.kind === "gone"
          ? m.s7_branch_disabled_ff_gone()
          : upstream.kind === "diverged"
            ? m.s7_branch_disabled_ff_diverged()
            : upstream.kind === "synced" || upstream.kind === "ahead"
              ? m.s7_branch_disabled_ff_no_incoming()
              : undefined;

    actions.push({
      kind: "fast-forward",
      label: m.s7_branch_action_fast_forward(),
      disabled: Boolean(ffDisabledReason),
      disabledReason: ffDisabledReason,
    });

    const pushLabel =
      upstream.kind === "none" || upstream.kind === "gone"
        ? m.s7_branch_action_publish()
        : m.s7_branch_action_push();
    const pushDisabledReason =
      upstream.kind === "diverged"
        ? m.s7_branch_disabled_push_diverged()
        : upstream.kind === "behind"
          ? m.s7_branch_disabled_push_behind()
          : upstream.kind === "synced"
            ? m.s7_branch_disabled_push_synced()
            : undefined;

    actions.push({
      kind: "push",
      label: pushLabel,
      disabled: Boolean(pushDisabledReason),
      disabledReason: pushDisabledReason,
    });

    actions.push(
      {
        kind: "rename-branch",
        label: m.s7_branch_action_rename(),
        separatorBefore: true,
      },
      {
        kind: "set-upstream",
        label: item.upstream
          ? m.s7_branch_action_edit_upstream()
          : m.s7_branch_action_set_upstream(),
      },
      {
        kind: "create-tag",
        label: m.s7_branch_action_create_tag(),
      },
    );

    const deleteDisabledReason = isCurrent
      ? m.s7_branch_disabled_delete_current()
      : item.occupiedBy
        ? m.s7_branch_disabled_delete_occupied()
        : undefined;

    actions.push({
      kind: "delete",
      label: m.s7_branch_action_delete(),
      destructive: true,
      disabled: Boolean(deleteDisabledReason),
      disabledReason: deleteDisabledReason,
      separatorBefore: true,
    });
  } else if (item.kind === "remote") {
    actions.push({
      kind: "create-tag",
      label: m.s7_branch_action_create_tag(),
      separatorBefore: true,
    });
  } else if (item.kind === "tag") {
    actions.push(
      {
        kind: "create-tag",
        label: m.s7_branch_action_create_another_tag(),
        separatorBefore: true,
      },
      {
        kind: "delete-tag",
        label: m.s7_branch_action_delete_tag(),
        destructive: true,
        separatorBefore: true,
      },
    );
  }

  return actions;
}
