package main

import (
	"context"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/relay"
)

// dataTransport registers or clears the source for the authenticated allocation.
func (rc *relayController) dataTransport(ctx context.Context, gen uint64, scope protocol.Scope, ready bool, reason protocol.Code) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if !rc.liveDataScope(ctx, gen, scope) || rc.settings.Binding == nil {
		return
	}
	if ready && rc.public.Set("relay", rc.settings.Binding.PublicURL) == nil {
		rc.status.PublicURL = rc.settings.Binding.PublicURL
		rc.status.State = "connecting"
		rc.status.AppliedRevision = rc.status.ConfigRevision
		rc.status.Error = ""
		rc.status.ReadinessError = ""
		return
	}
	rc.public.Clear("relay")
	rc.status.PublicURL = ""
	rc.status.State = "failed"
	rc.observation.ValidUntil = time.Time{}
	switch reason {
	case protocol.InvalidConfig:
		rc.status.Error = "relay transport configuration unavailable"
		rc.status.ReadinessError = string(protocol.InvalidConfig)
	case protocol.Unauthenticated:
		rc.status.Error = "relay data service identity rejected"
		rc.status.ReadinessError = "data-identity-rejected"
	default:
		rc.status.Error = "relay origin unavailable"
		rc.status.ReadinessError = "data-unavailable"
	}
}

func (rc *relayController) dataObserved(ctx context.Context, gen uint64, scope protocol.Scope, observation relay.DataObservation) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if !rc.liveDataScope(ctx, gen, scope) {
		return
	}
	if observation.RouteChanged {
		rc.observation = relay.DataObservation{}
	} else if rc.status.PublicURL == "" {
		return
	} else if observation.CheckedAt.IsZero() {
		rc.observation.ValidUntil = time.Time{}
	} else if observation.CheckedAt.After(rc.observation.CheckedAt) {
		rc.observation = observation
	}
}

// liveDataScope and resetData are used with rc.mu held.
func (rc *relayController) liveDataScope(ctx context.Context, gen uint64, scope protocol.Scope) bool {
	return gen == rc.generation && ctx.Err() == nil && rc.status.ControlReady && scope == rc.scope
}

func (rc *relayController) resetData() {
	rc.public.Clear("relay")
	rc.status.PublicURL = ""
	rc.observation = relay.DataObservation{}
}

func (rc *relayController) projectData(s *contract.RelayStatus) {
	s.DataReady = false
	s.DataReachability = "unknown"
	observation := rc.observation
	if observation.CheckedAt.IsZero() {
		return
	}
	s.DataCheckedAt = &observation.CheckedAt
	if !s.ControlReady || s.PublicURL == "" || !rc.clock().Before(observation.ValidUntil) {
		return
	}
	if observation.Ready {
		s.DataReady = true
		s.DataReachability = "reachable"
		s.State = "ready"
		return
	}
	s.DataReachability = "unavailable"
	s.State = "failed"
	s.Error = "relay origin probe failed"
	s.ReadinessError = "data-probe-failed"
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
