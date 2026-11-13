package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	maxHunkFileBytes = 2 << 20
	maxHunkFileLines = 20000
	hunkContextLines = 3
	minHunkGapLines  = 2
	maxMyersDepth    = 4096
)

// StagingHunkLine describes one display line inside a text hunk while preserving
// exact original line-ending metadata.
type StagingHunkLine struct {
	Kind       string `json:"kind"` // "context" | "delete" | "add"
	OldLine    int    `json:"oldLine,omitempty"`
	NewLine    int    `json:"newLine,omitempty"`
	Text       string `json:"text"`
	LineEnding string `json:"lineEnding"` // "lf" | "crlf" | "none"
}

// StagingHunk represents an independently stageable or unstageable text block
// bound to a specific file content snapshot.
type StagingHunk struct {
	ID       string            `json:"id"`
	Header   string            `json:"header"`
	OldStart int               `json:"oldStart"`
	OldLines int               `json:"oldLines"`
	NewStart int               `json:"newStart"`
	NewLines int               `json:"newLines"`
	Lines    []StagingHunkLine `json:"lines"`
}

type editSpan struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
}

// splitRawLines splits data into lines keeping each line's exact trailing
// "\r\n", "\n", or "" (when the final line has no trailing newline).
func splitRawLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	s := string(data)
	out := make([]string, 0, strings.Count(s, "\n")+1)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimLineEnding(raw string) (text, ending string) {
	if strings.HasSuffix(raw, "\r\n") {
		return raw[:len(raw)-2], "crlf"
	}
	if strings.HasSuffix(raw, "\n") {
		return raw[:len(raw)-1], "lf"
	}
	return raw, "none"
}

func buildStagingHunks(baseBytes, targetBytes []byte) ([]StagingHunk, []editSpan) {
	baseLines := splitRawLines(baseBytes)
	targetLines := splitRawLines(targetBytes)
	spans := myersDiffSpans(baseLines, targetLines)
	if len(spans) == 0 {
		return nil, nil
	}
	hunks := make([]StagingHunk, 0, len(spans))
	for _, sp := range spans {
		ctxBefore := min(hunkContextLines, sp.oldStart)
		ctxAfter := min(hunkContextLines, len(baseLines)-(sp.oldStart+sp.oldCount))

		dispOldStart := sp.oldStart - ctxBefore
		dispOldCount := ctxBefore + sp.oldCount + ctxAfter
		dispNewStart := sp.newStart - ctxBefore
		dispNewCount := ctxBefore + sp.newCount + ctxAfter

		oldHeaderStart := dispOldStart + 1
		if dispOldCount == 0 {
			oldHeaderStart = dispOldStart
		}
		newHeaderStart := dispNewStart + 1
		if dispNewCount == 0 {
			newHeaderStart = dispNewStart
		}

		lines := make([]StagingHunkLine, 0, ctxBefore+sp.oldCount+sp.newCount+ctxAfter)
		for i := 0; i < ctxBefore; i++ {
			txt, end := trimLineEnding(baseLines[dispOldStart+i])
			lines = append(lines, StagingHunkLine{
				Kind:       "context",
				OldLine:    dispOldStart + i + 1,
				NewLine:    dispNewStart + i + 1,
				Text:       txt,
				LineEnding: end,
			})
		}
		for i := 0; i < sp.oldCount; i++ {
			txt, end := trimLineEnding(baseLines[sp.oldStart+i])
			lines = append(lines, StagingHunkLine{
				Kind:       "delete",
				OldLine:    sp.oldStart + i + 1,
				Text:       txt,
				LineEnding: end,
			})
		}
		for i := 0; i < sp.newCount; i++ {
			txt, end := trimLineEnding(targetLines[sp.newStart+i])
			lines = append(lines, StagingHunkLine{
				Kind:       "add",
				NewLine:    sp.newStart + i + 1,
				Text:       txt,
				LineEnding: end,
			})
		}
		afterBase := sp.oldStart + sp.oldCount
		afterTarget := sp.newStart + sp.newCount
		for i := 0; i < ctxAfter; i++ {
			txt, end := trimLineEnding(baseLines[afterBase+i])
			lines = append(lines, StagingHunkLine{
				Kind:       "context",
				OldLine:    afterBase + i + 1,
				NewLine:    afterTarget + i + 1,
				Text:       txt,
				LineEnding: end,
			})
		}

		hunks = append(hunks, StagingHunk{
			ID:       computeHunkID(sp, baseLines, targetLines),
			Header:   fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldHeaderStart, dispOldCount, newHeaderStart, dispNewCount),
			OldStart: oldHeaderStart,
			OldLines: dispOldCount,
			NewStart: newHeaderStart,
			NewLines: dispNewCount,
			Lines:    lines,
		})
	}
	return hunks, spans
}

