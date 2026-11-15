package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile/agents"
)

// The Makefile and Playwright use the same checked-in executable list. Every
// runtime alias and actual ACP connector must resolve to the matching fake
// profile, not the fallback profile for unknown executable names.
func TestRuntimeExecutableCoverage(t *testing.T) {
	data, err := os.ReadFile("../../../../web/test/e2e/fake-agent-names.txt")
	if err != nil {
		t.Fatal(err)
	}
	names := strings.Fields(string(data))
	for _, a := range agents.Default().Agents() {
		p := a.Profile()
		executables := slices.Clone(p.Executables)
		if len(executables) == 0 {
			executables = append(executables, p.Name)
		}
		if p.Connector != nil {
			executables = append(executables, p.Connector.Executable)
		}
		for _, exe := range executables {
			if !slices.Contains(names, exe) {
				t.Errorf("agent %q executable %q is missing from shared fake list", p.Name, exe)
			}
			if got := profileFromName(filepath.Join("/fake/bin", exe)); got != p.Name {
				t.Errorf("executable %q fake profile = %q, want %q", exe, got, p.Name)
			}
		}
	}
}
