package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/relaysetting"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/transport"
)

type relayBrowserControl struct {
	mu      sync.Mutex
	binding relaysetting.AuthBinding
	grant   protocol.BrowserGrant
	offline bool
}

func relayBrowserToken(value byte) protocol.Token {
	return protocol.Token(base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat(string([]byte{value}), 32))))
}

func (c *relayBrowserControl) Scope() (protocol.Scope, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.binding.Scope, !c.offline
}

func (c *relayBrowserControl) snapshot() (relaysetting.AuthBinding, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.binding, !c.offline
}

func (c *relayBrowserControl) Exchange(_ context.Context, exchange protocol.LoginExchange) (protocol.BrowserGrant, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if exchange.Code != relayBrowserToken(3) || exchange.RedirectURI != c.binding.Binding.PublicURL+protocol.HostLoginCallback {
		return protocol.BrowserGrant{}, protocol.Error{Code: protocol.Unauthenticated}
	}
	return c.grant, nil
}

func (c *relayBrowserControl) Resume(ctx context.Context, request protocol.LoginResume) (protocol.BrowserGrant, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if request.GrantID != c.grant.ID {
		return protocol.BrowserGrant{}, protocol.Error{Code: protocol.NotFound}
	}
	grant := c.grant
	grant.ConnectorGeneration = c.binding.Scope.ConnectorGeneration
	grant.ControlEpoch = c.binding.Scope.ControlEpoch
	grant.NodeEpoch = c.binding.Scope.NodeEpoch
	return grant, ctx.Err()
}

func (c *relayBrowserControl) ICE(context.Context, protocol.ICERequest) (protocol.ICEConfig, error) {
	return protocol.ICEConfig{}, protocol.Error{Code: protocol.Unavailable}
}

type relayBrowserFixture struct {
	auth    *relaysetting.Auth
	control *relayBrowserControl
	server  *Server
	mux     *http.ServeMux
	handler http.Handler
	origin  string
	now     *atomic.Int64
}

func newRelayBrowserFixture(t *testing.T) *relayBrowserFixture {
	t.Helper()
	host := protocol.HostID("0123456789abcdef0123456789abcdef")
	origin := "https://" + host.Label() + ".example.com"
	binding := relaysetting.AuthBinding{Binding: protocol.Binding{HostID: host, OwnerID: "123", BindingGeneration: 1, PublicURL: origin}, Scope: protocol.Scope{HostID: host, BindingGeneration: 1, ConnectorGeneration: 1, ControlEpoch: relayBrowserToken(1), NodeEpoch: relayBrowserToken(2)}, ControlOrigin: "https://control.example.com"}
	grant := protocol.BrowserGrant{ID: relayBrowserToken(4), ConsoleSessionID: relayBrowserToken(5), OwnerID: "123", HostID: host, BindingGeneration: 1, ConnectorGeneration: 1, ControlEpoch: binding.Scope.ControlEpoch, NodeEpoch: binding.Scope.NodeEpoch, Origin: origin, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	control := &relayBrowserControl{binding: binding, grant: grant}
	now := &atomic.Int64{}
	now.Store(time.Now().Unix())
	auth, err := relaysetting.NewAuth(relaysetting.AuthConfig{}, relaysetting.AuthDependencies{Client: control, Snapshot: control.snapshot, Now: func() time.Time { return time.Unix(now.Load(), 0) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = auth.Close() })
	public := &transport.PublicOrigins{}
	if err := public.Set("relay", origin); err != nil {
		t.Fatal(err)
	}
	s := &Server{Deps: Deps{Config: config.Config{Listen: "127.0.0.1:7681", Token: "local-token"}, Public: public, Relay: &Relay{Control: &relayEnrollmentControl{}, Auth: auth}}}
	mux := http.NewServeMux()
	s.relayRoutes(mux)
	for _, path := range []string{"/api/work", "/ws/work", "/forward/work"} {
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	}
	return &relayBrowserFixture{auth: auth, control: control, server: s, mux: mux, handler: s.middleware(mux), origin: origin, now: now}
}

func (f *relayBrowserFixture) request(method, path string, cookie *http.Cookie, trusted bool) *http.Request {
	r := httptest.NewRequest(method, f.origin+path, nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if trusted {
		r = r.WithContext(transport.TrustedRelayContext(r.Context()))
	}
	return r
}

func (f *relayBrowserFixture) login(t *testing.T, destination string) *http.Cookie {
	t.Helper()
	start := httptest.NewRecorder()
	f.handler.ServeHTTP(start, f.request(http.MethodGet, "/auth/relay/start?returnTo="+url.QueryEscape(destination), nil, true))
	if start.Code != http.StatusSeeOther {
		t.Fatalf("login start = %d %s", start.Code, start.Body)
	}
	authorize, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	cookies := start.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing transaction cookie")
	}
	callback := "/auth/relay/callback?state=" + url.QueryEscape(authorize.Query().Get("state")) + "&code=" + string(relayBrowserToken(3))
	finish := httptest.NewRecorder()
	f.handler.ServeHTTP(finish, f.request(http.MethodGet, callback, cookies[0], true))
	if finish.Code != http.StatusSeeOther || finish.Header().Get("Location") != destination {
		t.Fatalf("login finish = %d %s location=%s", finish.Code, finish.Body, finish.Header().Get("Location"))
	}
	for _, cookie := range finish.Result().Cookies() {
		if cookie.Name == relaysetting.AuthCookieName {
			return cookie
		}
	}
	t.Fatal("missing browser session cookie")
	return nil
}

func (f *relayBrowserFixture) status(t *testing.T, cookie *http.Cookie, trusted bool) contract.RelayAuthSession {
	t.Helper()
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, f.request(http.MethodGet, "/api/relay/auth/session", cookie, trusted))
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status = %d %s", w.Code, w.Body)
	}
	var status contract.RelayAuthSession
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	return status
}

