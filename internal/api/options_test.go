package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/artifacts"
	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/files"
	"github.com/rainchan/webmux/internal/forward"
	"github.com/rainchan/webmux/internal/handoff"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/hybridresume"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/pluginapi"
	"github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/push"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/transport"
	"github.com/rainchan/webmux/internal/voice"
	"github.com/rainchan/webmux/internal/workspace"
)

// These are fixture construction inputs, deliberately not core host fields.
type testServices struct {
	Config           config.Config
	Manager          *session.Manager
	Settings         *settings.Service
	Workspace        *workspace.Service
	Code             *codeview.Service
	Files            *files.Service
	Artifacts        *artifacts.Service
	Handoffs         *handoff.Service
	Conversations    *conversation.Service
	Push             *push.Service
	Launch           *launch.Service
	Hybrid           *hybrid.Service
	Forward          *forward.Service
	Voice            *voice.Service
	VoiceSettings    pluginapi.VoiceControl
	Tunnel           func() contract.TunnelStatus
	TunnelSettings   pluginapi.TunnelSettingsControl
	Tailscale        pluginapi.TailscaleControl
	P2P              pluginapi.P2PControl
	MCP              http.Handler
	Public           *transport.PublicOrigins
	Guide            *agentguide.Instance
	DefaultShell     string
	InstalledVersion func() string
	LocalURL         string
	Launcher         string
}

func testSessionOptions(c config.Config) session.Options {
	return session.Options{ScrollbackBytes: c.ScrollbackBytes, DefaultCwd: c.DefaultCwd, IdleAfter: time.Duration(c.IdleAfterSeconds) * time.Second, PromptPatterns: c.PromptPatterns, DisableAgentProtocol: !c.AgentProtocol, DisableBellAttention: !c.BellAttention, DisableProgramStatus: !c.ProgramStatus}
}

// Existing all-feature HTTP regressions assemble the same explicit plugin
// boundary as cmd. Core-only regressions use api.Deps directly.
func testDeps(t testing.TB, d testServices) Deps {
	t.Helper()
	if d.Workspace == nil {
		d.Workspace = workspace.New(t.TempDir(), d.Manager, d.Launch, d.Config.DefaultCwd)
		t.Cleanup(d.Workspace.Close)
	}
	if d.Artifacts == nil {
		d.Artifacts = artifacts.New(artifacts.Options{TempDir: t.TempDir(), StateDir: t.TempDir(), Manager: d.Manager})
		d.Artifacts.Start(func(string) bool { return false })
		t.Cleanup(d.Artifacts.Close)
	}
	if d.Code == nil {
		d.Code = codeview.New(d.Manager, d.Artifacts)
		t.Cleanup(d.Code.Close)
	}
	d.Code.UseWorkspace(d.Workspace)
	if d.Files == nil {
		d.Files = files.New(d.Config.DefaultCwd)
	}
	options := sessioncreate.Options{DefaultCwd: d.Config.DefaultCwd, Locations: d.Workspace, Files: d.Artifacts}
	if d.Launch != nil {
		options.Commands = d.Launch
	}
	var recovery *hybridresume.Service
	if d.Hybrid != nil {
		options.Chat = d.Hybrid.Creation()
		recovery = hybridresume.New(d.Hybrid, d.Launch, hybridresume.Options{DefaultCwd: d.Config.DefaultCwd, Creation: options.Chat, Locations: d.Workspace, ResumeLocations: d.Workspace})
	}
	if d.Guide != nil {
		options.Brief = func(id, dir string) string { return d.Guide.Brief(agentguide.BriefInput{SessionID: id, Dir: dir}) }
	}
	projection := pluginwire.Projection{Workspace: d.Workspace, Chat: d.Hybrid, Forward: d.Forward}
	controllers := pluginapi.Bind(pluginapi.Deps{Config: d.Config, Manager: d.Manager, Settings: d.Settings, Workspace: d.Workspace, Code: d.Code, Files: d.Files, Artifacts: d.Artifacts, Handoffs: d.Handoffs, Conversations: d.Conversations, Push: d.Push, Launch: d.Launch, Hybrid: d.Hybrid, Resume: recovery, Forward: d.Forward, Voice: d.Voice, VoiceSettings: d.VoiceSettings, Tunnel: d.Tunnel, TunnelSettings: d.TunnelSettings, Tailscale: d.Tailscale, P2P: d.P2P, MCP: d.MCP, Guide: d.Guide, Public: d.Public, LocalURL: d.LocalURL, ProjectSession: projection.Session})
	return Deps{Config: d.Config, Manager: d.Manager, Settings: d.Settings, Create: sessioncreate.New(d.Manager, options), Sessions: controllers, Contributions: controllers.Contributions(), ProjectSession: projection.Session, SessionProjection: hub.Options{Project: projection.Session, Terminal: projection.Terminal, SubscribeForwards: projection.SubscribeForwards, Forwards: projection.Forwards}, EnrichDetails: controllers.EnrichDetails, KeepSessionPin: controllers.KeepSessionPin, EnrichConfig: controllers.EnrichConfig, Renamed: controllers.Renamed, DefaultShell: d.DefaultShell, Public: d.Public, Tunnel: d.Tunnel, InstalledVersion: d.InstalledVersion, Guide: d.Guide, LocalURL: d.LocalURL, Launcher: d.Launcher}
}

func pluginCommandFailure(w http.ResponseWriter, err error) {
	pluginapi.Bind(pluginapi.Deps{}).CommandFailure(w, err)
}
