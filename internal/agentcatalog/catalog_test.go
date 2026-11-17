package agentcatalog_test

import (
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agentcatalog"
	"github.com/rainchan/webmux/internal/agentprofile/agents"
)

func TestBuildIncludesHybridAndTerminalEntries(t *testing.T) {
	entries := agentcatalog.Build(agents.Default())
	if len(entries) != 13 {
		t.Fatalf("len(entries) = %d, want 13", len(entries))
	}
	seen := map[string]bool{}
	connectors := 0
	for _, e := range entries {
		if e.ID == "" || e.Title == "" || len(e.Executables) == 0 {
			t.Fatalf("incomplete entry: %+v", e)
		}
		if e.Distribution.DocsURL != "" && !strings.HasPrefix(e.Distribution.DocsURL, "https://") {
			t.Fatalf("entry %s docsUrl %q must use https://", e.ID, e.Distribution.DocsURL)
		}
		if seen[e.ID] {
			t.Fatalf("duplicate entry ID %q", e.ID)
		}
		seen[e.ID] = true
		if e.Connector != nil {
			connectors++
			if e.Connector.Executable == "" || e.Connector.Distribution.InstallCommand == "" {
				t.Fatalf("incomplete connector on %s: %+v", e.ID, e.Connector)
			}
			if e.Connector.Distribution.DocsURL != "" && !strings.HasPrefix(e.Connector.Distribution.DocsURL, "https://") {
				t.Fatalf("connector %s docsUrl %q must use https://", e.Connector.Executable, e.Connector.Distribution.DocsURL)
			}
		}
	}
	if connectors != 2 {
		t.Fatalf("connectors = %d, want 2 (claude-agent-acp and codex-acp)", connectors)
	}
	if got := agentcatalog.FromRegistry(nil); len(got) != 0 {
		t.Fatalf("FromRegistry(nil) = %d entries, want 0", len(got))
	}
}
