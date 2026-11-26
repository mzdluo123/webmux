package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/relay"
	"github.com/rainchan/webmux/internal/relaysetting"
	"github.com/rainchan/webmux/internal/transport"
)

func TestRelayDisableRevokesWhenPersistenceFails(t *testing.T) {
	ctx := t.Context()
	dir := filepath.Join(t.TempDir(), "state")
	cfg := config.Config{StateDir: dir, Token: strings.Repeat("x", 43), Relay: config.RelayConfig{Transport: "wss", ControlEndpoint: "https://control.example.com", ControlAudience: "control.example.com"}}
	public := &transport.PublicOrigins{}
	rc, err := newRelayController(ctx, cfg, public)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	host := protocol.HostID("0123456789abcdef0123456789abcdef")
	origin := "https://" + host.Label() + ".example.com"
	settings := relaysetting.Settings{Config: cfg.Relay, Seed: base64.RawURLEncoding.EncodeToString(key.Seed()), Binding: &protocol.Binding{HostID: host, OwnerID: "123", BindingGeneration: 1, PublicURL: origin}, Confirmed: true}
	settings.Config.Enabled = true
	if err := rc.store.Save(settings); err != nil {
		t.Fatal(err)
	}
	rc.settings = settings
	rc.status.State = "ready"
	rc.status.ControlReady = true
	rc.observation = relay.DataObservation{CheckedAt: time.Now(), ValidUntil: time.Now().Add(time.Minute), Ready: true}
	rc.status.PublicURL = origin
	if err := public.Set("relay", origin); err != nil {
		t.Fatal(err)
	}
	if err := public.Set("cloudflare", "https://other.example.com"); err != nil {
		t.Fatal(err)
	}
	active, cancel := context.WithCancel(ctx)
	rc.cancel = cancel
	moved := dir + "-preserved"
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	off := false
	state, err := rc.Update(ctx, contract.RelayUpdate{Enabled: &off})
	if !errors.Is(err, relaysetting.ErrNotPersisted) {
		t.Fatalf("disable error=%v, want not persisted", err)
	}
	if state.Enabled || state.ControlReady || state.DataReady || state.DataCheckedAt != nil || state.PublicURL != "" || public.SourceOrigin("relay") != "" {
		t.Fatalf("failed save retained public access: %+v", state)
	}
	if receipt := state.Operation; receipt.Kind != "disable" || receipt.Phase != "failed" || receipt.Error != "storage-unavailable" {
		t.Fatalf("failed durable disable receipt=%+v", receipt)
	}
	if public.SourceOrigin("cloudflare") != "https://other.example.com" {
		t.Fatal("disabling relay revoked independent origin")
	}
	select {
	case <-active.Done():
	default:
		t.Fatal("active relay worker was not cancelled")
	}
	stored, _, err := relaysetting.Inspect(moved, cfg.Relay)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Config.Enabled || stored.Binding == nil || stored.Binding.PublicURL != origin {
		t.Fatal("failed save destroyed recoverable identity or misreported durable disable")
	}
}

