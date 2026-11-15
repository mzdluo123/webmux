package main

import (
	"github.com/rainchan/webmux/internal/builtin"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
)

var featureCapabilities = [...]struct{ feature, capability string }{
	{"launch", "launch.configs"},
	{"workspace", "workspace.manage"},
	{"code", "code.browse"},
	{"files", "files.upload"},
	{"chat", "chat.acp"},
	{"notifications", "notifications.push"},
	{"quick-commands", "quick-commands.send"},
	{"voice", "voice.recognize"},
	{"forward", "forward.ports"},
	{"cloudflare", "cloudflare.manage"},
	{"tailscale", "tailscale.manage"},
	{"p2p", "p2p.direct"},
	{"mcp", "mcp.control"},
	{"immersive", "immersive.xr"},
}

// capabilities observes already constructed services; it never opens a store,
// probes a disabled external command or starts an optional producer.
func (p *featureServices) capabilities(c config.Config) contract.Capabilities {
	catalog := builtin.Catalog()
	out := contract.Capabilities{
		Version:      contract.CapabilitiesVersion,
		Features:     make([]contract.FeatureStatus, 0, len(catalog)),
		Capabilities: make([]contract.Capability, 0, len(featureCapabilities)+2),
	}
	out.Capabilities = append(out.Capabilities, contract.Capability{ID: "terminal.pty", Available: true})
	for _, definition := range catalog {
		status := contract.FeatureStatus{ID: definition.ID, Enabled: c.Features.Has(definition.ID), Status: contract.FeatureStatusDisabled}
		if !status.Enabled {
			status.Reason = contract.CapabilityReasonFeatureDisabled
		} else if reason := p.moduleUnavailable(definition.ID); reason != "" {
			status.Status, status.Reason = contract.FeatureStatusUnavailable, reason
		} else {
			status.Status = contract.FeatureStatusReady
		}
		out.Features = append(out.Features, status)
	}
	for _, feature := range featureCapabilities {
		reason := ""
		if !c.Features.Has(feature.feature) {
			reason = contract.CapabilityReasonFeatureDisabled
		} else if reason = p.moduleUnavailable(feature.feature); reason == "" {
			switch feature.feature {
			case "workspace":
				reason = p.workspaceRegistryReason()
			case "notifications":
				if !c.Push.Enabled {
					reason = contract.CapabilityReasonConfigurationOff
				}
			case "voice":
				if !p.voice.Enabled() {
					reason = contract.CapabilityReasonConfigurationOff
				} else if !p.voiceControl.recognizerAvailable() {
					reason = contract.CapabilityReasonCommandUnavailable
				}
			case "p2p":
				if !p.p2p.Enabled() {
					reason = contract.CapabilityReasonConfigurationOff
				}
			}
		}
		out.Capabilities = append(out.Capabilities, contract.Capability{ID: feature.capability, Available: reason == "", Reason: reason})
	}
	// Relay owner optimization uses the selected implementation, not the local
	// direct-upgrade toggle. Authorization is still checked per browser peer.
	hybridReason := ""
	if !c.Features.Has("p2p") {
		hybridReason = contract.CapabilityReasonFeatureDisabled
	} else if p.p2p == nil {
		hybridReason = contract.CapabilityReasonServiceUnavailable
	} else if !p.p2p.Supported() {
		hybridReason = contract.CapabilityReasonUnsupported
	}
	out.Capabilities = append(out.Capabilities, contract.Capability{ID: "relay.hybrid", Available: hybridReason == "", Reason: hybridReason})
	projectReason := ""
	if !c.Features.Has("workspace") {
		projectReason = contract.CapabilityReasonFeatureDisabled
	} else if p.workspace == nil {
		projectReason = contract.CapabilityReasonServiceUnavailable
	} else if p.workspace.ProjectStorageError() != nil {
		projectReason = contract.CapabilityReasonStorageUnavailable
	}
	out.Capabilities = append(out.Capabilities, contract.Capability{ID: "workspace.projects", Available: projectReason == "", Reason: projectReason})
	cloneReason := projectReason
	if cloneReason == "" {
		if p.workspace.ProjectCloneStorageError() != nil || p.workspace.StorageError() != nil {
			cloneReason = contract.CapabilityReasonStorageUnavailable
		} else if !p.workspace.Capability().GitAvailable {
			cloneReason = contract.CapabilityReasonCommandUnavailable
		}
	}
	out.Capabilities = append(out.Capabilities, contract.Capability{ID: "workspace.clone", Available: cloneReason == "", Reason: cloneReason})
	githubReason := cloneReason
	if githubReason == "" && !p.workspace.GitHubAvailable() {
		githubReason = contract.CapabilityReasonCommandUnavailable
	}
	out.Capabilities = append(out.Capabilities, contract.Capability{ID: "workspace.github", Available: githubReason == "", Reason: githubReason})
	workbenchReason := ""
	if !c.Features.Has("workspace") || !c.Features.Has("code") {
		workbenchReason = contract.CapabilityReasonFeatureDisabled
	} else {
		workbenchReason = p.workspaceRegistryReason()
	}
	out.Capabilities = append(out.Capabilities, contract.Capability{ID: "workspace.git-workbench", Available: workbenchReason == "", Reason: workbenchReason})
	return out
}

