package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hoststat"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/transport"
	"github.com/rainchan/webmux/internal/wire"
)

// Deps contains only terminal-host infrastructure and prebound extension ports.
// Optional controllers and business services are constructed by cmd, not here.
type Deps struct {
	Config            config.Config
	Manager           *session.Manager
	Create            *sessioncreate.Service
	Settings          *settings.Service
	Sessions          SessionExtensions
	Contributions     []transport.Contribution
	Capabilities      func() contract.Capabilities
	ProjectSession    func(session.Info) contract.SessionInfo
	SessionProjection hub.Options
	EnrichDetails     func(context.Context, session.Info, *contract.SessionDetails)
	KeepSessionPin    func(string) bool
	EnrichConfig      func(*contract.ServerConfig)
	Renamed           func(string, string) error
	DefaultShell      string
	Public            *transport.PublicOrigins
	Tunnel            func() contract.TunnelStatus
	InstalledVersion  func() string
	Guide             *agentguide.Instance
	LocalURL          string
	Launcher          string
}

type Server struct {
	Deps
	local   bool
	streams []transport.Stream
}

func New(d Deps) http.Handler { return NewBridge(d).HTTPHandler }

func newServer(d Deps) *Server {
	if d.Manager == nil {
		panic("api: Deps requires Manager")
	}
	server := &Server{Deps: d}
	server.streams = server.coreStreams()
	return server
}

// routes validates the complete composition before exposing a listener.
func (s *Server) routes(exposure transport.Exposure) *http.ServeMux {
	mux, err := transport.Mount(s.coreRoutes, s.Contributions, exposure, http.HandlerFunc(s.static))
	if err != nil {
		panic(err)
	}
	return mux
}

func (s *Server) coreRoutes(registrar transport.Registrar) {
	// Core route helpers still register on ServeMux; validation and listener muxes
	// both satisfy that concrete boundary.
	mux := registrar.(*http.ServeMux)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		load, memory := hoststat.Read()
		reply(w, http.StatusOK, wire.Health(len(s.Manager.List()), load, memory))
	})
	mux.HandleFunc("GET /api/capabilities", func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusOK, s.capabilitySnapshot())
	})
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		hostname, _ := os.Hostname()
		view := contract.ServerConfig{DefaultCwd: s.Config.DefaultCwd, DefaultShell: s.DefaultShell, Hostname: hostname, Tunnel: s.tunnelInfo(), PendingUpdate: s.pendingUpdate()}
		if s.EnrichConfig != nil {
			s.EnrichConfig(&view)
		}
		reply(w, http.StatusOK, view)
	})
	s.sessionRoutes(mux)
	serveIn(mux, "PATCH /api/sessions/{id}", http.StatusOK, s.commandFailure, s.rename)
	mux.HandleFunc("GET /api/sessions/{id}/buffer", s.buffer)
	mux.HandleFunc("GET /api/sessions/{id}/output", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		page, err := s.output(r)
		if err != nil {
			s.commandFailure(w, err)
			return
		}
		reply(w, http.StatusOK, page)
	})
	s.sessionMetaRoutes(mux)
	s.guideRoutes(mux)
	s.settingsRoutes(mux)
	// The same core stream descriptions are also used by the P2P bridge.
	for _, stream := range s.streams {
		mux.Handle(stream.Pattern, transport.StreamHTTP(stream))
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/ws/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/local/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Business pages must be mounted explicitly by an enabled contribution.
		// The shell fallback must not expose TCP-only pages on local or P2P.
		if r.URL.Path == "/forward" || strings.HasPrefix(r.URL.Path, "/forward/") || r.URL.Path == "/forward-auth" || r.URL.Path == "/mcp" || strings.HasPrefix(r.URL.Path, "/mcp/") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		s.static(w, r)
	})
}

func (s *Server) sessionSnapshot(info session.Info) contract.SessionInfo {
	if s.ProjectSession != nil {
		return s.ProjectSession(info)
	}
	if s.SessionProjection.Project != nil {
		return s.SessionProjection.Project(info)
	}
	return wire.Session(info)
}

