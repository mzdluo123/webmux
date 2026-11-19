import { m } from "$lib/paraglide/messages.js";
import type {
  GitConflictFileDetail,
  GitConflictKind,
  GitConflictSide,
} from "../../contracts/schemas";
import {
  buildSegmentLinesMap,
  normalizeConflictLineEndings,
  patchConflictDraftSegments,
} from "./git-conflict-slice";

export { normalizeConflictLineEndings };

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
  segmentLines?: Record<string, string[]>;
}

export function serializeConflictIdentity(
  identity: GitConflictFileDetail["identity"],
): string {
  const entries = Object.entries(identity)
    .filter(([, value]) => value !== undefined)
    .sort(([a], [b]) => a.localeCompare(b));
  return JSON.stringify(entries);
}

export function cloneConflictDraftState(
  source: GitConflictDraftState,
): GitConflictDraftState {
  return {
    choices: { ...source.choices },
    appliedNonConflicting: { ...source.appliedNonConflicting },
    manualEdited: source.manualEdited,
    resultText: source.resultText,
    savedResultText: source.savedResultText,
    ...(source.segmentLines
      ? {
          segmentLines: Object.fromEntries(
            Object.entries(source.segmentLines).map(([k, lines]) => [
              k,
              [...lines],
            ]),
          ),
        }
      : {}),
  };
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

  const segmentLines = buildSegmentLinesMap(
    detail,
    choices,
    appliedNonConflicting,
  );

  if (resumeSavedDiskDraft && detail.diskText !== undefined) {
    return {
      choices,
      appliedNonConflicting,
      manualEdited: true,
      resultText: detail.diskText,
      savedResultText: detail.diskText,
      segmentLines,
    };
  }

  return {
    choices,
    appliedNonConflicting,
    manualEdited: false,
    resultText: detail.initialResultText,
    savedResultText: null,
    segmentLines,
  };
}

export function applyConflictChunkChoice(
  detail: GitConflictFileDetail,
  draft: GitConflictDraftState,
  segmentId: string,
  choice: GitConflictChunkChoice,
): GitConflictDraftState {
  const nextChoices = { ...draft.choices, [segmentId]: choice };
  const patched = patchConflictDraftSegments(
    detail,
    draft,
    nextChoices,
    draft.appliedNonConflicting,
  );
  return {
    ...draft,
    choices: nextChoices,
    resultText: patched.resultText,
    segmentLines: patched.segmentLines,
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
  const patched = patchConflictDraftSegments(
    detail,
    draft,
    draft.choices,
    nextApplied,
  );
  return {
    ...draft,
    appliedNonConflicting: nextApplied,
    resultText: patched.resultText,
    segmentLines: patched.segmentLines,
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
  const patched = patchConflictDraftSegments(
    detail,
    draft,
    draft.choices,
    nextApplied,
  );
  return {
    ...draft,
    appliedNonConflicting: nextApplied,
    resultText: patched.resultText,
    segmentLines: patched.segmentLines,
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
    normalizeConflictLineEndings(draft.savedResultText, detail) ===
      normalizeConflictLineEndings(draft.resultText, detail);
  const hasUnsavedDraftChanges =
    draft.savedResultText === null
      ? draft.manualEdited ||
        draft.resultText !== detail.initialResultText ||
        unresolvedConflicts < conflictSegs.length ||
        pendingNonConflicting < nonConflictSegs.length
      : !hasSavedToDisk;

  let markResolvedBlockedReason = "";
  if (!detail.textMergeable) {
    markResolvedBlockedReason = m.s7_conflict_blocked_non_text();
  } else if (hasConflictMarkers) {
    markResolvedBlockedReason = m.s7_conflict_blocked_markers();
  } else if (unresolvedConflicts > 0) {
    markResolvedBlockedReason = m.s7_conflict_blocked_unresolved({
      count: String(unresolvedConflicts),
    });
  } else if (!hasSavedToDisk) {
    markResolvedBlockedReason = m.s7_conflict_blocked_unsaved();
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
      return m.s7_conflict_kind_text();
    case "binary":
      return m.s7_conflict_kind_binary();
    case "delete_modify":
      return m.s7_conflict_kind_delete_modify();
    case "submodule":
      return m.s7_conflict_kind_submodule();
    case "symlink":
      return m.s7_conflict_kind_symlink();
    case "too_large":
      return m.s7_conflict_kind_too_large();
  }
}

export function formatConflictSideMeta(side: GitConflictSide): string {
  if (!side.present) return m.s7_conflict_side_absent();
  const parts: string[] = [];
  if (side.mode) parts.push(`mode ${side.mode}`);
  if (side.blobOid) parts.push(side.blobOid.slice(0, 8));
  parts.push(`${side.sizeBytes} B`);
  return parts.join(" · ");
}
