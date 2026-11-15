package pluginwire

import (
	snapcompact "github.com/mzdluo123/snapcompact-go"
	"strconv"

	"github.com/rainchan/webmux/internal/codexrollout"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
)

func NativeFiles(files []codexrollout.File) []contract.NativeFile {
	return each(files, func(f codexrollout.File) contract.NativeFile {
		return contract.NativeFile{Path: f.Path, NativeID: f.Metadata.ID, Cwd: f.Metadata.Cwd, Timestamp: f.Metadata.Timestamp, Version: f.Metadata.CLIVersion, Problem: f.Problem}
	})
}
func NativeImported(in conversation.NativeImport) contract.NativeResult {
	out := EmptyNativeResult()
	out.SourceID = in.SourceID
	out.ConversationID = in.Binding.ConversationID
	out.Consumed = strconv.FormatInt(in.Consumed, 10)
	out.Pending = in.Pending
	for _, d := range in.Diagnostics {
		out.Gaps = append(out.Gaps, strconv.FormatInt(d.Offset, 10)+":"+d.Category)
	}
	return out
}
func NativeSnapshot(in conversation.NativeSnapshot) contract.NativeResult {
	out := EmptyNativeResult()
	out.SourceID = in.SourceID
	out.SnapshotID = in.ID
	out.Parser = in.Parser
	out.Model = in.Model
	out.Consumed = strconv.FormatInt(in.Consumed, 10)
	out.Gaps = append(out.Gaps, in.Gaps...)
	return out
}
func EmptyNativeResult() contract.NativeResult {
	return contract.NativeResult{Consumed: "0", Provenance: []contract.NativeProvenance{}, Files: []contract.NativeFile{}, Gaps: []string{}, Activities: ConversationActivities(conversation.ActivityPage{HistoryOrigin: "unknown", Items: []conversation.ActivityPart{}, Gaps: []conversation.HistoryGap{}})}
}

func NativePreview(in conversation.SnapshotPreview, problem string) contract.NativePreview {
	out := contract.NativePreview{Gaps: append([]string{}, in.Gaps...), ID: in.ID, Version: in.Projection.Version, CurrentTask: in.Projection.CurrentTask, Segments: []contract.NativeSegment{}, Omissions: []contract.NativeOmission{}, Pages: []contract.NativePNG{}, Uncovered: []contract.NativeRange{}, MissingGlyphCount: len(in.Render.Missing), Problem: problem}
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

func NativeProvenance(in []conversation.NativeProvenance) []contract.NativeProvenance {
	return each(in, func(p conversation.NativeProvenance) contract.NativeProvenance {
		return contract.NativeProvenance{ActivityID: p.ActivityID, SourceID: p.SourceID, Checksum: p.Checksum, CallID: p.CallID, ToolName: p.ToolName, Kind: p.Kind, Offset: strconv.FormatInt(p.Offset, 10), End: strconv.FormatInt(p.End, 10)}
	})
}
