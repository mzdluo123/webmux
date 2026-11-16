package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/relaysetting"
)

func (rc *relayController) Enroll(ctx context.Context, req contract.RelayEnrollmentRequest) (protocol.Enrollment, error) {
	rc.op.Lock()
	defer rc.op.Unlock()
	if !req.ConfirmPublic || rc.client == nil || rc.store.Unavailable() != nil {
		return protocol.Enrollment{}, relaysetting.ErrEnableBlocked
	}
	rc.mu.Lock()
	next := rc.settings
	pending := rc.pendingExpires > time.Now().Unix()
	rc.mu.Unlock()
	if pending || (next.Binding != nil && !req.Rebind) {
		return protocol.Enrollment{}, protocol.Error{Code: protocol.Conflict}
	}
	machineID, err := rc.machine.ID()
	if err != nil {
		return protocol.Enrollment{}, err
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return protocol.Enrollment{}, errors.New("relay identity generation failed")
	}
	next.PendingSeed = base64.RawURLEncoding.EncodeToString(key.Seed())
	if err := rc.store.Save(next); err != nil {
		return protocol.Enrollment{}, err
	}
	rc.mu.Lock()
	rc.settings = next
	rc.mu.Unlock()
	result, err := rc.client.Enroll(ctx, key, machineID, req.Rebind)
	if err != nil {
		return protocol.Enrollment{}, err
	}
	rc.mu.Lock()
	rc.pending = sha256.Sum256([]byte(result.Handle))
	rc.pendingExpires = result.ExpiresAt
	rc.pollInterval = time.Duration(result.PollIntervalSeconds) * time.Second
	rc.nextPoll = time.Now().Add(rc.pollInterval)
	rc.mu.Unlock()
	return result, nil
}

func (rc *relayController) pollHandle(handle protocol.Token) error {
	if handle.Validate() != nil {
		return relaysetting.ErrInvalid
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.pendingExpires <= time.Now().Unix() || sha256.Sum256([]byte(handle)) != rc.pending {
		return protocol.Error{Code: protocol.Expired}
	}
	return nil
}

func (rc *relayController) Poll(ctx context.Context, handle protocol.Token) (contract.RelayStatus, error) {
	rc.op.Lock()
	defer rc.op.Unlock()
	if rc.client == nil {
		return rc.Status(), protocol.Error{Code: protocol.Unavailable}
	}
	if err := rc.pollHandle(handle); err != nil {
		return rc.Status(), err
	}
	rc.mu.Lock()
	if time.Now().Before(rc.nextPoll) {
		rc.mu.Unlock()
		return rc.Status(), protocol.Error{Code: protocol.Pending}
	}
	rc.nextPoll = time.Now().Add(rc.pollInterval)
	next := rc.settings
	rc.mu.Unlock()
	result, err := rc.client.Poll(ctx, handle)
	if err != nil {
		return rc.Status(), err
	}
	if next.Binding != nil && (result.Binding.HostID != next.Binding.HostID || result.Binding.PublicURL != next.Binding.PublicURL || result.Binding.OwnerID != next.Binding.OwnerID || result.Binding.BindingGeneration <= next.Binding.BindingGeneration) {
		return rc.Status(), protocol.Error{Code: protocol.Forbidden}
	}
	rc.stop(ctx)
	next.Config.Enabled = false
	next.Binding = &result.Binding
	next.OwnerLogin = result.OwnerLogin
	next.Seed = next.PendingSeed
	next.PendingSeed = ""
	next.Confirmed = false
	if err := rc.store.Save(next); err != nil {
		return rc.Status(), err
	}
	rc.mu.Lock()
	rc.settings = next
	rc.pendingExpires = 0
	rc.pending = [32]byte{}
	rc.mu.Unlock()
	return rc.Status(), nil
}

func (rc *relayController) Cancel(ctx context.Context, handle protocol.Token) error {
	rc.op.Lock()
	defer rc.op.Unlock()
	if err := rc.pollHandle(handle); err != nil {
		return err
	}
	if rc.client == nil {
		return protocol.Error{Code: protocol.Unavailable}
	}
	if err := rc.client.Cancel(ctx, handle); err != nil {
		return err
	}
	rc.mu.Lock()
	next := rc.settings
	rc.pendingExpires = 0
	rc.pending = [32]byte{}
	rc.mu.Unlock()
	next.PendingSeed = ""
	if err := rc.store.Save(next); err != nil {
		return err
	}
	rc.mu.Lock()
	rc.settings = next
	rc.mu.Unlock()
	return nil
}

func (rc *relayController) Delete(ctx context.Context) (contract.RelayStatus, error) {
	rc.op.Lock()
	defer rc.op.Unlock()
	rc.mu.Lock()
	next := rc.settings
	rc.mu.Unlock()
	if next.Binding == nil || rc.client == nil {
		return rc.Status(), protocol.Error{Code: protocol.Conflict}
	}
	key, err := relaysetting.PrivateKey(next.Seed)
	if err != nil {
		return rc.Status(), err
	}
	result, err := rc.client.Delete(ctx, key, *next.Binding)
	if err != nil {
		return rc.Status(), err
	}
	if !result.Complete {
		return rc.Status(), protocol.Error{Code: protocol.Pending}
	}
	rc.stop(ctx)
	next.Config.Enabled = false
	next.Confirmed = false
	next.Binding = nil
	next.Seed = ""
	next.PendingSeed = ""
	next.OwnerLogin = ""
	if err := rc.store.Save(next); err != nil {
		return rc.Status(), err
	}
	rc.mu.Lock()
	rc.settings = next
	rc.mu.Unlock()
	return rc.Status(), nil
}
