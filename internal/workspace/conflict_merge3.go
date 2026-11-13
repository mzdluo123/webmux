package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

const (
	SegmentCommon         = "common"
	SegmentChangeLeft     = "change_left"
	SegmentChangeRight    = "change_right"
	SegmentChangeBothSame = "change_both_same"
	SegmentConflict       = "conflict"
)

func detectDominantLineEnding(raw []byte) string {
	if len(raw) == 0 {
		return "lf"
	}
	crlf := strings.Count(string(raw), "\r\n")
	totalLF := strings.Count(string(raw), "\n")
	lfOnly := totalLF - crlf
	switch {
	case crlf == 0 && lfOnly == 0:
		return "none"
	case crlf > 0 && lfOnly == 0:
		return "crlf"
	case crlf == 0 && lfOnly > 0:
		return "lf"
	default:
		return "mixed"
	}
}

func stripRawLineEndings(rawLines []string) []string {
	if len(rawLines) == 0 {
		return []string{}
	}
	out := make([]string, len(rawLines))
	for i, line := range rawLines {
		txt, _ := trimLineEnding(line)
		out[i] = txt
	}
	return out
}

func buildThreeWaySegments(baseBytes, leftBytes, rightBytes []byte) ([]ConflictSegment, string) {
	baseRaw := splitRawLines(baseBytes)
	leftRaw := splitRawLines(leftBytes)
	rightRaw := splitRawLines(rightBytes)

	leftSpans := myersRawDiffSpans(baseRaw, leftRaw)
	rightSpans := myersRawDiffSpans(baseRaw, rightRaw)

	segments := make([]ConflictSegment, 0, len(leftSpans)+len(rightSpans)+1)
	var initialBuilder strings.Builder

	baseIdx, leftIdx, rightIdx := 0, 0, 0
	li, ri := 0, 0

	appendCommon := func(count int) {
		if count <= 0 {
			return
		}
		bSlice := baseRaw[baseIdx : baseIdx+count]
		lSlice := leftRaw[leftIdx : leftIdx+count]
		rSlice := rightRaw[rightIdx : rightIdx+count]
		for _, s := range bSlice {
			initialBuilder.WriteString(s)
		}
		seg := ConflictSegment{
			Kind:       SegmentCommon,
			BaseStart:  baseIdx + 1,
			BaseLines:  stripRawLineEndings(bSlice),
			LeftStart:  leftIdx + 1,
			LeftLines:  stripRawLineEndings(lSlice),
			RightStart: rightIdx + 1,
			RightLines: stripRawLineEndings(rSlice),
		}
		seg.ID = computeConflictSegmentID(len(segments), seg)
		segments = append(segments, seg)
		baseIdx += count
		leftIdx += count
		rightIdx += count
	}

	for li < len(leftSpans) || ri < len(rightSpans) {
		nextBase := len(baseRaw)
		if li < len(leftSpans) && leftSpans[li].oldStart < nextBase {
			nextBase = leftSpans[li].oldStart
		}
		if ri < len(rightSpans) && rightSpans[ri].oldStart < nextBase {
			nextBase = rightSpans[ri].oldStart
		}
		if nextBase > baseIdx {
			appendCommon(nextBase - baseIdx)
		}

		// Determine contiguous/overlapping edit group on Base [groupStart, groupEnd).
		groupStart := baseIdx
		groupEnd := groupStart
		groupLeftStart := leftIdx
		groupRightStart := rightIdx
		tookLeft := false
		tookRight := false

		if li < len(leftSpans) && (ri >= len(rightSpans) || leftSpans[li].oldStart <= rightSpans[ri].oldStart) {
			groupEnd = leftSpans[li].oldStart + leftSpans[li].oldCount
			li++
			tookLeft = true
		} else if ri < len(rightSpans) {
			groupEnd = rightSpans[ri].oldStart + rightSpans[ri].oldCount
			ri++
			tookRight = true
		}

		for {
			expanded := false
			for li < len(leftSpans) {
				sp := leftSpans[li]
				overlaps := sp.oldStart < groupEnd || (sp.oldStart == groupEnd && tookRight && (sp.oldCount == 0 || groupEnd == groupStart))
				if !overlaps {
					break
				}
				if end := sp.oldStart + sp.oldCount; end > groupEnd {
					groupEnd = end
				}
				li++
				tookLeft = true
				expanded = true
			}
			for ri < len(rightSpans) {
				sp := rightSpans[ri]
				overlaps := sp.oldStart < groupEnd || (sp.oldStart == groupEnd && tookLeft && (sp.oldCount == 0 || groupEnd == groupStart))
				if !overlaps {
					break
				}
				if end := sp.oldStart + sp.oldCount; end > groupEnd {
					groupEnd = end
				}
				ri++
				tookRight = true
				expanded = true
			}
			if !expanded {
				break
			}
		}

		// Compute corresponding end positions in leftRaw and rightRaw.
		groupLeftEnd := mapBasePosToTarget(groupEnd, leftSpans[:li], len(baseRaw), len(leftRaw))
		groupRightEnd := mapBasePosToTarget(groupEnd, rightSpans[:ri], len(baseRaw), len(rightRaw))

		bSlice := baseRaw[groupStart:groupEnd]
		lSlice := leftRaw[groupLeftStart:groupLeftEnd]
		rSlice := rightRaw[groupRightStart:groupRightEnd]

		kind := classifyThreeWayGroup(bSlice, lSlice, rSlice, tookLeft, tookRight)
		for _, s := range bSlice {
			initialBuilder.WriteString(s)
		}

		seg := ConflictSegment{
			Kind:       kind,
			BaseStart:  groupStart + 1,
			BaseLines:  stripRawLineEndings(bSlice),
			LeftStart:  groupLeftStart + 1,
			LeftLines:  stripRawLineEndings(lSlice),
			RightStart: groupRightStart + 1,
			RightLines: stripRawLineEndings(rSlice),
		}
		seg.ID = computeConflictSegmentID(len(segments), seg)
		segments = append(segments, seg)

		baseIdx = groupEnd
		leftIdx = groupLeftEnd
		rightIdx = groupRightEnd
	}

	if baseIdx < len(baseRaw) {
		appendCommon(len(baseRaw) - baseIdx)
	}

	return segments, initialBuilder.String()
}

