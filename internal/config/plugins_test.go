package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPluginSelectionMissingAndExplicitEmpty(t *testing.T) {
	tests := []struct {
		name       string
		give       string
		wantNil    bool
		wantSource string
		wantCode   bool
	}{
		{name: "omitted", give: "", wantNil: true, wantSource: "default", wantCode: true},
		{name: "empty table", give: "[plugins]\n", wantNil: true, wantSource: "default", wantCode: true},
		{name: "explicit core", give: "[plugins]\nenabled = []\n", wantSource: "toml"},
		{name: "single plugin", give: "[plugins]\nenabled = ['code']\n", wantSource: "toml", wantCode: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(tt.give), 0600); err != nil {
				t.Fatal(err)
			}
			c, err := Load(path, true)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := c.Validate(); err != nil {
					t.Fatal(err)
				}
				if (c.Plugins.Enabled == nil) != tt.wantNil {
					t.Fatalf("selection nil = %v, want %v", c.Plugins.Enabled == nil, tt.wantNil)
				}
				if c.Plugins.Has("code") != tt.wantCode {
					t.Fatalf("code enabled = %v, want %v", c.Plugins.Has("code"), tt.wantCode)
				}
			}
			field := Fields(c)["plugins.enabled"]
			ids, ok := field.Value.([]string)
			if !ok || ids == nil || field.Source != tt.wantSource {
				t.Fatalf("diagnostic selection = %#v", field)
			}
			if slices.Contains(ids, "code") != tt.wantCode {
				t.Fatalf("diagnostics disagree with selected plugins: %v", ids)
			}
			if !tt.wantNil && !slices.Equal(ids, c.Plugins.Enabled) {
				t.Fatalf("diagnostics changed explicit selection: %v", ids)
			}
		})
	}
}

func TestPluginSelectionValidationFailsBeforeOtherConfiguration(t *testing.T) {
	tests := []struct {
		name string
		give []string
		want string
	}{
		{name: "unknown", give: []string{"unknown"}, want: "unknown"},
		{name: "duplicate", give: []string{"code", "code"}, want: "duplicate"},
		{name: "missing dependency", give: []string{"chat", "files"}, want: "chat -> launch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateDir := filepath.Join(t.TempDir(), "must-not-be-created")
			c := Config{Plugins: PluginConfig{Enabled: tt.give}, StateDir: stateDir}
			// Listen is deliberately invalid: plugin selection must fail before
			// any later configuration processing or service initialization.
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), "plugins.enabled") || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("invalid plugins did not fail first with %q: %v", tt.want, err)
			}
			if _, err := os.Stat(stateDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("plugin validation touched state path: %v", err)
			}
			if !slices.Equal(c.Plugins.Enabled, tt.give) {
				t.Fatal("validation rewrote the invalid selection")
			}
		})
	}
}

func TestPluginDiagnosticSelectionUsesWhitelist(t *testing.T) {
	c := Config{
		Plugins: PluginConfig{Enabled: []string{}},
		Token:   "private-owner-token",
		Tunnel:  TunnelConfig{Token: "private-tunnel-token"},
		Voice:   VoiceConfig{Env: []string{"PRIVATE=private-environment-value"}},
	}
	c.SetSource("plugins.enabled", "toml")
	c.SetSource("not-a-diagnostic-field", "private-source-value")
	fields := Fields(c)
	if _, exists := fields["not-a-diagnostic-field"]; exists {
		t.Fatal("source map entries escaped the diagnostic whitelist")
	}
	field := fields["plugins.enabled"]
	ids, ok := field.Value.([]string)
	if !ok || ids == nil || len(ids) != 0 || field.Source != "toml" {
		t.Fatalf("explicit empty selection lost at diagnostic boundary: %#v", field)
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-owner-token", "private-tunnel-token", "private-environment-value", "private-source-value"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("diagnostic response contains secret %q", secret)
		}
	}
}
