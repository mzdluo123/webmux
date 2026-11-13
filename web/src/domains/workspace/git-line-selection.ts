import type { SelectedLineRange } from "@pierre/diffs";
import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";

export interface GitHunkAndLineMutationProps {
  onStageHunk: (changeId: string, hunkId: string) => void;
  onUnstageHunk: (changeId: string, hunkId: string) => void;
  onStageLines?: (
    changeId: string,
    selections: readonly GitLineSelection[],
  ) => void;
  onUnstageLines?: (
    changeId: string,
    selections: readonly GitLineSelection[],
  ) => void;
  onDiscardHunk?: (changeId: string, hunkId: string) => void;
  onDiscardLines?: (
    changeId: string,
    selections: readonly GitLineSelection[],
  ) => void;
}
import type { GitLineSelection } from "../../contracts/schemas";

export type GitHunk = GitWorkingChange["hunks"][number];
export type GitHunkLine = GitHunk["lines"][number];

export type HunkLineSelectionMap = Record<string, number[]>;

export function isChangedHunkLine(line: GitHunkLine): boolean {
  return line.kind === "delete" || line.kind === "add";
}

export function changedLineIndicesInHunk(hunk: GitHunk): number[] {
  const out: number[] = [];
  for (let i = 0; i < hunk.lines.length; i++) {
    const line = hunk.lines[i];
    if (line && isChangedHunkLine(line)) out.push(i);
  }
  return out;
}

export function toggleHunkLineSelection(params: {
  current: HunkLineSelectionMap;
  hunk: GitHunk;
  lineIndex: number;
  anchorIndex: number | null;
  shiftKey?: boolean;
}): { next: HunkLineSelectionMap; nextAnchor: number } {
  const { current, hunk, lineIndex, anchorIndex, shiftKey = false } = params;
  const targetLine = hunk.lines[lineIndex];
  if (!targetLine || !isChangedHunkLine(targetLine)) {
    return { next: current, nextAnchor: anchorIndex ?? lineIndex };
  }

  const existing = new Set(current[hunk.id] ?? []);
  if (shiftKey && anchorIndex !== null && anchorIndex !== lineIndex) {
    const start = Math.min(anchorIndex, lineIndex);
    const end = Math.max(anchorIndex, lineIndex);
    const shouldSelect = !existing.has(lineIndex);
    for (let i = start; i <= end; i++) {
      const candidate = hunk.lines[i];
      if (!candidate || !isChangedHunkLine(candidate)) continue;
      if (shouldSelect) existing.add(i);
      else existing.delete(i);
    }
  } else if (existing.has(lineIndex)) {
    existing.delete(lineIndex);
  } else {
    existing.add(lineIndex);
  }

  const sorted = [...existing].sort((a, b) => a - b);
  const next: HunkLineSelectionMap = { ...current };
  if (sorted.length === 0) {
    delete next[hunk.id];
  } else {
    next[hunk.id] = sorted;
  }
  return { next, nextAnchor: lineIndex };
}

export function selectAllChangedInHunk(
  current: HunkLineSelectionMap,
  hunk: GitHunk,
): HunkLineSelectionMap {
  const all = changedLineIndicesInHunk(hunk);
  if (all.length === 0) return current;
  const existing = current[hunk.id] ?? [];
  const next: HunkLineSelectionMap = { ...current };
  if (existing.length === all.length) {
    delete next[hunk.id];
  } else {
    next[hunk.id] = all;
  }
  return next;
}

export function toGitLineSelections(
  map: HunkLineSelectionMap,
  hunks: readonly GitHunk[],
): GitLineSelection[] {
  const out: GitLineSelection[] = [];
  for (const hunk of hunks) {
    const raw = map[hunk.id];
    if (!raw || raw.length === 0) continue;
    const valid = raw
      .filter((idx) => {
        const line = hunk.lines[idx];
        return Boolean(line && isChangedHunkLine(line));
      })
      .sort((a, b) => a - b);
    if (valid.length > 0) {
      out.push({ hunkId: hunk.id, lineIndices: valid });
    }
  }
  return out;
}

export function countSelectedLines(
  selections: readonly GitLineSelection[],
): number {
  return selections.reduce((sum, sel) => sum + sel.lineIndices.length, 0);
}

/**
 * Maps a CodeDiffReader line range selection into matching changed lines inside
 * `hunks` (matching `oldLine` on deletions side or `newLine` on additions side,
 * plus paired adjacent changed lines when selecting a contiguous diff span).
 */
export function mapDiffRangeToLineSelections(
  hunks: readonly GitHunk[],
  range: SelectedLineRange | null,
): GitLineSelection[] {
  if (!range || hunks.length === 0) return [];
  const side: "old" | "new" = range.side === "deletions" ? "old" : "new";
  const start = Math.min(range.start, range.end);
  const end = Math.max(range.start, range.end);
  const out: GitLineSelection[] = [];

  for (const hunk of hunks) {
    const indices: number[] = [];
    for (let i = 0; i < hunk.lines.length; i++) {
      const line = hunk.lines[i];
      if (!line || !isChangedHunkLine(line)) continue;
      if (side === "old") {
        if (
          line.kind === "delete" &&
          typeof line.oldLine === "number" &&
          line.oldLine >= start &&
          line.oldLine <= end
        ) {
          indices.push(i);
        }
      } else if (
        line.kind === "add" &&
        typeof line.newLine === "number" &&
        line.newLine >= start &&
        line.newLine <= end
      ) {
        indices.push(i);
      }
    }
    if (indices.length > 0) {
      out.push({ hunkId: hunk.id, lineIndices: indices });
    }
  }
  return out;
}

