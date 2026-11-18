// Package history adapts the existing Codex rollout parser to read-only native
// snapshots. Runtime, injection and trust capabilities are deliberately absent.
package history

import (
	"context"
	"errors"
	"io"
	"strconv"

	"github.com/rainchan/webmux/internal/codexrollout"
	"github.com/rainchan/webmux/internal/historysource"
)

func Read(ctx context.Context, req historysource.Request) (historysource.Document, error) {
	var records []historysource.Record
	positions := make(map[string]int)
	var gaps []historysource.Gap
	var result codexrollout.Result
	out, err := historysource.ReadJSONLStream(ctx, req, func(r io.Reader) error {
		var err error
		result, err = codexrollout.Read(r, func(record codexrollout.Record) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			// Compaction changes model context, not the availability of older
			// records still retained in the rollout. Never discard that history.
			for _, d := range record.Diagnostics {
				gaps = append(gaps, historysource.Gap{Category: d.Category, Offset: d.Offset})
			}
			for index, item := range record.Items {
				id := item.NativeID
				if id == "" {
					id = strconv.FormatInt(record.Offset, 10) + ":" + record.Hash + ":" + strconv.Itoa(index)
				}
				role := item.Role
				if role == "assistant" {
					role = "agent"
				}
				normalized := historysource.Record{ID: req.Source.NativeID + ":" + id, NativeID: item.NativeID, Kind: item.Kind, Role: role, Text: item.Text, CallID: item.CallID, ToolName: item.ToolName, Status: item.Status, Offset: record.Offset, End: record.End}
				for _, a := range item.Attachments {
					normalized.Attachments = append(normalized.Attachments, historysource.Attachment{Reference: a.Reference, MimeType: a.MimeType})
				}
				if previous, exists := positions[normalized.ID]; exists {
					records[previous] = normalized
				} else {
					positions[normalized.ID] = len(records)
					records = append(records, normalized)
				}
			}
			return nil
		})
		return err
	})
	if err != nil {
		var parseError *codexrollout.ParseError
		if errors.As(err, &parseError) {
			cause := historysource.ErrFormat
			if errors.Is(err, codexrollout.ErrLimit) {
				cause = historysource.ErrLimit
			}
			return out, &historysource.ParseError{Offset: parseError.Offset, Cause: errors.Join(cause, parseError.Cause)}
		}
		switch {
		case errors.Is(err, codexrollout.ErrLimit):
			return out, errors.Join(historysource.ErrLimit, err)
		case errors.Is(err, codexrollout.ErrVersion), errors.Is(err, codexrollout.ErrFormat):
			return out, errors.Join(historysource.ErrFormat, err)
		default:
			return out, err
		}
	}
	if result.Metadata.ID != req.Source.NativeID || req.Source.Branch != "" && req.Source.Branch != result.Metadata.ID {
		return out, historysource.ErrIdentity
	}
	out.Source.Branch = result.Metadata.ID
	out.ParserVersion, out.FormatVersion = codexrollout.Version, result.Metadata.CLIVersion
	out.Records, out.Gaps = records, gaps
	return out, nil
}

func Discover(ctx context.Context, root string) ([]historysource.Source, error) {
	found, err := codexrollout.Discover(ctx, root)
	if err != nil {
		return nil, err
	}
	out := make([]historysource.Source, 0, len(found.Files))
	for _, file := range found.Files {
		if file.Metadata.ID != "" {
			out = append(out, historysource.Source{Agent: "codex", Path: file.Path, NativeID: file.Metadata.ID, Branch: file.Metadata.ID})
		}
	}
	if found.Truncated {
		return out, historysource.ErrLimit
	}
	return out, nil
}
