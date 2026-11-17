package main

// Pool health describes transport availability independently of the management
// stream that owns browser authorization and the public origin.
func (rc *relayController) poolsReady(gen uint64, interactive, bulk bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if gen != rc.generation {
		return
	}
	rc.status.BulkReady = bulk
	rc.status.ReadyConnections = 0
	if interactive {
		rc.status.ReadyConnections++
	}
	if bulk {
		rc.status.ReadyConnections++
	}
	rc.status.Error = ""
	rc.status.ReadinessError = ""
	switch {
	case !interactive:
		rc.status.State = "failed"
		rc.status.Error = "relay interactive connection unavailable"
		rc.status.ReadinessError = "interactive-unavailable"
	case !bulk:
		rc.status.State = "degraded"
	default:
		rc.status.State = "ready"
	}
}
