package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/relaysetting"
)

const relayPairingLifetime = 15 * time.Minute

func activeRelayPairing(phase string) bool {
	switch phase {
	case "preparing", "waiting", "retrying", "uncertain", "cancelling":
		return true
	default:
		return false
	}
}

func safeRelayError(err error) string {
	if err == nil {
		return ""
	}
	var remote protocol.Error
	if errors.As(err, &remote) && remote.Code.Valid() {
		return string(remote.Code)
	}
	switch {
	case errors.Is(err, relaysetting.ErrEnableBlocked):
		return "enable-blocked"
	case errors.Is(err, relaysetting.ErrUnavailable), errors.Is(err, relaysetting.ErrNotPersisted):
		return "storage-unavailable"
	default:
		return "unavailable"
	}
}

func terminalRelayError(err error) bool {
	var remote protocol.Error
	if !errors.As(err, &remote) {
		return false
	}
	switch remote.Code {
	case protocol.Conflict, protocol.Forbidden, protocol.Expired, protocol.Unauthenticated, protocol.Invalid, protocol.NotFound, protocol.Incompatible:
		return true
	default:
		return false
	}
}

func (rc *relayController) Pairing() contract.RelayPairing {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.pairing
}

func (rc *relayController) Enroll(_ context.Context, req contract.RelayEnrollmentRequest) (contract.RelayPairing, error) {
	if !req.ConfirmPublic {
		return rc.Pairing(), relaysetting.ErrEnableBlocked
	}
	// Observers never wait for an admitted network request.
	rc.mu.Lock()
	active := activeRelayPairing(rc.pairing.Phase)
	rc.mu.Unlock()
	if active {
		return rc.Pairing(), nil
	}
	rc.op.Lock()
	defer rc.op.Unlock()
	rc.mu.Lock()
	next := rc.settings
	active = activeRelayPairing(rc.pairing.Phase)
	closed := rc.closed
	rc.mu.Unlock()
	if active {
		return rc.Pairing(), nil
	}
	if closed || rc.client == nil || rc.store.Unavailable() != nil {
		return rc.Pairing(), relaysetting.ErrEnableBlocked
	}
	if next.Binding != nil {
		return rc.Pairing(), protocol.Error{Code: protocol.Conflict}
	}
	machineID, err := rc.machine.ID()
	if err != nil {
		return rc.Pairing(), err
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return rc.Pairing(), errors.New("relay identity generation failed")
	}
	next.PendingSeed = base64.RawURLEncoding.EncodeToString(key.Seed())
	if err := rc.store.Save(next); err != nil {
		return rc.Pairing(), err
	}
	ctx, stop := context.WithCancel(rc.ctx)
	done := make(chan struct{})
	rc.mu.Lock()
	if rc.closed {
		rc.mu.Unlock()
		stop()
		return rc.Pairing(), relaysetting.ErrEnableBlocked
	}
	wake := make(chan struct{}, 1) // Coalesce cancellation notifications.
	rc.settings = next
	rc.pairing = contract.RelayPairing{Phase: "preparing", ExpiresAt: rc.pairingNow().Add(relayPairingLifetime).Unix()}
	rc.pairingCancel = false
	rc.pairingWake = wake
	rc.pairingDone = done
	rc.pairingStop = stop
	rc.mu.Unlock()
	go func() {
		defer close(done)
		defer stop()
		rc.runPairing(ctx, wake, key, machineID)
	}()
	return rc.Pairing(), nil
}

func waitRelayPairing(ctx context.Context, wake <-chan struct{}, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-wake:
		return true
	case <-timer.C:
		return true
	}
}

