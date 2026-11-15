package historyimport

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/historysource"
)

func retainExport(ctx context.Context, doc historysource.Document) (historysource.Export, error) {
	out := historysource.Export{Records: doc.Records, Attachments: make(map[string][]byte)}
	total := 0
	for _, record := range doc.Records {
		total += len(record.Text)
		if total > historysource.MaxBytes {
			return out, historysource.ErrLimit
		}
		for _, attachment := range record.Attachments {
			// An omitted native body is already an explicit reader gap, not a
			// recoverable attachment. Do not invent bytes or block text forks.
			if attachment.Reference == "" {
				continue
			}
			if _, ok := out.Attachments[attachment.Reference]; ok {
				continue
			}
			path := strings.TrimPrefix(attachment.Reference, "file://")
			if !filepath.IsAbs(path) || strings.Contains(path, "://") {
				return out, historysource.ErrUnavailable
			}
			data, err := captureAttachment(ctx, path, historysource.MaxBytes-total)
			if err != nil {
				return out, err
			}
			total += len(data)
			out.Attachments[attachment.Reference] = data
		}
	}
	return out, nil
}

func captureAttachment(ctx context.Context, path string, remaining int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() {
		return nil, historysource.ErrUnavailable
	}
	if before.Size() > int64(remaining) {
		return nil, historysource.ErrLimit
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, historysource.ErrUnavailable
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !os.SameFile(before, info) {
		return nil, historysource.ErrChanged
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(remaining)+1))
	if err != nil {
		return nil, historysource.ErrUnavailable
	}
	if len(data) > remaining {
		return nil, historysource.ErrLimit
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, after) || after.Size() != int64(len(data)) || !info.ModTime().Equal(after.ModTime()) {
		return nil, historysource.ErrChanged
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

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
	doc, err := s.Document(ctx, id)
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
