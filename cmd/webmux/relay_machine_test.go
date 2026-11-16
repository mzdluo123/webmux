package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
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

type machineEnrollmentCall struct {
	id     protocol.MachineID
	public string
	rebind bool
}

type machineEnrollmentClient struct {
	*relay.Client
	calls         []machineEnrollmentCall
	failure       error
	pollFailure   error
	cancelFailure error
	binding       protocol.Binding
	deleteCalls   int
}

func (c *machineEnrollmentClient) Enroll(_ context.Context, key ed25519.PrivateKey, id protocol.MachineID, rebind bool) (protocol.Enrollment, error) {
	c.calls = append(c.calls, machineEnrollmentCall{id: id, public: base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), rebind: rebind})
	if c.failure != nil {
		return protocol.Enrollment{}, c.failure
	}
	return protocol.Enrollment{Version: protocol.V1, Handle: protocol.Token(strings.Repeat("A", 43)), ExpiresAt: time.Now().Add(time.Minute).Unix(), PollIntervalSeconds: 1}, nil
}

func (c *machineEnrollmentClient) Poll(context.Context, protocol.Token) (protocol.EnrollmentResult, error) {
	if c.pollFailure != nil {
		return protocol.EnrollmentResult{}, c.pollFailure
	}
	return protocol.EnrollmentResult{Version: protocol.V1, Binding: c.binding, OwnerLogin: "owner"}, nil
}

func (c *machineEnrollmentClient) Cancel(context.Context, protocol.Token) error {
	return c.cancelFailure
}

func (c *machineEnrollmentClient) Delete(context.Context, ed25519.PrivateKey, protocol.Binding) (protocol.LogoutResult, error) {
	c.deleteCalls++
	return protocol.LogoutResult{Version: protocol.V1, Complete: true}, nil
}