func computeHunkID(sp editSpan, baseLines, targetLines []string) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%d:%d:%d:%d\x00", sp.oldStart, sp.oldCount, sp.newStart, sp.newCount)
	for _, s := range baseLines[sp.oldStart : sp.oldStart+sp.oldCount] {
		_, _ = h.Write([]byte("-"))
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	for _, s := range targetLines[sp.newStart : sp.newStart+sp.newCount] {
		_, _ = h.Write([]byte("+"))
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return "hunk-" + hex.EncodeToString(sum[:8])
}

func spliceHunkSubset(baseBytes, targetBytes []byte, keep map[string]bool) []byte {
	baseLines := splitRawLines(baseBytes)
	targetLines := splitRawLines(targetBytes)
	hunks, spans := buildStagingHunks(baseBytes, targetBytes)
	if len(spans) == 0 {
		return append([]byte(nil), baseBytes...)
	}
	fallbackEnding := dominantSplicedEnding(baseLines, targetLines)
	outLines := make([]string, 0, max(len(baseLines), len(targetLines)))
	cursor := 0
	for i, sp := range spans {
		outLines = append(outLines, baseLines[cursor:sp.oldStart]...)
		if keep[hunks[i].ID] {
			outLines = append(outLines, targetLines[sp.newStart:sp.newStart+sp.newCount]...)
		} else {
			outLines = append(outLines, baseLines[sp.oldStart:sp.oldStart+sp.oldCount]...)
		}
		cursor = sp.oldStart + sp.oldCount
	}
	outLines = append(outLines, baseLines[cursor:]...)
	return normalizeSplicedLines(outLines, fallbackEnding)
}

func myersDiffSpans(base, target []string) []editSpan {
	raw := myersRawDiffSpans(base, target)
	if len(raw) <= 1 {
		return raw
	}
	merged := []editSpan{raw[0]}
	for _, next := range raw[1:] {
		last := &merged[len(merged)-1]
		gap := next.oldStart - (last.oldStart + last.oldCount)
		if gap < minHunkGapLines {
			last.oldCount = (next.oldStart + next.oldCount) - last.oldStart
			last.newCount = (next.newStart + next.newCount) - last.newStart
		} else {
			merged = append(merged, next)
		}
	}
	return merged
}

func myersRawDiffSpans(base, target []string) []editSpan {
	pref := 0
	for pref < len(base) && pref < len(target) && base[pref] == target[pref] {
		pref++
	}
	baseEnd := len(base)
	targetEnd := len(target)
	for baseEnd > pref && targetEnd > pref && base[baseEnd-1] == target[targetEnd-1] {
		baseEnd--
		targetEnd--
	}
	a := base[pref:baseEnd]
	b := target[pref:targetEnd]
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 || m == 0 {
		return []editSpan{{oldStart: pref, oldCount: n, newStart: pref, newCount: m}}
	}

	maxD := min(n+m, maxMyersDepth)
	offset := n + m
	v := make([]int, 2*offset+1)
	trace := make([][]int, 0, min(maxD+1, 128))
	reached := false

	for d := 0; d <= maxD; d++ {
		snapshot := make([]int, 2*d+1)
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			snapshot[k+d] = x
			if x >= n && y >= m {
				reached = true
				break
			}
		}
		trace = append(trace, snapshot)
		if reached {
			break
		}
	}
	if !reached {
		return []editSpan{{oldStart: pref, oldCount: n, newStart: pref, newCount: m}}
	}

	// Backtrack to identify which lines in a and b are matched vs edited.
	matchedA := make([]bool, n)
	matchedB := make([]bool, m)
	x, y := n, m
	for d := len(trace) - 1; d > 0; d-- {
		prev := trace[d-1]
		pd := d - 1
		k := x - y
		var prevK int
		prevLeft := -1
		if (k-1) >= -pd && (k-1) <= pd {
			prevLeft = prev[(k-1)+pd]
		}
		prevRight := -1
		if (k+1) >= -pd && (k+1) <= pd {
			prevRight = prev[(k+1)+pd]
		}
		if k == -d || (k != d && prevLeft < prevRight) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := prev[prevK+pd]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x--
			y--
			matchedA[x] = true
			matchedB[y] = true
		}
		x, y = prevX, prevY
	}
	for x > 0 && y > 0 {
		x--
		y--
		matchedA[x] = true
		matchedB[y] = true
	}

	var raw []editSpan
	ia, ib := 0, 0
	for ia < n || ib < m {
		if ia < n && ib < m && matchedA[ia] && matchedB[ib] {
			ia++
			ib++
			continue
		}
		sa, sb := ia, ib
		for ia < n && !matchedA[ia] {
			ia++
		}
		for ib < m && !matchedB[ib] {
			ib++
		}
		raw = append(raw, editSpan{
			oldStart: pref + sa,
			oldCount: ia - sa,
			newStart: pref + sb,
			newCount: ib - sb,
		})
	}
	return raw
}
