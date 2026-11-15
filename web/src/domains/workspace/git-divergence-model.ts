import { m } from "$lib/paraglide/messages.js";
import type {
  GitDivergenceKind,
  GitInProgressOperation,
  GitSequencerKind,
} from "../../contracts/schemas";

export function gitSequencerKindLabel(
  kind: GitSequencerKind | (string & {}) | undefined,
): string {
  switch (kind) {
    case "merge":
      return m.s7_seq_kind_merge();
    case "rebase":
      return m.s7_seq_kind_rebase();
    case "cherry-pick":
    case "cherry_pick":
      return m.s7_seq_kind_cherry_pick();
    case "revert":
      return m.s7_seq_kind_revert();
    case "bisect":
      return m.s7_seq_kind_bisect();
    case "unstash":
      return m.s7_seq_kind_unstash();
    default:
      return kind ? m.s7_seq_kind_named({ kind }) : m.s7_seq_kind_generic();
  }
}

export function gitDivergenceTitleLabel(kind: GitDivergenceKind): string {
  switch (kind) {
    case "merge":
      return m.s7_div_title_merge();
    case "rebase":
      return m.s7_div_title_rebase();
    case "cherry_pick":
      return m.s7_div_title_cherry_pick();
    case "revert":
      return m.s7_div_title_revert();
  }
}

export function gitDivergenceConfirmLabel(
  kind: GitDivergenceKind,
  fastForward = false,
): string {
  switch (kind) {
    case "merge":
      return fastForward
        ? m.s7_div_confirm_merge_ff()
        : m.s7_div_confirm_merge();
    case "rebase":
      return m.s7_div_confirm_rebase();
    case "cherry_pick":
      return m.s7_div_confirm_cherry_pick();
    case "revert":
      return m.s7_div_confirm_revert();
  }
}

export function formatInProgressHeadline(op: GitInProgressOperation): string {
  const kindText = gitSequencerKindLabel(op.kind);
  const stepText =
    op.stepTotal && op.stepTotal > 0 && op.stepCurrent
      ? m.s7_in_progress_step({
          current: String(op.stepCurrent),
          total: String(op.stepTotal),
        })
      : "";
  if (op.kind === "rebase") {
    const src = op.sourceLabel || op.currentBranch || m.s7_current_branch();
    const dst = op.targetLabel || m.s7_target_base();
    return m.s7_in_progress_rebase({
      kind: kindText,
      step: stepText,
      src,
      dst,
    });
  }
  if (op.kind === "merge") {
    const src = op.sourceLabel || m.s7_source_branch();
    const dst = op.targetLabel || op.currentBranch || m.s7_current_branch();
    return m.s7_in_progress_merge({ kind: kindText, src, dst });
  }
  if (op.kind === "cherry-pick" || op.kind === "cherry_pick") {
    const src =
      op.sourceLabel || op.sourceOid?.slice(0, 7) || m.s7_target_commit();
    return m.s7_in_progress_cherry_pick({
      kind: kindText,
      step: stepText,
      src,
    });
  }
  if (op.kind === "revert") {
    const src =
      op.sourceLabel || op.sourceOid?.slice(0, 7) || m.s7_target_commit();
    return m.s7_in_progress_revert({
      kind: kindText,
      step: stepText,
      src,
    });
  }
  if (op.kind === "unstash") {
    const src =
      op.sourceLabel || op.sourceOid?.slice(0, 7) || m.s7_stash_entry();
    return m.s7_in_progress_unstash({ kind: kindText, src });
  }
  return m.s7_in_progress_generic({ kind: kindText, step: stepText });
}

export function formatPushRemoteTrackingRef(
  remoteName: string,
  targetBranch: string,
): string {
  const cleanRemote = remoteName.trim() || "origin";
  const cleanTarget = targetBranch.trim();
  if (!cleanTarget) return "";
  if (cleanTarget.startsWith(`${cleanRemote}/`)) {
    return cleanTarget;
  }
  return `${cleanRemote}/${cleanTarget}`;
}