func TestRelayObservationDoesNotChangeOriginAuthorization(t *testing.T) {
	const origin = "https://h-0123456789abcdef0123456789abcdef.example.com"
	host := strings.TrimPrefix(origin, "https://")
	ctx := t.Context()
	now := time.Now()
	scope := protocol.Scope{ConnectorGeneration: 1}
	public := &transport.PublicOrigins{}
	rc := &relayController{public: public, generation: 1, pairingNow: func() time.Time { return now },
		settings: relaysetting.Settings{Binding: &protocol.Binding{PublicURL: origin}}, store: relaysetting.Open(t.TempDir(), config.RelayConfig{})}
	defer rc.store.Close()
	rc.setOriginHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !transport.IsTrustedRelay(r.Context()) || !public.HostAllowed(r.Host) || !transport.OriginAllowed(config.Config{}, public, r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := func(method, path, requestOrigin string) int {
		t.Helper()
		r := httptest.NewRequest(method, "https://"+host+path, nil)
		r.Header.Set("Origin", requestOrigin)
		if strings.HasPrefix(path, "/ws/") {
			r.Header.Set("Connection", "Upgrade")
			r.Header.Set("Upgrade", "websocket")
		}
		w := httptest.NewRecorder()
		rc.origin.ServeHTTP(w, r)
		return w.Code
	}
	assertRoute := func() {
		t.Helper()
		if link, ok := relayAccessLink(rc.Status()); !ok || link.URL != origin {
			t.Fatal("authorized allocation lost its access link")
		}
		if got := request(http.MethodPost, "/api/example", origin); got != http.StatusNoContent {
			t.Fatalf("authorized HTTP request returned %d", got)
		}
		if got := request(http.MethodGet, "/ws/example", origin); got != http.StatusNoContent {
			t.Fatalf("authorized WebSocket upgrade request returned %d", got)
		}
		if got := request(http.MethodGet, "/ws/example", "https://wrong.example"); got != http.StatusForbidden {
			t.Fatalf("incorrect Origin returned %d", got)
		}
	}
	assertObservation := func(reachability string, checkedAt time.Time) {
		t.Helper()
		s := rc.Status()
		if s.DataReachability != reachability || s.DataReady != (reachability == "reachable") ||
			checkedAt.IsZero() != (s.DataCheckedAt == nil) || s.DataCheckedAt != nil && !s.DataCheckedAt.Equal(checkedAt) {
			t.Fatalf("wrong observation: %+v", s)
		}
	}
	rc.connected(ctx, 1, scope)
	rc.dataTransport(ctx, 1, scope, true, "")
	rc.dataObserved(ctx, 1, scope, relay.DataObservation{})
	assertObservation("unknown", time.Time{})
	assertRoute()

	first := now.Add(-time.Second)
	rc.dataObserved(ctx, 1, scope, relay.DataObservation{CheckedAt: first, ValidUntil: now.Add(time.Second), Ready: true})
	assertObservation("reachable", first)
	rc.dataObserved(ctx, 0, scope, relay.DataObservation{})
	assertObservation("reachable", first)
	now = now.Add(2 * time.Second)
	assertObservation("unknown", first)
	assertRoute()

	second := now.Add(-time.Second)
	rc.dataObserved(ctx, 1, scope, relay.DataObservation{CheckedAt: second, ValidUntil: now.Add(2 * time.Second), Ready: true})
	assertObservation("reachable", second)
	rc.dataObserved(ctx, 1, scope, relay.DataObservation{CheckedAt: first, ValidUntil: now.Add(time.Second)})
	assertObservation("reachable", second)
	rc.dataObserved(ctx, 1, scope, relay.DataObservation{CheckedAt: now, ValidUntil: now.Add(time.Second)})
	assertObservation("unavailable", now)
	if rc.Status().ReadinessError != "data-probe-failed" {
		t.Fatal("fresh failed probe lost its diagnostic")
	}
	assertRoute()
	failedAt := now
	now = now.Add(time.Second)
	assertObservation("unknown", failedAt)
	if s := rc.Status(); s.State == "failed" || s.ReadinessError != "" {
		t.Fatalf("aged failure still presented as current: %+v", s)
	}
	assertRoute()
	if _, ok := rc.authSnapshot(); !ok {
		t.Fatal("data loss invalidated independent management authorization")
	}

	rc.dataTransport(ctx, 1, scope, false, protocol.Unavailable)
	assertObservation("unknown", failedAt)
	if got := request(http.MethodPost, "/api/example", origin); got != http.StatusForbidden {
		t.Fatalf("transport loss retained origin registration: %d", got)
	}
	rc.disconnected(1)
	assertObservation("unknown", time.Time{})
	rc.dataTransport(ctx, 1, scope, true, "")
	if s := rc.Status(); s.ControlReady || s.PublicURL != "" || public.SourceOrigin("relay") != "" {
		t.Fatalf("old allocation returned after management disconnect: %+v", s)
	}
	next := scope
	next.ConnectorGeneration++
	rc.connected(ctx, 1, next)
	rc.dataTransport(ctx, 1, scope, true, "")
	rc.dataObserved(ctx, 1, scope, relay.DataObservation{CheckedAt: now, ValidUntil: now.Add(time.Second), Ready: true})
	if s := rc.Status(); !s.ControlReady || s.PublicURL != "" || s.DataCheckedAt != nil {
		t.Fatalf("old allocation returned after a new management scope: %+v", s)
	}
	// A probe old enough to have been invalidated with its registration must
	// not be shown as the new route's last check.
	rc.dataTransport(ctx, 1, next, true, "")
	rc.dataObserved(ctx, 1, next, relay.DataObservation{CheckedAt: now, ValidUntil: now.Add(time.Minute), Ready: true})
	assertObservation("reachable", now)
	rc.dataObserved(ctx, 1, next, relay.DataObservation{})
	assertObservation("unknown", now)
	rc.dataTransport(ctx, 1, next, false, protocol.Unavailable)
	rc.dataObserved(ctx, 1, next, relay.DataObservation{RouteChanged: true})
	rc.dataTransport(ctx, 1, next, true, "")
	assertObservation("unknown", time.Time{})
	now = now.Add(time.Second)
	rc.dataObserved(ctx, 1, next, relay.DataObservation{CheckedAt: now, ValidUntil: now.Add(time.Minute), Ready: true})
	assertObservation("reachable", now)
}

func TestRelayConnectedClearsFailureOnlyForActiveGeneration(t *testing.T) {
	for _, tt := range []struct {
		name      string
		gen       uint64
		canceled  bool
		wantReady bool
	}{
		{name: "recovered", gen: 5, wantReady: true},
		{name: "stale generation", gen: 4},
		{name: "canceled worker", gen: 5, canceled: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.canceled {
				cancel()
			}
			public := &transport.PublicOrigins{}
			rc := &relayController{
				public: public, generation: 5,
				status: contract.RelayStatus{
					State: "failed", Error: "relay connection unavailable",
					ReadinessError: "connection-unavailable", ReconnectReason: "connection-lost",
				},
			}
			rc.connected(ctx, tt.gen, protocol.Scope{})
			if tt.wantReady {
				if !rc.status.ControlReady || rc.status.DataReady ||
					rc.status.State != "connecting" || rc.status.PublicURL != "" ||
					rc.status.Error != "" || rc.status.ReadinessError != "" ||
					rc.status.ReconnectReason != "" || public.SourceOrigin("relay") != "" {
					t.Fatalf("management recovery retained failure or published before transport registration: %+v", rc.status)
				}
			} else if rc.status.ControlReady || public.SourceOrigin("relay") != "" ||
				rc.status.ReconnectReason != "connection-lost" || rc.status.State != "failed" {
				t.Fatalf("inactive worker changed failure status or authorization: %+v", rc.status)
			}
		})
	}
}

