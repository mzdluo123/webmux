import type {
  GitConflictFileDetail,
  GitConflictKind,
  GitConflictSegment,
  GitConflictSide,
} from "../../contracts/schemas";

export type GitConflictChunkChoice =
  | "unresolved"
  | "left"
  | "right"
  | "both_lr"
  | "both_rl"
  | "manual";

export type GitBaseCompareMode = "off" | "left_base" | "right_base";

export interface GitConflictDraftState {
  choices: Record<string, GitConflictChunkChoice>;
  appliedNonConflicting: Record<string, boolean>;
  manualEdited: boolean;
  resultText: string;
  savedResultText: string | null;
}

export function detectTextConflictMarkers(text: string): boolean {
  if (!text) return false;
  let hasOpen = false;
  let hasSep = false;
  for (const rawLine of text.split(/\r?\n/)) {
    if (rawLine.startsWith("<<<<<<< ") || rawLine === "<<<<<<<") {
      hasOpen = true;
    } else if (hasOpen && rawLine === "=======") {
      hasSep = true;
    } else if (
      hasOpen &&
      hasSep &&
      (rawLine.startsWith(">>>>>>> ") || rawLine === ">>>>>>>")
    ) {
      return true;
    }
  }
  return false;
}

function resolvePreferredLineEnding(
  detail: GitConflictFileDetail,
): "\n" | "\r\n" {
  if (
    detail.left.lineEnding === "crlf" ||
    detail.right.lineEnding === "crlf" ||
    detail.base.lineEnding === "crlf"
  ) {
    return "\r\n";
  }
  return "\n";
}

function prefersTrailingNewline(detail: GitConflictFileDetail): boolean {
  for (const candidate of [
    detail.left.text,
    detail.right.text,
    detail.base.text,
    detail.initialResultText,
  ]) {
    if (candidate && candidate.length > 0) {
      return candidate.endsWith("\n");
    }
  }
  return true;
}

function linesForSegmentState(
  seg: GitConflictSegment,
  choices: Readonly<Record<string, GitConflictChunkChoice>>,
  appliedNonConflicting: Readonly<Record<string, boolean>>,
): string[] {
  switch (seg.kind) {
    case "common":
      return seg.baseLines;
    case "change_left":
      return appliedNonConflicting[seg.id] ? seg.leftLines : seg.baseLines;
    case "change_right":
      return appliedNonConflicting[seg.id] ? seg.rightLines : seg.baseLines;
    case "change_both_same":
      return appliedNonConflicting[seg.id] ? seg.leftLines : seg.baseLines;
    case "conflict": {
      const choice = choices[seg.id] ?? "unresolved";
      switch (choice) {
        case "left":
          return seg.leftLines;
        case "right":
          return seg.rightLines;
        case "both_lr":
          return [...seg.leftLines, ...seg.rightLines];
        case "both_rl":
          return [...seg.rightLines, ...seg.leftLines];
        case "unresolved":
        case "manual":
          return seg.baseLines;
      }
    }
  }
}

function assembleConflictResultText(
  detail: GitConflictFileDetail,
  choices: Readonly<Record<string, GitConflictChunkChoice>>,
  appliedNonConflicting: Readonly<Record<string, boolean>>,
): string {
  const eol = resolvePreferredLineEnding(detail);
  const lines: string[] = [];
  for (const seg of detail.segments) {
    lines.push(...linesForSegmentState(seg, choices, appliedNonConflicting));
  }
  if (lines.length === 0) return "";
  const body = lines.join(eol);
  return prefersTrailingNewline(detail) ? `${body}${eol}` : body;
}

export function createInitialConflictDraftState(
  detail: GitConflictFileDetail,
): GitConflictDraftState {
  const choices: Record<string, GitConflictChunkChoice> = {};
  const appliedNonConflicting: Record<string, boolean> = {};

  const resumeSavedDiskDraft =
    detail.textMergeable &&
    detail.diskModifiedFromBase &&
    !detail.diskHasConflictMarker &&
    detail.diskText !== undefined;

  for (const seg of detail.segments) {
    if (seg.kind === "conflict") {
      choices[seg.id] = resumeSavedDiskDraft ? "manual" : "unresolved";
    } else if (seg.kind !== "common") {
      appliedNonConflicting[seg.id] = resumeSavedDiskDraft;
    }
  }

  if (resumeSavedDiskDraft && detail.diskText !== undefined) {
    return {
      choices,
      appliedNonConflicting,
      manualEdited: true,
      resultText: detail.diskText,
      savedResultText: detail.diskText,
    };
  }

  return {
    choices,
    appliedNonConflicting,
    manualEdited: false,
    resultText: detail.initialResultText,
    savedResultText: null,
  };
}

