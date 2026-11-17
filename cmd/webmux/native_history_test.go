package main

import (
	"path/filepath"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile"
)

func TestNativeHistoryLaunchRoots(t *testing.T) {
	t.Setenv("HOME", "/server/home")
	t.Setenv("CODEX_HOME", "/server/codex")
	t.Setenv("CLAUDE_CONFIG_DIR", "/server/claude")
	t.Setenv("KIMI_CODE_HOME", "/server/kimi")
	t.Setenv("XDG_DATA_HOME", "/server/data")
	capabilities := nativeHistoryCapabilities()
	cases := []struct {
		agent string
		env   map[string]string
		want  string
	}{
		{"codex", map[string]string{"CODEX_HOME": "/launch/codex"}, "/launch/codex"},
		{"claudecode", map[string]string{"CLAUDE_CONFIG_DIR": "/launch/claude"}, "/launch/claude/projects"},
		{"kimi", map[string]string{"KIMI_CODE_HOME": "/launch/kimi"}, "/launch/kimi/sessions"},
		{"opencode", map[string]string{"XDG_DATA_HOME": "/launch/data"}, "/launch/data/opencode"},
		{"devin", map[string]string{"XDG_DATA_HOME": "/launch/data"}, "/launch/data/devin/cli"},
		{"omp", map[string]string{"HOME": "/launch/home"}, "/launch/home/.omp/agent/sessions"},
		{"agy", map[string]string{"HOME": "/launch/home"}, "/launch/home/.gemini/antigravity-cli/conversations"},
		{"qoder", map[string]string{"HOME": "/launch/home"}, "/launch/home/.qoder/projects"},
		{"qodercn", map[string]string{"HOME": "/launch/home"}, "/launch/home/.qoder-cn/projects"},
		{"codex", map[string]string{"HOME": "/launch/home", "CODEX_HOME": ""}, "/launch/home/.codex"},
		{"codex", map[string]string{"CODEX_HOME": "relative"}, "relative"},
		{"codex", nil, "/server/codex"},
	}
	for _, tc := range cases {
		t.Run(tc.agent+tc.want, func(t *testing.T) {
			if got := nativeHistoryRoot(capabilities, tc.agent, tc.env); got != filepath.FromSlash(tc.want) {
				t.Fatalf("root %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNativeHistoryIndependentOfRuntimeRegistry(t *testing.T) {
	p := &featureServices{agents: agentprofile.NewRegistry()}
	p.openNativeHistory()
	if p.nativeHistory == nil || p.historyProjection == nil || p.nativeRoot == nil {
		t.Fatal("native history requires a runtime agent")
	}
	capabilities := nativeHistoryCapabilities()
	if len(capabilities) != 9 {
		t.Fatalf("reader count %d", len(capabilities))
	}
	for agent, capability := range capabilities {
		if capability.read == nil || capability.discover == nil || capability.root == nil || p.nativeRoot(agent, map[string]string{"HOME": "/import"}) == "" {
			t.Fatalf("incomplete import-only capability %s", agent)
		}
	}
}
