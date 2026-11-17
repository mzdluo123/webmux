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

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/relaysetting"
	"github.com/rainchan/webmux/internal/transport"
)

func TestRelayDisableRevokesWhenPersistenceFails(t *testing.T) {
	ctx := t.Context()
	listener, localTLS, err := relayLoopback(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	dir := filepath.Join(t.TempDir(), "state")
	cfg := config.Config{StateDir: dir, Token: strings.Repeat("x", 43), Relay: config.RelayConfig{Transport: "wss", ControlEndpoint: "https://control.example.com", ControlAudience: "control.example.com"}}
	public := &transport.PublicOrigins{}
	rc, err := newRelayController(ctx, cfg, public, listener.Addr().String(), localTLS)
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
	if state.Enabled || state.ControlReady || state.PublicURL != "" || public.SourceOrigin("relay") != "" {
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

func TestRelayReadinessRequiresInteractivePool(t *testing.T) {
	rc := &relayController{generation: 1}
	rc.poolsReady(1, false, true)
	if rc.status.State != "degraded" || rc.status.ControlReady || !rc.status.BulkReady || rc.status.ReadyConnections != 1 {
		t.Fatalf("bulk-only transport reported healthy: %+v", rc.status)
	}
	rc.poolsReady(1, true, true)
	if rc.status.State != "ready" || !rc.status.ControlReady || rc.status.ReadyConnections != 2 {
		t.Fatalf("recovered interactive pool not ready: %+v", rc.status)
	}
	rc.poolsReady(0, false, false)
	if rc.status.State != "ready" {
		t.Fatal("old generation changed readiness")
	}
	rc.poolsReady(1, false, true)
	if rc.status.State != "degraded" || rc.status.ControlReady || rc.status.ReadyConnections != 1 {
		t.Fatalf("interactive loss retained ready state: %+v", rc.status)
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
			const origin = "https://h-0123456789abcdef0123456789abcdef.example.com"
			public := &transport.PublicOrigins{}
			rc := &relayController{
				public: public, generation: 5,
				status: contract.RelayStatus{
					State: "failed", Error: "relay connection unavailable",
					ReadinessError: "connection-unavailable", ReconnectReason: "connection-lost",
				},
			}
			settings := relaysetting.Settings{Binding: &protocol.Binding{PublicURL: origin}}
			rc.connected(ctx, tt.gen, settings, protocol.Scope{})
			if tt.wantReady {
				if rc.status.ControlReady || rc.status.ReadyConnections != 0 ||
					rc.status.State != "degraded" || rc.status.PublicURL != origin ||
					rc.status.Error != "" || rc.status.ReadinessError != "" ||
					rc.status.ReconnectReason != "" || public.SourceOrigin("relay") != origin {
					t.Fatalf("recovery retained failure or did not authorize origin: %+v", rc.status)
				}
			} else if rc.status.ControlReady || public.SourceOrigin("relay") != "" ||
				rc.status.ReconnectReason != "connection-lost" || rc.status.State != "failed" {
				t.Fatalf("inactive worker changed failure status or authorization: %+v", rc.status)
			}
		})
	}
}
