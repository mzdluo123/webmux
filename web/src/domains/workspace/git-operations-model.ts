import * as m from "$lib/paraglide/messages.js";
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
      return m.s7_op_kind_commit();
    case "push":
      return m.s7_op_kind_push();
    case "fetch":
      return m.s7_op_kind_fetch();
    case "fast_forward":
      return m.s7_op_kind_fast_forward();
    case "switch":
      return m.s7_op_kind_switch();
    case "merge":
      return m.s7_op_kind_merge();
    case "rebase":
      return m.s7_op_kind_rebase();
    case "cherry_pick":
      return m.s7_op_kind_cherry_pick();
    case "revert":
      return m.s7_op_kind_revert();
    case "continue":
      return m.s7_op_kind_continue();
    case "abort":
      return m.s7_op_kind_abort();
    case "stash_push":
      return m.s7_op_kind_stash_push();
    case "stash_apply":
      return m.s7_op_kind_stash_apply();
    case "stash_pop":
      return m.s7_op_kind_stash_pop();
    case "stash_drop":
      return m.s7_op_kind_stash_drop();
    case "branch_rename":
      return m.s7_op_kind_branch_rename();
    case "branch_upstream":
      return m.s7_op_kind_branch_upstream();
    case "tag_create":
      return m.s7_op_kind_tag_create();
    case "tag_delete":
      return m.s7_op_kind_tag_delete();
    case "remote_add":
      return m.s7_op_kind_remote_add();
    case "remote_update":
      return m.s7_op_kind_remote_update();
    case "remote_remove":
      return m.s7_op_kind_remote_remove();
    case "discard":
      return m.s7_op_kind_discard();
    case "smart_checkout":
      return "Smart Checkout";
  }
}

export function gitOperationStatusLabel(status: GitOperationStatus): string {
  switch (status) {
    case "pending":
      return m.s7_op_status_pending();
    case "running":
      return m.s7_op_status_running();
    case "succeeded":
      return m.s7_op_status_succeeded();
    case "needs_review":
      return m.s7_op_status_needs_review();
    case "failed":
      return m.s7_op_status_failed();
    case "uncertain":
      return m.s7_op_status_uncertain();
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
    const oid = op.actualOid?.slice(0, 7) ?? "";
    return {
      operationId: op.id,
      status: "matched",
      commitOid: op.actualOid,
      expectedTreeOid: op.preview.expectedTreeOid,
      actualTreeOid: op.actualTreeOid,
      summary:
        op.summary ||
        (op.subject
          ? m.s7_op_committed_with_subject({ oid, subject: op.subject })
          : m.s7_op_committed({ oid })),
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
      summary: op.summary || m.s7_op_needs_review_summary(),
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
        m.s7_op_uncertain_summary({ kind: gitOperationKindLabel(op.kind) }),
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