function replaceSegmentSliceInResultText(
  detail: GitConflictFileDetail,
  prevResultText: string,
  prevChoices: Readonly<Record<string, GitConflictChunkChoice>>,
  prevApplied: Readonly<Record<string, boolean>>,
  nextChoices: Readonly<Record<string, GitConflictChunkChoice>>,
  nextApplied: Readonly<Record<string, boolean>>,
  manualEdited: boolean,
): string {
  if (!manualEdited) {
    return assembleConflictResultText(detail, nextChoices, nextApplied);
  }
  // If user made manual edits, rebuild from segments when line counts still match
  // or fall back to clean segment assembly when choosing a structured chunk action.
  const prevAssembled = assembleConflictResultText(
    detail,
    prevChoices,
    prevApplied,
  );
  if (prevResultText === prevAssembled) {
    return assembleConflictResultText(detail, nextChoices, nextApplied);
  }
  const eol = resolvePreferredLineEnding(detail);
  const trailing = prevResultText.endsWith("\n");
  const rawLines = prevResultText.split(/\r?\n/);
  if (trailing && rawLines[rawLines.length - 1] === "") {
    rawLines.pop();
  }
  let cursor = 0;
  const patched: string[] = [];
  let offsetAligned = true;
  for (const seg of detail.segments) {
    const oldLines = linesForSegmentState(seg, prevChoices, prevApplied);
    const newLines = linesForSegmentState(seg, nextChoices, nextApplied);
    if (cursor + oldLines.length > rawLines.length) {
      offsetAligned = false;
      break;
    }
    const changed =
      (prevChoices[seg.id] ?? "unresolved") !==
        (nextChoices[seg.id] ?? "unresolved") ||
      Boolean(prevApplied[seg.id]) !== Boolean(nextApplied[seg.id]);
    if (changed) {
      patched.push(...newLines);
    } else {
      patched.push(...rawLines.slice(cursor, cursor + oldLines.length));
    }
    cursor += oldLines.length;
  }
  if (!offsetAligned) {
    return assembleConflictResultText(detail, nextChoices, nextApplied);
  }
  if (cursor < rawLines.length) {
    patched.push(...rawLines.slice(cursor));
  }
  if (patched.length === 0) return "";
  const joined = patched.join(eol);
  return trailing ? `${joined}${eol}` : joined;
}

export function applyConflictChunkChoice(
  detail: GitConflictFileDetail,
  draft: GitConflictDraftState,
  segmentId: string,
  choice: GitConflictChunkChoice,
): GitConflictDraftState {
  const nextChoices = { ...draft.choices, [segmentId]: choice };
  const resultText = replaceSegmentSliceInResultText(
    detail,
    draft.resultText,
    draft.choices,
    draft.appliedNonConflicting,
    nextChoices,
    draft.appliedNonConflicting,
    draft.manualEdited,
  );
  return {
    ...draft,
    choices: nextChoices,
    resultText,
  };
}

export function toggleNonConflictingSegment(
  detail: GitConflictFileDetail,
  draft: GitConflictDraftState,
  segmentId: string,
  applied?: boolean,
): GitConflictDraftState {
  const nextVal = applied ?? !draft.appliedNonConflicting[segmentId];
  const nextApplied = { ...draft.appliedNonConflicting, [segmentId]: nextVal };
  const resultText = replaceSegmentSliceInResultText(
    detail,
    draft.resultText,
    draft.choices,
    draft.appliedNonConflicting,
    draft.choices,
    nextApplied,
    draft.manualEdited,
  );
  return {
    ...draft,
    appliedNonConflicting: nextApplied,
    resultText,
  };
}

