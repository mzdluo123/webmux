package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/builtin"
)

func TestLoadIgnoresLegacyFeaturesAndPluginsSections(t *testing.T) {
	tests := []struct {
		name string
		give string
	}{
		{name: "omitted", give: ""},
		{name: "empty features table", give: "[features]\n"},
		{name: "legacy empty features", give: "[features]\nenabled = []\n"},
		{name: "legacy subset features", give: "[features]\nenabled = ['code']\n"},
		{name: "legacy retired feature id", give: "[features]\nenabled = ['immersive']\n"},
		{name: "legacy plugins table", give: "[plugins]\nenabled = []\n"},
		{name: "legacy plugins dotted key", give: "plugins.enabled = []\n"},
		{name: "both legacy tables", give: "[plugins]\nenabled = []\n[features]\nenabled = ['code']\n"},
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
			if err := c.Validate(); err != nil {
				t.Fatal(err)
			}
			if c.Features.Enabled != nil {
				t.Fatalf("TOML populated programmatic Features.Enabled = %v, want nil", c.Features.Enabled)
			}
			for _, def := range builtin.Catalog() {
				if !c.Features.Has(def.ID) {
					t.Fatalf("feature %q disabled for %s", def.ID, tt.name)
				}
			}
		})
	}
}

func TestFeatureSelectionValidationFailsBeforeOtherConfiguration(t *testing.T) {
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
			c := Config{Features: FeatureConfig{Enabled: tt.give}, StateDir: stateDir}
			// Listen is deliberately invalid: programmatic feature selection must
			// fail before any later configuration processing or initialization.
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), "features.enabled") || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("invalid features did not fail first with %q: %v", tt.want, err)
			}
			if _, err := os.Stat(stateDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("feature validation touched state path: %v", err)
			}
			if !slices.Equal(c.Features.Enabled, tt.give) {
				t.Fatal("validation rewrote the invalid selection")
			}
		})
	}
}

func TestDiagnosticWhitelistExcludesSecretsAndProgrammaticFeatures(t *testing.T) {
	c := Config{
		Features: FeatureConfig{Enabled: []string{}},
		Token:    "private-owner-token",
		Tunnel:   TunnelConfig{Token: "private-tunnel-token"},
		Voice:    VoiceConfig{Env: []string{"PRIVATE=private-environment-value"}},
	}
	c.SetSource("features.enabled", "toml")
	c.SetSource("not-a-diagnostic-field", "private-source-value")
	fields := Fields(c)
	for _, excluded := range []string{"not-a-diagnostic-field", "features.enabled"} {
		if _, exists := fields[excluded]; exists {
			t.Fatalf("%q escaped the diagnostic whitelist", excluded)
		}
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
