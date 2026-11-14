package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/client"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/p2psettings"
	"github.com/rainchan/webmux/internal/push"
	"github.com/rainchan/webmux/internal/runtime"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/statefile"
	"github.com/rainchan/webmux/internal/tailscalesettings"
	"github.com/rainchan/webmux/internal/tunnelsettings"
	"github.com/rainchan/webmux/internal/workspace"
)

func configuration(path string, c config.Config, view string, exists bool) contract.Configuration {
	fields := map[string]contract.ConfigField{}
	for key, value := range config.Fields(c) {
		fields[key] = contract.ConfigField{Value: value.Value, Source: value.Source}
	}
	return contract.Configuration{ConfigV1: contract.ConfigV1{V: contract.V, Path: path, Exists: exists, Listen: c.Listen, TokenConfigured: c.Token != "", DefaultCwd: c.DefaultCwd, StateDir: c.StateDir, AllowedOrigins: append([]string{}, c.AllowedOrigins...)}, View: view, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Fields: fields, Stores: []contract.StoreHealth{}}
}

func storeHealth(dir string, id statefile.ID, err error, live bool) contract.StoreHealth {
	description := statefile.Describe(id)
	health := contract.StoreHealth{ID: string(id), Path: filepath.Join(dir, description.Path), Status: "available", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Lock: "not_checked"}
	if live {
		health.Lock = "held"
	}
	if err != nil {
		health.Status = "unavailable"
		var storageError *statefile.Error
		if errors.As(err, &storageError) {
			health.Status = string(storageError.Kind)
			health.Phase = string(storageError.Phase)
			if storageError.Kind == statefile.Locked {
				health.Lock = "locked"
			}
		}
		if live && health.Status != "commit_uncertain" {
			health.Lock = "unavailable"
		}
	}
	return health
}

// Store ownership is separate from business settings. Disabled owners are never
// inspected, including by offline diagnostics, and never look initialized.
func storePlugin(id statefile.ID) string {
	switch id {
	case statefile.Launch:
		return "launch"
	case statefile.Repositories, statefile.GitOperations, statefile.GitTaskBaselines, statefile.GitWorkbenchState:
		return "workspace"
	case statefile.Tunnel:
		return "cloudflare"
	case statefile.TailscaleSettings, statefile.TailscaleOwnership:
		return "tailscale"
	case statefile.VoiceSettings:
		return "voice"
	case statefile.P2PSettings:
		return "p2p"
	case statefile.Push:
		return "notifications"
	case statefile.Hybrid, statefile.AgentOptions, statefile.Conversation:
		return "chat"
	default:
		return ""
	}
}

func disabledStore(c config.Config, id statefile.ID) (contract.StoreHealth, bool) {
	owner := storePlugin(id)
	if owner == "" || c.Plugins.Has(owner) {
		return contract.StoreHealth{}, false
	}
	health := storeHealth(c.StateDir, id, nil, false)
	health.Status, health.Lock = "disabled", "not_opened"
	return health, true
}

func webSources(view *contract.Configuration, tunnel, tailscale, p2pWeb bool) {
	for _, group := range []struct {
		web  bool
		keys []string
	}{{tunnel, []string{"tunnel.enabled", "tunnel.token", "tunnel.hostname"}}, {tailscale, []string{"tailscale.mode", "tailscale.port", "tailscale.provider"}}, {p2pWeb, []string{"p2p.enabled", "p2p.port_mapping", "p2p.stun_servers"}}} {
		if group.web {
			for _, key := range group.keys {
				field := view.Fields[key]
				field.Source = "web"
				view.Fields[key] = field
			}
		}
	}
}

func configuredView(path string, c config.Config) contract.Configuration {
	var tunnelWeb, tailscaleWeb, p2pWeb bool
	var tunnelError, tailscaleError, p2pError error
	if c.Plugins.Has("cloudflare") {
		c.Tunnel, tunnelWeb, tunnelError = tunnelsettings.Inspect(c.StateDir, c.Tunnel)
		if tunnelError != nil && !statefile.IsKind(tunnelError, statefile.Missing) {
			c.Tunnel.Enabled = false
		}
	}
	if c.Plugins.Has("tailscale") {
		c.Tailscale, tailscaleWeb, tailscaleError = tailscalesettings.Inspect(c.StateDir, c.Tailscale)
		if tailscaleError != nil && !statefile.IsKind(tailscaleError, statefile.Missing) {
			c.Tailscale.Mode = config.TailscaleOff
		}
	}
	if c.Plugins.Has("p2p") {
		c.P2P, p2pWeb, p2pError = p2psettings.Inspect(c.StateDir, c.P2P)
		if p2pError != nil && !statefile.IsKind(p2pError, statefile.Missing) {
			c.P2P.Enabled, c.P2P.PortMapping = false, false
		}
	}
	view := configuration(path, c, "configured", exists(path))
	webSources(&view, tunnelWeb, tailscaleWeb, p2pWeb)
	for _, description := range statefile.Catalog() {
		if health, disabled := disabledStore(c, description.ID); disabled {
			view.Stores = append(view.Stores, health)
			continue
		}
		var err error
		inspected := true
		switch description.ID {
		case statefile.Launch:
			err = launch.Inspect(c.StateDir)
		case statefile.Repositories:
			err = workspace.Inspect(c.StateDir)
		case statefile.GitOperations:
			err = workspace.InspectGitOperations(c.StateDir)
		case statefile.GitTaskBaselines:
			err = workspace.InspectGitTaskBaselines(c.StateDir)
		case statefile.GitWorkbenchState:
			err = workspace.InspectGitWorkbenchState(c.StateDir)
		case statefile.Push:
			err = push.Inspect(c.StateDir)
		case statefile.Tunnel:
			err = tunnelError
		case statefile.TailscaleSettings:
			err = tailscaleError
		case statefile.P2PSettings:
			err = p2pError
		case statefile.TailscaleOwnership:
			_, err = statefile.ReadSnapshot[tailscaleState](c.StateDir, description.ID, validateOwnership)
		case statefile.Settings:
			_, err = settings.Inspect(c.StateDir)
		case statefile.Runtime:
			_, err = statefile.ReadRuntime[runtime.Record](runtime.Dir(c.StateDir), func(record runtime.Record) error {
				if record.PID <= 0 || record.Identity.Start == "" || record.Listen == "" {
					return errors.New("invalid runtime record")
				}
				return nil
			})
		default:
			inspected = false
		}
		health := storeHealth(c.StateDir, description.ID, err, false)
		if !inspected {
			health.Status = "not_checked"
		}
		view.Stores = append(view.Stores, health)
	}
	return view
}

