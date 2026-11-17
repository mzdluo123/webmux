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

// Built-in selection belongs to composition roots, not services, controllers,
// DTOs or the generic registry. Tests may deliberately choose the built-ins.
func TestAgentRegistryAssemblyBoundary(t *testing.T) {
	root := filepath.Join("..", "..", "internal")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if name == "github.com/rainchan/webmux/internal/agentprofile/agents" {
				t.Errorf("%s imports built-in agent selection; inject the selected registry from cmd", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
