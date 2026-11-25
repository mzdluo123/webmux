package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
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
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		for _, path := range []string{"/api/relay", "/api/relay/enrollments", "/api/relay/ice", "/api/relay/auth/session", "/api/relay/reconnect", "/api/relay/binding"} {
			r := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader("{}"))
			r = r.WithContext(context.WithValue(r.Context(), peerKey{}, 123))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("session process route %s %s returned %d", method, path, w.Code)
			}
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

type relayEnrollmentControl struct {
	RelayControl
	requests  []contract.RelayEnrollmentRequest
	operation contract.RelayOperation
	status    contract.RelayStatus
}

func (c *relayEnrollmentControl) Enroll(_ context.Context, req contract.RelayEnrollmentRequest) (contract.RelayPairing, error) {
	c.requests = append(c.requests, req)
	return contract.RelayPairing{Phase: "preparing"}, nil
}

func (c *relayEnrollmentControl) Pairing() contract.RelayPairing {
	return contract.RelayPairing{Phase: "waiting", UserCode: "ABCD-EFGH", VerificationURL: "https://github.com/login/device", ExpiresAt: 1800000000, PollIntervalSeconds: 5}
}

func (c *relayEnrollmentControl) Cancel(context.Context) (contract.RelayPairing, error) {
	return contract.RelayPairing{Phase: "cancelling"}, nil
}

func (c *relayEnrollmentControl) Status() contract.RelayStatus {
	operation := c.operation
	if operation.Kind == "" {
		operation = contract.RelayOperation{Kind: "idle", Phase: "idle"}
	}
	if c.status.State != "" {
		status := c.status
		status.Operation = operation
		return status
	}
	return contract.RelayStatus{
		Operation: operation, State: "off", Transport: "wss",
		TCPAlgorithm: "unknown", ControlSource: "default",
		ControlReachability: "unknown", DataReachability: "unknown",
		AssignmentRevision: "0", ConfigRevision: "0", AppliedRevision: "0",
		AssignmentSource: "unassigned", Capabilities: []string{},
		Sessions: []contract.RelaySessionDiagnostic{},
	}
}

func TestRelayEnrollmentAcceptsOnlyFreshPairingRequest(t *testing.T) {
	control := &relayEnrollmentControl{}
	s := &Server{Deps: Deps{Relay: &Relay{Control: control}}}
	mux := http.NewServeMux()
	s.relayRoutes(mux)
	for _, tc := range []struct {
		body string
		want int
	}{
		{body: `{"confirmPublic":true}`, want: http.StatusCreated},
		{body: `{"confirmPublic":true,"rebind":false}`, want: http.StatusBadRequest},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/relay/enrollments", strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("request %s returned %d, want %d", tc.body, w.Code, tc.want)
		}
	}
	if len(control.requests) != 1 || !control.requests[0].ConfirmPublic {
		t.Fatalf("retired request reached enrollment controller: %+v", control.requests)
	}
}

