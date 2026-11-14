// Package pluginapi binds the built-in plugin controllers. It does not import
// the core API host and never constructs business services.
package pluginapi

import (
	"net/http"
	"net/http/httputil"

	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/artifacts"
	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/files"
	"github.com/rainchan/webmux/internal/forward"
	"github.com/rainchan/webmux/internal/handoff"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/hybridresume"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/push"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/transport"
	"github.com/rainchan/webmux/internal/voice"
	"github.com/rainchan/webmux/internal/wire"
	"github.com/rainchan/webmux/internal/workspace"
)

type Deps struct {
	Config         config.Config
	Manager        *session.Manager
	Settings       *settings.Service
	Workspace      *workspace.Service
	Code           *codeview.Service
	Files          *files.Service
	Artifacts      *artifacts.Service
	Handoffs       *handoff.Service
	Conversations  *conversation.Service
	Push           *push.Service
	Launch         *launch.Service
	Hybrid         *hybrid.Service
	Resume         *hybridresume.Service
	Forward        *forward.Service
	Voice          *voice.Service
	VoiceSettings  VoiceControl
	Tunnel         func() contract.TunnelStatus
	TunnelSettings TunnelSettingsControl
	Tailscale      TailscaleControl
	P2P            P2PControl
	MCP            http.Handler
	Guide          *agentguide.Instance
	Public         transport.PublicOriginPolicy
	LocalURL       string
	ProjectSession func(session.Info) contract.SessionInfo
}

type Server struct {
	Deps
	gatewayProxy *httputil.ReverseProxy
}

func Bind(deps Deps) *Server {
	server := &Server{Deps: deps}
	if deps.Forward != nil {
		server.gatewayProxy = newGatewayProxy(deps.Forward)
	}
	return server
}

func (s *Server) Contributions() []transport.Contribution {
	result := []transport.Contribution{}
	add := func(id string, register func(transport.Registrar), streams ...transport.Stream) {
		if !s.Config.Plugins.Has(id) {
			return
		}
		collector := &transport.Collector{}
		if register != nil {
			register(collector)
		}
		result = append(result, transport.Contribution{Owner: id, Routes: collector.Routes, Streams: streams})
	}
	add("code", s.codeRoutes)
	add("workspace", func(mux transport.Registrar) {
		s.workspaceRoutes(mux)
		s.gitRoutes(mux)
	})
	if s.Config.Plugins.Has("workspace") && s.Config.Plugins.Has("code") {
		add("workspace", s.gitCodeRoutes)
	}
	if s.Config.Plugins.Has("workspace") && s.Config.Plugins.Has("chat") {
		add("workspace", s.workspaceChatRoutes)
	}
	add("files", s.fileRoutes)
	add("chat", func(mux transport.Registrar) {
		s.chatRoutes(mux)
		s.conversationRoutes(mux)
		s.handoffRoutes(mux)
		s.sessionExtensionRoutes(mux)
		s.launchChatRoutes(mux)
	}, s.chatStream())
	add("launch", s.launchRoutes)
	add("notifications", s.pushRoutes)
	add("cloudflare", s.tunnelRoutes)
	add("tailscale", s.tailscaleRoutes)
	add("voice", s.voiceSettingsRoutes, s.voiceStream())
	add("p2p", s.p2pRoutes)
	add("forward", func(mux transport.Registrar) {
		s.forwardRoutes(mux)
		collector := mux.(*transport.Collector)
		collector.Exposure = transport.TCP
		collector.Routes = append(collector.Routes, transport.Route{
			Pattern: "GET /forward-auth", Exposure: transport.TCP,
			Shell: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !forwardAuthPage(w, r) {
						next.ServeHTTP(w, r)
					}
				})
			},
		})
		collector.HandleFunc("/forward", s.gateway)
		collector.HandleFunc("/forward/", s.gateway)
	})
	if s.Config.Plugins.Has("mcp") && s.Config.MCP.Enabled && s.MCP != nil {
		endpoint := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.MCP.ServeHTTP(w, r.WithContext(contract.WithMCPOwner(r.Context())))
		})
		result = append(result, transport.Contribution{Owner: "mcp", Routes: []transport.Route{{Pattern: "/mcp", Exposure: transport.TCP, Handler: endpoint}}})
	}
	return result
}

func (s *Server) originAllowed(r *http.Request) bool {
	return transport.OriginAllowed(s.Config, s.Public, r)
}
func (s *Server) find(w http.ResponseWriter, r *http.Request) *session.Session {
	item, err := s.Manager.Get(r.PathValue("id"))
	if err != nil {
		failure(w, http.StatusNotFound, err)
		return nil
	}
	return item
}
func (s *Server) sessionSnapshot(info session.Info) contract.SessionInfo {
	if s.ProjectSession != nil {
		return s.ProjectSession(info)
	}
	return wire.Session(info)
}
func (s *Server) fileRoutes(mux transport.Registrar) {
	mux.HandleFunc("GET /api/sessions/{id}/upload", s.readUpload)
	mux.HandleFunc("POST /api/sessions/{id}/upload", s.upload)
	mux.HandleFunc("POST /api/sessions/{id}/artifacts", s.ensureArtifacts)
	mux.HandleFunc("POST /api/sessions/{id}/agent-guide", s.agentGuide)
	mux.HandleFunc("POST /api/uploads", s.stagedUpload)
	mux.HandleFunc("GET /api/fs/dirs", s.dirs)
	mux.HandleFunc("POST /api/fs/mkdir", s.mkdir)
}
func (s *Server) dirs(w http.ResponseWriter, r *http.Request) {
	out, err := s.Files.Complete(r.URL.Query().Get("prefix"))
	if err != nil {
		filesFailure(w, err)
		return
	}
	reply(w, http.StatusOK, contract.DirectoryList(out))
}
func (s *Server) mkdir(w http.ResponseWriter, r *http.Request) {
	var request contract.MkdirRequest
	if !decode(w, r, &request) {
		return
	}
	if err := s.Files.Mkdir(request.Path); err != nil {
		filesFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// These package-local names keep the generic endpoint helpers and controller
// code concise while all body limits, redaction and encoding live in transport.
func reply(w http.ResponseWriter, status int, value any)   { transport.Reply(w, status, value) }
func failure(w http.ResponseWriter, status int, err error) { transport.Failure(w, status, err) }
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	return transport.Decode(w, r, value)
}
func decodeStrict(w http.ResponseWriter, r *http.Request, value any, limit int64, invalid error) bool {
	return transport.DecodeStrict(w, r, value, limit, invalid)
}
func decodeOptional(w http.ResponseWriter, r *http.Request, value any) bool {
	return transport.DecodeOptional(w, r, value)
}
