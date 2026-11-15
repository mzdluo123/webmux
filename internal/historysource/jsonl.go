package historysource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strconv"
)

// ReadJSONL reads only complete lines and rechecks the captured bytes before
// returning. A frozen cursor is a byte boundary, not a request for latest data.
// Cancellation is checked between reads; it cannot interrupt a blocked kernel
// filesystem operation. No goroutine outlives the call.
func ReadJSONL(ctx context.Context, req Request, visit func([]byte, int64, int64) error) (Document, error) {
	return readJSONL(ctx, req, visit, nil)
}

// ReadJSONLStream shares the verified bounded capture with an existing
// io.Reader parser, without copying the captured prefix into a second buffer.
func ReadJSONLStream(ctx context.Context, req Request, parse func(io.Reader) error) (Document, error) {
	return readJSONL(ctx, req, nil, parse)
}

func readJSONL(ctx context.Context, req Request, visit func([]byte, int64, int64) error, parse func(io.Reader) error) (Document, error) {
	out := Document{Source: req.Source}
	before, err := os.Lstat(req.Source.Path)
	if err != nil || !before.Mode().IsRegular() {
		return out, ErrUnavailable
	}
	f, err := os.Open(req.Source.Path)
	if err != nil {
		return out, ErrUnavailable
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !os.SameFile(before, info) {
		return out, ErrChanged
	}
	end := info.Size()
	if req.Cursor != "" {
		end, err = strconv.ParseInt(req.Cursor, 10, 64)
		if err != nil || end < 0 || end > info.Size() {
			return out, ErrChanged
		}
	}
	if end > MaxBytes {
		return out, ErrLimit
	}
	data, err := io.ReadAll(io.LimitReader(contextReader{ctx, f}, end))
	if err != nil {
		return out, readError(ctx, err)
	}
	if int64(len(data)) != end {
		return out, ErrChanged
	}
	complete := bytes.LastIndexByte(data, '\n') + 1
	if req.Cursor != "" && complete != len(data) {
		return out, ErrChanged
	}
	out.Pending = complete != len(data)
	data = data[:complete]
	out.Cursor = strconv.Itoa(complete)
	// The digest excludes append-only suffixes. Orchestration stores it with the
	// frozen manifest and compares it on every subsequent source read.
	sum := sha256.Sum256(data)
	out.Fingerprint = fileIdentity(info) + ":" + hex.EncodeToString(sum[:])
	count := 0
	for offset := 0; offset < len(data); {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		n := bytes.IndexByte(data[offset:], '\n') + 1
		if n > MaxRecordBytes || count >= MaxRecords {
			return out, ErrLimit
		}
		if visit != nil {
			if err := visit(data[offset:offset+n-1], int64(offset), int64(offset+n)); err != nil {
				return out, err
			}
		}
		offset += n
		count++
	}
	if parse != nil {
		if err := parse(bytes.NewReader(data)); err != nil {
			return out, err
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return out, ErrUnavailable
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(contextReader{ctx, f}, int64(complete))); err != nil {
		return out, readError(ctx, err)
	}
	after, err := os.Lstat(req.Source.Path)
	if err != nil || !os.SameFile(info, after) || !bytes.Equal(sum[:], hash.Sum(nil)) {
		return out, ErrChanged
	}
	return out, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func readError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, ErrLimit) {
		return ErrLimit
	}
	return ErrUnavailable
}
