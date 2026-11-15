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
	features := featureServices{workspace: service}
	c := config.Config{Features: config.FeatureConfig{Enabled: []string{"workspace"}}}
	snapshot := features.capabilities(c)
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
		if (capability.ID == "workspace.manage" || capability.ID == "workspace.clone" || capability.ID == "workspace.github") && capability.Available {
			t.Fatal("Git capability available without Git")
		}
	}
	if !found {
		t.Fatal("missing project capability")
	}
	snapshot = features.capabilities(config.Config{Features: config.FeatureConfig{Enabled: []string{}}})
	if err := localclient.ValidateCapabilities(snapshot); err != nil {
		t.Fatalf("disabled snapshot invalid: %v", err)
	}
	for _, capability := range snapshot.Capabilities {
		if (capability.ID == "workspace.projects" || capability.ID == "workspace.clone" || capability.ID == "workspace.github") && (capability.Available || capability.Reason != contract.CapabilityReasonFeatureDisabled) {
			t.Fatalf("disabled: %+v", capability)
		}
	}
}

func TestProjectCapabilityFailureSnapshotsValidate(t *testing.T) {
	t.Parallel()
	c := config.Config{Features: config.FeatureConfig{Enabled: []string{"workspace"}}}
	features := featureServices{}
	if err := localclient.ValidateCapabilities(features.capabilities(c)); err != nil {
		t.Fatalf("unassembled: %v", err)
	}
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "projects.json"), []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	service := workspace.New(state, nil, nil, t.TempDir())
	defer service.Close()
	features.workspace = service
	if err := localclient.ValidateCapabilities(features.capabilities(c)); err != nil {
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
	features := featureServices{workspace: service}
	snapshot := features.capabilities(config.Config{Features: config.FeatureConfig{Enabled: []string{"workspace"}}})
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

func TestProjectCloneCapabilityWithCorruptLedger(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "project-clones.json"), []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	service := workspace.New(state, nil, nil, t.TempDir())
	defer service.Close()
	features := featureServices{workspace: service}
	snapshot := features.capabilities(config.Config{Features: config.FeatureConfig{Enabled: []string{"workspace"}}})
	if err := localclient.ValidateCapabilities(snapshot); err != nil {
		t.Fatal(err)
	}
	for _, capability := range snapshot.Capabilities {
		if capability.ID == "workspace.clone" && (capability.Available || capability.Reason != contract.CapabilityReasonStorageUnavailable) {
			t.Fatalf("corrupt clone ledger: %+v", capability)
		}
		if capability.ID == "workspace.projects" && !capability.Available {
			t.Fatalf("clone ledger disabled ordinary projects: %+v", capability)
		}
	}
}

func TestGitHubCapabilityDiscoveryDoesNotRunCLI(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "invoked")
	t.Setenv("WEBMUX_CAPABILITY_PROBE_MARKER", marker)
	for _, name := range []string{"git", "gh"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("#!/bin/sh\nprintf invoked > \"$WEBMUX_CAPABILITY_PROBE_MARKER\"\nexit 87\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", root)
	service := workspace.New(t.TempDir(), nil, nil, root)
	defer service.Close()
	features := featureServices{workspace: service}
	snapshot := features.capabilities(config.Config{Features: config.FeatureConfig{Enabled: []string{"workspace"}}})
	if err := localclient.ValidateCapabilities(snapshot); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, capability := range snapshot.Capabilities {
		if capability.ID == "workspace.github" {
			found = true
			if !capability.Available {
				t.Fatalf("executable discovery failed: %+v", capability)
			}
		}
	}
	if !found {
		t.Fatal("missing GitHub capability")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("capability discovery executed external commands: %v", err)
	}
}
