package codexrollout

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Discover lists metadata with at most one bounded record read per file. root
// is CODEX_HOME, not its sessions directory. Archived rollouts are included.
func Discover(root string) ([]File, error) {
	if root == "" {
		root = os.Getenv("CODEX_HOME")
	}
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(home, ".codex")
	}
	out := []File{}
	for _, dir := range []string{"sessions", "archived_sessions"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if len(out) >= 10000 {
				return ErrLimit
			}
			if d.IsDir() || d.Type()&os.ModeSymlink != 0 || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			m, err := Inspect(path)
			if err != nil {
				out = append(out, File{Path: path, Problem: "unreadable_or_unsupported"})
				return nil
			}
			out = append(out, File{Path: path, Metadata: m})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

type File struct {
	Path     string
	Metadata Metadata
	Problem  string
}

func Inspect(path string) (Metadata, error) {
	var m Metadata
	f, err := os.Open(path)
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
	b := bufio.NewReader(io.LimitReader(f, MaxRecord+1))
	var line []byte
	for {
		p, e := b.ReadSlice('\n')
		line = append(line, p...)
		if len(line) > MaxRecord {
			return m, ErrLimit
		}
		if e == bufio.ErrBufferFull {
			continue
		}
		if e != nil {
			return m, ErrFormat
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
