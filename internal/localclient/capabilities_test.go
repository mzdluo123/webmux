package localclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/builtin"
	"github.com/rainchan/webmux/internal/contract"
)

func coreCapabilities() contract.Capabilities {
	s := contract.Capabilities{Version: 1, Plugins: []contract.PluginStatus{}, Capabilities: []contract.Capability{{ID: "terminal.pty", Available: true}}}
	for _, p := range builtin.Catalog() {
		s.Plugins = append(s.Plugins, contract.PluginStatus{ID: p.ID, Status: contract.PluginStatusDisabled, Reason: contract.CapabilityReasonPluginDisabled})
	}
	return s
}

func capabilityClient(t *testing.T, body string) *Client {
	t.Helper()
	c := New(serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != contract.PathCapabilities {
			t.Errorf("unexpected capability probe %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, body)
	})))
	t.Cleanup(c.Close)
	return c
}

func TestCoreCapabilitiesAndDisabledPlugin(t *testing.T) {
	data, err := json.Marshal(coreCapabilities())
	if err != nil {
		t.Fatal(err)
	}
	client := capabilityClient(t, string(data))
	snapshot, err := client.Capabilities(context.Background())
	if err != nil || !CapabilityAvailable(snapshot, "terminal.pty") {
		t.Fatalf("core discovery: %+v %v", snapshot, err)
	}
	err = client.RequireCapability(context.Background(), "launch.configs")
	var unavailable *CapabilityError
	if !errors.As(err, &unavailable) || unavailable.Reason != contract.CapabilityReasonPluginDisabled {
		t.Fatalf("disabled plugin: %v", err)
	}
	if CapabilityAvailable(snapshot, "unknown") {
		t.Fatal("unknown capability available")
	}
}

func TestCapabilityReaderFailsClosed(t *testing.T) {
	data, err := json.Marshal(coreCapabilities())
	if err != nil {
		t.Fatal(err)
	}
	valid := string(data)
	tests := []struct{ name, give string }{
		{"version", strings.Replace(valid, `"version":1`, `"version":2`, 1)},
		{"unknown capability", strings.Replace(valid, `terminal.pty`, `terminal.remote`, 1)},
		{"unknown plugin", strings.Replace(valid, `"id":"launch"`, `"id":"external"`, 1)},
		{"unknown status", strings.Replace(valid, `"status":"disabled"`, `"status":"maybe"`, 1)},
		{"enabled disabled", strings.Replace(valid, `"enabled":false`, `"enabled":true`, 1)},
		{"missing enabled", strings.Replace(valid, `"enabled":false,`, ``, 1)},
		{"missing available", strings.Replace(valid, `,"available":true`, ``, 1)},
		{"unknown reason", strings.Replace(valid, `plugin_disabled`, `trust_me`, 1)},
		{"duplicate field", strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1)},
		{"case folded field", strings.Replace(valid, `"version":1`, `"Version":1`, 1)},
		{"unknown field", strings.Replace(valid, `"version":1`, `"version":1,"allowEverything":true`, 1)},
		{"null arrays", `{"version":1,"plugins":null,"capabilities":null}`},
		{"trailing JSON", valid + `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := capabilityClient(t, tt.give)
			if snapshot, err := client.Capabilities(context.Background()); err == nil || len(snapshot.Capabilities) != 0 {
				t.Fatalf("accepted invalid snapshot: %+v, %v", snapshot, err)
			}
		})
	}
}

func TestCapabilityValidationRejectsDuplicateAndContradictoryEntries(t *testing.T) {
	tests := []struct {
		name   string
		change func(*contract.Capabilities)
	}{
		{"duplicate plugin", func(s *contract.Capabilities) { s.Plugins = append(s.Plugins, s.Plugins[0]) }},
		{"duplicate capability", func(s *contract.Capabilities) { s.Capabilities = append(s.Capabilities, s.Capabilities[0]) }},
		{"disabled owner", func(s *contract.Capabilities) {
			s.Capabilities = append(s.Capabilities, contract.Capability{ID: "launch.configs", Available: true})
		}},
		{"core unavailable", func(s *contract.Capabilities) {
			s.Capabilities[0].Available = false
			s.Capabilities[0].Reason = contract.CapabilityReasonServiceUnavailable
		}},
		{"missing plugin", func(s *contract.Capabilities) { s.Plugins = s.Plugins[1:] }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := coreCapabilities()
			tt.change(&s)
			if err := ValidateCapabilities(s); err == nil {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
}

func TestCombinedCapabilityRequiresBothOwners(t *testing.T) {
	snapshot := coreCapabilities()
	for i := range snapshot.Plugins {
		if snapshot.Plugins[i].ID == "workspace" {
			snapshot.Plugins[i].Enabled = true
			snapshot.Plugins[i].Status = contract.PluginStatusReady
			snapshot.Plugins[i].Reason = ""
		}
	}
	snapshot.Capabilities = append(snapshot.Capabilities, contract.Capability{ID: "workspace.git-workbench", Reason: contract.CapabilityReasonPluginDisabled})
	if err := ValidateCapabilities(snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Capabilities[len(snapshot.Capabilities)-1].Available = true
	snapshot.Capabilities[len(snapshot.Capabilities)-1].Reason = ""
	if err := ValidateCapabilities(snapshot); err == nil {
		t.Fatal("combined workbench advertised without code plugin")
	}
}
