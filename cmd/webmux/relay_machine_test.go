package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/relaysetting"
	"github.com/rainchan/webmux/internal/transport"
)

type machineEnrollmentCall struct {
	id     protocol.MachineID
	public string
	ctx    context.Context
}

type machinePollReply struct {
	result protocol.EnrollmentResult
	err    error
}

type machineEnrollmentClient struct {
	relayControlClient
	enrolled       chan machineEnrollmentCall
	polled         chan struct{}
	replies        chan machinePollReply
	enrollRelease  <-chan struct{}
	failure        error
	deleteAdmitted chan context.Context
	deleteRelease  <-chan struct{}
	cancelFailure  error
	deleteFailure  error
	accessClosed   bool
	binding        protocol.Binding
}

func newMachineEnrollmentClient() *machineEnrollmentClient {
	host := protocol.HostID("0123456789abcdef0123456789abcdef")
	return &machineEnrollmentClient{
		enrolled: make(chan machineEnrollmentCall, 1),
		polled:   make(chan struct{}, 1),
		replies:  make(chan machinePollReply, 1),
		binding:  protocol.Binding{HostID: host, OwnerID: "123", BindingGeneration: 1, PublicURL: "https://" + host.Label() + ".example.com"},
	}
}

func (c *machineEnrollmentClient) Enroll(ctx context.Context, key ed25519.PrivateKey, id protocol.MachineID) (protocol.Enrollment, error) {
	c.enrolled <- machineEnrollmentCall{id: id, public: base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), ctx: ctx}
	if c.enrollRelease != nil {
		select {
		case <-ctx.Done():
			return protocol.Enrollment{}, ctx.Err()
		case <-c.enrollRelease:
		}
	}
	if c.failure != nil {
		return protocol.Enrollment{}, c.failure
	}
	return protocol.Enrollment{Version: protocol.BindingAPIVersion, Handle: protocol.Token(strings.Repeat("A", 43)), UserCode: "ABCD-EFGH", VerificationURL: "https://github.com/login/device", ExpiresAt: time.Now().Add(time.Minute).Unix(), PollIntervalSeconds: 1}, nil
}

func (c *machineEnrollmentClient) Poll(ctx context.Context, _ protocol.Token) (protocol.EnrollmentResult, error) {
	c.polled <- struct{}{}
	select {
	case <-ctx.Done():
		return protocol.EnrollmentResult{}, ctx.Err()
	case reply := <-c.replies:
		return reply.result, reply.err
	}
}

func (c *machineEnrollmentClient) Cancel(context.Context, protocol.Token) error {
	return c.cancelFailure
}

func (c *machineEnrollmentClient) Delete(ctx context.Context, _ ed25519.PrivateKey, _ protocol.Binding) (protocol.BindingDeletionResult, error) {
	if c.deleteAdmitted != nil {
		c.deleteAdmitted <- ctx
	}
	if c.deleteRelease != nil {
		select {
		case <-ctx.Done():
			return protocol.BindingDeletionResult{}, ctx.Err()
		case <-c.deleteRelease:
		}
	}
	if ctx.Err() != nil {
		return protocol.BindingDeletionResult{}, ctx.Err()
	}
	if c.deleteFailure != nil {
		return protocol.BindingDeletionResult{}, c.deleteFailure
	}
	return protocol.BindingDeletionResult{Version: protocol.BindingAPIVersion, RevocationID: protocol.Token(strings.Repeat("A", 43)), Deleted: true, AccessClosed: c.accessClosed}, nil
}

func (c *machineEnrollmentClient) Assignment(ctx context.Context, _ ed25519.PrivateKey, _ protocol.Binding) (protocol.Assignment, error) {
	<-ctx.Done()
	return protocol.Assignment{}, ctx.Err()
}

type machinePairingWait struct {
	delay   time.Duration
	release chan struct{}
}

type machinePairingClock struct {
	mu    sync.Mutex
	now   time.Time
	waits chan machinePairingWait
}

func (c *machinePairingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *machinePairingClock) Wait(ctx context.Context, wake <-chan struct{}, delay time.Duration) bool {
	wait := machinePairingWait{delay: delay, release: make(chan struct{})}
	select {
	case c.waits <- wait:
	case <-ctx.Done():
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-wake:
		return true
	case <-wait.release:
		return true
	}
}

