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
	s := contract.Capabilities{Version: contract.CapabilitiesVersion, Features: []contract.FeatureStatus{}, Capabilities: []contract.Capability{{ID: "terminal.pty", Available: true}}}
	for _, p := range builtin.Catalog() {
		s.Features = append(s.Features, contract.FeatureStatus{ID: p.ID, Status: contract.FeatureStatusDisabled, Reason: contract.CapabilityReasonFeatureDisabled})
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

func TestCoreCapabilitiesAndDisabledFeature(t *testing.T) {
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
	if !errors.As(err, &unavailable) || unavailable.Reason != contract.CapabilityReasonFeatureDisabled {
		t.Fatalf("disabled feature: %v", err)
	}
	if CapabilityAvailable(snapshot, "unknown") {
		t.Fatal("unknown capability available")
	}
}

func TestCapabilitiesV1SnapshotDecodes(t *testing.T) {
	// The retired wire shape carried plugins and the plugin_disabled reason;
	// a transition-period client reads it without weakening validation.
	v1 := `{"version":1,"plugins":[{"id":"launch","enabled":true,"status":"ready"},{"id":"workspace","enabled":false,"status":"disabled","reason":"plugin_disabled"},{"id":"code","enabled":false,"status":"disabled","reason":"plugin_disabled"},{"id":"files","enabled":true,"status":"ready"},{"id":"chat","enabled":true,"status":"ready"},{"id":"notifications","enabled":false,"status":"disabled","reason":"plugin_disabled"},{"id":"quick-commands","enabled":false,"status":"disabled","reason":"plugin_disabled"},{"id":"voice","enabled":false,"status":"disabled","reason":"plugin_disabled"},{"id":"forward","enabled":false,"status":"disabled","reason":"plugin_disabled"},{"id":"cloudflare","enabled":false,"status":"disabled","reason":"plugin_disabled"},{"id":"tailscale","enabled":false,"status":"disabled","reason":"plugin_disabled"},{"id":"p2p","enabled":false,"status":"disabled","reason":"plugin_disabled"},{"id":"mcp","enabled":false,"status":"disabled","reason":"plugin_disabled"},{"id":"immersive","enabled":false,"status":"disabled","reason":"plugin_disabled"}],"capabilities":[{"id":"terminal.pty","available":true},{"id":"launch.configs","available":true},{"id":"files.upload","available":true},{"id":"chat.acp","available":true},{"id":"workspace.manage","available":false,"reason":"plugin_disabled"},{"id":"code.browse","available":false,"reason":"plugin_disabled"},{"id":"notifications.push","available":false,"reason":"plugin_disabled"},{"id":"quick-commands.send","available":false,"reason":"plugin_disabled"},{"id":"voice.recognize","available":false,"reason":"plugin_disabled"},{"id":"forward.ports","available":false,"reason":"plugin_disabled"},{"id":"cloudflare.manage","available":false,"reason":"plugin_disabled"},{"id":"tailscale.manage","available":false,"reason":"plugin_disabled"},{"id":"p2p.direct","available":false,"reason":"plugin_disabled"},{"id":"mcp.control","available":false,"reason":"plugin_disabled"},{"id":"immersive.xr","available":false,"reason":"plugin_disabled"},{"id":"workspace.projects","available":false,"reason":"plugin_disabled"},{"id":"workspace.clone","available":false,"reason":"plugin_disabled"},{"id":"workspace.github","available":false,"reason":"plugin_disabled"},{"id":"workspace.git-workbench","available":false,"reason":"plugin_disabled"}]}`
	client := capabilityClient(t, v1)
	snapshot, err := client.Capabilities(context.Background())
	if err != nil || !FeatureEnabled(snapshot, "launch") || FeatureEnabled(snapshot, "workspace") {
		t.Fatalf("v1 discovery: %+v %v", snapshot, err)
	}
	if err := client.RequireFeature(context.Background(), "workspace"); err == nil {
		t.Fatal("disabled v1 feature accepted")
	} else {
		var unavailable *CapabilityError
		if !errors.As(err, &unavailable) || unavailable.Reason != contract.CapabilityReasonFeatureDisabled {
			t.Fatalf("v1 disabled reason: %v", err)
		}
	}
}

func TestCapabilityReaderFailsClosed(t *testing.T) {
	data, err := json.Marshal(coreCapabilities())
	if err != nil {
		t.Fatal(err)
	}
	valid := string(data)
	tests := []struct{ name, give string }{
		{"version", strings.Replace(valid, `"version":2`, `"version":3`, 1)},
		{"unknown capability", strings.Replace(valid, `terminal.pty`, `terminal.remote`, 1)},
		{"unknown feature", strings.Replace(valid, `"id":"launch"`, `"id":"external"`, 1)},
		{"unknown status", strings.Replace(valid, `"status":"disabled"`, `"status":"maybe"`, 1)},
		{"enabled disabled", strings.Replace(valid, `"enabled":false`, `"enabled":true`, 1)},
		{"missing enabled", strings.Replace(valid, `"enabled":false,`, ``, 1)},
		{"missing available", strings.Replace(valid, `,"available":true`, ``, 1)},
		{"unknown reason", strings.Replace(valid, `feature_disabled`, `trust_me`, 1)},
		{"duplicate field", strings.Replace(valid, `"version":2`, `"version":2,"version":2`, 1)},
		{"case folded field", strings.Replace(valid, `"version":2`, `"Version":2`, 1)},
		{"unknown field", strings.Replace(valid, `"version":2`, `"version":2,"allowEverything":true`, 1)},
		{"null arrays", `{"version":2,"features":null,"capabilities":null}`},
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
		{"duplicate feature", func(s *contract.Capabilities) { s.Features = append(s.Features, s.Features[0]) }},
		{"duplicate capability", func(s *contract.Capabilities) { s.Capabilities = append(s.Capabilities, s.Capabilities[0]) }},
		{"disabled owner", func(s *contract.Capabilities) {
			s.Capabilities = append(s.Capabilities, contract.Capability{ID: "launch.configs", Available: true})
		}},
		{"core unavailable", func(s *contract.Capabilities) {
			s.Capabilities[0].Available = false
			s.Capabilities[0].Reason = contract.CapabilityReasonServiceUnavailable
		}},
		{"missing feature", func(s *contract.Capabilities) { s.Features = s.Features[1:] }},
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
	for i := range snapshot.Features {
		if snapshot.Features[i].ID == "workspace" {
			snapshot.Features[i].Enabled = true
			snapshot.Features[i].Status = contract.FeatureStatusReady
			snapshot.Features[i].Reason = ""
		}
	}
	snapshot.Capabilities = append(snapshot.Capabilities, contract.Capability{ID: "workspace.git-workbench", Reason: contract.CapabilityReasonFeatureDisabled})
	if err := ValidateCapabilities(snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Capabilities[len(snapshot.Capabilities)-1].Available = true
	snapshot.Capabilities[len(snapshot.Capabilities)-1].Reason = ""
	if err := ValidateCapabilities(snapshot); err == nil {
		t.Fatal("combined workbench advertised without code feature")
	}
}
