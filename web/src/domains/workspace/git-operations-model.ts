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
    if (op.status === "uncertain" || op.status === "needs_review") {
      return op;
    }
  }
  return null;
}