func mapBasePosToTarget(basePos int, consumed []editSpan, baseLen, targetLen int) int {
	delta := 0
	for _, sp := range consumed {
		delta += sp.newCount - sp.oldCount
	}
	pos := basePos + delta
	if pos < 0 {
		return 0
	}
	if pos > targetLen {
		return targetLen
	}
	_ = baseLen
	return pos
}

func classifyThreeWayGroup(bSlice, lSlice, rSlice []string, tookLeft, tookRight bool) string {
	leftSameAsBase := !tookLeft || slices.Equal(lSlice, bSlice)
	rightSameAsBase := !tookRight || slices.Equal(rSlice, bSlice)
	switch {
	case leftSameAsBase && rightSameAsBase:
		return SegmentCommon
	case !leftSameAsBase && rightSameAsBase:
		return SegmentChangeLeft
	case leftSameAsBase && !rightSameAsBase:
		return SegmentChangeRight
	case slices.Equal(lSlice, rSlice):
		return SegmentChangeBothSame
	default:
		return SegmentConflict
	}
}

func computeConflictSegmentID(idx int, seg ConflictSegment) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%d:%s:%d:%d:%d\x00", idx, seg.Kind, seg.BaseStart, seg.LeftStart, seg.RightStart)
	for _, s := range seg.LeftLines {
		_, _ = h.Write([]byte("L:"))
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	for _, s := range seg.RightLines {
		_, _ = h.Write([]byte("R:"))
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return fmt.Sprintf("seg-%d-%s", idx, hex.EncodeToString(sum[:6]))
}

func hasGitConflictMarkers(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	hasOpen, hasSep, hasClose := false, false, false
	for _, line := range splitRawLines(raw) {
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(trimmed, "<<<<<<< ") || trimmed == "<<<<<<<":
			hasOpen = true
		case hasOpen && (trimmed == "=======" || strings.HasPrefix(trimmed, "||||||| ")):
			if trimmed == "=======" {
				hasSep = true
			}
		case hasOpen && hasSep && (strings.HasPrefix(trimmed, ">>>>>>> ") || trimmed == ">>>>>>>"):
			hasClose = true
		}
	}
	return hasOpen && hasSep && hasClose
}
