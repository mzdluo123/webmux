package main

import (
	"context"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/relay"
	"github.com/rainchan/webmux/internal/relaysetting"
)

type relayTransportClient interface {
	Scope() (protocol.Scope, bool)
	TransportSelection() relay.TransportSelection
	SetPreferredTransport(protocol.FRPTransport)
	ApplyTransportInScope(context.Context, protocol.Scope, protocol.FRPTransport, func() error) error
}

var _ relayTransportClient = (*relay.Client)(nil)

func (rc *relayController) SelectTransport(ctx context.Context, desired protocol.FRPTransport) (contract.RelayStatus, error) {
	if !desired.Valid() {
		return rc.Status(), relaysetting.ErrInvalid
	}
	if !rc.transportOp.TryLock() {
		return rc.Status(), protocol.Error{Code: protocol.Conflict}
	}
	defer rc.transportOp.Unlock()
	if !rc.op.TryLock() {
		return rc.Status(), protocol.Error{Code: protocol.Conflict}
	}
	// Snapshot admission under the settings-operation lock, but do not hold
	// it across the trial: disable, delete and reconnect must be able to stop it.
	rc.mu.Lock()
	client, ok := rc.client.(relayTransportClient)
	gen, scope := rc.generation, rc.scope
	ready := !rc.closed && rc.settings.Config.Enabled && rc.status.ControlReady && rc.store.Unavailable() == nil
	rc.mu.Unlock()
	rc.op.Unlock()
	if !ok || !ready {
		return rc.Status(), protocol.Error{Code: protocol.Unavailable}
	}

	// Only management must be ready: an explicit WSS request must also work
	// when the saved QUIC preference did not bring up a data route.
	err := client.ApplyTransportInScope(ctx, scope, desired, func() error {
		// Serialize the local write with operations that save a settings copy.
		rc.op.Lock()
		defer rc.op.Unlock()
		rc.mu.Lock()
		defer rc.mu.Unlock()
		if rc.closed || gen != rc.generation || scope != rc.scope || !rc.status.ControlReady {
			return protocol.Error{Code: protocol.StaleGeneration}
		}
		if liveScope, live := client.Scope(); !live || liveScope != scope {
			return protocol.Error{Code: protocol.StaleGeneration}
		}
		next := rc.settings
		next.PreferredTransport = desired
		if err := rc.store.Save(next); err != nil {
			return err
		}
		rc.settings = next
		// A reconnect may start as soon as this callback returns.
		client.SetPreferredTransport(desired)
		return nil
	})
	return rc.Status(), err
}

// projectTransport is used with rc.mu held. The client's last attempt remains
// useful after disconnect, but its active/pending fields must match this scope.
func (rc *relayController) projectTransport() *contract.RelayTransportSelection {
	out := &contract.RelayTransportSelection{Preferred: rc.settings.DataProtocol()}
	client, ok := rc.client.(relayTransportClient)
	if !ok {
		return out
	}
	selection := client.TransportSelection()
	if scope, live := client.Scope(); live && rc.status.ControlReady && scope == rc.scope {
		out.Supported = selection.Supported
		out.Active = selection.Active
		out.Pending = selection.Pending
	}
	if attempt := selection.LastAttempt; attempt != nil {
		out.LastAttempt = &contract.RelayTransportAttempt{
			NodeID: attempt.NodeID, Protocol: attempt.Protocol, Stage: attempt.Stage, Error: attempt.Error,
			DurationMS: attempt.DurationMs, CompletedAt: attempt.CompletedAt,
		}
	}
	return out
}