func (rc *relayController) runPairing(ctx context.Context, wake <-chan struct{}, key ed25519.PrivateKey, machineID protocol.MachineID) {
	request, cancel := context.WithTimeout(ctx, protocol.RequestTimeout)
	enrollment, err := rc.client.Enroll(request, key, machineID)
	cancel()
	rc.mu.Lock()
	if err == nil {
		rc.pairing = contract.RelayPairing{Phase: "waiting", UserCode: enrollment.UserCode, VerificationURL: enrollment.VerificationURL, ExpiresAt: enrollment.ExpiresAt, PollIntervalSeconds: enrollment.PollIntervalSeconds}
	} else {
		// An unsuccessful response does not prove creation was never accepted.
		rc.pairing.Phase = "uncertain"
		rc.pairing.Error = safeRelayError(err)
		rc.pairing.ExpiresAt = rc.pairingNow().Add(relayPairingLifetime).Unix()
	}
	cancelled := rc.pairingCancel
	if cancelled {
		rc.pairing.Phase = "cancelling"
	}
	rc.mu.Unlock()

	interval := time.Duration(enrollment.PollIntervalSeconds) * time.Second
	if interval < time.Second {
		interval = time.Second
	}
	delay := interval
	var approved *protocol.EnrollmentResult
	var terminalPhase, terminalCode string
	for {
		rc.mu.Lock()
		expiry := rc.pairing.ExpiresAt
		now := rc.pairingNow()
		if remaining := time.Unix(expiry, 0).Sub(now); remaining > 0 && delay > remaining {
			delay = remaining
		}
		rc.pairing.RetryAt = now.Add(delay).Unix()
		rc.mu.Unlock()
		if !rc.pairingWait(ctx, wake, delay) {
			return
		}
		rc.op.Lock()
		if ctx.Err() != nil {
			rc.op.Unlock()
			return
		}
		rc.mu.Lock()
		cancelled = rc.pairingCancel
		expired := rc.pairingNow().Unix() >= rc.pairing.ExpiresAt
		rc.mu.Unlock()
		if approved != nil {
			err = rc.savePairing(*approved)
		} else if terminalPhase != "" {
			if rc.finishPairing(terminalPhase, terminalCode) {
				rc.op.Unlock()
				return
			}
			err = relaysetting.ErrUnavailable
		} else if expired || cancelled && enrollment.Handle == "" {
			phase := "expired"
			if cancelled {
				phase = "cancelled"
			}
			terminalPhase = phase
			if rc.finishPairing(phase, "") {
				rc.op.Unlock()
				return
			}
			err = relaysetting.ErrUnavailable
		} else if enrollment.Handle == "" {
			// There is no recovery protocol for a lost creation response.
			// Wait for explicit cancellation or expiry; never replay creation.
			rc.op.Unlock()
			delay = time.Unix(expiry, 0).Sub(rc.pairingNow())
			continue
		} else {
			rc.op.Unlock()
			approved, err = rc.pollPairing(ctx, enrollment.Handle, cancelled)
			rc.op.Lock()
			if ctx.Err() != nil {
				rc.op.Unlock()
				return
			}
			if cancelled && approved == nil && (err == nil || terminalRelayError(err)) {
				terminalPhase = "cancelled"
				if rc.finishPairing("cancelled", "") {
					rc.op.Unlock()
					return
				}
				err = relaysetting.ErrUnavailable
			}
			if approved != nil {
				err = rc.savePairing(*approved)
			}
		}
		if err == nil && approved != nil {
			rc.op.Unlock()
			return
		}
		var remote protocol.Error
		if terminalRelayError(err) && approved == nil && !cancelled {
			phase := "failed"
			if errors.As(err, &remote) {
				switch remote.Code {
				case protocol.Expired, protocol.NotFound:
					phase = "expired"
				case protocol.Forbidden, protocol.Unauthenticated:
					phase = "denied"
				}
			}
			terminalPhase, terminalCode = phase, safeRelayError(err)
			if rc.finishPairing(phase, safeRelayError(err)) {
				rc.op.Unlock()
				return
			}
			err = relaysetting.ErrUnavailable
		}
		rc.mu.Lock()
		rc.pairing.Phase = "retrying"
		rc.pairing.Error = safeRelayError(err)
		if errors.As(err, &remote) && remote.Code == protocol.Pending {
			rc.pairing.Phase = "waiting"
			rc.pairing.Error = ""
			delay = interval
		} else {
			delay *= 2
			if delay < interval {
				delay = interval
			}
			if delay > time.Minute {
				delay = time.Minute
			}
		}
		if rc.pairingCancel {
			rc.pairing.Phase = "cancelling"
		} else if enrollment.Handle == "" {
			rc.pairing.Phase = "uncertain"
		}
		if errors.As(err, &remote) && time.Duration(remote.RetryAfterSeconds)*time.Second > delay {
			delay = time.Duration(remote.RetryAfterSeconds) * time.Second
		}
		rc.mu.Unlock()
		rc.op.Unlock()
	}
}

