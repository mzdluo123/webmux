package handoff

import (
	"cmp"
	"slices"
	"unicode/utf8"

	snapcompact "github.com/mzdluo123/snapcompact-go"
)

// archiveCovered requires an exact partition of projected source into text,
// images and explicit omissions. Overlapping coverage is rejected as well as gaps.
func archiveCovered(segments []snapcompact.Segment, archive snapcompact.Archive) bool {
	if len(archive.Render.Uncovered) > 0 || len(archive.Render.Missing) > 0 {
		return false
	}
	lengths := make(map[string]int, len(segments))
	ranges := make(map[string][]snapcompact.Range, len(segments))
	var source, omitted int
	for _, segment := range segments {
		if _, exists := lengths[segment.SourceID]; exists {
			return false
		}
		length := utf8.RuneCountInString(segment.Text)
		lengths[segment.SourceID] = length
		source += length
	}
	add := func(items []snapcompact.Range) bool {
		for _, r := range items {
			length, exists := lengths[r.SourceID]
			if !exists || r.Field != "projection" || r.Start < 0 || r.End < r.Start || r.End > length {
				return false
			}
			if r.Start != r.End {
				ranges[r.SourceID] = append(ranges[r.SourceID], r)
			}
		}
		return true
	}
	if !add(archive.TextRanges) || !add(archive.Omitted) {
		return false
	}
	for _, r := range archive.Omitted {
		omitted += r.End - r.Start
	}
	for i, page := range archive.Render.Pages {
		if page.Index != i || page.Width < 1 || page.Height < 1 || len(page.PNG) == 0 || !add(page.Ranges) {
			return false
		}
	}
	for id, length := range lengths {
		items := ranges[id]
		slices.SortFunc(items, func(a, b snapcompact.Range) int { return cmp.Compare(a.Start, b.Start) })
		end := 0
		for _, r := range items {
			if r.Start != end {
				return false
			}
			end = r.End
		}
		if end != length {
			return false
		}
	}
	return archive.SourceRunes == source && archive.RetainedRunes == source-omitted
}
