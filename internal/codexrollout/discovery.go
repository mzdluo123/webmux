package codexrollout

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const MaxMetadata = 64 << 10
const MaxDiscoveryBytes = 16 << 20
const MaxDiscoveryFiles = 10000

type Discovery struct {
	Files     []File
	Truncated bool
}
type discoveryLimits struct{ files, bytes, visits int }

// Discover reads only bounded metadata, retaining partial results at a limit.
// root is CODEX_HOME, not its sessions directory. No symlinks are traversed.
func Discover(ctx context.Context, root string) (Discovery, error) {
	return discover(ctx, root, discoveryLimits{MaxDiscoveryFiles, MaxDiscoveryBytes, 20000})
}

func discover(ctx context.Context, root string, limits discoveryLimits) (Discovery, error) {
	out := Discovery{Files: []File{}}
	if root == "" {
		root = os.Getenv("CODEX_HOME")
	}
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return out, err
		}
		root = filepath.Join(home, ".codex")
	}
	remaining, visits := limits.bytes, 0
	stop := false
	for _, dir := range []string{"sessions", "archived_sessions"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			visits++
			if visits > limits.visits {
				out.Truncated = true
				stop = true
				return filepath.SkipAll
			}
			if os.IsNotExist(walkErr) {
				return nil
			}
			if walkErr != nil {
				out.Truncated = true
				if len(out.Files) < limits.files {
					out.Files = append(out.Files, File{Path: path, Problem: "unreadable_directory"})
				}
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() || d.Type()&os.ModeType != 0 || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			if len(out.Files) >= limits.files || remaining <= 0 {
				out.Truncated = true
				stop = true
				return filepath.SkipAll
			}
			m, err := inspect(ctx, path, &remaining)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			problem := ""
			if err != nil {
				problem = "unreadable_or_unsupported"
				if errors.Is(err, ErrLimit) {
					problem = "metadata_limit"
				}
			}
			out.Files = append(out.Files, File{Path: path, Metadata: m, Problem: problem})
			if remaining <= 0 {
				out.Truncated = true
				stop = true
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil {
			return out, err
		}
		if stop {
			break
		}
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	return out, nil
}

type File struct {
	Path     string
	Metadata Metadata
	Problem  string
}

func Inspect(ctx context.Context, path string) (Metadata, error) {
	remaining := MaxMetadata + 1
	return inspect(ctx, path, &remaining)
}

type metadataReader struct {
	ctx       context.Context
	source    io.Reader
	remaining *int
}

func (r *metadataReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if *r.remaining <= 0 {
		return 0, ErrLimit
	}
	p = p[:min(len(p), *r.remaining)]
	n, err := r.source.Read(p)
	*r.remaining -= n
	return n, err
}

func inspect(ctx context.Context, path string, remaining *int) (Metadata, error) {
	var m Metadata
	if err := ctx.Err(); err != nil {
		return m, err
	}
	f, err := OpenFile(path)
	if err != nil {
		return m, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return m, err
	}
	if !info.Mode().IsRegular() {
		return m, ErrFormat
	}
	reader := metadataReader{ctx: ctx, source: io.LimitReader(f, MaxMetadata+1), remaining: remaining}
	b := bufio.NewReader(&reader)
	var line []byte
	for {
		part, e := b.ReadSlice('\n')
		line = append(line, part...)
		if len(line) > MaxMetadata {
			return m, ErrLimit
		}
		if e == bufio.ErrBufferFull {
			continue
		}
		if e != nil {
			if e == io.EOF {
				return m, ErrFormat
			}
			return m, e
		}
		break
	}
	var e envelope
	if json.Unmarshal(line, &e) != nil || e.Type != "session_meta" {
		return m, ErrFormat
	}
	result := Result{}
	rec := Record{}
	if err := decode(e, &result, &rec); err != nil {
		return m, err
	}
	return result.Metadata, nil
}
