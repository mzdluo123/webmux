package main

import (
	"testing"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
	"github.com/rainchan/webmux/internal/workspace"
	"os"
	"path/filepath"
)

func TestProjectCapabilityWithoutGitOrFiles(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	service := workspace.New(t.TempDir(), nil, nil, t.TempDir())
	defer service.Close()
	plugins := pluginServices{workspace: service}
	c := config.Config{Plugins: config.PluginConfig{Enabled: []string{"workspace"}}}
	snapshot := plugins.capabilities(c)
	if err := localclient.ValidateCapabilities(snapshot); err != nil {
		t.Fatalf("real snapshot invalid: %v", err)
	}
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
	if err := localclient.ValidateCapabilities(snapshot); err != nil {
		t.Fatalf("disabled snapshot invalid: %v", err)
	}
	for _, capability := range snapshot.Capabilities {
		if capability.ID == "workspace.projects" && (capability.Available || capability.Reason != contract.CapabilityReasonPluginDisabled) {
			t.Fatalf("disabled: %+v", capability)
		}
	}
}

func TestProjectCapabilityFailureSnapshotsValidate(t *testing.T) {
	t.Parallel()
	c := config.Config{Plugins: config.PluginConfig{Enabled: []string{"workspace"}}}
	plugins := pluginServices{}
	if err := localclient.ValidateCapabilities(plugins.capabilities(c)); err != nil {
		t.Fatalf("unassembled: %v", err)
	}
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "projects.json"), []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	service := workspace.New(state, nil, nil, t.TempDir())
	defer service.Close()
	plugins.workspace = service
	if err := localclient.ValidateCapabilities(plugins.capabilities(c)); err != nil {
		t.Fatalf("project storage failure: %v", err)
	}
}

func TestProjectCapabilitySurvivesCorruptGitRegistry(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "repositories.json"), []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	service := workspace.New(state, nil, nil, t.TempDir())
	defer service.Close()
	plugins := pluginServices{workspace: service}
	snapshot := plugins.capabilities(config.Config{Plugins: config.PluginConfig{Enabled: []string{"workspace"}}})
	if err := localclient.ValidateCapabilities(snapshot); err != nil {
		t.Fatalf("partial workspace invalid: %v", err)
	}
	for _, capability := range snapshot.Capabilities {
		if capability.ID == "workspace.projects" && !capability.Available {
			t.Fatalf("Git registry disabled directory projects: %+v", capability)
		}
		if capability.ID == "workspace.manage" && capability.Available {
			t.Fatal("corrupt Git registry advertised available")
		}
	}
}
