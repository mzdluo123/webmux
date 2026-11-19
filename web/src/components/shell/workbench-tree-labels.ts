import { m } from "$lib/paraglide/messages.js";
import type {
  ArchivedSession,
  HomeWorktrees,
  Repository,
  SessionInfo,
  WorktreePins,
} from "../../contracts/schemas";
import { recordTitle } from "../../domains/sessions/archived";
import {
  rowStatusLabels,
  type RowStatus,
} from "../../domains/sessions/session-status";
import { homeLayout } from "../../domains/workspace/home-worktrees";
import type { TreeNode } from "../../domains/workspace/workbench-tree";
import { buildWorktreePinMenuProps } from "../../domains/workspace/worktree-pins";
import { worktreePins } from "../../domains/workspace/worktree-pins.svelte";
import { worktreeUpstream } from "../../domains/workspace/worktree-sync";

// Positive ahead or behind counts take space beside the worktree name.
export function notableUpstream(node: TreeNode) {
  const upstream = node.worktree && worktreeUpstream(node.worktree);
  return upstream?.kind === "tracked" && (upstream.ahead || upstream.behind)
    ? upstream
    : undefined;
}

function recordTitleText(record: ArchivedSession) {
  return record.released
    ? m.s1_record_released_title({
        title: recordTitle(record),
        agent: record.agent.name,
        path: record.released.path,
        commit: record.released.commit.slice(0, 12),
      })
    : m.s1_record_ended_title({
        title: recordTitle(record),
        agent: record.agent.name,
      });
}

export function treeNodeStateText(
  node: TreeNode,
  resuming: string,
  status: RowStatus | undefined,
) {
  if (node.record) {
    if (resuming === node.record.id) return m.s1_state_resuming();
    if (status === "dormant") return rowStatusLabels.dormant;
    return node.record.released
      ? m.s1_state_released_ended()
      : m.s1_state_ended();
  }
  return status ? rowStatusLabels[status] : undefined;
}

export function treeNodeAriaLabel(
  node: TreeNode,
  stateText: string | undefined,
) {
  if (stateText) {
    return m.s1_node_with_state({ label: node.label, state: stateText });
  }
  if (node.reason) return m.s1_node_unavailable({ label: node.label });
  const baseLabel =
    node.worktree?.kind === "main"
      ? m.s1_node_main_worktree({ label: node.label })
      : node.label;
  const label = node.pinned
    ? m.s1_node_pinned_worktree({ label: baseLabel })
    : baseLabel;
  if (node.waiting) {
    return m.s1_node_sessions_waiting({
      label,
      count: node.count,
      waiting: node.waiting,
    });
  }
  if (node.count) {
    return m.s1_node_sessions_count({ label, count: node.count });
  }
  return label;
}

export function treeNodeTitle(node: TreeNode, status: RowStatus | undefined) {
  if (node.reason) {
    return m.s1_node_reason_title({ label: node.label, reason: node.reason });
  }
  if (node.record && status !== "dormant") return recordTitleText(node.record);
  if (status) {
    return m.s1_node_with_state({
      label: node.label,
      state: rowStatusLabels[status],
    });
  }
  return node.path ?? node.label;
}

export function treeWorktreePinMenuProps(input: {
  repositories: readonly Repository[];
  sessions: readonly SessionInfo[];
  home: HomeWorktrees;
  pins: WorktreePins;
  repositoryId: string;
  worktreeId: string;
  ontoggle?: (repositoryId: string, worktreeId: string) => void;
  onmove?: (repositoryId: string, worktreeId: string, offset: -1 | 1) => void;
}) {
  const repo = input.repositories.find(
    (item) => item.id === input.repositoryId,
  );
  const shownTrees = repo
    ? homeLayout(
        input.home,
        repo,
        input.sessions.filter(
          (s) => s.workspace?.repositoryId === input.repositoryId,
        ),
        new Map(),
        input.pins,
      ).shown.map((item) => item.tree)
    : undefined;
  if (input.ontoggle) {
    return buildWorktreePinMenuProps({
      pins: input.pins,
      repo,
      worktreeId: input.worktreeId,
      candidates: shownTrees,
      ontoggle: input.ontoggle,
      onmove: input.onmove,
    });
  }
  return worktreePins.menuProps(repo, input.worktreeId, shownTrees);
}
