package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func relayTestConfig() RelayConfig {
	return RelayConfig{
		ControlEndpoint: "https://control.example.com",
		ControlAudience: "webmux-control",
		Transport:       "wss",
	}
}

func TestRelayRequiresCanonicalControlOrigin(t *testing.T) {
	for _, endpoint := range []string{"https://control.example.com", "https://control.example.com:443"} {
		c := relayTestConfig()
		c.ControlEndpoint = endpoint
		got, err := c.Validated()
		if err != nil || got.ControlEndpoint != endpoint {
			t.Fatalf("valid origin rejected or changed: %q, %v", endpoint, err)
		}
	}
	for _, endpoint := range []string{
		"http://control.example.com", "wss://control.example.com", "https://user@control.example.com",
		"https://control.example.com/", "https://control.example.com/path", "https://control.example.com?", "https://control.example.com?x=1",
		"https://control.example.com#", "https://control.example.com#fragment", "https://control.example.com:444", "https://control.example.com:",
		"https://Control.example.com", "https://control.example.com.", "https://127.0.0.1", "https://[::1]", "https://[::1]:443",
		"https://-control.example.com", "https://control-.example.com", "https://control..example.com", "https://control_example.com",
		"https://" + strings.Repeat("a", 64) + ".example.com", "https://" + strings.Repeat("a.", 128) + "com", "https://control.example.com\\path",
	} {
		t.Run(endpoint, func(t *testing.T) {
			c := relayTestConfig()
			c.ControlEndpoint = endpoint
			if _, err := c.Validated(); err == nil {
				t.Fatal("accepted an ambiguous or invalid control origin")
			}
		})
	}
}

func TestRelayRejectsPartialAndMalformedBootstrap(t *testing.T) {
	for _, c := range []RelayConfig{
		{Enabled: true},
		{ControlAudience: "webmux-control"},
		{ControlEndpoint: "https://control.example.com"},
	} {
		if _, err := c.Validated(); err == nil {
			t.Fatal("accepted an incomplete bootstrap")
		}
	}
	cases := []struct {
		name   string
		mutate func(*RelayConfig)
	}{
		{"transport", func(c *RelayConfig) { c.Transport = "quic" }},
		{"empty audience", func(c *RelayConfig) { c.ControlAudience = "" }},
		{"audience whitespace", func(c *RelayConfig) { c.ControlAudience = "control audience" }},
		{"audience newline", func(c *RelayConfig) { c.ControlAudience = "control\nother" }},
		{"audience control", func(c *RelayConfig) { c.ControlAudience = "control\x00" }},
		{"audience nonascii", func(c *RelayConfig) { c.ControlAudience = "控制" }},
		{"audience length", func(c *RelayConfig) { c.ControlAudience = strings.Repeat("a", 256) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := relayTestConfig()
			tc.mutate(&c)
			if _, err := c.Validated(); err == nil {
				t.Fatal("accepted invalid bootstrap trust")
			}
		})
	}
	c := relayTestConfig()
	c.ControlAudience = strings.Repeat("a", 255)
	if _, err := c.Validated(); err != nil {
		t.Fatalf("valid protocol audience limit: %v", err)
	}
}

func TestRelayRejectsRemovedPoolKeys(t *testing.T) {
	for _, key := range []string{"interactive_connections", "bulk_connections"} {
		t.Run(key, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte("[relay]\n"+key+" = 0\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path, true); err == nil || !strings.Contains(err.Error(), "relay."+key) {
				t.Fatalf("removed configuration key %s did not produce a migration error: %v", key, err)
			}
		})
	}
}
