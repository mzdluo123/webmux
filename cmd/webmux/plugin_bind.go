package main

import (
	"net/http"

	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/pluginapi"
	"github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/transport"
)

func (p *pluginServices) bind(c config.Config, manager *session.Manager, preferences *settings.Service, public *transport.PublicOrigins, guide *agentguide.Instance, local string) api.Deps {
	projection := pluginwire.Projection{Workspace: p.workspace, Chat: p.chat, Forward: p.forward}
	deps := api.Deps{Config: c, Manager: manager, Create: p.creation, Settings: preferences, Public: public,
		Capabilities:      func() contract.Capabilities { return p.capabilities(c) },
		ProjectSession:    projection.Session,
		SessionProjection: hub.Options{Project: projection.Session, Terminal: projection.Terminal, SubscribeForwards: projection.SubscribeForwards, Forwards: projection.Forwards},
	}
	if len(c.Plugins.Enabled) == 0 && c.Plugins.Enabled != nil {
		return deps
	}
	options := pluginapi.Deps{
		Config: c, Manager: manager, Settings: preferences,
		Workspace: p.workspace, Code: p.code, Files: p.files, Artifacts: p.artifacts,
		Handoffs: p.handoff, Conversations: p.conversations,
		Push: p.push, Launch: p.launch, Hybrid: p.chat, Forward: p.forward,
		Voice: p.voice, Public: public, ProjectSession: projection.Session,
		Guide: guide, LocalURL: local, Resume: p.recovery,
	}
	// Do not put a typed nil in an optional interface: the controller treats an
	// absent port differently from an enabled module with an unavailable store.
	if p.voiceControl != nil {
		options.VoiceSettings = p.voiceControl
	}
	if p.p2pControl != nil {
		options.P2P = p.p2pControl
	}
	if p.tailscale != nil {
		options.Tailscale = p.tailscale
	}
	if p.tunnel != nil {
		options.Tunnel, options.TunnelSettings = p.tunnel.Status, p.tunnel
		deps.Tunnel = p.tunnel.Status
	}
	controllers := pluginapi.Bind(options)
	deps.Sessions = controllers
	deps.Contributions = controllers.Contributions()
	deps.EnrichDetails = controllers.EnrichDetails
	deps.KeepSessionPin = controllers.KeepSessionPin
	deps.EnrichConfig = controllers.EnrichConfig
	deps.Renamed = controllers.Renamed
	return deps
}

func networkMCPContribution(handler http.Handler) transport.Contribution {
	authenticated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(contract.WithMCPOwner(r.Context())))
	})
	return transport.Contribution{Owner: "mcp", Routes: []transport.Route{
		{Pattern: "POST /mcp", Exposure: transport.TCP, Handler: authenticated},
		{Pattern: "GET /mcp", Exposure: transport.TCP, Handler: authenticated},
		{Pattern: "DELETE /mcp", Exposure: transport.TCP, Handler: authenticated},
	}}
}
