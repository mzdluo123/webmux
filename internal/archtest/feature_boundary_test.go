package archtest

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Core packages may receive typed extension ports, but must never import their
// optional implementations, including through the transport infrastructure.
func TestTerminalCoreDoesNotImportFeatures(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	features := []string{
		"agenttrust", "agentprofile/agents", "workspace", "codeview", "files",
		"artifacts", "launch", "hybrid", "hybridresume", "conversation", "handoff",
		"forward", "push", "voice", "voicesettings", "p2p", "p2psettings",
		"tailscale", "tailscalesettings", "tunnel", "tunnelsettings",
		"featureapi", "featurehub", "featurewire",
	}
	for _, core := range []string{"api", "hub", "wire", "transport", "session", "sessioncreate"} {
		for _, file := range parseDirGoFiles(t, filepath.Join(root, "internal", core), false) {
			for _, imported := range file.ast.Imports {
				path, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				for _, feature := range features {
					prefix := "github.com/rainchan/webmux/internal/" + feature
					if path == prefix || strings.HasPrefix(path, prefix+"/") {
						t.Errorf("%s imports optional implementation %s; bind a narrow core port from cmd instead", file.rel, path)
					}
				}
			}
		}
	}
}
