package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/relaysetting"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/transport"
)

func TestRelayCookieDoesNotAuthenticateOrdinaryTransport(t *testing.T) {
	public := &transport.PublicOrigins{}
	if err := public.Set("relay", "https://h.example.com"); err != nil {
		t.Fatal(err)
	}
	s := &Server{Deps: Deps{Config: config.Config{Listen: "127.0.0.1:7681", Token: strings.Repeat("x", 43)}, Public: public, Relay: &Relay{}}}
	handler := s.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, path := range []string{"/api/sessions", "/ws/sessions"} {
		r := httptest.NewRequest("GET", "https://h.example.com"+path, nil)
		r.AddCookie(&http.Cookie{Name: "__Host-webmux-relay", Value: strings.Repeat("A", 43)})
		r.Header.Set("Origin", "https://h.example.com")
		r.Header.Set("X-Forwarded-Proto", "https")
		r.Header.Set("X-WebMux-Relay", "true")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("forged relay context accepted on %s: %d", path, w.Code)
		}
		r.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 43))
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatalf("ordinary recovery Token rejected on %s: %d", path, w.Code)
		}
	}
}

func TestRelayLocalSessionProcessDeniedAllSensitiveRoutes(t *testing.T) {
	manager := session.NewManager(session.Options{})
	t.Cleanup(func() { manager.Shutdown(context.Background()) })
	handler := NewLocal(Deps{Manager: manager}, Local{InSession: func(int) bool { return true }})
	for _, path := range []string{"/api/relay", "/api/relay/enrollments", "/api/relay/enrollments/poll", "/api/relay/ice", "/api/relay/auth/session", "/api/relay/reconnect", "/api/relay/binding"} {
		r := httptest.NewRequest("POST", "http://localhost"+path, strings.NewReader("{}"))
		r = r.WithContext(context.WithValue(r.Context(), peerKey{}, 123))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("session process route %s returned %d", path, w.Code)
		}
	}
}

func TestRelayAuthCookieSecurityAttributes(t *testing.T) {
	w := httptest.NewRecorder()
	relaySetCookie(w, relaysetting.AuthCookie{Name: "__Host-webmux-relay", Value: "opaque", Expires: time.Now().Add(time.Hour), MaxAge: 3600})
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("session cookie missing")
	}
	cookie := cookies[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Domain != "" {
		t.Fatalf("unsafe cookie attributes: %+v", cookie)
	}
}