func (c *machinePairingClock) advance(wait machinePairingWait, delta time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(delta)
	c.mu.Unlock()
	close(wait.release)
}

func machineReceive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("host pairing worker did not reach expected checkpoint")
		var zero T
		return zero
	}
}

func machineTestController(t *testing.T, dir string, client *machineEnrollmentClient) (*relayController, *machinePairingClock) {
	t.Helper()
	cfg := config.Config{StateDir: dir, Token: strings.Repeat("x", 43), Relay: config.RelayConfig{Transport: "wss", ControlEndpoint: "https://control.example.com", ControlAudience: "control.example.com"}}
	rc, err := newRelayController(t.Context(), cfg, &transport.PublicOrigins{})
	if err != nil {
		t.Fatal(err)
	}
	clock := &machinePairingClock{now: time.Now(), waits: make(chan machinePairingWait, 1)}
	rc.client = client
	rc.pairingNow = clock.Now
	rc.pairingWait = clock.Wait
	return rc, clock
}

func machineStart(t *testing.T, rc *relayController, client *machineEnrollmentClient, clock *machinePairingClock) (machineEnrollmentCall, machinePairingWait) {
	t.Helper()
	if _, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true}); err != nil {
		t.Fatal(err)
	}
	return machineReceive(t, client.enrolled), machineReceive(t, clock.waits)
}

func machineApprove(t *testing.T, rc *relayController, client *machineEnrollmentClient, clock *machinePairingClock, wait machinePairingWait) {
	t.Helper()
	client.replies <- machinePollReply{result: protocol.EnrollmentResult{Version: protocol.BindingAPIVersion, Binding: client.binding, OwnerLogin: "owner"}}
	clock.advance(wait, wait.delay)
	machineReceive(t, client.polled)
	machineReceive(t, rc.pairingDone)
	if state := rc.Status(); !state.Paired || state.Enabled || state.Pairing || rc.Pairing().Phase != "paired" {
		t.Fatalf("pairing must save identity without enabling public Shell: %+v", state)
	}
}

func TestRelayPairingDuplicateStartsSurviveRequestCancellation(t *testing.T) {
	client := newMachineEnrollmentClient()
	release := make(chan struct{})
	client.enrollRelease = release
	rc, clock := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	ctx, cancel := context.WithCancel(t.Context())
	if flow, err := rc.Enroll(ctx, contract.RelayEnrollmentRequest{ConfirmPublic: true}); err != nil || flow.Phase != "preparing" {
		t.Fatalf("start=%+v %v", flow, err)
	}
	call := machineReceive(t, client.enrolled)
	cancel()
	if call.ctx.Err() != nil {
		t.Fatal("browser cancellation reached admitted upstream request")
	}
	var starts sync.WaitGroup
	for range 8 {
		starts.Go(func() {
			if flow, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true}); err != nil || flow.Phase != "preparing" {
				t.Errorf("duplicate start=%+v %v", flow, err)
			}
		})
	}
	starts.Wait()
	close(release)
	wait := machineReceive(t, clock.waits)
	if flow := rc.Pairing(); flow.UserCode != "ABCD-EFGH" || flow.VerificationURL != "https://github.com/login/device" {
		t.Fatalf("observer did not resume original flow: %+v", flow)
	}
	select {
	case <-client.enrolled:
		t.Fatal("duplicate start created another upstream enrollment")
	default:
	}
	machineApprove(t, rc, client, clock, wait)
}

func TestRelayPairingTransientPollsContinueAndClearErrors(t *testing.T) {
	client := newMachineEnrollmentClient()
	rc, clock := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	_, wait := machineStart(t, rc, client, clock)
	seed := rc.settings.PendingSeed
	for _, reply := range []machinePollReply{
		{err: protocol.Error{Code: protocol.Unavailable}},
		{err: protocol.Error{Code: protocol.RateLimited, RetryAfterSeconds: 9}},
		{err: protocol.Error{Code: protocol.Pending}},
	} {
		client.replies <- reply
		clock.advance(wait, wait.delay)
		machineReceive(t, client.polled)
		wait = machineReceive(t, clock.waits)
		flow := rc.Pairing()
		if flow.UserCode != "ABCD-EFGH" || !rc.Status().Pairing || rc.settings.PendingSeed != seed {
			t.Fatalf("transient error lost live authorization: %+v", flow)
		}
		var remote protocol.Error
		if errors.As(reply.err, &remote) && remote.Code == protocol.Pending {
			if flow.Error != "" || flow.Phase != "waiting" || wait.delay != time.Second {
				t.Fatalf("successful pending check retained old error: %+v", flow)
			}
		} else if flow.Phase != "retrying" || flow.Error == "" {
			t.Fatalf("transient check did not retain retry state: %+v", flow)
		}
		if remote.Code == protocol.RateLimited && wait.delay != 9*time.Second {
			t.Fatalf("Retry-After ignored: %v", wait.delay)
		}
	}
	machineApprove(t, rc, client, clock, wait)
	if rc.Pairing().Error != "" {
		t.Fatal("approval retained stale failure")
	}
}

