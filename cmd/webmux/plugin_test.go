package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/statefile"
)

func TestCoreOnlyServeDoesNotInitializePluginState(t *testing.T) {
	run := startServe(t, false, "core-plugin-test-owner", []string{})
	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	request := func(path string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, run.local+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer core-plugin-test-owner")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	response := request(contract.PathCapabilities)
	var snapshot contract.Capabilities
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || snapshot.Version != contract.CapabilitiesVersion {
		t.Fatalf("capabilities: HTTP %d version %d", response.StatusCode, snapshot.Version)
	}
	for _, plugin := range snapshot.Plugins {
		if plugin.Enabled || plugin.Status != contract.PluginStatusDisabled {
			t.Fatalf("core-only instance initialized plugin %+v", plugin)
		}
	}
	for _, path := range []string{"/api/repositories", "/api/launch-configs", "/api/fs/dirs", "/ws/voice"} {
		response := request(path)
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("disabled endpoint %s: HTTP %d", path, response.StatusCode)
		}
	}
	for _, description := range statefile.Catalog() {
		if storePlugin(description.ID) == "" {
			continue
		}
		for _, name := range []string{description.Path, description.LockPath()} {
			if _, err := os.Stat(filepath.Join(run.state, name)); !os.IsNotExist(err) {
				t.Errorf("disabled plugin state was opened: %s (%v)", name, err)
			}
		}
	}
}

func TestDisabledPluginConfigurationDoesNotInspectCorruptStores(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte(`{"PRIVATE_PLUGIN_BODY":"must remain unread",broken`)
	count := 0
	for _, description := range statefile.Catalog() {
		if storePlugin(description.ID) == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(state, description.Path), corrupt, 0600); err != nil {
			t.Fatal(err)
		}
		count++
	}
	path := filepath.Join(root, "config.toml")
	body := fmt.Sprintf("state_dir = %q\nlisten = '127.0.0.1:0'\ntoken = 'short-but-local'\n[plugins]\nenabled = []\n[tunnel]\nenabled = true\n[tailscale]\nmode = 'serve'\n[mcp]\nenabled = true\n", state)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig([]string{"--config", path})
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.closeSettings()
	if cfg.TunnelSettings != nil || cfg.TailscaleSettings != nil || cfg.P2PSettings != nil {
		t.Fatal("configuration opened a disabled plugin store")
	}
	view := configuredView(path, cfg.Config)
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("PRIVATE_PLUGIN_BODY")) {
		t.Fatal("diagnostics leaked inactive storage contents")
	}
	for _, health := range view.Stores {
		if storePlugin(statefile.ID(health.ID)) != "" && (health.Status != "disabled" || health.Lock != "not_opened") {
			t.Errorf("inactive store has runtime health: %+v", health)
		}
	}
	entries, err := os.ReadDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != count {
		t.Fatalf("configuration created optional state: have %d entries, want %d", len(entries), count)
	}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(state, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(body, corrupt) {
			t.Errorf("inactive state changed: %s", entry.Name())
		}
	}
}