func machineTestController(t *testing.T, dir string, client *machineEnrollmentClient) *relayController {
	t.Helper()
	listener, localTLS, err := relayLoopback(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	cfg := config.Config{StateDir: dir, Token: strings.Repeat("x", 43), Relay: config.RelayConfig{Transport: "wss", ControlEndpoint: "https://control.example.com", ControlAudience: "control.example.com"}}
	rc, err := newRelayController(t.Context(), cfg, &transport.PublicOrigins{}, listener.Addr().String(), localTLS)
	if err != nil {
		t.Fatal(err)
	}
	rc.client = client
	return rc
}

func TestRelayMachineStableAcrossRetriesRekeyUnbindAndLostSettings(t *testing.T) {
	dir := t.TempDir()
	host := protocol.HostID("0123456789abcdef0123456789abcdef")
	client := &machineEnrollmentClient{failure: protocol.Error{Code: protocol.Unavailable}, binding: protocol.Binding{HostID: host, OwnerID: "123", BindingGeneration: 1, PublicURL: "https://" + host.Label() + ".example.com"}}
	rc := machineTestController(t, dir, client)
	defer func() { rc.Close() }()
	path := filepath.Join(dir, "relay-machine.json")
	if _, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{}); !errors.Is(err, relaysetting.ErrEnableBlocked) {
		t.Fatalf("enrollment without explicit confirmation: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unconfirmed enrollment generated identity: %v", err)
	}
	req := contract.RelayEnrollmentRequest{ConfirmPublic: true}
	if _, err := rc.Enroll(t.Context(), req); err == nil {
		t.Fatal("failed remote enrollment reported success")
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	client.failure = nil
	enrollment, err := rc.Enroll(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := rc.Cancel(t.Context(), enrollment.Handle); err != nil {
		t.Fatal(err)
	}
	enrollment, err = rc.Enroll(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	rc.nextPoll = time.Time{}
	status, err := rc.Poll(t.Context(), enrollment.Handle)
	if err != nil || !status.Paired || status.Enabled {
		t.Fatalf("pairing must not enable public Shell: paired=%v enabled=%v err=%v", status.Paired, status.Enabled, err)
	}
	oldSeed := rc.settings.Seed
	req.Rebind = true
	enrollment, err = rc.Enroll(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	client.binding.BindingGeneration++
	rc.nextPoll = time.Time{}
	if _, err := rc.Poll(t.Context(), enrollment.Handle); err != nil {
		t.Fatal(err)
	}
	if rc.settings.Seed == oldSeed || rc.settings.Config.Enabled || rc.settings.Confirmed {
		t.Fatal("rebind did not rotate key while retaining explicit enable boundary")
	}
	status, err = rc.Delete(t.Context())
	if err != nil || status.Paired || status.Enabled || client.deleteCalls != 1 || rc.settings.Seed != "" || rc.settings.PendingSeed != "" {
		t.Fatalf("unbind did not clear binding credentials: %v", err)
	}
	rc.Close()
	rc = machineTestController(t, dir, client)
	req.Rebind = false
	enrollment, err = rc.Enroll(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := rc.Cancel(t.Context(), enrollment.Handle); err != nil {
		t.Fatal(err)
	}
	rc.Close()
	if err := os.Remove(filepath.Join(dir, "relay-settings.json")); err != nil {
		t.Fatal(err)
	}
	rc = machineTestController(t, dir, client)
	if _, err := rc.Enroll(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	if len(client.calls) != 6 {
		t.Fatalf("enrollment calls=%d, want 6", len(client.calls))
	}
	seenKeys := map[string]bool{}
	for n, call := range client.calls {
		if call.id.Validate() != nil || call.id != client.calls[0].id || call.rebind != (n == 3) {
			t.Fatalf("call %d changed machine identity or replacement intent", n)
		}
		if seenKeys[call.public] {
			t.Fatal("new enrollment reused an old relay key")
		}
		seenKeys[call.public] = true
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatalf("binding lifecycle rewrote machine identity: %v", err)
	}
	body, err := json.Marshal(rc.Status())
	if err != nil || bytes.Contains(body, []byte(client.calls[0].id)) || bytes.Contains(body, []byte(rc.settings.PendingSeed)) {
		t.Fatalf("status exposed machine identity or pending seed: %v", err)
	}
}

func TestRelayMachineFailureBlocksPairingWithoutBreakingController(t *testing.T) {
	for _, mode := range []string{"malformed", "locked", "io"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "relay-machine.json")
			var original []byte
			switch mode {
			case "malformed":
				original = []byte(`{"v":1,"machineId":"broken"}`)
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
			case "locked":
				owner := relaysetting.OpenMachine(dir)
				defer owner.Close()
			case "io":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			client := &machineEnrollmentClient{}
			rc := machineTestController(t, dir, client)
			defer rc.Close()
			if !rc.Status().StorageAvailable {
				t.Fatal("machine failure broke independent relay settings")
			}
			for range 2 {
				if _, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true}); err == nil {
					t.Fatal("unavailable machine identity allowed enrollment")
				}
			}
			if len(client.calls) != 0 || rc.settings.PendingSeed != "" {
				t.Fatal("machine failure sent enrollment or replaced pending key")
			}
			if _, err := os.Stat(filepath.Join(dir, "relay-settings.json")); !os.IsNotExist(err) {
				t.Fatalf("machine failure changed independent settings: %v", err)
			}
			if mode == "malformed" {
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(original, after) {
					t.Fatalf("corrupt machine identity was overwritten: %v", err)
				}
			}
		})
	}
}

func TestRelayMachinePollRejectionAllowsImmediateRecovery(t *testing.T) {
	for _, code := range []protocol.Code{protocol.Conflict, protocol.Forbidden, protocol.Expired, protocol.Unauthenticated, protocol.Invalid, protocol.NotFound, protocol.Incompatible, protocol.Unavailable, protocol.UpstreamFailed, protocol.RateLimited} {
		t.Run(string(code), func(t *testing.T) {
			dir := t.TempDir()
			client := &machineEnrollmentClient{pollFailure: protocol.Error{Code: code}, cancelFailure: protocol.Error{Code: protocol.Unavailable}}
			rc := machineTestController(t, dir, client)
			defer rc.Close()
			enrollment, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true})
			if err != nil {
				t.Fatal(err)
			}
			seed := rc.settings.PendingSeed
			rc.nextPoll = time.Time{}
			_, err = rc.Poll(t.Context(), enrollment.Handle)
			var remote protocol.Error
			if !errors.As(err, &remote) || remote.Code != code {
				t.Fatalf("poll rejection changed: %v", err)
			}
			uncertain := code == protocol.Unavailable || code == protocol.UpstreamFailed || code == protocol.RateLimited
			if rc.Status().Pairing != uncertain || uncertain && rc.settings.PendingSeed != seed || !uncertain && rc.settings.PendingSeed != "" {
				t.Fatal("poll outcome lost the pending-key boundary")
			}
			stored, fromWeb, err := relaysetting.Inspect(dir, rc.settings.Config)
			if err != nil || !fromWeb || stored.PendingSeed != rc.settings.PendingSeed {
				t.Fatalf("pending-key transition was not persisted: %v", err)
			}
			_, err = rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true, Rebind: true})
			if uncertain {
				if !errors.As(err, &remote) || remote.Code != protocol.Conflict {
					t.Fatalf("uncertain enrollment was replaced: %v", err)
				}
			} else if err != nil {
				t.Fatalf("definitive rejection blocked explicit recovery: %v", err)
			}
		})
	}
}

func TestRelayMachineTerminalCancellationAllowsRecovery(t *testing.T) {
	for _, code := range []protocol.Code{protocol.Expired, protocol.Conflict, protocol.NotFound, protocol.Unavailable} {
		t.Run(string(code), func(t *testing.T) {
			client := &machineEnrollmentClient{cancelFailure: protocol.Error{Code: code}}
			rc := machineTestController(t, t.TempDir(), client)
			defer rc.Close()
			enrollment, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true})
			if err != nil {
				t.Fatal(err)
			}
			err = rc.Cancel(t.Context(), enrollment.Handle)
			var remote protocol.Error
			if !errors.As(err, &remote) || remote.Code != code {
				t.Fatalf("cancellation outcome changed: %v", err)
			}
			uncertain := code == protocol.Unavailable
			if rc.Status().Pairing != uncertain {
				t.Fatal("cancellation retained an unusable attempt or discarded an uncertain one")
			}
			_, err = rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true, Rebind: true})
			if uncertain {
				if !errors.As(err, &remote) || remote.Code != protocol.Conflict {
					t.Fatalf("uncertain cancellation replaced a live attempt: %v", err)
				}
			} else if err != nil {
				t.Fatalf("terminal cancellation blocked recovery: %v", err)
			}
		})
	}
}