func TestRelayBrowserRecoveryStatusAndNoTokenFallback(t *testing.T) {
	f := newRelayBrowserFixture(t)
	cookie := f.login(t, "/s/abcdefgh")
	before := f.status(t, cookie, true)
	if !before.Authenticated || !before.Relay || before.CSRFToken == "" {
		t.Fatalf("authenticated status = %+v", before)
	}
	oldPrincipal, err := f.auth.Authenticate(cookie.Value, f.origin)
	if err != nil {
		t.Fatal(err)
	}
	if external := f.status(t, cookie, false); external.Authenticated || external.Relay || external.Reason != "" || external.CSRFToken != "" {
		t.Fatalf("external transport exposed relay authority: %+v", external)
	}
	f.control.mu.Lock()
	f.control.offline = true
	f.control.mu.Unlock()
	f.server.Public.Clear("relay")
	if err := f.auth.SuspendAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := f.status(t, cookie, true)
	if status.Authenticated || !status.Relay || status.Reason != "reconnecting" || status.CSRFToken != "" || status.ExpiresAt != before.ExpiresAt {
		t.Fatalf("suspended status = %+v", status)
	}
	for _, path := range []string{"/api/work", "/ws/work", "/forward/work"} {
		r := f.request(http.MethodGet, path, cookie, true)
		r.Header.Set("Authorization", "Bearer local-token")
		r.Header.Set("Origin", f.origin)
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("suspended %s fell through to Token: %d %s", path, w.Code, w.Body)
		}
		var failure contract.RelayAuthFailure
		if err := json.Unmarshal(w.Body.Bytes(), &failure); err != nil || !failure.Relay || failure.Reason != "reconnecting" {
			t.Fatalf("suspended %s missing recovery hint: %s %v", path, w.Body, err)
		}
	}
	f.control.mu.Lock()
	f.control.offline = false
	f.control.binding.Scope.ConnectorGeneration++
	f.control.binding.Scope.NodeEpoch = relayBrowserToken(6)
	f.control.mu.Unlock()
	if err := f.server.Public.Set("relay", f.origin); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		status = f.status(t, cookie, true)
		if status.Authenticated {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("browser cookie did not recover")
		}
		time.Sleep(time.Millisecond)
	}
	if status.ExpiresAt != before.ExpiresAt || status.CSRFToken != before.CSRFToken {
		t.Fatal("recovery rotated browser cookie authority or extended expiry")
	}
	stale := f.request(http.MethodGet, "/api/work", nil, false)
	stale = stale.WithContext(RelayOwnerContext(context.Background(), oldPrincipal))
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, stale)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("stale P2P principal dispatched after resume: %d", w.Code)
	}
	current, err := f.auth.Authenticate(cookie.Value, f.origin)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.auth.Logout(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	if status := f.status(t, cookie, true); status.Authenticated || status.Reason != "revoked" {
		t.Fatalf("logout status = %+v", status)
	}
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, f.request(http.MethodGet, "/api/work", cookie, true))
	var failure contract.RelayAuthFailure
	if err := json.Unmarshal(w.Body.Bytes(), &failure); err != nil || w.Code != http.StatusUnauthorized || !failure.Relay || failure.Reason != "revoked" {
		t.Fatalf("revoked business failure = %d %s %v", w.Code, w.Body, err)
	}
	if status := f.status(t, nil, true); status.Authenticated || status.Reason != "unauthenticated" || !status.Relay {
		t.Fatalf("anonymous relay status = %+v", status)
	}
}