func (rc *relayController) pollPairing(ctx context.Context, handle protocol.Token, cancelling bool) (*protocol.EnrollmentResult, error) {
	request, cancel := context.WithTimeout(ctx, protocol.RequestTimeout)
	defer cancel()
	if cancelling {
		err := rc.client.Cancel(request, handle)
		var remote protocol.Error
		if !errors.As(err, &remote) || remote.Code != protocol.Conflict {
			return nil, err
		}
		// Approval may have won upstream before cancellation.
	}
	result, err := rc.client.Poll(request, handle)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Called only by the worker under op: late network responses cannot cross into
// a fresh attempt. Persist success before publishing paired, always disabled.
func (rc *relayController) savePairing(result protocol.EnrollmentResult) error {
	rc.mu.Lock()
	next := rc.settings
	rc.mu.Unlock()
	next.Config.Enabled = false
	next.Binding = &result.Binding
	next.OwnerLogin = result.OwnerLogin
	next.Seed = next.PendingSeed
	next.PendingSeed = ""
	next.Confirmed = false
	if err := rc.store.Save(next); err != nil {
		return err
	}
	rc.mu.Lock()
	rc.settings = next
	rc.pairing = contract.RelayPairing{Phase: "paired"}
	rc.mu.Unlock()
	return nil
}

func (rc *relayController) finishPairing(phase, code string) bool {
	rc.mu.Lock()
	next := rc.settings
	rc.mu.Unlock()
	next.PendingSeed = ""
	if err := rc.store.Save(next); err != nil {
		rc.mu.Lock()
		rc.pairing.Phase = "uncertain"
		rc.pairing.Error = "storage-unavailable"
		rc.mu.Unlock()
		return false
	}
	rc.mu.Lock()
	rc.settings = next
	rc.pairing = contract.RelayPairing{Phase: phase, Error: code}
	rc.mu.Unlock()
	return true
}

func (rc *relayController) Cancel(_ context.Context) (contract.RelayPairing, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if activeRelayPairing(rc.pairing.Phase) {
		rc.pairingCancel = true
		rc.pairing.Phase = "cancelling"
		select {
		case rc.pairingWake <- struct{}{}:
		default:
		}
	}
	return rc.pairing, nil
}

func (rc *relayController) Delete(_ context.Context) (contract.RelayStatus, error) {
	rc.op.Lock()
	defer rc.op.Unlock()
	rc.beginOperation("delete")
	rc.mu.Lock()
	next := rc.settings
	rc.mu.Unlock()
	if next.Binding == nil || rc.client == nil {
		return rc.finishOperation(protocol.Error{Code: protocol.Conflict}, false)
	}
	ctx, cancel := context.WithTimeout(rc.ctx, protocol.RequestTimeout)
	defer cancel()
	// Every admitted deletion closes this connector, including unknown remote
	// outcomes. Keep the identity until deletion itself has been confirmed.
	rc.stop(ctx)
	next.Config.Enabled = false
	next.Confirmed = false
	saveErr := rc.store.Save(next)
	key, err := relaysetting.PrivateKey(next.Seed)
	if err != nil {
		return rc.finishOperation(err, false)
	}
	result, err := rc.client.Delete(ctx, key, *next.Binding)
	if err != nil {
		if saveErr != nil {
			return rc.finishOperation(saveErr, true)
		}
		return rc.finishOperation(err, !terminalRelayError(err))
	}
	rc.mu.Lock()
	rc.status.Operation.Deleted = result.Deleted
	rc.status.Operation.AccessClosed = result.AccessClosed
	rc.mu.Unlock()
	if !result.Deleted {
		return rc.finishOperation(protocol.Error{Code: protocol.Invalid}, true)
	}
	next.Binding = nil
	next.Seed = ""
	next.PendingSeed = ""
	next.OwnerLogin = ""
	rc.mu.Lock()
	rc.settings = next
	rc.pairing = contract.RelayPairing{Phase: "idle"}
	rc.mu.Unlock()
	if err := rc.store.Save(next); err != nil {
		return rc.finishOperation(err, false)
	}
	return rc.finishOperation(nil, false)
}
