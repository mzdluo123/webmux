package main

import (
	"github.com/rainchan/webmux/internal/builtin"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
)

var pluginCapabilities = [...]struct{ plugin, capability string }{
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
func (p *pluginServices) capabilities(c config.Config) contract.Capabilities {
	catalog := builtin.Catalog()
	out := contract.Capabilities{
		Version:      contract.CapabilitiesVersion,
		Plugins:      make([]contract.PluginStatus, 0, len(catalog)),
		Capabilities: make([]contract.Capability, 0, len(pluginCapabilities)+2),
	}
	out.Capabilities = append(out.Capabilities, contract.Capability{ID: "terminal.pty", Available: true})
	for _, definition := range catalog {
		status := contract.PluginStatus{ID: definition.ID, Enabled: c.Plugins.Has(definition.ID), Status: contract.PluginStatusDisabled}
		if !status.Enabled {
			status.Reason = contract.CapabilityReasonPluginDisabled
		} else if reason := p.moduleUnavailable(definition.ID); reason != "" {
			status.Status, status.Reason = contract.PluginStatusUnavailable, reason
		} else {
			status.Status = contract.PluginStatusReady
		}
		out.Plugins = append(out.Plugins, status)
	}
	for _, feature := range pluginCapabilities {
		reason := ""
		if !c.Plugins.Has(feature.plugin) {
			reason = contract.CapabilityReasonPluginDisabled
		} else if reason = p.moduleUnavailable(feature.plugin); reason == "" {
			switch feature.plugin {
			case "workspace":
				if !p.workspace.Capability().GitAvailable {
					reason = contract.CapabilityReasonCommandUnavailable
				}
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
	workbenchReason := ""
	if !c.Plugins.Has("workspace") || !c.Plugins.Has("code") {
		workbenchReason = contract.CapabilityReasonPluginDisabled
	} else if reason := p.moduleUnavailable("workspace"); reason != "" {
		workbenchReason = reason
	} else if !p.workspace.Capability().GitAvailable {
		workbenchReason = contract.CapabilityReasonCommandUnavailable
	}
	out.Capabilities = append(out.Capabilities, contract.Capability{ID: "workspace.git-workbench", Available: workbenchReason == "", Reason: workbenchReason})
	return out
}

func (p *pluginServices) moduleUnavailable(id string) string {
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
		if p.workspace.StorageError() != nil {
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

func (p *pluginServices) accessProviders() []accessProvider {
	providers := make([]accessProvider, 0, 2)
	if p.tunnel != nil {
		providers = append(providers, p.tunnel)
	}
	if p.tailscale != nil {
		providers = append(providers, p.tailscale)
	}
	return providers
}