func TestRelayReturnToRejectsExternalAndCredentialRoutes(t *testing.T) {
	f := newRelayBrowserFixture(t)
	for _, destination := range []string{"https://evil.example/app", "//evil.example/app", "/\\evil.example/app", "/api/sessions", "/auth/relay/callback", "/forward-auth", "/forward/8080/", "/s/abcdefgh?token=secret", "/app#token=secret", "/app/../auth/relay/start", "/%61pp", "/git?token=secret", "/git?view=log&view=commit", "/git?path=%0d%0aLocation:evil"} {
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, f.request(http.MethodGet, "/auth/relay/start?returnTo="+url.QueryEscape(destination), nil, true))
		if w.Code != http.StatusBadRequest || len(w.Result().Cookies()) != 0 {
			t.Errorf("unsafe returnTo %q returned %d", destination, w.Code)
		}
	}
	f.login(t, "/git/repo/worktrees/work?view=log&branch=topic%2Fbranch&returnSession=abcdefgh")
}

func TestRelaySuspensionClosesAttachmentNotPTY(t *testing.T) {
	f := newRelayBrowserFixture(t)
	cookie := f.login(t, "/app")
	manager := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	t.Cleanup(func() { manager.Shutdown(context.Background()) })
	terminal, err := manager.Create(session.CreateInput{Cmd: []string{"/bin/cat"}})
	if err != nil {
		t.Fatal(err)
	}
	attached, exited := make(chan struct{}), make(chan struct{})
	f.mux.HandleFunc("GET /ws/pty", func(w http.ResponseWriter, r *http.Request) {
		observer, err := terminal.Attach()
		if err != nil {
			t.Error(err)
			close(attached)
			return
		}
		defer terminal.Detach(observer)
		close(attached)
		<-r.Context().Done()
	})
	r := f.request(http.MethodGet, "/ws/pty", cookie, true)
	r.Header.Set("Origin", f.origin)
	go func() {
		defer close(exited)
		f.handler.ServeHTTP(httptest.NewRecorder(), r)
	}()
	select {
	case <-attached:
	case <-time.After(time.Second):
		t.Fatal("PTY observer was not attached")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := f.auth.SuspendAll(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-ctx.Done():
		t.Fatal("suspension did not join PTY observer access")
	}
	written, err := terminal.SubmitServer(ctx, session.ServerInput{RequestID: "relay-after-suspend", Label: "test", Body: []byte("still alive"), Submit: []byte("\r")})
	if err != nil || written.State != session.SubmissionWritten || terminal.Info().Status == session.StatusExited {
		t.Fatalf("suspension killed persistent PTY: %+v %v", written, err)
	}
}

func TestRelaySuspendedLogoutRequiresCSRFAndPreventsRecovery(t *testing.T) {
	f := newRelayBrowserFixture(t)
	cookie := f.login(t, "/app")
	before := f.status(t, cookie, true)
	if err := f.auth.SuspendAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, credentials := range []struct {
		origin string
		csrf   string
		want   int
	}{
		{f.origin, "", http.StatusForbidden},
		{"https://evil.example", before.CSRFToken, http.StatusForbidden},
		{f.origin, before.CSRFToken, http.StatusNoContent},
	} {
		r := f.request(http.MethodPost, "/api/relay/auth/logout", cookie, true)
		r.Header.Set("Origin", credentials.origin)
		r.Header.Set("X-WebMux-CSRF", credentials.csrf)
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		if w.Code != credentials.want {
			t.Fatalf("suspended logout = %d %s, want %d", w.Code, w.Body, credentials.want)
		}
		if credentials.want == http.StatusNoContent {
			cleared := w.Result().Cookies()
			if len(cleared) != 1 || cleared[0].Name != relaysetting.AuthCookieName || cleared[0].MaxAge != -1 {
				t.Fatal("logout did not clear browser cookie")
			}
		}
	}
	if status := f.status(t, cookie, true); status.Authenticated || status.Reason != "revoked" {
		t.Fatalf("suspended logout restored authority: %+v", status)
	}
}

func TestRelayExpiredBusinessFailureIsTrustedOnly(t *testing.T) {
	f := newRelayBrowserFixture(t)
	cookie := f.login(t, "/app")
	f.now.Store(cookie.Expires.Unix())
	for _, trusted := range []bool{true, false} {
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, f.request(http.MethodGet, "/api/work", cookie, trusted))
		var failure contract.RelayAuthFailure
		if err := json.Unmarshal(w.Body.Bytes(), &failure); err != nil || w.Code != http.StatusUnauthorized {
			t.Fatalf("expired business status = %d %s %v", w.Code, w.Body, err)
		}
		if trusted && (!failure.Relay || failure.Reason != "expired") {
			t.Fatalf("expired trusted cookie lost reason: %+v", failure)
		}
		if !trusted && (failure.Relay || failure.Reason != "") {
			t.Fatalf("ordinary listener exposed relay state: %+v", failure)
		}
	}
}

