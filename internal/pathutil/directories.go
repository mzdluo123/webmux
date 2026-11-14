package pathutil

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CompleteDirectories lists directories (including directory symlinks) matching
// prefix. A trailing slash or ~ lists a directory; visible names sort first.
func CompleteDirectories(prefix, defaultCwd string) ([]string, error) {
	if prefix == "" {
		prefix = defaultCwd + string(os.PathSeparator)
	}
	expanded, err := Expand(prefix)
	if err != nil {
		return nil, err
	}
	dir, base := filepath.Split(expanded)
	if strings.HasSuffix(prefix, "/") || prefix == "~" {
		dir, base = expanded, ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i].Name(), entries[j].Name()
		if strings.HasPrefix(a, ".") != strings.HasPrefix(b, ".") {
			return !strings.HasPrefix(a, ".")
		}
		return a < b
	})
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), base) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		isDir := entry.IsDir()
		if entry.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			isDir = info.IsDir()
		}
		if isDir {
			out = append(out, path)
		}
	}
	return out, nil
}
