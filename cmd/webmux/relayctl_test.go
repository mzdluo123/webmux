package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
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
	rc.status.DataReady = true
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
	if state.Enabled || state.ControlReady || state.DataReady || state.PublicURL != "" || public.SourceOrigin("relay") != "" {
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

func TestRelayReadinessRequiresCurrentOriginProbe(t *testing.T) {
	const origin = "https://h-0123456789abcdef0123456789abcdef.example.com"
	public := &transport.PublicOrigins{}
	rc := &relayController{public: public, generation: 1, status: contract.RelayStatus{ControlReady: true},
		settings: relaysetting.Settings{Binding: &protocol.Binding{PublicURL: origin}}, store: relaysetting.Open(t.TempDir(), config.RelayConfig{})}
	defer rc.store.Close()
	rc.dataReady(t.Context(), 1, false, protocol.UpstreamFailed)
	if _, linked := relayAccessLink(rc.Status()); !rc.status.ControlReady || rc.status.DataReady || rc.status.ReadinessError != "data-probe-failed" || linked {
		t.Fatalf("failed origin probe published access: %+v", rc.status)
	}
	rc.dataReady(t.Context(), 1, true, "")
	if link, linked := relayAccessLink(rc.Status()); rc.status.State != "ready" || !rc.status.ControlReady || !rc.status.DataReady || public.SourceOrigin("relay") != origin || !linked || link.URL != origin {
		t.Fatalf("recovered origin probe did not publish access: %+v", rc.status)
	}
	rc.dataReady(t.Context(), 0, false, protocol.Unavailable)
	if !rc.status.DataReady {
		t.Fatal("stale generation changed readiness")
	}
	rc.dataReady(t.Context(), 1, false, protocol.Unavailable)
	if _, linked := relayAccessLink(rc.Status()); rc.status.DataReady || public.SourceOrigin("relay") != "" || linked {
		t.Fatalf("probe loss retained access: %+v", rc.status)
	}
	if _, ok := rc.authSnapshot(); !ok {
		t.Fatal("data loss invalidated independent management authorization")
	}
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
					t.Fatalf("management recovery retained failure or published before probing: %+v", rc.status)
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
