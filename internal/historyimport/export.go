package historyimport

import (
	"context"
	"github.com/rainchan/webmux/internal/historysource"
	"unicode/utf8"
)

// Page bounds output independently from parser and projection budgets. Text
// offsets are UTF-8 byte boundaries; continuation never drops a partial rune.
func (s *Service) Page(ctx context.Context, id string, req historysource.PageRequest) (historysource.Page, error) {
	out := historysource.Page{Records: make([]historysource.Record, 0)}
	if req.After < 0 || req.TextOffset < 0 || req.Limit < 0 || req.MaxBytes < 0 {
		return out, historysource.ErrFormat
	}
	if req.Limit == 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.MaxBytes == 0 || req.MaxBytes > 64<<10 {
		req.MaxBytes = 4000
	}
	if req.MaxBytes < utf8.UTFMax {
		return out, historysource.ErrLimit
	}

	scope, err := s.Scope(id)
	if err != nil {
		return out, err
	}
	out.Snapshot = scope.Snapshot
	doc, err := s.ScopeDocument(ctx, id)
	if err != nil {
		return out, err
	}
	if req.After > len(doc.Records) || (req.After == len(doc.Records) && req.TextOffset != 0) {
		return out, historysource.ErrFormat
	}
	out.Next = req.After
	remaining := req.MaxBytes
	for out.Next < len(doc.Records) && len(out.Records) < req.Limit {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		record := doc.Records[out.Next]
		if !record.Exportable() {
			out.Next++
			req.TextOffset = 0
			continue
		}
		record.ByteLength = int64(len(record.Text))
		start := req.TextOffset
		if start > len(record.Text) || (start < len(record.Text) && !utf8.RuneStart(record.Text[start])) {
			return out, historysource.ErrFormat
		}
		end := len(record.Text)
		if end-start > remaining {
			end = start + remaining
			for end > start && !utf8.RuneStart(record.Text[end]) {
				end--
			}
			if end == start {
				out.TextOffset = start
				break
			}
		}
		record.Text = record.Text[start:end]
		out.Records = append(out.Records, record)
		remaining -= end - start
		if end != len(doc.Records[out.Next].Text) {
			out.TextOffset = end
			break
		}
		out.Next++
		req.TextOffset = 0
		if remaining == 0 {
			break
		}
	}
	out.More = out.Next < len(doc.Records)
	return out, nil
}
