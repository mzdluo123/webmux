package archtest

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Core packages may receive typed extension ports, but must never import their
// optional implementations, including through the transport infrastructure.
func TestTerminalCoreDoesNotImportPlugins(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	plugins := []string{
		"agenttrust", "agentprofile/agents", "workspace", "codeview", "files",
		"artifacts", "launch", "hybrid", "hybridresume", "conversation", "handoff",
		"forward", "push", "voice", "voicesettings", "p2p", "p2psettings",
		"tailscale", "tailscalesettings", "tunnel", "tunnelsettings",
		"pluginapi", "pluginhub", "pluginwire",
	}
	for _, core := range []string{"api", "hub", "wire", "transport", "session", "sessioncreate"} {
		for _, file := range parseDirGoFiles(t, filepath.Join(root, "internal", core), false) {
			for _, imported := range file.ast.Imports {
				path, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				for _, plugin := range plugins {
					prefix := "github.com/rainchan/webmux/internal/" + plugin
					if path == prefix || strings.HasPrefix(path, prefix+"/") {
						t.Errorf("%s imports optional implementation %s; bind a narrow core port from cmd instead", file.rel, path)
					}
				}
			}
		}
	}
}