export function applyAllNonConflictingSegments(
  detail: GitConflictFileDetail,
  draft: GitConflictDraftState,
): GitConflictDraftState {
  const nextApplied = { ...draft.appliedNonConflicting };
  for (const seg of detail.segments) {
    if (
      seg.kind === "change_left" ||
      seg.kind === "change_right" ||
      seg.kind === "change_both_same"
    ) {
      nextApplied[seg.id] = true;
    }
  }
  const resultText = replaceSegmentSliceInResultText(
    detail,
    draft.resultText,
    draft.choices,
    draft.appliedNonConflicting,
    draft.choices,
    nextApplied,
    draft.manualEdited,
  );
  return {
    ...draft,
    appliedNonConflicting: nextApplied,
    resultText,
  };
}

export function markAllUnresolvedAsManual(
  draft: GitConflictDraftState,
): GitConflictDraftState {
  const nextChoices = { ...draft.choices };
  for (const [segId, choice] of Object.entries(nextChoices)) {
    if (choice === "unresolved") {
      nextChoices[segId] = "manual";
    }
  }
  return {
    ...draft,
    choices: nextChoices,
  };
}

export interface GitConflictProgressSummary {
  totalConflicts: number;
  unresolvedConflicts: number;
  totalNonConflicting: number;
  pendingNonConflicting: number;
  hasConflictMarkers: boolean;
  hasSavedToDisk: boolean;
  hasUnsavedDraftChanges: boolean;
  canSaveResult: boolean;
  canMarkResolved: boolean;
  markResolvedBlockedReason: string;
}

export function summarizeConflictDraftProgress(
  detail: GitConflictFileDetail,
  draft: GitConflictDraftState,
): GitConflictProgressSummary {
  const conflictSegs = detail.segments.filter((s) => s.kind === "conflict");
  const nonConflictSegs = detail.segments.filter(
    (s) =>
      s.kind === "change_left" ||
      s.kind === "change_right" ||
      s.kind === "change_both_same",
  );
  const unresolvedConflicts = conflictSegs.filter(
    (s) => (draft.choices[s.id] ?? "unresolved") === "unresolved",
  ).length;
  const pendingNonConflicting = nonConflictSegs.filter(
    (s) => !draft.appliedNonConflicting[s.id],
  ).length;
  const hasConflictMarkers = detectTextConflictMarkers(draft.resultText);
  const hasSavedToDisk =
    draft.savedResultText !== null &&
    draft.savedResultText === draft.resultText;
  const hasUnsavedDraftChanges =
    draft.savedResultText === null
      ? draft.resultText !== detail.initialResultText ||
        unresolvedConflicts < conflictSegs.length
      : draft.savedResultText !== draft.resultText;

  let markResolvedBlockedReason = "";
  if (!detail.textMergeable) {
    markResolvedBlockedReason =
      "非文本冲突请使用保留左侧、保留右侧或删除文件按钮解决";
  } else if (hasConflictMarkers) {
    markResolvedBlockedReason =
      "结果文本中仍包含冲突标记（<<<<<<< / ======= / >>>>>>>），请先清除";
  } else if (unresolvedConflicts > 0) {
    markResolvedBlockedReason = `仍有 ${unresolvedConflicts} 处冲突块未决策（接受一侧或确认手动合并）`;
  } else if (!hasSavedToDisk) {
    markResolvedBlockedReason =
      "请先点击「保存结果」将合并文本写入工作树，再标记已解决";
  }

  return {
    totalConflicts: conflictSegs.length,
    unresolvedConflicts,
    totalNonConflicting: nonConflictSegs.length,
    pendingNonConflicting,
    hasConflictMarkers,
    hasSavedToDisk,
    hasUnsavedDraftChanges,
    canSaveResult: detail.textMergeable && !hasConflictMarkers,
    canMarkResolved: detail.textMergeable && markResolvedBlockedReason === "",
    markResolvedBlockedReason,
  };
}

export function gitConflictKindLabel(kind: GitConflictKind): string {
  switch (kind) {
    case "text":
      return "文本三方冲突";
    case "binary":
      return "二进制冲突";
    case "delete_modify":
      return "增删修改冲突";
    case "submodule":
      return "子模块冲突";
    case "symlink":
      return "符号链接冲突";
    case "too_large":
      return "超大文件冲突";
  }
}

export function formatConflictSideMeta(side: GitConflictSide): string {
  if (!side.present) return "此侧不存在（已删除或未创建）";
  const parts: string[] = [];
  if (side.mode) parts.push(`mode ${side.mode}`);
  if (side.blobOid) parts.push(side.blobOid.slice(0, 8));
  parts.push(`${side.sizeBytes} B`);
  return parts.join(" · ");
}
