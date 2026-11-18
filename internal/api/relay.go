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
	Pairing() contract.RelayPairing
	Enroll(context.Context, contract.RelayEnrollmentRequest) (contract.RelayPairing, error)
	Cancel(context.Context) (contract.RelayPairing, error)
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
	if errors.As(err, &remote) && remote.Code.Valid() {
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

// Browser failures carry recovery hints only on the authenticated connector
// ingress. CSRF/origin failures are not a signal to discard a valid login.
func (s *Server) relayBrowserFailure(w http.ResponseWriter, r *http.Request, err error) {
	var remote protocol.Error
	origin := s.relayOrigin(r)
	if origin == "" || s.Relay.Auth == nil || !errors.As(err, &remote) || (remote.Code != protocol.Unauthenticated && remote.Code != protocol.Expired && remote.Code != protocol.Unavailable) {
		relayFailure(w, err)
		return
	}
	cookie := ""
	if value, cookieErr := r.Cookie(relaysetting.AuthCookieName); cookieErr == nil {
		cookie = value.Value
	}
	status := s.Relay.Auth.Probe(cookie, origin)
	reason := status.Reason
	if status.Authenticated {
		// This operation lost its old epoch while another request recovered.
		reason = "reconnecting"
	}
	httpStatus := http.StatusUnauthorized
	if reason == "reconnecting" {
		httpStatus = http.StatusServiceUnavailable
	}
	reply(w, httpStatus, contract.RelayAuthFailure{Error: string(remote.Code), Relay: true, Reason: reason})
}

func (s *Server) relayRoutes(mux *http.ServeMux) {
	if s.Relay == nil {
		return
	}
	c := s.Relay.Control
	mux.HandleFunc("GET /api/relay", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, c.Status())
	})
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
	mux.HandleFunc("GET /api/relay/enrollments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, c.Pairing())
	})
	mux.HandleFunc("POST /api/relay/enrollments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
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
	mux.HandleFunc("DELETE /api/relay/enrollments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var req contract.RelayEnrollmentCancel
		if !decodeStrict(w, r, &req, 4096, errors.New("invalid relay cancel")) {
			return
		}
		v, err := c.Cancel(r.Context())
		if err != nil {
			relayFailure(w, err)
			return
		}
		reply(w, 200, v)
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
		v.Relay = s.relayOrigin(r) != ""
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
	if origin == "" && s.Relay.Auth != nil {
		origin = s.Relay.Auth.PublicOrigin()
	}
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
	query := r.URL.Query()
	if len(query) > 1 || len(query["returnTo"]) > 1 || len(query) == 1 && len(query["returnTo"]) != 1 {
		failure(w, http.StatusBadRequest, errors.New("invalid relay destination"))
		return
	}
	v, err := s.Relay.Auth.Start(r.Context(), origin, query.Get("returnTo"))
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
	http.Redirect(w, r, v.ReturnTo, http.StatusSeeOther)
}

func isRelayForwardTopNavigation(r *http.Request) bool {
	if (r.URL.Path != "/forward" && !strings.HasPrefix(r.URL.Path, "/forward/")) || r.URL.Path == "/forward/_auth" {
		return false
	}
	if (r.Method != http.MethodGet && r.Method != http.MethodHead) || strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	switch strings.ToLower(r.Header.Get("Sec-Fetch-Dest")) {
	case "iframe", "frame", "embed", "object":
		return false
	}
	if mode := r.Header.Get("Sec-Fetch-Mode"); mode != "" {
		return mode == "navigate"
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// withRelayBrowser authenticates only the dedicated connector transport. A
// forged Host/forwarded header on the ordinary listener cannot mint this context.
func (s *Server) withRelayBrowser(w http.ResponseWriter, r *http.Request, next http.Handler) bool {
	if s.Relay == nil || !transport.IsTrustedRelay(r.Context()) {
		return false
	}
	isForward := r.URL.Path == "/forward" || strings.HasPrefix(r.URL.Path, "/forward/")
	if !strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/ws/") && r.URL.Path != "/mcp" && !strings.HasPrefix(r.URL.Path, "/mcp/") && !isForward {
		return false
	}
	relayAuthHeaders(w)
	if s.Relay.Auth == nil {
		relayFailure(w, protocol.Error{Code: protocol.Unavailable})
		return true
	}
	origin := s.relayOrigin(r)
	if origin == "" {
		failure(w, http.StatusForbidden, errors.New("relay origin required"))
		return true
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" || site == "same-site" {
		if !isRelayForwardTopNavigation(r) {
			s.relayBrowserFailure(w, r, protocol.Error{Code: protocol.Forbidden})
			return true
		}
	}
	cookie := ""
	if value, err := r.Cookie(relaysetting.AuthCookieName); err == nil {
		cookie = value.Value
	}
	if r.URL.Path == "/api/relay/auth/session" && r.Method == http.MethodGet {
		if requestOrigin := r.Header.Get("Origin"); requestOrigin != "" && requestOrigin != origin {
			failure(w, http.StatusForbidden, errors.New("origin not allowed"))
			return true
		}
		reply(w, http.StatusOK, wire.RelayAuthSession(s.Relay.Auth.Probe(cookie, origin)))
		return true
	}
	if r.URL.Path == "/api/relay/auth/logout" && r.Method == http.MethodPost {
		if err := s.Relay.Auth.LogoutBrowser(r.Context(), cookie, r.Header.Get("Origin"), r.Header.Get("X-WebMux-CSRF")); err != nil {
			s.relayBrowserFailure(w, r, err)
			return true
		}
		http.SetCookie(w, &http.Cookie{Name: relaysetting.AuthCookieName, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	p, err := s.Relay.Auth.Authenticate(cookie, origin)
	if err != nil {
		s.relayBrowserFailure(w, r, err)
		return true
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
		if isForward {
			if reqOrigin := r.Header.Get("Origin"); reqOrigin != "" {
				err = s.Relay.Auth.CheckOrigin(p, reqOrigin)
			} else if r.Header.Get("Sec-Fetch-Site") != "same-origin" {
				err = protocol.Error{Code: protocol.Forbidden}
			}
		} else {
			err = s.Relay.Auth.CheckMutation(p, r.Header.Get("Origin"), r.Header.Get("X-WebMux-CSRF"))
		}
	} else if strings.HasPrefix(r.URL.Path, "/ws/") || strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || r.Header.Get("Origin") != "" {
		err = s.Relay.Auth.CheckOrigin(p, r.Header.Get("Origin"))
	}
	if err != nil {
		s.relayBrowserFailure(w, r, err)
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
		s.relayBrowserFailure(w, r, err)
		return true
	}
	defer release()
	defer close(done)
	ctx = context.WithValue(ctx, relayPrincipalKey{}, p)
	ctx = transport.RelayOwnerContext(ctx)
	next.ServeHTTP(w, r.WithContext(ctx))
	return true
}