func TestRelayPairingTerminalPollsCleanSeed(t *testing.T) {
	tests := []struct {
		code  protocol.Code
		phase string
	}{
		{protocol.Forbidden, "denied"},
		{protocol.Expired, "expired"},
		{protocol.NotFound, "expired"},
		{protocol.Invalid, "failed"},
	}
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			dir := t.TempDir()
			client := newMachineEnrollmentClient()
			rc, clock := machineTestController(t, dir, client)
			defer rc.Close()
			_, wait := machineStart(t, rc, client, clock)
			client.replies <- machinePollReply{err: protocol.Error{Code: tt.code}}
			clock.advance(wait, wait.delay)
			machineReceive(t, client.polled)
			machineReceive(t, rc.pairingDone)
			if rc.Pairing().Phase != tt.phase || rc.Status().Pairing {
				t.Fatalf("terminal state=%+v", rc.Pairing())
			}
			stored, _, err := relaysetting.Inspect(dir, rc.settings.Config)
			if err != nil || stored.PendingSeed != "" {
				t.Fatalf("terminal seed cleanup failed: %v", err)
			}
			_, next := machineStart(t, rc, client, clock)
			machineApprove(t, rc, client, clock, next)
		})
	}
}

func TestRelayPairingExpiresWithoutAnotherPoll(t *testing.T) {
	client := newMachineEnrollmentClient()
	rc, clock := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	_, wait := machineStart(t, rc, client, clock)
	clock.advance(wait, 2*time.Minute)
	machineReceive(t, rc.pairingDone)
	if rc.Pairing().Phase != "expired" || rc.settings.PendingSeed != "" {
		t.Fatalf("expired attempt retained seed: %+v", rc.Pairing())
	}
	select {
	case <-client.polled:
		t.Fatal("expired authorization was polled")
	default:
	}
}

func TestRelayPairingUnknownCreationIsNotReplayed(t *testing.T) {
	client := newMachineEnrollmentClient()
	client.failure = errors.New("private upstream URL and token")
	rc, clock := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	_, wait := machineStart(t, rc, client, clock)
	if flow := rc.Pairing(); flow.Phase != "uncertain" || flow.Error != "unavailable" {
		t.Fatalf("unknown creation misreported: %+v", flow)
	}
	seed := rc.settings.PendingSeed
	if _, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true}); err != nil {
		t.Fatal(err)
	}
	clock.advance(wait, wait.delay)
	machineReceive(t, clock.waits)
	if rc.settings.PendingSeed != seed {
		t.Fatal("unknown creation rotated key")
	}
	select {
	case <-client.enrolled:
		t.Fatal("unknown creation was automatically replayed")
	default:
	}
	if _, err := rc.Cancel(t.Context()); err != nil {
		t.Fatal(err)
	}
	machineReceive(t, rc.pairingDone)
	if rc.Pairing().Phase != "cancelled" || rc.settings.PendingSeed != "" {
		t.Fatal("unknown creation could not be cancelled locally")
	}
}

func TestRelayPairingCancellationSerializesLateSuccess(t *testing.T) {
	client := newMachineEnrollmentClient()
	rc, clock := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	_, wait := machineStart(t, rc, client, clock)
	clock.advance(wait, wait.delay)
	machineReceive(t, client.polled)
	if _, err := rc.Cancel(t.Context()); err != nil {
		t.Fatal(err)
	}
	client.replies <- machinePollReply{result: protocol.EnrollmentResult{Binding: client.binding, OwnerLogin: "owner"}}
	machineReceive(t, rc.pairingDone)
	if !rc.Status().Paired || rc.Pairing().Phase != "paired" {
		t.Fatal("cancel discarded authoritative late approval")
	}
}