func TestRelayAuthSnapshotRetainsBindingWithoutReadiness(t *testing.T) {
	binding := protocol.Binding{HostID: "0123456789abcdef0123456789abcdef", OwnerID: "123", BindingGeneration: 1, PublicURL: "https://h-0123456789abcdef0123456789abcdef.example.com"}
	rc := &relayController{settings: relaysetting.Settings{Binding: &binding}}
	got, ready := rc.authSnapshot()
	if ready || got.Binding != binding {
		t.Fatalf("offline snapshot lost binding identity or authorized access: %+v ready=%v", got, ready)
	}
	rc.settings.Binding = nil
	if got, ready := rc.authSnapshot(); ready || got.Binding.HostID != "" {
		t.Fatal("unbound snapshot retained identity")
	}
}

type fakeRelayReconnectClient struct {
	assignment func(attempt int) (protocol.Assignment, error)
	connect    func(attempt int, cb relay.Callbacks) error
	attempts   int
}

func (f *fakeRelayReconnectClient) Enroll(context.Context, ed25519.PrivateKey, protocol.MachineID) (protocol.Enrollment, error) {
	return protocol.Enrollment{}, protocol.Error{Code: protocol.Unavailable}
}

func (f *fakeRelayReconnectClient) Poll(context.Context, protocol.Token) (protocol.EnrollmentResult, error) {
	return protocol.EnrollmentResult{}, protocol.Error{Code: protocol.Unavailable}
}

func (f *fakeRelayReconnectClient) Cancel(context.Context, protocol.Token) error { return nil }

func (f *fakeRelayReconnectClient) Delete(context.Context, ed25519.PrivateKey, protocol.Binding) (protocol.BindingDeletionResult, error) {
	return protocol.BindingDeletionResult{}, protocol.Error{Code: protocol.Unavailable}
}

func (f *fakeRelayReconnectClient) Assignment(_ context.Context, _ ed25519.PrivateKey, _ protocol.Binding) (protocol.Assignment, error) {
	f.attempts++
	return f.assignment(f.attempts)
}

func (f *fakeRelayReconnectClient) Connect(_ context.Context, _ ed25519.PrivateKey, _ protocol.Assignment, cb relay.Callbacks) error {
	return f.connect(f.attempts, cb)
}