function splitHunkBySelectedIndices(
  hunk: GitHunk,
  selectedIndices: ReadonlySet<number>,
  direction: "forward" | "backward",
): { appliedHunk: GitHunk | null; remainingHunk: GitHunk | null } {
  const appliedLines: GitHunkLine[] = [];
  const remainingLines: GitHunkLine[] = [];
  let appliedChanged = 0;
  let remainingChanged = 0;

  for (let i = 0; i < hunk.lines.length; i++) {
    const line = hunk.lines[i]!;
    if (!isChangedHunkLine(line)) {
      appliedLines.push(line);
      remainingLines.push(line);
      continue;
    }
    const chosen = selectedIndices.has(i);
    if (chosen) {
      appliedLines.push(line);
      appliedChanged++;
      if (direction === "forward") {
        // After staging: `+` is now context in index, `-` is gone from index
        if (line.kind === "add") {
          remainingLines.push({ ...line, kind: "context" });
        }
      } else {
        // After unstaging/discarding: `-` returns as context, `+` is removed
        if (line.kind === "delete") {
          remainingLines.push({ ...line, kind: "context" });
        }
      }
    } else {
      remainingLines.push(line);
      remainingChanged++;
      if (direction === "forward") {
        // Unselected `-` remains in index as context; unselected `+` not in index yet
        if (line.kind === "delete") {
          appliedLines.push({ ...line, kind: "context" });
        }
      } else {
        if (line.kind === "add") {
          appliedLines.push({ ...line, kind: "context" });
        }
      }
    }
  }

  return {
    appliedHunk:
      appliedChanged > 0
        ? { ...hunk, id: `${hunk.id}:sel`, lines: appliedLines }
        : null,
    remainingHunk:
      remainingChanged > 0
        ? { ...hunk, id: `${hunk.id}:rem`, lines: remainingLines }
        : null,
  };
}

/**
 * Pure offline simulation of sub-hunk line stage / unstage / discard for
 * `GitWorkbenchShell` and Storybook interactive fixtures.
 */
export function applyOfflineLineMutation(
  changes: readonly GitWorkingChange[],
  changeId: string,
  selections: readonly GitLineSelection[],
  op: "stage" | "unstage" | "discard",
): GitWorkingChange[] {
  const target = changes.find((c) => c.id === changeId);
  if (!target || selections.length === 0) return [...changes];
  const byHunk = new Map(
    selections.map((s) => [s.hunkId, new Set(s.lineIndices)]),
  );

  const movedHunks: GitHunk[] = [];
  const remainingHunks: GitHunk[] = [];
  const direction = op === "stage" ? "forward" : "backward";

  for (const hunk of target.hunks) {
    const sel = byHunk.get(hunk.id);
    if (!sel || sel.size === 0) {
      remainingHunks.push(hunk);
      continue;
    }
    const { appliedHunk, remainingHunk } = splitHunkBySelectedIndices(
      hunk,
      sel,
      direction,
    );
    if (appliedHunk) movedHunks.push(appliedHunk);
    if (remainingHunk) remainingHunks.push(remainingHunk);
  }

  if (op === "discard") {
    const next: GitWorkingChange[] = [];
    for (const item of changes) {
      if (item.id !== changeId) {
        next.push(item);
        continue;
      }
      if (remainingHunks.length > 0) {
        next.push({ ...item, hunks: remainingHunks });
      }
    }
    return next;
  }

  const destGroup = op === "stage" ? "staged" : "unstaged";
  const next: GitWorkingChange[] = [];
  let merged = false;

  for (const item of changes) {
    if (item.id === changeId) {
      if (remainingHunks.length > 0) {
        next.push({ ...item, hunks: remainingHunks });
      }
      continue;
    }
    if (item.group === destGroup && item.path === target.path) {
      merged = true;
      next.push({
        ...item,
        hunks:
          op === "stage"
            ? [...item.hunks, ...movedHunks]
            : [...movedHunks, ...item.hunks],
      });
      continue;
    }
    next.push(item);
  }

  if (!merged && movedHunks.length > 0) {
    next.push({
      ...target,
      id: `${destGroup}:${target.path}`,
      group: destGroup,
      status: target.status === "?" ? "A" : target.status,
      preStagedFromCli: false,
      hunks: movedHunks,
    });
  }
  return next;
}

export function applyOfflineDiscardChanges(
  changes: readonly GitWorkingChange[],
  params: {
    scope: "files" | "hunks" | "lines";
    paths?: readonly string[];
    changeId?: string;
    hunkId?: string;
    lineSelections?: readonly GitLineSelection[];
  },
): GitWorkingChange[] {
  if (params.scope === "files") {
    const pathSet = new Set(params.paths ?? []);
    // Strictly preserve staged changes (`group === "staged"`)!
    return changes.filter(
      (c) =>
        c.group === "staged" ||
        c.group === "conflicted" ||
        !pathSet.has(c.path),
    );
  }
  if (params.scope === "hunks" && params.changeId && params.hunkId) {
    const out: GitWorkingChange[] = [];
    for (const item of changes) {
      if (item.id !== params.changeId || item.group === "staged") {
        out.push(item);
        continue;
      }
      const rem = item.hunks.filter((h) => h.id !== params.hunkId);
      if (rem.length > 0) out.push({ ...item, hunks: rem });
    }
    return out;
  }
  if (params.scope === "lines" && params.changeId && params.lineSelections) {
    return applyOfflineLineMutation(
      changes,
      params.changeId,
      params.lineSelections,
      "discard",
    );
  }
  return [...changes];
}
