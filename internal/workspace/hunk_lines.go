package workspace

import "strings"

// LineSelection identifies specific changed lines (0-based indices into
// StagingHunk.Lines, referencing only "delete" or "add" entries) within one hunk.
type LineSelection struct {
	HunkID      string `json:"hunkId"`
	LineIndices []int  `json:"lineIndices"`
}

func dominantSplicedEnding(baseLines, targetLines []string) string {
	crlf, lf := 0, 0
	for _, s := range baseLines {
		if strings.HasSuffix(s, "\r\n") {
			crlf++
		} else if strings.HasSuffix(s, "\n") {
			lf++
		}
	}
	for _, s := range targetLines {
		if strings.HasSuffix(s, "\r\n") {
			crlf++
		} else if strings.HasSuffix(s, "\n") {
			lf++
		}
	}
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
}

func normalizeSplicedLines(lines []string, fallbackEnding string) []byte {
	if len(lines) == 0 {
		return nil
	}
	if fallbackEnding == "" {
		fallbackEnding = "\n"
	}
	var out strings.Builder
	for i, line := range lines {
		out.WriteString(line)
		if i < len(lines)-1 && !strings.HasSuffix(line, "\n") {
			out.WriteString(fallbackEnding)
		}
	}
	return []byte(out.String())
}

func validateLineSelections(available []StagingHunk, requested []LineSelection) (map[string]map[int]bool, error) {
	if len(requested) == 0 {
		return nil, problemf(ErrInvalid, "未选择任何代码行")
	}
	byID := make(map[string]StagingHunk, len(available))
	for _, h := range available {
		byID[h.ID] = h
	}
	selected := make(map[string]map[int]bool, len(requested))
	totalSelected := 0
	for _, item := range requested {
		if item.HunkID == "" {
			return nil, problemf(ErrInvalid, "代码块标识不能为空")
		}
		hunk, ok := byID[item.HunkID]
		if !ok {
			return nil, problemf(ErrStale, "代码块标识 %s 不存在于当前快照，请刷新后重试", item.HunkID)
		}
		if len(item.LineIndices) == 0 {
			return nil, problemf(ErrInvalid, "代码块 %s 未指定行索引", item.HunkID)
		}
		set := selected[item.HunkID]
		if set == nil {
			set = make(map[int]bool, len(item.LineIndices))
			selected[item.HunkID] = set
		}
		for _, idx := range item.LineIndices {
			if idx < 0 || idx >= len(hunk.Lines) {
				return nil, problemf(ErrInvalid, "代码块 %s 的行索引 %d 超出范围", item.HunkID, idx)
			}
			kind := hunk.Lines[idx].Kind
			if kind != "delete" && kind != "add" {
				return nil, problemf(ErrInvalid, "代码块 %s 的行索引 %d 为上下文行，仅允许选择新增或删除行", item.HunkID, idx)
			}
			if !set[idx] {
				set[idx] = true
				totalSelected++
			}
		}
	}
	if totalSelected == 0 {
		return nil, problemf(ErrInvalid, "未选择任何有效的新增或删除行")
	}
	return selected, nil
}

func allHunkChangesSelected(available []StagingHunk, selected map[string]map[int]bool) bool {
	if len(available) == 0 {
		return false
	}
	for _, hunk := range available {
		set := selected[hunk.ID]
		for idx, line := range hunk.Lines {
			if (line.Kind == "delete" || line.Kind == "add") && !set[idx] {
				return false
			}
		}
	}
	return true
}

// spliceLineSelections synthesizes file bytes from baseBytes and targetBytes
// according to selected hunk line indices.
//   - forward == true (Stage: index -> worktree): applies ONLY selected deletions
//     and additions onto baseBytes.
//   - forward == false (Unstage: HEAD -> index, or Discard: index -> worktree):
//     reverts ONLY selected deletions and additions from targetBytes back to baseBytes.
func spliceLineSelections(baseBytes, targetBytes []byte, selected map[string]map[int]bool, forward bool) []byte {
	baseLines := splitRawLines(baseBytes)
	targetLines := splitRawLines(targetBytes)
	hunks, spans := buildStagingHunks(baseBytes, targetBytes)
	if len(spans) == 0 {
		if forward {
			return append([]byte(nil), baseBytes...)
		}
		return append([]byte(nil), targetBytes...)
	}

	fallbackEnding := dominantSplicedEnding(baseLines, targetLines)
	outLines := make([]string, 0, max(len(baseLines), len(targetLines)))
	cursor := 0
	for i, sp := range spans {
		outLines = append(outLines, baseLines[cursor:sp.oldStart]...)
		hunk := hunks[i]
		selSet := selected[hunk.ID]
		if len(selSet) == 0 {
			if forward {
				outLines = append(outLines, baseLines[sp.oldStart:sp.oldStart+sp.oldCount]...)
			} else {
				outLines = append(outLines, targetLines[sp.newStart:sp.newStart+sp.newCount]...)
			}
			cursor = sp.oldStart + sp.oldCount
			continue
		}

		ctxBefore := min(hunkContextLines, sp.oldStart)
		subBase := baseLines[sp.oldStart : sp.oldStart+sp.oldCount]
		subTarget := targetLines[sp.newStart : sp.newStart+sp.newCount]
		delSelected := func(relOld int) bool {
			return selSet[ctxBefore+relOld]
		}
		addSelected := func(relNew int) bool {
			return selSet[ctxBefore+sp.oldCount+relNew]
		}

		outLines = appendSpanLines(outLines, subBase, subTarget, delSelected, addSelected, forward)
		cursor = sp.oldStart + sp.oldCount
	}
	outLines = append(outLines, baseLines[cursor:]...)
	return normalizeSplicedLines(outLines, fallbackEnding)
}

func appendSpanLines(
	out []string,
	subBase, subTarget []string,
	delSelected, addSelected func(int) bool,
	forward bool,
) []string {
	rawSpans := myersRawDiffSpans(subBase, subTarget)
	if len(rawSpans) == 0 {
		return append(out, subBase...)
	}
	oldCur, newCur := 0, 0
	emitSub := func(oldStart, oldCount, newStart, newCount int) {
		limit := max(oldCount, newCount)
		for t := 0; t < limit; t++ {
			if t < oldCount {
				idx := oldStart + t
				keepBase := !delSelected(idx)
				if !forward {
					keepBase = delSelected(idx)
				}
				if keepBase {
					out = append(out, subBase[idx])
				}
			}
			if t < newCount {
				idx := newStart + t
				keepTarget := addSelected(idx)
				if !forward {
					keepTarget = !addSelected(idx)
				}
				if keepTarget {
					out = append(out, subTarget[idx])
				}
			}
		}
	}
	for _, raw := range rawSpans {
		for oldCur < raw.oldStart && newCur < raw.newStart {
			// Merged unchanged gap line inside a combined hunk.
			emitSub(oldCur, 1, newCur, 1)
			oldCur++
			newCur++
		}
		emitSub(raw.oldStart, raw.oldCount, raw.newStart, raw.newCount)
		oldCur = raw.oldStart + raw.oldCount
		newCur = raw.newStart + raw.newCount
	}
	for oldCur < len(subBase) && newCur < len(subTarget) {
		emitSub(oldCur, 1, newCur, 1)
		oldCur++
		newCur++
	}
	return out
}
