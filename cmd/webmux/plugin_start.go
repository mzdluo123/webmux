package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"slices"

	"github.com/rainchan/webmux/cmd/internal/assemble"
	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/artifacts"
	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/files"
	"github.com/rainchan/webmux/internal/forward"
	"github.com/rainchan/webmux/internal/handoff"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/hybridresume"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/p2p"
	"github.com/rainchan/webmux/internal/p2psettings"
	"github.com/rainchan/webmux/internal/push"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/streamproto"
	"github.com/rainchan/webmux/internal/tailscale"
	"github.com/rainchan/webmux/internal/tailscalesettings"
	"github.com/rainchan/webmux/internal/transport"
	"github.com/rainchan/webmux/internal/tunnel"
	"github.com/rainchan/webmux/internal/tunnelsettings"
	"github.com/rainchan/webmux/internal/voice"
	"github.com/rainchan/webmux/internal/voicesettings"
	"github.com/rainchan/webmux/internal/workspace"
	"github.com/rainchan/webmux/internal/wsl"
)

// open is called only after the selection is validated. Constructors remain
// explicit here: neither services nor the HTTP host look up another plugin.
func (p *pluginServices) open(ctx context.Context, c serverRun, m *session.Manager, preferences *settings.Service, public *transport.PublicOrigins, address net.Addr, guide *agentguide.Instance, localAPI func() *localRuntime, bridge *transport.Bridge, out io.Writer) error {
	if c.Plugins.Has("launch") {
		p.launch = launch.Open(c.StateDir)
		if err := p.launch.Unavailable(); err != nil {
			slog.Warn("launch configs unavailable", "error", err)
		} else if err := p.launch.Seed(launch.Presets(os.Getenv("SHELL"))); err != nil {
			slog.Warn("seed default launch configs", "error", err)
		}
	}
	if c.Plugins.Has("files") {
		p.files = files.New(c.DefaultCwd)
		p.artifacts = artifacts.New(artifacts.Options{StateDir: c.StateDir, Manager: m, Logger: slog.Default()})
	}
	if c.Plugins.Has("chat") {
		p.conversations = conversation.Open(conversation.Options{StateDir: c.StateDir, DisableCapture: !c.HistoryCapture})
		options := hybrid.Options{Conversations: p.conversations, StateDir: c.StateDir, Manager: m, ScrollbackBytes: c.ScrollbackBytes, Version: version, Logger: slog.Default(), Uploads: p.artifacts.UploadPath}
		if c.Plugins.Has("mcp") {
			options.MCPServers = controlMCPServers(serverExecutable(), func() string {
				rt := localAPI()
				if rt == nil || rt.listener == nil {
					return ""
				}
				return rt.listener.Addr().String()
			})
			options.HistoryReader = historyReader(c.StateDir, serverExecutable(), p.conversations)
		}
		p.chat = hybrid.New(options)
		if err := p.chat.Unavailable(); err != nil {
			slog.Warn("hybrid records unreadable; session artifacts folders kept", "error", err)
		}
		p.launch.OnDelete(func(id string) {
			if err := p.chat.ForgetAgentOptions(id); err != nil {
				slog.Warn("forget agent options of a deleted launch config", "error", err)
			}
		})
	}
	if p.artifacts != nil {
		p.artifacts.Start(func(id string) bool {
			// Without chat there is no authority to declare a historical folder
			// unowned. Do not open its database just to garbage-collect it.
			if p.chat == nil {
				return true
			}
			_, err := p.chat.Record(id)
			return !errors.Is(err, hybrid.ErrNotFound)
		})
	}
	if c.Plugins.Has("workspace") {
		p.workspace = workspace.New(c.StateDir, m, p.launch, c.DefaultCwd)
		if p.chat != nil {
			p.workspace.UseHybrid(p.chat)
			p.chat.SetWorkspaceSource(func(id string) *hybrid.WorkspaceRef {
				association := p.workspace.Association(id)
				if association == nil {
					return nil
				}
				return &hybrid.WorkspaceRef{RepositoryID: association.RepositoryID, WorktreeID: association.WorktreeID}
			})
		}
		if c.Plugins.Has("quick-commands") {
			p.workspace.UseQuickCommands(func() []workspace.QuickCommand {
				snapshot, err := preferences.Snapshot()
				if err != nil || snapshot.Values.QuickCommands == nil {
					return nil
				}
				list := make([]workspace.QuickCommand, 0, len(*snapshot.Values.QuickCommands))
				for _, command := range *snapshot.Values.QuickCommands {
					list = append(list, workspace.QuickCommand{Name: command.Name, Text: command.Command})
				}
				return list
			})
		}
	}
	if p.chat != nil {
		p.chat.OnEvicted(func(ctx context.Context, rec hybrid.Record) {
			if rec.Released != nil && p.workspace != nil {
				if err := p.workspace.DropRelease(ctx, *rec.Released); err != nil {
					slog.Warn("released ref removal of an evicted record failed", "release", rec.Released.ID, "error", err)
				}
			}
			p.artifacts.Remove(rec.ID)
		})
		creation := p.chat.Creation()
		resumeOptions := hybridresume.Options{DefaultCwd: c.DefaultCwd, Creation: creation}
		handoffOptions := handoff.Options{Creation: creation, Conversations: p.conversations, Launch: p.launch, Artifacts: p.artifacts, Hybrid: p.chat, TokenCap: int64(c.HandoffTokenCap), MaxPromptBytes: hybrid.MaxPromptBytes, MaxPromptWireBytes: hybrid.MaxPromptWireBytes}
		if p.workspace != nil {
			resumeOptions.Locations, resumeOptions.ResumeLocations = p.workspace, p.workspace
			handoffOptions.Locations = p.workspace
		}
		p.recovery = hybridresume.New(p.chat, p.launch, resumeOptions)
		handoffOptions.Recovery = p.recovery
		p.handoff = handoff.New(handoffOptions)
	}
	createOptions := sessioncreate.Options{DefaultCwd: c.DefaultCwd, Brief: func(id, dir string) string {
		return guide.Brief(agentguide.BriefInput{SessionID: id, Dir: dir})
	}}
	if p.launch != nil {
		createOptions.Commands = p.launch
	}
	if p.workspace != nil {
		createOptions.Locations = p.workspace
	}
	if p.artifacts != nil {
		createOptions.Files = p.artifacts
	}
	if p.chat != nil {
		createOptions.Chat = p.chat.Creation()
	}
	p.creation = sessioncreate.New(m, createOptions)
	if c.Plugins.Has("code") {
		p.code = codeview.New(m, p.artifacts)
		if p.workspace != nil {
			p.code.UseWorkspace(p.workspace)
		}
	}
	if c.Plugins.Has("notifications") {
		p.push = push.New(c.Push, c.StateDir, m, slog.Default(), nil)
	}
	if c.Plugins.Has("forward") {
		options := assemble.ForwardOptions(c.Config, m)
		if address, ok := address.(*net.TCPAddr); ok && !slices.Contains(options.ExcludePorts, address.Port) {
			options.ExcludePorts = append(options.ExcludePorts, address.Port)
		}
		p.forward = forward.New(options)
	}
	if err := p.openAccess(ctx, c, public, address, out); err != nil {
		return err
	}
	if c.Plugins.Has("voice") {
		p.voiceStore = voicesettings.Open(c.StateDir, c.Voice)
		if err := p.voiceStore.Unavailable(); err != nil {
			slog.Warn("voice settings unavailable; speech recognition stays off", "error", err)
		}
		voiceConfig := c.Config
		voiceConfig.Voice, _ = p.voiceStore.Effective()
		p.voice = voice.New(assemble.VoiceOptions(voiceConfig))
		p.voiceControl = newVoiceController(p.voiceStore, p.voice, voiceConfig)
	}
	if c.Plugins.Has("p2p") {
		p.p2pStore = c.P2PSettings
		if p.p2pStore == nil {
			p.p2pStore = p2psettings.Open(c.StateDir, c.P2P)
		}
		cfg := c.Config
		cfg.P2P, _ = p.p2pStore.Effective()
		p.p2p = p2p.New(assemble.P2POptions(cfg, version, func(sctx context.Context, path string, conn streamproto.StreamConn) {
			if bridge.ServeStream != nil {
				bridge.ServeStream(sctx, path, conn)
			}
		}, func(rctx context.Context, req streamproto.RPCRequest) streamproto.RPCResponse {
			if bridge.ServeRPC != nil {
				return bridge.ServeRPC(rctx, req)
			}
			return streamproto.RPCResponse{Status: http.StatusServiceUnavailable, StatusText: http.StatusText(http.StatusServiceUnavailable)}
		}))
		p.p2pControl = newP2PController(p.p2pStore, p.p2p)
	}
	return nil
}

