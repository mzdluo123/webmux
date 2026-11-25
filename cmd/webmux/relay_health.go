package main

import (
	"context"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// Only a current authenticated origin probe may publish the bound HTTPS origin.
func (rc *relayController) dataReady(ctx context.Context, gen uint64, ready bool, reason protocol.Code) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if gen != rc.generation || ctx.Err() != nil || !rc.status.ControlReady || rc.settings.Binding == nil {
		return
	}
	if ready && rc.public.Set("relay", rc.settings.Binding.PublicURL) == nil {
		rc.status.DataReady = true
		rc.status.PublicURL = rc.settings.Binding.PublicURL
		rc.status.State = "ready"
		rc.status.DataReachability = "reachable"
		rc.status.AppliedRevision = rc.status.ConfigRevision
		rc.status.Error = ""
		rc.status.ReadinessError = ""
		return
	}
	rc.public.Clear("relay")
	rc.status.DataReady = false
	rc.status.PublicURL = ""
	rc.status.State = "failed"
	rc.status.DataReachability = "unavailable"
	switch reason {
	case protocol.InvalidConfig:
		rc.status.Error = "relay transport configuration unavailable"
		rc.status.ReadinessError = string(protocol.InvalidConfig)
	case protocol.Unauthenticated:
		rc.status.Error = "relay data service identity rejected"
		rc.status.ReadinessError = "data-identity-rejected"
	case protocol.UpstreamFailed:
		rc.status.Error = "relay origin probe failed"
		rc.status.ReadinessError = "data-probe-failed"
	default:
		rc.status.Error = "relay origin unavailable"
		rc.status.ReadinessError = "data-unavailable"
	}
}
