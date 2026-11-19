package api

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/forward"
	"github.com/rainchan/webmux/internal/relaysetting"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/transport"
)

func TestForwardNeverLeaksRelayCookieOrCSRF(t *testing.T) {
	env := newGatewayEnv(t, "secret", nil)
	env.add(env.port, nil)
	credential := env.cookie()
	response := env.do(http.MethodPost, env.path("submit"), strings.NewReader("input"), func(r *http.Request) {
		r.AddCookie(credential)
		r.AddCookie(&http.Cookie{Name: "__Host-webmux-relay", Value: "private-browser-session"})
		r.AddCookie(&http.Cookie{Name: "__Host-webmux-relay-tx", Value: "private-transaction"})
		r.AddCookie(&http.Cookie{Name: "app", Value: "upstream-state"})
		r.Header.Set("Origin", env.server.URL)
		r.Header.Set("X-WebMux-CSRF", "private-csrf")
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("forward status %d", response.StatusCode)
	}
	seen := env.last().header
	if seen.Get("X-WebMux-CSRF") != "" {
		t.Fatal("host CSRF escaped into upstream")
	}
	cookies := (&http.Request{Header: seen}).Cookies()
	if len(cookies) != 1 || cookies[0].Name != "app" || cookies[0].Value != "upstream-state" {
		t.Fatalf("host credential escaped or app cookie lost: %v", cookies)
	}
}

func TestRelayBrowserForwardGateway(t *testing.T) {
	var upstreamMu sync.Mutex
	var upstreamMethods []string
	dev := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamMu.Lock()
		upstreamMethods = append(upstreamMethods, r.Method+":"+r.URL.Path+":"+string(body))
		upstreamMu.Unlock()
		_, _ = io.WriteString(w, "upstream ok")
	}))
	t.Cleanup(dev.Close)
	port := dev.Listener.Addr().(*net.TCPAddr).Port

	forwards := forward.New(forward.Config{ExcludePorts: []int{7681}, ProbeInterval: time.Hour})
	t.Cleanup(forwards.Close)
	if _, err := forwards.Add(t.Context(), forward.Input{Port: port}); err != nil {
		t.Fatal(err)
	}

	host := protocol.HostID("0123456789abcdef0123456789abcdef")
	origin := "https://" + host.Label() + ".example.com"
	binding := relaysetting.AuthBinding{
		Binding:       protocol.Binding{HostID: host, OwnerID: "123", BindingGeneration: 1, PublicURL: origin},
		Scope:         protocol.Scope{HostID: host, BindingGeneration: 1, ConnectorGeneration: 1, ControlEpoch: relayBrowserToken(1), NodeEpoch: relayBrowserToken(2)},
		ControlOrigin: "https://control.example.com",
	}
	grant := protocol.BrowserGrant{
		ID: relayBrowserToken(4), ConsoleSessionID: relayBrowserToken(5), OwnerID: "123", HostID: host,
		BindingGeneration: 1, ConnectorGeneration: 1, ControlEpoch: binding.Scope.ControlEpoch, NodeEpoch: binding.Scope.NodeEpoch,
		Origin: origin, ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	control := &relayBrowserControl{binding: binding, grant: grant}
	now := &atomic.Int64{}
	now.Store(time.Now().Unix())
	auth, err := relaysetting.NewAuth(relaysetting.AuthConfig{}, relaysetting.AuthDependencies{
		Client: control, Snapshot: control.snapshot, Now: func() time.Time { return time.Unix(now.Load(), 0) },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = auth.Close() })

	public := &transport.PublicOrigins{}
	if err := public.Set("relay", origin); err != nil {
		t.Fatal(err)
	}
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	t.Cleanup(func() { m.Shutdown(context.Background()) })
	deps := testDeps(t, testServices{
		Config:    config.Config{Listen: "127.0.0.1:7681", Token: "local-token"},
		Manager:   m,
		Workspace: testWorkspace(t, m, t.TempDir()),
		Forward:   forwards,
		Public:    public,
	})
	deps.Relay = &Relay{Control: &relayEnrollmentControl{}, Auth: auth}
	handler := New(deps)

	send := func(method, path string, body string, cookie *http.Cookie, edit func(*http.Request)) *httptest.ResponseRecorder {
		t.Helper()
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		r := httptest.NewRequest(method, origin+path, reader)
		r = r.WithContext(transport.TrustedRelayContext(r.Context()))
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if edit != nil {
			edit(r)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	start := send(http.MethodGet, "/auth/relay/start?returnTo=%2Fapp", "", nil, nil)
	if start.Code != http.StatusSeeOther {
		t.Fatalf("relay start = %d %s", start.Code, start.Body.String())
	}
	authURL, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	finish := send(http.MethodGet, "/auth/relay/callback?state="+url.QueryEscape(authURL.Query().Get("state"))+"&code="+string(relayBrowserToken(3)), "", start.Result().Cookies()[0], nil)
	if finish.Code != http.StatusSeeOther {
		t.Fatalf("relay callback = %d %s", finish.Code, finish.Body.String())
	}
	var relayCookie *http.Cookie
	for _, c := range finish.Result().Cookies() {
		if c.Name == relaysetting.AuthCookieName {
			relayCookie = c
		}
	}
	if relayCookie == nil {
		t.Fatal("missing relay session cookie")
	}

	sessRes := send(http.MethodGet, "/api/relay/auth/session", "", relayCookie, nil)
	var sess contract.RelayAuthSession
	if err := json.Unmarshal(sessRes.Body.Bytes(), &sess); err != nil || !sess.Authenticated || sess.CSRFToken == "" {
		t.Fatalf("relay session = %+v %v", sess, err)
	}

	forwardPath := "/forward/" + strconv.Itoa(port) + "/"

	// Relay browser session POST to /forward/{port}/ succeeds without X-WebMux-CSRF.
	postRes := send(http.MethodPost, forwardPath+"submit", "payload=1", relayCookie, func(r *http.Request) {
		r.Header.Set("Origin", origin)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	})
	if postRes.Code != http.StatusOK || postRes.Body.String() != "upstream ok" {
		t.Fatalf("relay forward POST = %d %q", postRes.Code, postRes.Body.String())
	}
	formPostRes := send(http.MethodPost, forwardPath+"form", "payload=2", relayCookie, func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	})
	if formPostRes.Code != http.StatusOK {
		t.Fatalf("relay forward form POST without Origin = %d %q", formPostRes.Code, formPostRes.Body.String())
	}

	// Without Sec-Fetch-Site, Relay owner cookie alone does not bypass forward cookie check.
	noFetchMeta := send(http.MethodGet, forwardPath+"data", "", relayCookie, nil)
	if noFetchMeta.Code != http.StatusUnauthorized {
		t.Fatalf("missing Sec-Fetch-Site without forward cookie = %d, want 401", noFetchMeta.Code)
	}

	// Issue a ticket (control-plane API still requires CSRF) and redeem via /forward/_auth (form POST without CSRF).
	if got := send(http.MethodPost, "/api/forwards/tickets", "", relayCookie, func(r *http.Request) {
		r.Header.Set("Origin", origin)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	}).Code; got != http.StatusForbidden {
		t.Fatalf("ticket endpoint without CSRF = %d, want 403", got)
	}
	ticketRes := send(http.MethodPost, "/api/forwards/tickets", "", relayCookie, func(r *http.Request) {
		r.Header.Set("Origin", origin)
		r.Header.Set("X-WebMux-CSRF", sess.CSRFToken)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	})
	if ticketRes.Code != http.StatusCreated {
		t.Fatalf("ticket issue = %d %s", ticketRes.Code, ticketRes.Body.String())
	}
	var ticket contract.ForwardTicket
	if err := json.Unmarshal(ticketRes.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	redeemBody := url.Values{"code": {ticket.Code}, "next": {forwardPath}}.Encode()
	redeemRes := send(http.MethodPost, "/forward/_auth", redeemBody, relayCookie, func(r *http.Request) {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", origin)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	})
	if redeemRes.Code != http.StatusSeeOther || redeemRes.Header().Get("Location") != forwardPath {
		t.Fatalf("relay /forward/_auth = %d %q %s", redeemRes.Code, redeemRes.Header().Get("Location"), redeemRes.Body.String())
	}
	var fwdCookie *http.Cookie
	for _, c := range redeemRes.Result().Cookies() {
		if c.Name == "webmux_forward" {
			fwdCookie = c
		}
	}
	if fwdCookie == nil || !fwdCookie.Secure || fwdCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("forward cookie = %+v", fwdCookie)
	}

	// With webmux_forward cookie, fallback without Sec-Fetch-Site succeeds.
	fallbackOK := send(http.MethodGet, forwardPath+"data", "", relayCookie, func(r *http.Request) {
		r.AddCookie(fwdCookie)
	})
	if fallbackOK.Code != http.StatusOK {
		t.Fatalf("fallback with forward cookie = %d", fallbackOK.Code)
	}

	// Cross-site top-level GET/HEAD navigation redirects to /forward-auth?confirm=1.
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		navRes := send(method, forwardPath+"index.html?tab=1", "", relayCookie, func(r *http.Request) {
			r.Header.Set("Sec-Fetch-Site", "cross-site")
			r.Header.Set("Sec-Fetch-Mode", "navigate")
			r.Header.Set("Sec-Fetch-Dest", "document")
		})
		want := "/forward-auth?confirm=1&next=" + url.QueryEscape(forwardPath+"index.html?tab=1")
		if navRes.Code != http.StatusSeeOther || navRes.Header().Get("Location") != want {
			t.Fatalf("%s cross-site navigation = %d %q, want 303 %q", method, navRes.Code, navRes.Header().Get("Location"), want)
		}
	}

	// Sibling-subdomain same-site subresource, iframe, and cross-site POST are rejected with 403.
	upstreamMu.Lock()
	beforeRejects := len(upstreamMethods)
	upstreamMu.Unlock()
	for _, tc := range []struct {
		name   string
		method string
		edit   func(*http.Request)
	}{
		{"same-site script", http.MethodGet, func(r *http.Request) {
			r.AddCookie(fwdCookie)
			r.Header.Set("Sec-Fetch-Site", "same-site")
			r.Header.Set("Sec-Fetch-Mode", "no-cors")
			r.Header.Set("Sec-Fetch-Dest", "script")
		}},
		{"same-site iframe", http.MethodGet, func(r *http.Request) {
			r.AddCookie(fwdCookie)
			r.Header.Set("Sec-Fetch-Site", "same-site")
			r.Header.Set("Sec-Fetch-Mode", "navigate")
			r.Header.Set("Sec-Fetch-Dest", "iframe")
		}},
		{"cross-site POST", http.MethodPost, func(r *http.Request) {
			r.Header.Set("Origin", "https://evil.example")
			r.Header.Set("Sec-Fetch-Site", "cross-site")
		}},
	} {
		if got := send(tc.method, forwardPath+"asset.js", "", relayCookie, tc.edit).Code; got != http.StatusForbidden {
			t.Errorf("%s = %d, want 403", tc.name, got)
		}
	}
	upstreamMu.Lock()
	afterRejects := len(upstreamMethods)
	upstreamMu.Unlock()
	if afterRejects != beforeRejects {
		t.Fatal("rejected relay request reached upstream")
	}
}
