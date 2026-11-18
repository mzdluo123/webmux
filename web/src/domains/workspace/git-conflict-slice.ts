import type {
  GitConflictFileDetail,
  GitConflictSegment,
} from "../../contracts/schemas";

type ChunkChoice =
  | "unresolved"
  | "left"
  | "right"
  | "both_lr"
  | "both_rl"
  | "manual";

interface DraftSliceSnapshot {
  choices: Readonly<Record<string, ChunkChoice>>;
  appliedNonConflicting: Readonly<Record<string, boolean>>;
  manualEdited: boolean;
  resultText: string;
  segmentLines?: Readonly<Record<string, readonly string[]>>;
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

export function normalizeConflictLineEndings(
  text: string,
  detail: GitConflictFileDetail,
): string {
  if (!text || resolvePreferredLineEnding(detail) !== "\r\n") return text;
  return text.replace(/\r?\n/g, "\r\n");
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
  choices: Readonly<Record<string, ChunkChoice>>,
  appliedNonConflicting: Readonly<Record<string, boolean>>,
): string[] {
  switch (seg.kind) {
    case "common":
      return [...seg.baseLines];
    case "change_left":
      return [
        ...(appliedNonConflicting[seg.id] ? seg.leftLines : seg.baseLines),
      ];
    case "change_right":
      return [
        ...(appliedNonConflicting[seg.id] ? seg.rightLines : seg.baseLines),
      ];
    case "change_both_same":
      return [
        ...(appliedNonConflicting[seg.id] ? seg.leftLines : seg.baseLines),
      ];
    case "conflict": {
      const choice = choices[seg.id] ?? "unresolved";
      switch (choice) {
        case "left":
          return [...seg.leftLines];
        case "right":
          return [...seg.rightLines];
        case "both_lr":
          return [...seg.leftLines, ...seg.rightLines];
        case "both_rl":
          return [...seg.rightLines, ...seg.leftLines];
        case "unresolved":
        case "manual":
          return [...seg.baseLines];
      }
    }
  }
}

export function buildSegmentLinesMap(
  detail: GitConflictFileDetail,
  choices: Readonly<Record<string, ChunkChoice>>,
  appliedNonConflicting: Readonly<Record<string, boolean>>,
): Record<string, string[]> {
  const map: Record<string, string[]> = {};
  for (const seg of detail.segments) {
    map[seg.id] = linesForSegmentState(seg, choices, appliedNonConflicting);
  }
  return map;
}

function assembleFromSegmentLines(
  detail: GitConflictFileDetail,
  segmentLines: Readonly<Record<string, readonly string[]>>,
): string {
  const eol = resolvePreferredLineEnding(detail);
  const lines: string[] = [];
  for (const seg of detail.segments) {
    const segLines = segmentLines[seg.id];
    if (segLines) lines.push(...segLines);
  }
  if (lines.length === 0) return "";
  const body = lines.join(eol);
  return prefersTrailingNewline(detail) ? `${body}${eol}` : body;
}

function hasNonBlankLine(lines: readonly string[]): boolean {
  return lines.some((line) => line.trim().length > 0);
}

function sliceMatchesAt(
  haystack: readonly string[],
  start: number,
  needle: readonly string[],
): boolean {
  if (start < 0 || start + needle.length > haystack.length) return false;
  for (let i = 0; i < needle.length; i++) {
    if (haystack[start + i] !== needle[i]) return false;
  }
  return true;
}

function countPrefixContextMatch(
  lines: readonly string[],
  startIdx: number,
  prefixLines: readonly string[],
): number {
  const maxLen = Math.min(startIdx, prefixLines.length);
  let matched = 0;
  while (
    matched < maxLen &&
    lines[startIdx - 1 - matched] ===
      prefixLines[prefixLines.length - 1 - matched]
  ) {
    matched++;
  }
  return matched;
}

function countSuffixContextMatch(
  lines: readonly string[],
  endIdx: number,
  suffixLines: readonly string[],
): number {
  const maxLen = Math.min(lines.length - endIdx, suffixLines.length);
  let matched = 0;
  while (matched < maxLen && lines[endIdx + matched] === suffixLines[matched]) {
    matched++;
  }
  return matched;
}

function findUniquePrefixBoundary(
  lines: readonly string[],
  prefixLines: readonly string[],
): number | null {
  let maxPre = 0;
  let bestIdx: number | null = null;
  let tied = false;
  for (let s = 0; s <= lines.length; s++) {
    const pre = countPrefixContextMatch(lines, s, prefixLines);
    if (pre > maxPre) {
      maxPre = pre;
      bestIdx = s;
      tied = false;
    } else if (pre > 0 && pre === maxPre) {
      tied = true;
    }
  }
  if (
    maxPre === 0 ||
    tied ||
    bestIdx === null ||
    !hasNonBlankLine(prefixLines.slice(prefixLines.length - maxPre))
  ) {
    return null;
  }
  return bestIdx;
}

function findUniqueSuffixBoundary(
  lines: readonly string[],
  suffixLines: readonly string[],
  minIdx: number,
): number | null {
  let maxSuf = 0;
  let bestIdx: number | null = null;
  let tied = false;
  for (let e = minIdx; e <= lines.length; e++) {
    const suf = countSuffixContextMatch(lines, e, suffixLines);
    if (suf > maxSuf) {
      maxSuf = suf;
      bestIdx = e;
      tied = false;
    } else if (suf > 0 && suf === maxSuf) {
      tied = true;
    }
  }
  if (
    maxSuf === 0 ||
    tied ||
    bestIdx === null ||
    !hasNonBlankLine(suffixLines.slice(0, maxSuf))
  ) {
    return null;
  }
  return bestIdx;
}

function locateTargetSegmentSlice(
  lines: readonly string[],
  segments: readonly GitConflictSegment[],
  targetIdx: number,
  currentSegLines: Readonly<Record<string, readonly string[]>>,
): [number, number] | null {
  const targetSeg = segments[targetIdx];
  if (!targetSeg) return null;

  const oldLines = currentSegLines[targetSeg.id] ?? [];
  const prefixLines: string[] = [];
  for (let i = 0; i < targetIdx; i++) {
    prefixLines.push(...(currentSegLines[segments[i]!.id] ?? []));
  }
  const suffixLines: string[] = [];
  for (let i = targetIdx + 1; i < segments.length; i++) {
    suffixLines.push(...(currentSegLines[segments[i]!.id] ?? []));
  }

  if (oldLines.length > 0) {
    const candidates: number[] = [];
    for (let s = 0; s <= lines.length - oldLines.length; s++) {
      if (sliceMatchesAt(lines, s, oldLines)) {
        candidates.push(s);
      }
    }

    if (candidates.length === 1) {
      const s = candidates[0]!;
      const e = s + oldLines.length;
      if (hasNonBlankLine(oldLines)) return [s, e];
      const boundaryConfirmed =
        (prefixLines.length === 0 && s === 0) ||
        (suffixLines.length === 0 && e === lines.length) ||
        countPrefixContextMatch(lines, s, prefixLines) > 0 ||
        countSuffixContextMatch(lines, e, suffixLines) > 0;
      return boundaryConfirmed ? [s, e] : null;
    }

    if (candidates.length > 1) {
      let maxScore = 0;
      let bestStart: number | null = null;
      let tied = false;
      for (const s of candidates) {
        const e = s + oldLines.length;
        const score =
          countPrefixContextMatch(lines, s, prefixLines) +
          countSuffixContextMatch(lines, e, suffixLines) +
          (prefixLines.length === 0 && s === 0 ? 1 : 0) +
          (suffixLines.length === 0 && e === lines.length ? 1 : 0);
        if (score > maxScore) {
          maxScore = score;
          bestStart = s;
          tied = false;
        } else if (score > 0 && score === maxScore) {
          tied = true;
        }
      }
      return !tied && bestStart !== null
        ? [bestStart, bestStart + oldLines.length]
        : null;
    }
  } else {
    if (prefixLines.length === 0 && suffixLines.length === 0) {
      return lines.length === 0 ? [0, 0] : null;
    }
    let bestScore = 0;
    let bestPos: number | null = null;
    let tied = false;
    for (let s = 0; s <= lines.length; s++) {
      const pre = countPrefixContextMatch(lines, s, prefixLines);
      const suf = countSuffixContextMatch(lines, s, suffixLines);
      const leftOk = (prefixLines.length === 0 && s === 0) || pre > 0;
      const rightOk =
        (suffixLines.length === 0 && s === lines.length) || suf > 0;
      if (!leftOk || !rightOk) continue;
      const score =
        pre +
        suf +
        (prefixLines.length === 0 && s === 0 ? 1 : 0) +
        (suffixLines.length === 0 && s === lines.length ? 1 : 0);
      if (score > bestScore) {
        bestScore = score;
        bestPos = s;
        tied = false;
      } else if (score === bestScore) {
        tied = true;
      }
    }
    if (!tied && bestPos !== null) return [bestPos, bestPos];

    const prePos =
      prefixLines.length > 0
        ? findUniquePrefixBoundary(lines, prefixLines)
        : null;
    const sufPos =
      suffixLines.length > 0
        ? findUniqueSuffixBoundary(lines, suffixLines, 0)
        : null;
    if (prePos !== null && sufPos === null) return [prePos, prePos];
    if (sufPos !== null && prePos === null) return [sufPos, sufPos];
    if (prePos !== null && sufPos !== null && prePos <= sufPos) {
      return [prePos, sufPos];
    }
    return null;
  }

  let startIdx: number;
  let anchoredLeft = false;
  if (prefixLines.length === 0) {
    startIdx = 0;
  } else {
    const found = findUniquePrefixBoundary(lines, prefixLines);
    if (found === null) return null;
    startIdx = found;
    anchoredLeft = true;
  }

  let endIdx: number;
  let anchoredRight = false;
  if (suffixLines.length === 0) {
    endIdx = lines.length;
  } else {
    const found = findUniqueSuffixBoundary(lines, suffixLines, startIdx);
    if (found === null) return null;
    endIdx = found;
    anchoredRight = true;
  }

  if ((!anchoredLeft && !anchoredRight) || startIdx > endIdx) {
    return null;
  }
  return [startIdx, endIdx];
}

export function patchConflictDraftSegments(
  detail: GitConflictFileDetail,
  prev: DraftSliceSnapshot,
  nextChoices: Readonly<Record<string, ChunkChoice>>,
  nextApplied: Readonly<Record<string, boolean>>,
): {
  resultText: string;
  segmentLines: Record<string, string[]>;
} {
  const prevSegLines: Record<string, string[]> = {};
  for (const seg of detail.segments) {
    const tracked = prev.segmentLines?.[seg.id];
    prevSegLines[seg.id] = tracked
      ? [...tracked]
      : linesForSegmentState(seg, prev.choices, prev.appliedNonConflicting);
  }
  const targetSegLines = buildSegmentLinesMap(detail, nextChoices, nextApplied);

  if (
    !prev.manualEdited ||
    prev.resultText === assembleFromSegmentLines(detail, prevSegLines)
  ) {
    return {
      resultText: assembleFromSegmentLines(detail, targetSegLines),
      segmentLines: targetSegLines,
    };
  }

  const changedIndices: number[] = [];
  for (let i = 0; i < detail.segments.length; i++) {
    const seg = detail.segments[i]!;
    const choiceChanged =
      (prev.choices[seg.id] ?? "unresolved") !==
      (nextChoices[seg.id] ?? "unresolved");
    const appliedChanged =
      Boolean(prev.appliedNonConflicting[seg.id]) !==
      Boolean(nextApplied[seg.id]);
    if (choiceChanged || appliedChanged) {
      changedIndices.push(i);
    }
  }
  if (changedIndices.length === 0) {
    return {
      resultText: normalizeConflictLineEndings(prev.resultText, detail),
      segmentLines: prevSegLines,
    };
  }

  const preferredEol = resolvePreferredLineEnding(detail);
  const eol =
    preferredEol === "\r\n"
      ? "\r\n"
      : prev.resultText.includes("\r\n")
        ? "\r\n"
        : preferredEol;
  const hadContent = prev.resultText.length > 0;
  const trailing = hadContent
    ? prev.resultText.endsWith("\n")
    : prefersTrailingNewline(detail);
  const lines = hadContent ? prev.resultText.split(/\r?\n/) : [];
  if (trailing && lines[lines.length - 1] === "") {
    lines.pop();
  }

  const workingSegLines: Record<string, string[]> = { ...prevSegLines };
  let anyReplaced = false;
  for (const idx of changedIndices) {
    const seg = detail.segments[idx]!;
    const range = locateTargetSegmentSlice(
      lines,
      detail.segments,
      idx,
      workingSegLines,
    );
    if (!range) continue;
    const replacement = targetSegLines[seg.id] ?? [];
    lines.splice(range[0], range[1] - range[0], ...replacement);
    workingSegLines[seg.id] = replacement;
    anyReplaced = true;
  }

  if (!anyReplaced) {
    return {
      resultText: normalizeConflictLineEndings(prev.resultText, detail),
      segmentLines: workingSegLines,
    };
  }
  if (lines.length === 0) {
    return { resultText: "", segmentLines: workingSegLines };
  }
  const joined = lines.join(eol);
  return {
    resultText: normalizeConflictLineEndings(
      trailing ? `${joined}${eol}` : joined,
      detail,
    ),
    segmentLines: workingSegLines,
  };
}
