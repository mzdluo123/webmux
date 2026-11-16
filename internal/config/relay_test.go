package config

import (
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

func TestRelayFixedPoolCounts(t *testing.T) {
	for _, count := range []int{-1, 17} {
		cfg := DefaultRelayConfig()
		cfg.InteractiveConnections = count
		if _, err := cfg.Validated(); err == nil {
			t.Fatalf("interactive count %d accepted", count)
		}
		cfg = DefaultRelayConfig()
		cfg.BulkConnections = count
		if _, err := cfg.Validated(); err == nil {
			t.Fatalf("bulk count %d accepted", count)
		}
	}
	cfg := DefaultRelayConfig()
	cfg.InteractiveConnections, cfg.BulkConnections = 0, 0
	got, err := cfg.Validated()
	if err != nil || got.InteractiveConnections != 2 || got.BulkConnections != 4 {
		t.Fatalf("default fixed pool counts: %+v %v", got, err)
	}
}