func TestRelayReconnectBackoffResetsAfterStableReadyAndHonorsRetryAfter(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dir := filepath.Join(t.TempDir(), "state")
	store := relaysetting.Open(dir, config.RelayConfig{Transport: "wss", ControlEndpoint: "https://control.example.com", ControlAudience: "control.example.com"})
	defer store.Close()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	host := protocol.HostID("0123456789abcdef0123456789abcdef")
	origin := "https://" + host.Label() + ".example.com"
	binding := protocol.Binding{HostID: host, OwnerID: "123", BindingGeneration: 1, PublicURL: origin}
	settings := relaysetting.Settings{
		Config:    config.RelayConfig{Enabled: true, Transport: "wss", ControlEndpoint: "https://control.example.com", ControlAudience: "control.example.com"},
		Seed:      base64.RawURLEncoding.EncodeToString(key.Seed()),
		Binding:   &binding,
		Confirmed: true,
	}
	if err := store.Save(settings); err != nil {
		t.Fatal(err)
	}

	now := time.Unix(1_700_000_000, 0)
	var delays []time.Duration
	fake := &fakeRelayReconnectClient{
		assignment: func(attempt int) (protocol.Assignment, error) {
			if attempt == 2 {
				return protocol.Assignment{}, protocol.Error{Code: protocol.RateLimited, RetryAfterSeconds: 12}
			}
			return protocol.Assignment{
				Version:        protocol.V1,
				Revision:       1,
				Binding:        binding,
				NodeID:         "node-a",
				Capabilities:   protocol.Capabilities{protocol.CapabilityWSS},
				ConfigRevision: 1,
				ConnectorURL:   "wss://node.example.com/v1/connector",
				DataOrigin:     "https://node.example.com",
			}, nil
		},
		connect: func(attempt int, cb relay.Callbacks) error {
			switch attempt {
			case 1:
				// Fails before Ready: backoff starts at 1s and doubles to 2s.
				return protocol.Error{Code: protocol.Unavailable}
			case 3:
				// Enters Ready briefly (< 30s threshold): backoff continues from 4s to 8s.
				cb.Ready(protocol.Scope{HostID: host, BindingGeneration: 1, ConnectorGeneration: 1})
				now = now.Add(5 * time.Second)
				return protocol.Error{Code: protocol.Unavailable}
			case 4:
				// Enters Ready and stays alive >= 30s threshold: resets backoff back to 1s.
				cb.Ready(protocol.Scope{HostID: host, BindingGeneration: 1, ConnectorGeneration: 2})
				now = now.Add(protocol.ControlLease + time.Second)
				return protocol.Error{Code: protocol.Unavailable}
			default:
				cancel()
				return ctx.Err()
			}
		},
	}

	rc := &relayController{
		ctx:        ctx,
		store:      store,
		client:     fake,
		public:     &transport.PublicOrigins{},
		settings:   settings,
		generation: 1,
		pairingNow: func() time.Time { return now },
		reconnectWait: func(_ context.Context, d time.Duration) bool {
			delays = append(delays, d)
			return true
		},
	}
	rc.run(ctx, 1, settings)

	if len(delays) != 4 {
		t.Fatalf("recorded delays = %v, want 4 entries", delays)
	}
	// Attempt 1: base 1s + [0, 0.5s) jitter.
	if delays[0] < time.Second || delays[0] >= 1500*time.Millisecond {
		t.Fatalf("attempt 1 delay = %v, want in [1s, 1.5s)", delays[0])
	}
	// Attempt 2: RetryAfterSeconds=12 overrides 2s base, plus bounded jitter.
	if delays[1] < 12*time.Second || delays[1] >= 18*time.Second {
		t.Fatalf("attempt 2 delay = %v, want in [12s, 18s)", delays[1])
	}
	// Attempt 3: short Ready (< ControlLease) did not reset; base is 4s.
	if delays[2] < 4*time.Second || delays[2] >= 6*time.Second {
		t.Fatalf("attempt 3 delay = %v, want in [4s, 6s)", delays[2])
	}
	// Attempt 4: stable Ready (>= ControlLease) reset backoff back to 1s.
	if delays[3] < time.Second || delays[3] >= 1500*time.Millisecond {
		t.Fatalf("attempt 4 delay after stable session = %v, want in [1s, 1.5s)", delays[3])
	}
}

type relaySelectionTestClient struct {
	relayControlClient
	scope     protocol.Scope
	selection relay.TransportSelection
	apply     func(context.Context, protocol.Scope, protocol.FRPTransport, func() error) error
}

