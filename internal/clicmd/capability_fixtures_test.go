package clicmd

import (
	"encoding/json"
	"net/http"

	"github.com/rainchan/webmux/internal/builtin"
	"github.com/rainchan/webmux/internal/contract"
)

// Capability fixtures are shared by Unix-only and cross-platform CLI tests.
func testCapabilities(enabled ...string) contract.Capabilities {
	if enabled == nil {
		enabled = []string{}
	}
	snapshot := contract.Capabilities{Version: contract.CapabilitiesVersion, Features: []contract.FeatureStatus{}, Capabilities: []contract.Capability{{ID: "terminal.pty", Available: true}}}
	owners := map[string]string{"launch": "launch.configs", "workspace": "workspace.manage", "chat": "chat.acp", "files": "files.upload", "code": "code.browse", "notifications": "notifications.push", "quick-commands": "quick-commands.send", "voice": "voice.recognize", "forward": "forward.ports", "cloudflare": "cloudflare.manage", "tailscale": "tailscale.manage", "p2p": "p2p.direct", "mcp": "mcp.control", "messaging": "messaging.messages"}
	for _, p := range builtin.Catalog() {
		if owners[p.ID] == "" {
			panic("missing capability fixture for feature " + p.ID)
		}
		on := builtin.Has(enabled, p.ID)
		status, reason := contract.FeatureStatusDisabled, contract.CapabilityReasonFeatureDisabled
		if on {
			status, reason = contract.FeatureStatusReady, ""
		}
		snapshot.Features = append(snapshot.Features, contract.FeatureStatus{ID: p.ID, Enabled: on, Status: status, Reason: reason})
		snapshot.Capabilities = append(snapshot.Capabilities, contract.Capability{ID: owners[p.ID], Available: on, Reason: reason})
	}
	return snapshot
}

func withTestCapabilities(handler http.Handler, snapshots ...contract.Capabilities) http.Handler {
	all, _ := builtin.Resolve(nil)
	snapshot := testCapabilities(all...)
	if len(snapshots) > 0 {
		snapshot = snapshots[0]
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == contract.PathCapabilities {
			_ = json.NewEncoder(w).Encode(snapshot)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
