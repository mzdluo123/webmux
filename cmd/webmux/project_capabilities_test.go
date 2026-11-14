package main

import (
	"testing"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/workspace"
)

func TestProjectCapabilityWithoutGitOrFiles(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	service := workspace.New(t.TempDir(), nil, nil, t.TempDir())
	defer service.Close()
	plugins := pluginServices{workspace: service}
	c := config.Config{Plugins: config.PluginConfig{Enabled: []string{"workspace"}}}
	snapshot := plugins.capabilities(c)
	found := false
	for _, capability := range snapshot.Capabilities {
		if capability.ID == "workspace.projects" {
			found = true
			if !capability.Available {
				t.Fatalf("local projects unavailable: %+v", capability)
			}
		}
		if capability.ID == "workspace.manage" && capability.Available {
			t.Fatal("Git capability available without Git")
		}
	}
	if !found {
		t.Fatal("missing project capability")
	}
	snapshot = plugins.capabilities(config.Config{Plugins: config.PluginConfig{Enabled: []string{}}})
	for _, capability := range snapshot.Capabilities {
		if capability.ID == "workspace.projects" && (capability.Available || capability.Reason != contract.CapabilityReasonPluginDisabled) {
			t.Fatalf("disabled: %+v", capability)
		}
	}
}