func TestRelayPairingCancellationDuringCreationDoesNotReachNewAttempt(t *testing.T) {
	client := newMachineEnrollmentClient()
	release := make(chan struct{})
	client.enrollRelease = release
	rc, clock := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	if _, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true}); err != nil {
		t.Fatal(err)
	}
	first := machineReceive(t, client.enrolled)
	if _, err := rc.Cancel(t.Context()); err != nil {
		t.Fatal(err)
	}
	close(release)
	machineReceive(t, clock.waits)
	machineReceive(t, rc.pairingDone)
	if rc.Pairing().Phase != "cancelled" || rc.Status().Paired {
		t.Fatal("creation cancellation did not clean pending attempt")
	}
	second, wait := machineStart(t, rc, client, clock)
	if first.public == second.public || first.id != second.id {
		t.Fatal("fresh attempt did not preserve machine ID and rotate binding key")
	}
	machineApprove(t, rc, client, clock, wait)
}

func TestRelayPairingRestartReportsInterruption(t *testing.T) {
	dir := t.TempDir()
	client := newMachineEnrollmentClient()
	rc, clock := machineTestController(t, dir, client)
	first, _ := machineStart(t, rc, client, clock)
	rc.Close()
	rc, clock = machineTestController(t, dir, client)
	defer rc.Close()
	if flow := rc.Pairing(); flow.Phase != "interrupted" || rc.settings.PendingSeed != "" {
		t.Fatalf("restart pretended to recover device context: %+v", flow)
	}
	stored, _, err := relaysetting.Inspect(dir, rc.settings.Config)
	if err != nil || stored.PendingSeed != "" {
		t.Fatalf("orphan seed was not durably cleaned: %v", err)
	}
	second, wait := machineStart(t, rc, client, clock)
	if first.id != second.id || first.public == second.public {
		t.Fatal("restart changed machine ID or reused orphan binding key")
	}
	machineApprove(t, rc, client, clock, wait)
}

