package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNativeHistoryCapabilityBoundaries(t *testing.T) {
	for _, dir := range []string{"historysource", "historyimport", "historyprojection", "agent"} {
		root := filepath.Join("..", dir)
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			leaf := dir == "agent" && strings.Contains(filepath.ToSlash(path), "/history/")
			if dir == "agent" && !leaf {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imported := range file.Imports {
				name, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				local := strings.TrimPrefix(name, "github.com/rainchan/webmux/internal/")
				if local == name {
					continue
				}
				if !leaf && (strings.HasPrefix(local, "agent/") || strings.HasPrefix(local, "agentprofile") || local == "codexrollout") {
					t.Errorf("%s imports concrete native capability %s", path, local)
				}
				if leaf && (local == "conversation" || local == "historyimport" || local == "historyprojection" || local == "hybrid" || local == "session" || local == "agenttrust" || local == "acp" || local == "nativerpc") {
					t.Errorf("read-only leaf %s obtains mutable/runtime capability %s", path, local)
				}
				if dir == "historyprojection" && local != "historysource" {
					t.Errorf("projection %s must read frozen values, not %s", path, local)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