func TestRelayBrowserRejectsForeignFetchSiteAndKeepsAPICSRF(t *testing.T) {
	f := newRelayBrowserFixture(t)
	f.mux.HandleFunc("POST /api/work", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	f.mux.HandleFunc("POST /mcp", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	cookie := f.login(t, "/app")
	status := f.status(t, cookie, true)

	for _, site := range []string{"cross-site", "same-site"} {
		for _, path := range []string{"/api/work", "/api/relay/auth/session", "/ws/work", "/mcp", "/forward/work"} {
			r := f.request(http.MethodGet, path, cookie, true)
			r.Header.Set("Sec-Fetch-Site", site)
			r.Header.Set("Sec-Fetch-Mode", "cors")
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Errorf("%s %s = %d, want 403", site, path, w.Code)
			}
		}
	}

	for _, path := range []string{"/api/work", "/mcp"} {
		noCSRF := f.request(http.MethodPost, path, cookie, true)
		noCSRF.Header.Set("Origin", f.origin)
		noCSRF.Header.Set("Sec-Fetch-Site", "same-origin")
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, noCSRF)
		if w.Code != http.StatusForbidden {
			t.Errorf("POST %s without CSRF = %d, want 403", path, w.Code)
		}

		withCSRF := f.request(http.MethodPost, path, cookie, true)
		withCSRF.Header.Set("Origin", f.origin)
		withCSRF.Header.Set("X-WebMux-CSRF", status.CSRFToken)
		withCSRF.Header.Set("Sec-Fetch-Site", "same-origin")
		w = httptest.NewRecorder()
		f.handler.ServeHTTP(w, withCSRF)
		if w.Code != http.StatusNoContent {
			t.Errorf("POST %s with CSRF = %d, want 204", path, w.Code)
		}
	}
}