func (c *relaySelectionTestClient) Scope() (protocol.Scope, bool) {
	return c.scope, true
}

func (c *relaySelectionTestClient) TransportSelection() relay.TransportSelection {
	return c.selection
}

func (c *relaySelectionTestClient) SetPreferredTransport(preferred protocol.FRPTransport) {
	c.selection.Preferred = preferred
}

func (c *relaySelectionTestClient) ApplyTransportInScope(ctx context.Context, scope protocol.Scope, desired protocol.FRPTransport, persist func() error) error {
	return c.apply(ctx, scope, desired, persist)
}

func relaySelectionTestController(t *testing.T, preferred protocol.FRPTransport) (config.Config, *relayController, *relaySelectionTestClient) {
	t.Helper()
	cfg := config.Config{
		StateDir: t.TempDir(),
		Token:    strings.Repeat("x", 43),
		Relay:    config.RelayConfig{Transport: "wss", ControlEndpoint: "https://control.example.com", ControlAudience: "control.example.com"},
	}
	settings := relaysetting.Settings{
		Config: cfg.Relay, Confirmed: true, PreferredTransport: preferred,
		Seed: base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SeedSize)),
	}
	host := protocol.HostID("0123456789abcdef0123456789abcdef")
	settings.Binding = &protocol.Binding{HostID: host, OwnerID: "123", BindingGeneration: 1, PublicURL: "https://" + host.Label() + ".example.com"}
	settings.Config.Enabled = true
	store := relaysetting.Open(cfg.StateDir, cfg.Relay)
	err := store.Save(settings)
	store.Close()
	if err != nil {
		t.Fatal(err)
	}
	rc, err := newRelayController(t.Context(), cfg, &transport.PublicOrigins{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rc.Close)
	status := rc.Status()
	selection := status.TransportSelection
	if selection == nil || selection.Preferred != preferred || selection.Supported || selection.Active != "" || status.DataReady {
		t.Fatalf("startup misreported persisted preference as live data: %+v", status)
	}
	real, ok := rc.client.(relayTransportClient)
	if !ok || real.TransportSelection().Preferred != preferred {
		t.Fatal("connector did not receive the saved preference")
	}
	client := &relaySelectionTestClient{scope: protocol.Scope{HostID: host, BindingGeneration: 1, ConnectorGeneration: 1}, selection: relay.TransportSelection{Preferred: preferred}}
	rc.client = client
	rc.generation = 1
	rc.connected(t.Context(), 1, client.scope)
	return cfg, rc, client
}

func inspectRelayPreference(t *testing.T, cfg config.Config) protocol.FRPTransport {
	t.Helper()
	settings, _, err := relaysetting.Inspect(cfg.StateDir, cfg.Relay)
	if err != nil {
		t.Fatal(err)
	}
	return settings.DataProtocol()
}

