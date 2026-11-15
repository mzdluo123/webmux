package config

import (
	"encoding/base64"
	"strings"
	"testing"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

func relayTestConfig() RelayConfig {
	return RelayConfig{
		ControlEndpoint: "https://control.example.com",
		ControlAudience: "webmux-control",
		InitialTrust:    protocol.ServiceTrust{Algorithm: protocol.ServiceAlgorithm, Pins: []string{base64.RawURLEncoding.EncodeToString(make([]byte, 32))}},
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
		{InitialTrust: protocol.ServiceTrust{Algorithm: "unknown"}},
		{InitialTrust: protocol.ServiceTrust{Algorithm: protocol.ServiceAlgorithm}},
		{InitialTrust: protocol.ServiceTrust{Pins: []string{"not-a-pin"}}},
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
		{"wrong algorithm", func(c *RelayConfig) { c.InitialTrust.Algorithm = protocol.HostAlgorithm }},
		{"missing pins", func(c *RelayConfig) { c.InitialTrust.Pins = nil }},
		{"malformed pin", func(c *RelayConfig) { c.InitialTrust.Pins = []string{"not-a-pin"} }},
		{"noncanonical pin", func(c *RelayConfig) { c.InitialTrust.Pins[0] = strings.Repeat("A", 42) + "B" }},
		{"padded pin", func(c *RelayConfig) { c.InitialTrust.Pins[0] += "=" }},
		{"duplicate pins", func(c *RelayConfig) { c.InitialTrust.Pins = append(c.InitialTrust.Pins, c.InitialTrust.Pins[0]) }},
		{"too many pins", func(c *RelayConfig) {
			c.InitialTrust.Pins = nil
			for i := byte(0); i <= protocol.MaxPins; i++ {
				pin := make([]byte, 32)
				pin[0] = i
				c.InitialTrust.Pins = append(c.InitialTrust.Pins, base64.RawURLEncoding.EncodeToString(pin))
			}
		}},
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