func (p *featureServices) moduleUnavailable(id string) string {
	switch id {
	case "launch":
		if p.launch == nil {
			return contract.CapabilityReasonServiceUnavailable
		}
		if p.launch.Unavailable() != nil {
			return contract.CapabilityReasonStorageUnavailable
		}
	case "workspace":
		if p.workspace == nil {
			return contract.CapabilityReasonServiceUnavailable
		}
		if p.workspace.StorageError() != nil && p.workspace.ProjectStorageError() != nil {
			return contract.CapabilityReasonStorageUnavailable
		}
	case "code":
		if p.code == nil {
			return contract.CapabilityReasonServiceUnavailable
		}
	case "files":
		if p.files == nil || p.artifacts == nil {
			return contract.CapabilityReasonServiceUnavailable
		}
	case "chat":
		if p.chat == nil || p.conversations == nil {
			return contract.CapabilityReasonServiceUnavailable
		}
		if p.chat.Unavailable() != nil || p.conversations.Unavailable() != nil {
			return contract.CapabilityReasonStorageUnavailable
		}
	case "notifications":
		if p.push != nil && p.push.Unavailable() != nil {
			return contract.CapabilityReasonStorageUnavailable
		}
	case "voice":
		if p.voice == nil || p.voiceStore == nil {
			return contract.CapabilityReasonServiceUnavailable
		}
		if p.voiceStore.Unavailable() != nil {
			return contract.CapabilityReasonStorageUnavailable
		}
	case "forward":
		if p.forward == nil {
			return contract.CapabilityReasonServiceUnavailable
		}
	case "cloudflare":
		if p.tunnel == nil || p.tunnelStore == nil {
			return contract.CapabilityReasonServiceUnavailable
		}
		if p.tunnelStore.Unavailable() != nil {
			return contract.CapabilityReasonStorageUnavailable
		}
	case "tailscale":
		if p.tailscale == nil || p.tailscaleStore == nil {
			return contract.CapabilityReasonServiceUnavailable
		}
		if p.tailscaleStore.Unavailable() != nil {
			return contract.CapabilityReasonStorageUnavailable
		}
	case "p2p":
		if p.p2p == nil || p.p2pStore == nil {
			return contract.CapabilityReasonServiceUnavailable
		}
		if !p.p2p.Supported() {
			return contract.CapabilityReasonUnsupported
		}
		if p.p2pStore.Unavailable() != nil {
			return contract.CapabilityReasonStorageUnavailable
		}
	}
	return ""
}

func (p *featureServices) accessProviders() []accessProvider {
	providers := make([]accessProvider, 0, 2)
	if p.tunnel != nil {
		providers = append(providers, p.tunnel)
	}
	if p.tailscale != nil {
		providers = append(providers, p.tailscale)
	}
	return providers
}

// Workspace may have healthy directory projects even when its Git registry is
// unavailable; capability ownership must not flatten that partial health.
func (p *featureServices) workspaceRegistryReason() string {
	if p.workspace == nil {
		return contract.CapabilityReasonServiceUnavailable
	}
	if p.workspace.StorageError() != nil {
		return contract.CapabilityReasonStorageUnavailable
	}
	if !p.workspace.Capability().GitAvailable {
		return contract.CapabilityReasonCommandUnavailable
	}
	return ""
}
