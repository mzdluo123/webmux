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
      return "合并 (Merge)";
    case "rebase":
      return "变基 (Rebase)";
    case "cherry-pick":
    case "cherry_pick":
      return "遴选 (Cherry-pick)";
    case "revert":
      return "还原 (Revert)";
    case "bisect":
      return "二分排错 (Bisect)";
    default:
      return kind ? `操作 (${kind})` : "操作";
  }
}

export function gitDivergenceTitleLabel(kind: GitDivergenceKind): string {
  switch (kind) {
    case "merge":
      return "合并分支预览 · Merge";
    case "rebase":
      return "变基分支预览 · Rebase";
    case "cherry_pick":
      return "遴选提交预览 · Cherry-pick";
    case "revert":
      return "还原提交预览 · Revert";
  }
}

export function gitDivergenceConfirmLabel(
  kind: GitDivergenceKind,
  fastForward = false,
): string {
  switch (kind) {
    case "merge":
      return fastForward ? "快进合入当前分支" : "确认执行合并";
    case "rebase":
      return "确认执行变基";
    case "cherry_pick":
      return "确认遴选提交";
    case "revert":
      return "确认生成还原提交";
  }
}

export function formatInProgressHeadline(op: GitInProgressOperation): string {
  const kindText = gitSequencerKindLabel(op.kind);
  const stepText =
    op.stepTotal && op.stepTotal > 0 && op.stepCurrent
      ? ` · 步骤 ${op.stepCurrent}/${op.stepTotal}`
      : "";
  if (op.kind === "rebase") {
    const src = op.sourceLabel || op.currentBranch || "当前分支";
    const dst = op.targetLabel || "目标基底";
    return `正在进行 ${kindText}${stepText}：将 ${src} 变基到 ${dst} 之上`;
  }
  if (op.kind === "merge") {
    const src = op.sourceLabel || "来源分支";
    const dst = op.targetLabel || op.currentBranch || "当前分支";
    return `正在进行 ${kindText}：将 ${src} 合入 ${dst}`;
  }
  if (op.kind === "cherry-pick" || op.kind === "cherry_pick") {
    const src = op.sourceLabel || op.sourceOid?.slice(0, 7) || "目标提交";
    return `正在进行 ${kindText}${stepText}：应用提交 ${src}`;
  }
  if (op.kind === "revert") {
    const src = op.sourceLabel || op.sourceOid?.slice(0, 7) || "目标提交";
    return `正在进行 ${kindText}${stepText}：还原提交 ${src}`;
  }
  return `正在进行 ${kindText}${stepText}`;
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