func TestRelayRecoveryPersistsOnlyAfterConfirmedWSS(t *testing.T) {
	cfg, rc, client := relaySelectionTestController(t, protocol.FRPTransportQUIC)
	checkedAt := rc.clock()
	client.apply = func(_ context.Context, _ protocol.Scope, desired protocol.FRPTransport, _ func() error) error {
		client.selection.LastAttempt = &relay.TransportAttempt{NodeID: "node-a", Protocol: desired, Stage: "probe",
			Error: string(protocol.Expired), DurationMs: 12000, CompletedAt: checkedAt}
		return protocol.Error{Code: protocol.Expired}
	}
	status, err := rc.SelectTransport(t.Context(), protocol.FRPTransportWSS)
	if !errors.Is(err, protocol.Error{Code: protocol.Expired}) || inspectRelayPreference(t, cfg) != protocol.FRPTransportQUIC {
		t.Fatalf("failed probe changed saved preference: status=%+v err=%v", status, err)
	}
	selection := status.TransportSelection
	if selection == nil || selection.Active != "" || selection.Preferred != protocol.FRPTransportQUIC ||
		selection.LastAttempt == nil || selection.LastAttempt.NodeID != "node-a" || selection.LastAttempt.Stage != "probe" ||
		selection.LastAttempt.Protocol != protocol.FRPTransportWSS || selection.LastAttempt.Error != string(protocol.Expired) ||
		selection.LastAttempt.DurationMS != 12000 || !selection.LastAttempt.CompletedAt.Equal(checkedAt) || status.DataCheckedAt != nil {
		t.Fatalf("failed recovery missing attempt diagnostics: selection=%+v status=%+v", selection, status)
	}

	client.apply = func(ctx context.Context, scope protocol.Scope, desired protocol.FRPTransport, persist func() error) error {
		if desired != protocol.FRPTransportWSS || scope != client.scope {
			t.Fatalf("recovery received protocol=%q scope=%+v", desired, scope)
		}
		client.selection.Pending = desired
		busy, err := rc.SelectTransport(ctx, protocol.FRPTransportQUIC)
		if !errors.Is(err, protocol.Error{Code: protocol.Conflict}) || busy.TransportSelection.Pending != desired ||
			busy.TransportSelection.Preferred != protocol.FRPTransportQUIC || busy.TransportSelection.Active != "" {
			t.Fatalf("concurrent attempt not fenced: selection=%+v err=%v", busy.TransportSelection, err)
		}
		if inspectRelayPreference(t, cfg) != protocol.FRPTransportQUIC {
			t.Fatal("unconfirmed WSS recovery overwrote the preference")
		}
		rc.dataTransport(ctx, 1, scope, true, "")
		rc.dataObserved(ctx, 1, scope, relay.DataObservation{CheckedAt: checkedAt, ValidUntil: checkedAt.Add(time.Minute), Ready: true})
		client.selection.Active = desired
		if err := persist(); err != nil {
			return err
		}
		client.selection.Pending = ""
		client.selection.LastAttempt = &relay.TransportAttempt{NodeID: "node-a", Protocol: desired, Stage: "commit", CompletedAt: checkedAt}
		return nil
	}
	status, err = rc.SelectTransport(t.Context(), protocol.FRPTransportWSS)
	selection = status.TransportSelection
	if err != nil || inspectRelayPreference(t, cfg) != protocol.FRPTransportWSS ||
		selection == nil || selection.Preferred != protocol.FRPTransportWSS || selection.Active != protocol.FRPTransportWSS ||
		selection.Pending != "" || selection.Supported || selection.LastAttempt == nil || selection.LastAttempt.Stage != "commit" ||
		!status.DataReady || status.DataReachability != "reachable" || status.DataCheckedAt == nil || !status.DataCheckedAt.Equal(checkedAt) {
		t.Fatalf("verified local recovery not published: selection=%+v status=%+v err=%v", selection, status, err)
	}
}

func TestRelayTransportCommitCannotPersistInAnotherScope(t *testing.T) {
	for _, updateController := range []bool{false, true} {
		name := "connector superseded before callback"
		if updateController {
			name = "controller already on next connector scope"
		}
		t.Run(name, func(t *testing.T) {
			cfg, rc, client := relaySelectionTestController(t, protocol.FRPTransportWSS)
			client.apply = func(ctx context.Context, _ protocol.Scope, _ protocol.FRPTransport, persist func() error) error {
				client.scope.ConnectorGeneration++
				if updateController {
					rc.connected(ctx, 1, client.scope)
				}
				return persist()
			}
			_, err := rc.SelectTransport(t.Context(), protocol.FRPTransportQUIC)
			if !errors.Is(err, protocol.Error{Code: protocol.StaleGeneration}) || inspectRelayPreference(t, cfg) != protocol.FRPTransportWSS {
				t.Fatalf("stale commit overwrote preference: %v", err)
			}
		})
	}
}

func TestRelayTransportFailedPreferenceWriteKeepsPreviousPreference(t *testing.T) {
	cfg, rc, client := relaySelectionTestController(t, protocol.FRPTransportWSS)
	client.selection.Supported = true
	moved := filepath.Join(t.TempDir(), "preserved")
	client.apply = func(_ context.Context, _ protocol.Scope, desired protocol.FRPTransport, persist func() error) error {
		if err := os.Rename(cfg.StateDir, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cfg.StateDir, []byte("blocked"), 0600); err != nil {
			t.Fatal(err)
		}
		client.selection.Active = desired // The node already committed.
		return persist()
	}
	status, err := rc.SelectTransport(t.Context(), protocol.FRPTransportQUIC)
	selection := status.TransportSelection
	cfg.StateDir = moved
	if !errors.Is(err, relaysetting.ErrUnavailable) || status.StorageAvailable ||
		selection.Preferred != protocol.FRPTransportWSS || selection.Active != protocol.FRPTransportQUIC ||
		inspectRelayPreference(t, cfg) != protocol.FRPTransportWSS {
		t.Fatalf("failed preference write misreported: selection=%+v err=%v", selection, err)
	}
}
