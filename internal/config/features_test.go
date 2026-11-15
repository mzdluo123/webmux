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

func TestFeatureSelectionMissingAndExplicitEmpty(t *testing.T) {
	tests := []struct {
		name       string
		give       string
		wantNil    bool
		wantSource string
		wantCode   bool
	}{
		{name: "omitted", give: "", wantNil: true, wantSource: "default", wantCode: true},
		{name: "empty table", give: "[features]\n", wantNil: true, wantSource: "default", wantCode: true},
		{name: "explicit core", give: "[features]\nenabled = []\n", wantSource: "toml"},
		{name: "single feature", give: "[features]\nenabled = ['code']\n", wantSource: "toml", wantCode: true},
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
				if (c.Features.Enabled == nil) != tt.wantNil {
					t.Fatalf("selection nil = %v, want %v", c.Features.Enabled == nil, tt.wantNil)
				}
				if c.Features.Has("code") != tt.wantCode {
					t.Fatalf("code enabled = %v, want %v", c.Features.Has("code"), tt.wantCode)
				}
			}
			field := Fields(c)["features.enabled"]
			ids, ok := field.Value.([]string)
			if !ok || ids == nil || field.Source != tt.wantSource {
				t.Fatalf("diagnostic selection = %#v", field)
			}
			if slices.Contains(ids, "code") != tt.wantCode {
				t.Fatalf("diagnostics disagree with selected features: %v", ids)
			}
			if !tt.wantNil && !slices.Equal(ids, c.Features.Enabled) {
				t.Fatalf("diagnostics changed explicit selection: %v", ids)
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
			// Listen is deliberately invalid: feature selection must fail before
			// any later configuration processing or service initialization.
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

func TestFeatureDiagnosticSelectionUsesWhitelist(t *testing.T) {
	c := Config{
		Features: FeatureConfig{Enabled: []string{}},
		Token:    "private-owner-token",
		Tunnel:   TunnelConfig{Token: "private-tunnel-token"},
		Voice:    VoiceConfig{Env: []string{"PRIVATE=private-environment-value"}},
	}
	c.SetSource("features.enabled", "toml")
	c.SetSource("not-a-diagnostic-field", "private-source-value")
	fields := Fields(c)
	if _, exists := fields["not-a-diagnostic-field"]; exists {
		t.Fatal("source map entries escaped the diagnostic whitelist")
	}
	field := fields["features.enabled"]
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