func liveConfiguration(path string, explicit bool) (contract.Configuration, error) {
	target := client.Locate(path, explicit)
	if target.ConfigErr != nil {
		return contract.Configuration{}, fmt.Errorf("configuration unavailable: %s", path)
	}
	if target.Record == nil {
		return contract.Configuration{}, errors.New("live configuration unavailable: no verified running instance")
	}
	connection := target.LocalClient()
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), client.RequestTimeout)
	defer cancel()
	var view contract.Configuration
	if err := connection.Do(ctx, http.MethodGet, contract.PathConfiguration, nil, &view); err != nil {
		return view, err
	}
	if view.View != "effective" || view.Instance == nil || view.Instance.PID != target.Record.PID || view.Instance.Identity != target.Record.Identity.Start {
		return contract.Configuration{}, errors.New("live configuration instance changed; retry")
	}
	return view, nil
}

func effectiveView(c serverRun, record runtime.Record, deps api.Deps, plugins *pluginServices) contract.Configuration {
	// Only adopted memory is observed: never reopen stores or read TOML here.
	var tunnelWeb, tailscaleWeb, p2pWeb bool
	tunnelEnabledSource := ""
	if plugins.tunnel != nil {
		plugins.tunnel.mu.Lock()
		c.Tunnel, tunnelWeb = plugins.tunnel.cfg, plugins.tunnel.web
		tunnelEnabledSource = plugins.tunnel.enabledSource
		plugins.tunnel.mu.Unlock()
	}
	if plugins.tailscale != nil {
		plugins.tailscale.mu.Lock()
		c.Tailscale, tailscaleWeb = plugins.tailscale.cfg, plugins.tailscale.web
		plugins.tailscale.mu.Unlock()
	}
	if plugins.p2pControl != nil {
		plugins.p2pControl.mu.Lock()
		c.P2P, p2pWeb = plugins.p2pControl.cfg, plugins.p2pControl.web
		plugins.p2pControl.mu.Unlock()
	}
	view := configuration(c.ConfigPath, c.Config, "effective", c.ConfigFound)
	webSources(&view, tunnelWeb, tailscaleWeb, p2pWeb)
	if tunnelEnabledSource != "" {
		field := view.Fields["tunnel.enabled"]
		field.Source = tunnelEnabledSource
		view.Fields["tunnel.enabled"] = field
	}
	view.Listen = record.Listen
	field := view.Fields["listen"]
	field.Value = record.Listen
	view.Fields["listen"] = field
	view.Instance = &contract.ConfigInstance{PID: record.PID, Identity: record.Identity.Start, StartedAt: record.StartedAt.UTC().Format(time.RFC3339Nano)}
	for _, description := range statefile.Catalog() {
		if health, disabled := disabledStore(c.Config, description.ID); disabled {
			view.Stores = append(view.Stores, health)
			continue
		}
		var err error
		switch description.ID {
		case statefile.Launch:
			err = plugins.launch.Unavailable()
		case statefile.Repositories:
			err = plugins.workspace.StorageError()
		case statefile.GitOperations:
			err = plugins.workspace.OperationsStorageError()
		case statefile.GitTaskBaselines:
			err = plugins.workspace.TaskBaselinesStorageError()
		case statefile.GitWorkbenchState:
			err = plugins.workspace.WorkbenchStateStorageError()
		case statefile.Push:
			if plugins.push != nil {
				err = plugins.push.Unavailable()
			}
		case statefile.Tunnel:
			err = plugins.tunnelStore.Unavailable()
		case statefile.TailscaleSettings:
			err = plugins.tailscaleStore.Unavailable()
		case statefile.P2PSettings:
			err = plugins.p2pStore.Unavailable()
		case statefile.VoiceSettings:
			err = plugins.voiceStore.Unavailable()
		case statefile.Hybrid, statefile.AgentOptions:
			err = plugins.chat.Unavailable()
		case statefile.Conversation:
			err = plugins.conversations.Unavailable()
		case statefile.Settings:
			_, err = deps.Settings.Snapshot()
		case statefile.TailscaleOwnership:
			plugins.tailscale.mu.Lock()
			err = plugins.tailscale.storageErr
			if err == nil && plugins.tailscale.ownership != nil {
				err = plugins.tailscale.ownership.Err()
			}
			plugins.tailscale.mu.Unlock()
		}
		health := storeHealth(c.StateDir, description.ID, err, true)
		if description.ID == statefile.Push && plugins.push == nil {
			health.Status, health.Lock = "disabled", "not_opened"
		}
		if description.ID == statefile.TailscaleOwnership {
			plugins.tailscale.mu.Lock()
			if plugins.tailscale.ownership == nil {
				if err == nil {
					health.Status = "not_opened"
				}
				health.Lock = "not_opened"
			}
			plugins.tailscale.mu.Unlock()
		}
		view.Stores = append(view.Stores, health)
	}
	return view
}
