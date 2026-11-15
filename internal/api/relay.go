package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/relaysetting"
	"github.com/rainchan/webmux/internal/transport"
	"github.com/rainchan/webmux/internal/wire"
)

// RelayControl is bound by cmd; controllers never construct network services.
type RelayControl interface {
	Status() contract.RelayStatus
	Update(context.Context, contract.RelayUpdate) (contract.RelayStatus, error)
	Reconnect(context.Context) (contract.RelayStatus, error)
	Enroll(context.Context, contract.RelayEnrollmentRequest) (contract.RelayEnrollment, error)
	Poll(context.Context, protocol.Token) (contract.RelayStatus, error)
	Cancel(context.Context, protocol.Token) error
	Delete(context.Context) (contract.RelayStatus, error)
}

type Relay struct {
	Control RelayControl
	Auth    *relaysetting.Auth
}

type relayPrincipalKey struct{}

// RelayPrincipalContext exposes already authenticated owner identity to cmd's
// P2P authorization adapter, never to an incoming HTTP header.
func RelayPrincipalContext(ctx context.Context) (relaysetting.AuthPrincipal, bool) {
	principal, ok := ctx.Value(relayPrincipalKey{}).(relaysetting.AuthPrincipal)
	return principal, ok
}

func RelayOwnerContext(ctx context.Context, principal relaysetting.AuthPrincipal) context.Context {
	return transport.RelayOwnerContext(context.WithValue(ctx, relayPrincipalKey{}, principal))
}
func relayPrincipal(r *http.Request) (relaysetting.AuthPrincipal, bool) {
	p, ok := r.Context().Value(relayPrincipalKey{}).(relaysetting.AuthPrincipal)
	return p, ok
}

func relayFailure(w http.ResponseWriter, err error) {
	var remote protocol.Error
	if errors.As(err, &remote) {
		failure(w, protocol.HTTPStatus(remote.Code), errors.New(string(remote.Code)))
		return
	}
	switch {
	case errors.Is(err, relaysetting.ErrEnableBlocked):
		failure(w, http.StatusConflict, relaysetting.ErrEnableBlocked)
	case errors.Is(err, relaysetting.ErrInvalid):
		failure(w, http.StatusBadRequest, relaysetting.ErrInvalid)
	case errors.Is(err, relaysetting.ErrNotPersisted):
		failure(w, http.StatusServiceUnavailable, relaysetting.ErrNotPersisted)
	default:
		failure(w, http.StatusServiceUnavailable, errors.New("relay unavailable"))
	}
}