// Controller-local names all delegate to the common body/security boundary.
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
func baseHeaders(w http.ResponseWriter) func() { return transport.BaseHeaders(w) }

const chatBodyBytes = transport.InputBodyBytes

func (s *Server) middleware(next http.Handler) http.Handler {
	hostAllowed := hostCheck(s.Config)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer baseHeaders(w)()
		if !hostAllowed(r.Host) && !s.Public.HostAllowed(r.Host) {
			failure(w, http.StatusForbidden, errors.New("host not allowed"))
			return
		}
		mcp := r.URL.Path == "/mcp" && s.Config.Plugins.Has("mcp") && s.Config.MCP.Enabled && s.hasMCP()
		mcpPath := r.URL.Path == "/mcp" || strings.HasPrefix(r.URL.Path, "/mcp/")
		if mcp && (s.Config.Token == "" || len(r.Header.Values("Authorization")) != 1) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="webmux"`)
			failure(w, http.StatusUnauthorized, errors.New("authentication required"))
			return
		}
		if mcpPath || strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
			if !s.authorized(r) {
				if mcpPath {
					w.Header().Set("WWW-Authenticate", `Bearer realm="webmux"`)
				}
				failure(w, 401, errors.New("authentication required"))
				return
			}
			if !s.originAllowed(r) {
				failure(w, 403, errors.New("origin not allowed"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) authorized(r *http.Request) bool {
	if s.Config.Token == "" {
		return true
	}
	token := ""
	scheme, credential, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if found && strings.EqualFold(scheme, "Bearer") {
		token = credential
	}
	if token == "" && strings.HasPrefix(r.URL.Path, "/ws/") {
		token = protocolToken(r)
	}
	want := sha256.Sum256([]byte(s.Config.Token))
	got := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

// wsProtocol is the subprotocol the server selects. Browsers cannot set
// Authorization on a WebSocket handshake, so the frontend also offers
// wsTokenProtocol+base64url(token). That keeps the token out of request URLs
// and out of cookies, which browsers share across every port of a host; the
// server never selects or echoes the token protocol.
const (
	wsProtocol      = "webmux"
	wsTokenProtocol = "webmux.token."
)

func protocolToken(r *http.Request) string {
	for _, header := range r.Header.Values("Sec-WebSocket-Protocol") {
		for offered := range strings.SplitSeq(header, ",") {
			encoded, ok := strings.CutPrefix(strings.TrimSpace(offered), wsTokenProtocol)
			if !ok {
				continue
			}
			if token, err := base64.RawURLEncoding.DecodeString(encoded); err == nil {
				return string(token)
			}
		}
	}
	return ""
}
func (s *Server) originAllowed(r *http.Request) bool {
	return transport.OriginAllowed(s.Config, s.Public, r)
}
func (s *Server) find(w http.ResponseWriter, r *http.Request) *session.Session {
	item, err := s.Manager.Get(r.PathValue("id"))
	if err != nil {
		failure(w, 404, err)
		return nil
	}
	return item
}

// rename sets the user-chosen name of a session; a hybrid session also keeps
// it in its record, so a recovered session is still named. A broken record
// store fails only hybrid sessions; an ordinary shell's rename never depends
// on it.
func (s *Server) rename(r *http.Request, req contract.RenameSessionRequest) (contract.SessionInfo, error) {
	id := r.PathValue("id")
	info, err := s.Manager.Rename(id, req.Name)
	if err != nil {
		return contract.SessionInfo{}, err
	}
	if s.Renamed != nil {
		if err := s.Renamed(id, strings.TrimSpace(req.Name)); err != nil {
			return contract.SessionInfo{}, err
		}
	}
	return s.sessionSnapshot(info), nil
}

func (s *Server) buffer(w http.ResponseWriter, r *http.Request) {
	if item := s.find(w, r); item != nil {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(item.Buffer())
	}
}
