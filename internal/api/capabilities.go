package api

import (
	"github.com/rainchan/webmux/internal/builtin"
	"github.com/rainchan/webmux/internal/contract"
)

// The host fallback never claims an optional service is healthy merely because
// it was selected. cmd supplies runtime health after binding the controllers.
func (s *Server) capabilitySnapshot() contract.Capabilities {
	if s.Capabilities != nil {
		return s.Capabilities()
	}
	snapshot := contract.Capabilities{Version: contract.CapabilitiesVersion, Features: []contract.FeatureStatus{}, Capabilities: []contract.Capability{{ID: "terminal.pty", Available: true}}}
	ids := map[string]string{"workspace": "workspace.manage", "code": "code.browse", "files": "files.upload", "launch": "launch.configs", "chat": "chat.acp", "notifications": "notifications.push", "quick-commands": "quick-commands.send", "voice": "voice.recognize", "forward": "forward.ports", "cloudflare": "cloudflare.manage", "tailscale": "tailscale.manage", "p2p": "p2p.direct", "mcp": "mcp.control", "messaging": "messaging.messages"}
	for _, definition := range builtin.Catalog() {
		enabled := s.Config.Features.Has(definition.ID)
		status, reason := contract.FeatureStatusDisabled, contract.CapabilityReasonFeatureDisabled
		if enabled {
			status, reason = contract.FeatureStatusUnavailable, contract.CapabilityReasonServiceUnavailable
		}
		snapshot.Features = append(snapshot.Features, contract.FeatureStatus{ID: definition.ID, Enabled: enabled, Status: status, Reason: reason})
		snapshot.Capabilities = append(snapshot.Capabilities, contract.Capability{ID: ids[definition.ID], Available: false, Reason: reason})
	}
	for _, feature := range []struct {
		id      string
		enabled bool
	}{
		{"workspace.projects", s.Config.Features.Has("workspace")},
		{"workspace.clone", s.Config.Features.Has("workspace")},
		{"workspace.github", s.Config.Features.Has("workspace")},
		{"workspace.git-workbench", s.Config.Features.Has("workspace") && s.Config.Features.Has("code")},
	} {
		reason := contract.CapabilityReasonFeatureDisabled
		if feature.enabled {
			reason = contract.CapabilityReasonServiceUnavailable
		}
		snapshot.Capabilities = append(snapshot.Capabilities, contract.Capability{ID: feature.id, Reason: reason})
	}
	return snapshot
}
