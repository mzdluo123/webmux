import { APIError } from "../../platform/api";
import type {
  GitCommitOutcome,
  GitOperation,
  GitOperationKind,
  GitOperationStatus,
} from "../../contracts/git-workbench-schemas";

export function generateGitClientRequestId(kind: GitOperationKind): string {
  const rand =
    typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
      ? crypto.randomUUID().replace(/-/g, "").slice(0, 20)
      : `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 10)}`;
  return `${kind}-${rand}`;
}

export function isRecoverableNetworkFailure(err: unknown): boolean {
  if (err instanceof APIError) {
    // 5xx or 408/502/504 gateway timeouts can mean the server received and is running or finished the op.
    return err.status >= 500 || err.status === 408;
  }
  if (err instanceof DOMException && err.name === "AbortError") {
    return false;
  }
  return true;
}

export function isTerminalGitOperationStatus(
  status: GitOperationStatus,
): boolean {
  return (
    status === "succeeded" ||
    status === "needs_review" ||
    status === "failed" ||
    status === "uncertain"
  );
}

export function gitOperationKindLabel(kind: GitOperationKind): string {
  switch (kind) {
    case "commit":
      return "提交";
    case "push":
      return "推送";
    case "fetch":
      return "获取远程";
    case "fast_forward":
      return "快进更新";
    case "switch":
      return "切换分支";
    case "merge":
      return "合并分支";
    case "rebase":
      return "变基分支";
    case "cherry_pick":
      return "遴选提交";
    case "revert":
      return "还原提交";
    case "continue":
      return "继续操作";
    case "abort":
      return "中止操作";
    case "stash_push":
      return "新建贮藏";
    case "stash_apply":
      return "应用贮藏";
    case "stash_pop":
      return "弹出贮藏";
    case "stash_drop":
      return "丢弃贮藏";
    case "branch_rename":
      return "重命名分支";
    case "branch_upstream":
      return "设置上游跟踪";
    case "tag_create":
      return "新建标签";
    case "tag_delete":
      return "删除标签";
    case "remote_add":
      return "添加远程仓库";
    case "remote_update":
      return "编辑远程仓库";
    case "remote_remove":
      return "移除远程仓库";
    case "discard":
      return "丢弃更改";
    case "smart_checkout":
      return "Smart Checkout";
  }
}

export function gitOperationStatusLabel(status: GitOperationStatus): string {
  switch (status) {
    case "pending":
      return "等待校验";
    case "running":
      return "执行中";
    case "succeeded":
      return "已完成";
    case "needs_review":
      return "需复核";
    case "failed":
      return "已失败";
    case "uncertain":
      return "结果待核实";
  }
}

const SEQUENCER_KINDS = new Set([
  "merge",
  "rebase",
  "cherry_pick",
  "revert",
  "continue",
  "abort",
]);

export function mapGitOperationToCommitOutcome(
  op: GitOperation,
): GitCommitOutcome | null {
  if (op.status === "succeeded") {
    if (op.kind !== "commit") return null;
    return {
      operationId: op.id,
      status: "matched",
      commitOid: op.actualOid,
      expectedTreeOid: op.preview.expectedTreeOid,
      actualTreeOid: op.actualTreeOid,
      summary:
        op.summary ||
        `已提交 ${op.actualOid?.slice(0, 7) ?? ""}${op.subject ? `：${op.subject}` : ""}`,
      reasons: op.reviewReasons,
    };
  }
  if (op.status === "needs_review") {
    if (SEQUENCER_KINDS.has(op.kind)) return null;
    return {
      operationId: op.id,
      status: "needs_review",
      commitOid: op.actualOid,
      expectedTreeOid: op.preview.expectedTreeOid,
      actualTreeOid: op.actualTreeOid,
      summary:
        op.summary || "操作已完成，但检测到钩子改写或树差异，请复核详情。",
      reasons: op.reviewReasons,
    };
  }
  if (op.status === "uncertain") {
    return {
      operationId: op.id,
      status: "uncertain",
      commitOid: op.actualOid,
      expectedTreeOid: op.preview.expectedTreeOid,
      actualTreeOid: op.actualTreeOid,
      summary:
        op.summary ||
        `${gitOperationKindLabel(op.kind)}结果未能唯一确认，请先刷新并检查历史与暂存区。`,
      reasons: op.reviewReasons,
    };
  }
  return null;
}

/**
 * Finds the newest operation requiring user attention (`uncertain` or `needs_review`)
 * that has not been dismissed/acknowledged yet in this browser session.
 */
export function findRestorableAttentionOperation(
  operations: readonly GitOperation[],
  acknowledgedIds: ReadonlySet<string>,
): GitOperation | null {
  for (const op of operations) {
    if (acknowledgedIds.has(op.id)) continue;
    if (op.status === "uncertain") return op;
    if (op.status === "needs_review" && !SEQUENCER_KINDS.has(op.kind)) {
      return op;
    }
  }
  return null;
}