func TestRelayMachineStableAcrossFreshPairingDeletionAndLostSettings(t *testing.T) {
	dir := t.TempDir()
	client := newMachineEnrollmentClient()
	rc, clock := machineTestController(t, dir, client)
	if _, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{}); !errors.Is(err, relaysetting.ErrEnableBlocked) {
		t.Fatalf("unconfirmed enrollment=%v", err)
	}
	path := filepath.Join(dir, "relay-machine.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unconfirmed enrollment generated machine ID: %v", err)
	}
	first, wait := machineStart(t, rc, client, clock)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	machineApprove(t, rc, client, clock, wait)
	if _, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true}); err == nil {
		t.Fatal("existing binding was replaced without deletion")
	}
	if _, err := rc.Delete(t.Context()); err != nil {
		t.Fatal(err)
	}
	rc.Close()
	if err := os.Remove(filepath.Join(dir, "relay-settings.json")); err != nil {
		t.Fatal(err)
	}
	rc, clock = machineTestController(t, dir, client)
	defer rc.Close()
	second, wait := machineStart(t, rc, client, clock)
	if first.id != second.id || first.public == second.public {
		t.Fatal("lost settings changed machine ID or reused binding key")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatalf("binding lifecycle rewrote machine ID: %v", err)
	}
	body, err := json.Marshal(rc.Status())
	if err != nil || bytes.Contains(body, []byte(first.id)) || bytes.Contains(body, []byte(rc.settings.PendingSeed)) || bytes.Contains(body, []byte("ABCD-EFGH")) || bytes.Contains(body, []byte(strings.Repeat("A", 43))) {
		t.Fatalf("public status exposed pairing material: %v", err)
	}
	machineApprove(t, rc, client, clock, wait)
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
			client := newMachineEnrollmentClient()
			rc, _ := machineTestController(t, dir, client)
			defer rc.Close()
			if !rc.Status().StorageAvailable {
				t.Fatal("machine failure broke independent relay settings")
			}
			for range 2 {
				if _, err := rc.Enroll(t.Context(), contract.RelayEnrollmentRequest{ConfirmPublic: true}); err == nil {
					t.Fatal("unavailable machine identity allowed enrollment")
				}
			}
			if rc.settings.PendingSeed != "" {
				t.Fatal("machine failure replaced pending key")
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

func TestRelayDeleteStopsOwnAccessWithoutRemoteClosureProof(t *testing.T) {
	for _, accessClosed := range []bool{false, true} {
		t.Run(fmt.Sprint(accessClosed), func(t *testing.T) {
			client := newMachineEnrollmentClient()
			client.accessClosed = accessClosed
			rc, clock := machineTestController(t, t.TempDir(), client)
			defer rc.Close()
			_, wait := machineStart(t, rc, client, clock)
			machineApprove(t, rc, client, clock, wait)
			stopped := false
			rc.cancel = func() { stopped = true }
			rc.settings.Config.Enabled = true
			rc.settings.Confirmed = true
			rc.status.ControlReady = true
			if err := rc.public.Set("relay", client.binding.PublicURL); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			status, err := rc.Delete(ctx)
			if err != nil || status.Paired || status.Enabled || status.ControlReady || !stopped || rc.public.SourceOrigin("relay") != "" {
				t.Fatalf("deletion retained local access: %+v %v", status, err)
			}
			if receipt := status.Operation; receipt.Kind != "delete" || receipt.Phase != "completed" || !receipt.Deleted || receipt.AccessClosed != accessClosed || receipt.Error != "" {
				t.Fatalf("deletion lost independent closure evidence: %+v", receipt)
			}
			_, next := machineStart(t, rc, client, clock)
			machineApprove(t, rc, client, clock, next)
		})
	}
}

func TestRelayDeleteFailureStopsConnectorWithoutDiscardingBinding(t *testing.T) {
	for _, code := range []protocol.Code{protocol.NotFound, protocol.Unauthenticated, protocol.Forbidden, protocol.Unavailable} {
		t.Run(string(code), func(t *testing.T) {
			client := newMachineEnrollmentClient()
			client.deleteFailure = protocol.Error{Code: code}
			dir := t.TempDir()
			rc, clock := machineTestController(t, dir, client)
			defer rc.Close()
			_, wait := machineStart(t, rc, client, clock)
			machineApprove(t, rc, client, clock, wait)
			seed := rc.settings.Seed
			rc.settings.Config.Enabled = true
			rc.status.ControlReady = true
			status, err := rc.Delete(t.Context())
			var remote protocol.Error
			if !errors.As(err, &remote) || remote.Code != code || !status.Paired || rc.settings.Seed != seed || status.Enabled || status.ControlReady {
				t.Fatalf("unproven deletion lost identity or retained connector: %+v %v", status, err)
			}
			want := "failed"
			if code == protocol.Unavailable {
				want = "uncertain"
			}
			if receipt := rc.Status().Operation; receipt.Phase != want || receipt.Deleted || receipt.AccessClosed {
				t.Fatalf("unproven deletion receipt=%+v", receipt)
			}
			stored, _, err := relaysetting.Inspect(dir, rc.settings.Config)
			if err != nil || stored.Config.Enabled || stored.Binding == nil {
				t.Fatalf("failed deletion did not persist safe disabled identity: %v", err)
			}
		})
	}
}

func TestRelayDeleteForgetsCredentialsWhenLocalPersistenceFails(t *testing.T) {
	client := newMachineEnrollmentClient()
	rc, _ := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	rc.settings.Binding = &client.binding
	rc.settings.Seed = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	rc.settings.Config.Enabled = true
	rc.settings.Confirmed = true
	if err := rc.store.Save(rc.settings); err != nil {
		t.Fatal(err)
	}
	rc.store.Close()
	status, err := rc.Delete(t.Context())
	if !errors.Is(err, relaysetting.ErrUnavailable) || status.Enabled || status.Paired || status.Pairing || rc.settings.Seed != "" || !status.Operation.Deleted {
		t.Fatalf("storage failure retained deleted identity or lost deletion receipt: %+v %v", status, err)
	}
}

func TestRelayPairingCancellationRetriesUntilExpiry(t *testing.T) {
	client := newMachineEnrollmentClient()
	client.cancelFailure = protocol.Error{Code: protocol.Unavailable}
	rc, clock := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	machineStart(t, rc, client, clock)
	if _, err := rc.Cancel(t.Context()); err != nil {
		t.Fatal(err)
	}
	wait := machineReceive(t, clock.waits)
	flow := rc.Pairing()
	if flow.Phase != "cancelling" || flow.Error != "unavailable" || !rc.Status().Pairing || wait.delay < time.Second {
		t.Fatalf("uncertain cancellation lost retry or busy-looped: %+v delay=%v", flow, wait.delay)
	}
	clock.advance(wait, 2*time.Minute)
	machineReceive(t, rc.pairingDone)
	if rc.Pairing().Phase != "cancelled" || rc.settings.PendingSeed != "" {
		t.Fatal("expired cancellation retained a pending seed")
	}
}

func TestRelayPairingCancelConflictReconcilesApprovedBinding(t *testing.T) {
	client := newMachineEnrollmentClient()
	client.cancelFailure = protocol.Error{Code: protocol.Conflict}
	rc, clock := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	machineStart(t, rc, client, clock)
	client.replies <- machinePollReply{result: protocol.EnrollmentResult{Binding: client.binding, OwnerLogin: "owner"}}
	if _, err := rc.Cancel(t.Context()); err != nil {
		t.Fatal(err)
	}
	machineReceive(t, client.polled)
	machineReceive(t, rc.pairingDone)
	if !rc.Status().Paired || rc.Pairing().Phase != "paired" || rc.Pairing().Error != "" {
		t.Fatal("cancel conflict did not reconcile already approved binding")
	}
}

func TestRelayOperationReceiptsSurviveRequestLossAndClearOldErrors(t *testing.T) {
	client := newMachineEnrollmentClient()
	rc, clock := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	on, off := true, false
	if state, err := rc.Update(t.Context(), contract.RelayUpdate{Enabled: &on}); err == nil || state.Operation.Kind != "enable" || state.Operation.Phase != "failed" || state.Operation.Error != "enable-blocked" {
		t.Fatalf("rejected enable receipt=%+v %v", state.Operation, err)
	}
	_, wait := machineStart(t, rc, client, clock)
	machineApprove(t, rc, client, clock, wait)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if state, err := rc.Update(ctx, contract.RelayUpdate{Enabled: &on, ConfirmPublic: true}); err != nil || !state.Enabled || state.Operation.Kind != "enable" || state.Operation.Phase != "completed" || state.Operation.Error != "" {
		t.Fatalf("admitted enable was tied to lost browser request: %+v %v", state.Operation, err)
	}
	if state, err := rc.Reconnect(ctx); err != nil || state.Operation.Kind != "reconnect" || state.Operation.Phase != "completed" || state.Operation.Error != "" {
		t.Fatalf("reconnect receipt=%+v %v", state.Operation, err)
	}
	if state, err := rc.Update(ctx, contract.RelayUpdate{Enabled: &off}); err != nil || state.Enabled || state.Operation.Kind != "disable" || state.Operation.Phase != "completed" || state.Operation.Error != "" {
		t.Fatalf("disable receipt=%+v %v", state.Operation, err)
	}
	if receipt := rc.Status().Operation; receipt.Kind != "disable" || receipt.Phase != "completed" || receipt.Error != "" {
		t.Fatalf("GET reconciliation retained stale error: %+v", receipt)
	}
}

func TestRelayDeletePublishesSubmittingReceiptBeforeRequestLoss(t *testing.T) {
	client := newMachineEnrollmentClient()
	client.deleteAdmitted = make(chan context.Context, 1)
	release := make(chan struct{})
	client.deleteRelease = release
	rc, clock := machineTestController(t, t.TempDir(), client)
	defer rc.Close()
	_, wait := machineStart(t, rc, client, clock)
	machineApprove(t, rc, client, clock, wait)
	ctx, cancel := context.WithCancel(t.Context())
	type outcome struct {
		state contract.RelayStatus
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		state, err := rc.Delete(ctx)
		done <- outcome{state: state, err: err}
	}()
	upstream := machineReceive(t, client.deleteAdmitted)
	cancel()
	if upstream.Err() != nil {
		t.Fatal("request loss cancelled admitted deletion")
	}
	if receipt := rc.Status().Operation; receipt.Kind != "delete" || receipt.Phase != "submitting" || receipt.Deleted {
		t.Fatalf("in-flight deletion has no authoritative receipt: %+v", receipt)
	}
	close(release)
	result := machineReceive(t, done)
	if result.err != nil || result.state.Paired || result.state.Operation.Phase != "completed" || !rc.Status().Operation.Deleted {
		t.Fatalf("request loss discarded deletion outcome: %+v %v", result.state, result.err)
	}
}