func TestRelayPairingObservationAndCancellationContract(t *testing.T) {
	control := &relayEnrollmentControl{}
	s := &Server{Deps: Deps{Relay: &Relay{Control: control}}}
	mux := http.NewServeMux()
	s.relayRoutes(mux)
	tests := []struct {
		method string
		path   string
		body   string
		want   int
		phase  string
	}{
		{http.MethodGet, "/api/relay/enrollments", "", http.StatusOK, "waiting"},
		{http.MethodPost, "/api/relay/enrollments", `{"confirmPublic":true}`, http.StatusCreated, "preparing"},
		{http.MethodDelete, "/api/relay/enrollments", `{}`, http.StatusOK, "cancelling"},
		{http.MethodDelete, "/api/relay/enrollments", `{"handle":"retired"}`, http.StatusBadRequest, ""},
		{http.MethodPost, "/api/relay/enrollments/poll", `{}`, http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Fatalf("%s %s: got %d want %d", tt.method, tt.path, w.Code, tt.want)
		}
		if tt.phase == "" {
			continue
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("pairing response is cacheable")
		}
		var flow contract.RelayPairing
		if err := json.Unmarshal(w.Body.Bytes(), &flow); err != nil || flow.Phase != tt.phase {
			t.Fatalf("invalid pairing response: %s %v", w.Body, err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 7 {
			t.Fatalf("pairing response changed required field set: %s", w.Body)
		}
		if _, ok := fields["handle"]; ok {
			t.Fatal("pairing response exposed opaque polling handle")
		}
	}
}

func TestWireSnapshotsRelay(t *testing.T) {
	control := &relayEnrollmentControl{}
	control.status = control.Status()
	control.status.Enabled = true
	control.status.Paired = true
	control.status.State = "failed"
	control.status.ControlReady = true
	control.status.ControlReachability = "reachable"
	control.status.DataReachability = "unavailable"
	checkedAt := time.Date(2026, time.November, 25, 10, 42, 16, 0, time.UTC)
	control.status.DataCheckedAt = &checkedAt
	control.status.ReadinessError = "data-probe-failed"
	control.status.Error = "relay origin probe failed"
	control.status.BoundPublicURL = "https://host.relay.example"
	control.status.PublicURL = control.status.BoundPublicURL
	s := &Server{Deps: Deps{Relay: &Relay{Control: control}}}
	mux := http.NewServeMux()
	s.relayRoutes(mux)
	recorder := &wireRecorder{t: t, norm: newWireNormalizer(), seen: map[string]bool{}}
	for _, tt := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"relay-pairing-waiting", http.MethodGet, "/api/relay/enrollments", ""},
		{"relay-pairing-preparing", http.MethodPost, "/api/relay/enrollments", `{"confirmPublic":true}`},
		{"relay-pairing-cancelling", http.MethodDelete, "/api/relay/enrollments", `{}`},
		{"relay-status", http.MethodGet, "/api/relay", ""},
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
		recorder.record(tt.name, w.Code, w.Body.Bytes())
	}
	control.status = contract.RelayStatus{}
	for _, closed := range []bool{false, true} {
		control.operation = contract.RelayOperation{Kind: "delete", Phase: "completed", Deleted: true, AccessClosed: closed}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/relay", nil))
		name := "relay-deleted-access-open"
		if closed {
			name = "relay-deleted-access-closed"
		}
		recorder.record(name, w.Code, w.Body.Bytes())
	}
}

func TestRelayPairingRetainsAuthenticationAndOriginChecks(t *testing.T) {
	control := &relayEnrollmentControl{}
	s := &Server{Deps: Deps{
		Config: config.Config{Listen: "127.0.0.1:7681", Token: "secret"},
		Public: &transport.PublicOrigins{}, Relay: &Relay{Control: control},
	}}
	mux := http.NewServeMux()
	s.relayRoutes(mux)
	handler := s.middleware(mux)
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		for _, tt := range []struct {
			token  string
			origin string
			want   int
		}{
			{"", "", http.StatusUnauthorized},
			{"secret", "https://evil.example", http.StatusForbidden},
		} {
			r := httptest.NewRequest(method, "http://localhost:7681/api/relay/enrollments", strings.NewReader(`{"confirmPublic":true}`))
			if tt.token != "" {
				r.Header.Set("Authorization", "Bearer "+tt.token)
			}
			r.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tt.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("%s authorization boundary: got %d cache=%q", method, w.Code, w.Header().Get("Cache-Control"))
			}
		}
	}
	if len(control.requests) != 0 {
		t.Fatal("rejected request reached pairing controller")
	}
}

func TestRelayFailureNeverExposesUnsafeErrors(t *testing.T) {
	for _, err := range []error{
		errors.New("private-token https://internal.example"),
		protocol.Error{Code: protocol.Code("private-token https://internal.example")},
	} {
		w := httptest.NewRecorder()
		relayFailure(w, err)
		if w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "private-token") || strings.Contains(w.Body.String(), "internal.example") {
			t.Fatalf("relay error leaked unsafe material: %d %s", w.Code, w.Body)
		}
	}
}
