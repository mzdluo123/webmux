package api

import (
	"net/http"
	"strings"
	"testing"
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
