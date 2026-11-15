package featurewire

import (
	"strconv"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
)

func NativeFiles(files []historysource.Source) []contract.NativeFile {
	return each(files, func(f historysource.Source) contract.NativeFile {
		return contract.NativeFile{Agent: f.Agent, Path: f.Path, NativeID: f.NativeID, Branch: f.Branch}
	})
}

func NativeSnapshot(in historysource.Snapshot) contract.NativeResult {
	out := EmptyNativeResult()
	out.Agent, out.NativeID, out.Branch = in.Source.Agent, in.Source.NativeID, in.Source.Branch
	out.SnapshotID, out.Parser, out.Format = in.ID, in.ParserVersion, in.FormatVersion
	out.Boundary, out.ContentHash, out.Retained = in.Cursor, in.ContentHash, in.Retained
	out.RecordCount = in.RecordCount
	out.Gaps = nativeGaps(in.Gaps)
	return out
}

func EmptyNativeResult() contract.NativeResult {
	return contract.NativeResult{NextPosition: "0", NextTextOffset: "0", Files: []contract.NativeFile{}, Gaps: []string{}, Records: []contract.NativeRecord{}}
}

func NativePage(page historysource.Page, after, offset int) contract.NativeResult {
	out := NativeSnapshot(page.Snapshot)
	out.NextPosition, out.NextTextOffset, out.More = strconv.Itoa(page.Next), strconv.Itoa(page.TextOffset), page.More
	for index, record := range page.Records {
		position := after + index
		textOffset := 0
		if index == 0 {
			textOffset = offset
		}
		end := textOffset + len(record.Text)
		hash := ""
		if position < len(page.Snapshot.Records) {
			hash = page.Snapshot.Records[position].Hash
		}
		out.Records = append(out.Records, contract.NativeRecord{
			ID: record.ID, NativeID: record.NativeID, ParentID: record.ParentID, Hash: hash,
			Kind: record.Kind, Role: record.Role, Status: record.Status, Text: record.Text,
			CallID: record.CallID, NativeCallID: record.NativeCallID, ToolName: record.ToolName,
			Position: strconv.Itoa(position + 1), Offset: strconv.FormatInt(record.Offset, 10), End: strconv.FormatInt(record.End, 10),
			ByteLength: strconv.FormatInt(record.ByteLength, 10), NextTextOffset: strconv.Itoa(end), Truncated: int64(end) < record.ByteLength,
			Attachments: each(record.Attachments, func(a historysource.Attachment) contract.ConversationAttachment {
				return contract.ConversationAttachment{Name: a.Name, MimeType: a.MimeType, Reference: a.Reference, Size: a.Size}
			}),
		})
	}
	return out
}

func nativeGaps(in []historysource.Gap) []string {
	return each(in, func(g historysource.Gap) string { return strconv.FormatInt(g.Offset, 10) + ":" + g.Category })
}

func NativePreview(in historyprojection.Preview, problem string) contract.NativePreview {
	out := contract.NativePreview{Gaps: nativeGaps(in.Gaps), ID: in.ID, Version: in.Projection.Version, CurrentTask: in.Projection.CurrentTask, Segments: []contract.NativeSegment{}, Omissions: []contract.NativeOmission{}, Pages: []contract.NativePNG{}, Uncovered: []contract.NativeRange{}, MissingGlyphCount: len(in.Render.Missing), Problem: problem}
	for _, v := range in.Projection.Segments {
		out.Segments = append(out.Segments, contract.NativeSegment{SourceID: v.SourceID, Text: v.Text, Role: v.Role, CallID: v.CallID, ToolName: v.ToolName, Status: v.Status, Sources: each(v.Sources, nativeRange)})
	}
	for _, v := range in.Projection.Omissions {
		out.Omissions = append(out.Omissions, contract.NativeOmission{Range: nativeRange(v.Range), Reason: v.Reason, OriginalRunes: v.OriginalRunes})
	}
	for _, v := range in.Render.Pages {
		out.Pages = append(out.Pages, contract.NativePNG{PNG: v.PNG, Ranges: each(v.Ranges, nativeRange)})
	}
	out.Uncovered = each(in.Render.Uncovered, nativeRange)
	return out
}

func nativeRange(v snapcompact.Range) contract.NativeRange {
	return contract.NativeRange{SourceID: v.SourceID, Field: v.Field, Start: v.Start, End: v.End}
}
