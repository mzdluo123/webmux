import { m } from "$lib/paraglide/messages.js";
import type { ArchivedSession } from "../../contracts/schemas";
import { recordTitle } from "../../domains/sessions/archived";
import {
  rowStatusLabels,
  type RowStatus,
} from "../../domains/sessions/session-status";
import type { TreeNode } from "../../domains/workspace/workbench-tree";
import { worktreeUpstream } from "../../domains/workspace/worktree-sync";

// A worktree's ahead/behind counts or deleted upstream; nothing when synced.
export function notableUpstream(node: TreeNode) {
  const upstream = node.worktree && worktreeUpstream(node.worktree);
  return upstream?.kind === "gone" ||
    (upstream?.kind === "tracked" && (upstream.ahead || upstream.behind))
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
  const label =
    node.worktree?.kind === "main"
      ? m.s1_node_main_worktree({ label: node.label })
      : node.label;
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
  if (node.record) return recordTitleText(node.record);
  if (status) {
    return m.s1_node_with_state({
      label: node.label,
      state: rowStatusLabels[status],
    });
  }
  return node.path ?? node.label;
}