func (s *Server) relayRoutes(mux *http.ServeMux) {
	if s.Relay == nil {
		return
	}
	c := s.Relay.Control
	mux.HandleFunc("GET /api/relay", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, c.Status()) })
	mux.HandleFunc("PUT /api/relay", func(w http.ResponseWriter, r *http.Request) {
		var req contract.RelayUpdate
		if !decodeStrict(w, r, &req, 4096, errors.New("invalid relay settings")) {
			return
		}
		if req.Enabled == nil {
			failure(w, 400, errors.New("enabled is required"))
			return
		}
		v, err := c.Update(r.Context(), req)
		if err != nil {
			relayFailure(w, err)
			return
		}
		reply(w, 200, v)
	})
	mux.HandleFunc("POST /api/relay/reconnect", func(w http.ResponseWriter, r *http.Request) {
		v, err := c.Reconnect(r.Context())
		if err != nil {
			relayFailure(w, err)
			return
		}
		reply(w, 200, v)
	})
	mux.HandleFunc("POST /api/relay/disconnect", func(w http.ResponseWriter, r *http.Request) {
		off := false
		v, err := c.Update(r.Context(), contract.RelayUpdate{Enabled: &off})
		if err != nil {
			relayFailure(w, err)
			return
		}
		reply(w, 200, v)
	})
	mux.HandleFunc("POST /api/relay/enrollments", func(w http.ResponseWriter, r *http.Request) {
		var req contract.RelayEnrollmentRequest
		if !decodeStrict(w, r, &req, 4096, errors.New("invalid relay enrollment")) {
			return
		}
		v, err := c.Enroll(r.Context(), req)
		if err != nil {
			relayFailure(w, err)
			return
		}
		reply(w, 201, v)
	})
	mux.HandleFunc("POST /api/relay/enrollments/poll", func(w http.ResponseWriter, r *http.Request) {
		var req contract.RelayEnrollmentPoll
		if !decodeStrict(w, r, &req, 4096, errors.New("invalid relay poll")) {
			return
		}
		v, err := c.Poll(r.Context(), req.Handle)
		if err != nil {
			relayFailure(w, err)
			return
		}
		reply(w, 200, v)
	})
	mux.HandleFunc("DELETE /api/relay/enrollments", func(w http.ResponseWriter, r *http.Request) {
		var req contract.RelayEnrollmentPoll
		if !decodeStrict(w, r, &req, 4096, errors.New("invalid relay cancel")) {
			return
		}
		if err := c.Cancel(r.Context(), req.Handle); err != nil {
			relayFailure(w, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("DELETE /api/relay/binding", func(w http.ResponseWriter, r *http.Request) {
		v, err := c.Delete(r.Context())
		if err != nil {
			relayFailure(w, err)
			return
		}
		reply(w, 200, v)
	})
	mux.HandleFunc("GET /auth/relay/start", s.relayStart)
	mux.HandleFunc("GET /auth/relay/callback", s.relayCallback)
	mux.HandleFunc("GET /api/relay/auth/session", func(w http.ResponseWriter, r *http.Request) {
		p, ok := relayPrincipal(r)
		if !ok {
			reply(w, 200, contract.RelayAuthSession{})
			return
		}
		v, err := s.Relay.Auth.Status(p)
		if err != nil {
			relayFailure(w, err)
			return
		}
		reply(w, 200, wire.RelayAuthSession(v))
	})
	mux.HandleFunc("POST /api/relay/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		p, ok := relayPrincipal(r)
		if !ok {
			failure(w, 403, errors.New("owner browser session required"))
			return
		}
		if err := s.Relay.Auth.Logout(r.Context(), p); err != nil {
			relayFailure(w, err)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "__Host-webmux-relay", Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /api/relay/ice", func(w http.ResponseWriter, r *http.Request) {
		p, ok := relayPrincipal(r)
		if !ok {
			failure(w, 403, errors.New("owner browser session required"))
			return
		}
		var req contract.RelayBrowserICERequest
		if !decodeStrict(w, r, &req, 4096, errors.New("invalid peer connection")) {
			return
		}
		v, err := s.Relay.Auth.ICE(r.Context(), p, req.PeerConnectionID)
		if err != nil {
			relayFailure(w, err)
			return
		}
		reply(w, 200, v)
	})
}

func (s *Server) relayOrigin(r *http.Request) string {
	if s.Relay == nil || !transport.IsTrustedRelay(r.Context()) {
		return ""
	}
	origin := s.Public.SourceOrigin("relay")
	if origin == "https://"+r.Host {
		return origin
	}
	return ""
}

func relayAuthHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func relaySetCookie(w http.ResponseWriter, material relaysetting.AuthCookie) {
	http.SetCookie(w, &http.Cookie{Name: material.Name, Value: material.Value, Expires: material.Expires, MaxAge: material.MaxAge, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func (s *Server) relayStart(w http.ResponseWriter, r *http.Request) {
	relayAuthHeaders(w)
	origin := s.relayOrigin(r)
	if origin == "" {
		failure(w, 403, errors.New("relay origin required"))
		return
	}
	v, err := s.Relay.Auth.Start(r.Context(), origin)
	if err != nil {
		relayFailure(w, err)
		return
	}
	relaySetCookie(w, v.Cookie)
	http.Redirect(w, r, v.URL, http.StatusSeeOther)
}

func (s *Server) relayCallback(w http.ResponseWriter, r *http.Request) {
	relayAuthHeaders(w)
	origin := s.relayOrigin(r)
	if origin == "" {
		failure(w, 403, errors.New("relay origin required"))
		return
	}
	cookie, err := r.Cookie("__Host-webmux-relay-tx")
	if err != nil {
		failure(w, 403, errors.New("invalid relay transaction"))
		return
	}
	query := r.URL.Query()
	if len(query) != 2 || len(query["state"]) != 1 || len(query["code"]) != 1 {
		failure(w, 400, errors.New("invalid relay callback"))
		return
	}
	v, err := s.Relay.Auth.Finish(r.Context(), origin, cookie.Value, query.Get("state"), query.Get("code"))
	if err != nil {
		relayFailure(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "__Host-webmux-relay-tx", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	relaySetCookie(w, v.Cookie)
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

// withRelayBrowser authenticates only the dedicated connector transport. A
// forged Host/forwarded header on the ordinary listener cannot mint this context.
func (s *Server) withRelayBrowser(w http.ResponseWriter, r *http.Request, next http.Handler) bool {
	origin := s.relayOrigin(r)
	if origin == "" {
		return false
	}
	relayAuthHeaders(w)
	cookie, err := r.Cookie("__Host-webmux-relay")
	if err != nil {
		return false
	}
	p, err := s.Relay.Auth.Authenticate(cookie.Value, origin)
	if err != nil {
		return false
	}
	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
		err = s.Relay.Auth.CheckMutation(p, r.Header.Get("Origin"), r.Header.Get("X-WebMux-CSRF"))
	} else if strings.HasPrefix(r.URL.Path, "/ws/") || strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || r.Header.Get("Origin") != "" {
		err = s.Relay.Auth.CheckOrigin(p, r.Header.Get("Origin"))
	}
	if err != nil {
		relayFailure(w, err)
		return true
	}
	if r.URL.Path == "/api/relay/auth/logout" {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), relayPrincipalKey{}, p)))
		return true
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(p.Context, cancel)
	defer stop()
	done := make(chan struct{})
	release, err := s.Relay.Auth.RegisterAccess(p, cancel, done)
	if err != nil {
		close(done)
		relayFailure(w, err)
		return true
	}
	defer release()
	defer close(done)
	ctx = context.WithValue(ctx, relayPrincipalKey{}, p)
	ctx = transport.RelayOwnerContext(ctx)
	next.ServeHTTP(w, r.WithContext(ctx))
	return true
}