func (p *pluginServices) openAccess(ctx context.Context, c serverRun, public *transport.PublicOrigins, address net.Addr, out io.Writer) error {
	local := localOrigin(address)
	if c.Plugins.Has("cloudflare") {
		p.tunnelStore = c.TunnelSettings
		if p.tunnelStore == nil {
			p.tunnelStore = tunnelsettings.Open(c.StateDir, c.Tunnel)
		}
		cfg, web := p.tunnelStore.Effective()
		if c.Source("tunnel.enabled") == "cli" {
			cfg.Enabled, web = c.Tunnel.Enabled, false
		}
		locator := tunnel.Locator{Explicit: cfg.Cloudflared, StateDir: c.StateDir, Mirrors: cfg.DownloadMirrors}
		p.tunnel = newTunnelController(tunnelControllerOptions{Cfg: cfg, Store: p.tunnelStore, Local: local, Token: c.Token, Public: public, Locate: locator.Find, Out: out, Logger: slog.Default()})
		p.tunnel.defaultEnabledSource, p.tunnel.enabledSource = c.BaseSource("tunnel.enabled"), c.Source("tunnel.enabled")
		if web {
			p.tunnel.enabledSource = "web"
		}
		if cfg.Enabled {
			if err := p.tunnelStore.Unavailable(); err == nil && !web {
				path, err := locator.Find(ctx)
				if err != nil {
					return fmt.Errorf("cloudflare tunnel: %w", err)
				}
				p.tunnelPath = path
			}
		}
	}
	if c.Plugins.Has("tailscale") {
		p.tailscaleStore = c.TailscaleSettings
		if p.tailscaleStore == nil {
			p.tailscaleStore = tailscalesettings.Open(c.StateDir, c.Tailscale)
		}
		cfg := c.Config
		cfg.Tailscale, _ = p.tailscaleStore.Effective()
		p.tailscale = newTailscaleController(tailscaleControllerOptions{Cfg: cfg.Tailscale, Store: p.tailscaleStore, Service: tailscale.New(assemble.TailscaleOptions(cfg)), StateDir: c.StateDir, Public: public, Token: c.Token, TokenGenerated: c.TokenGenerated, Out: out, Logger: slog.Default(), IsWSL: wsl.Detected, NetworkingMode: wsl.NetworkingMode})
		if err := p.tailscale.Prepare(ctx, c.Listen); err != nil {
			return err
		}
	}
	return nil
}
