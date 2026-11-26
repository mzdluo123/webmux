package relay

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

type candidateResult struct {
	grant     *protocol.AllocationGrant
	key       *ecdsa.PrivateKey
	data      *frpRegistration
	outcome   selectionOutcome
	uncertain bool // the node may have switched without a verifiable receipt
}

func (s *connection) callTransport(ctx context.Context, request protocol.TransportRequest) (protocol.TransportResult, error) {
	id, err := newToken()
	if err != nil {
		return protocol.TransportResult{}, err
	}
	message := s.rpc.message(protocol.ControlTransportRequest, id)
	message.TransportRequest = &request
	response, err := s.rpc.call(ctx, message, protocol.ControlTransportResult, nil)
	if err != nil {
		return protocol.TransportResult{}, err
	}
	result := *response.TransportResult
	if result.OperationID != request.OperationID {
		return protocol.TransportResult{}, protocol.Error{Code: protocol.StaleGeneration}
	}
	return result, nil
}

func (s *connection) startTransportCandidate(ctx context.Context, op *transportOperation, dir, executable string, base *protocol.AllocationGrant) <-chan candidateResult {
	result := make(chan candidateResult, 1)
	go func() { result <- s.tryTransport(ctx, op, dir, executable, base) }()
	return result
}

func (s *connection) tryTransport(ctx context.Context, op *transportOperation, dir, executable string, base *protocol.AllocationGrant) (result candidateResult) {
	result.outcome.stage = "prepare"
	if !s.control.selection {
		result.outcome.err = protocol.Error{Code: protocol.Incompatible}
		return result
	}
	deadline := op.started.Add(protocol.TransportCandidateTTL)
	trial, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	stop := context.AfterFunc(op.ctx, cancel)
	defer stop()
	if err := op.ctx.Err(); err != nil {
		result.outcome.err = err
		return result
	}
	state, err := s.callTransport(trial, protocol.TransportRequest{Action: protocol.TransportStateAction})
	if err != nil {
		result.outcome.err = err
		return result
	}
	if state.State != protocol.TransportIdle || state.Grant != nil || state.Active != "" &&
		(base == nil || state.Active != base.Protocol() || state.ActiveAllocationID != base.AllocationID) {
		result.outcome.err = protocol.Error{Code: protocol.StaleGeneration}
		return result
	}
	id, err := newToken()
	if err != nil {
		result.outcome.err = err
		return result
	}
	key, csr, err := newOriginCSR()
	if err != nil {
		result.outcome.err = protocol.Error{Code: protocol.Unavailable}
		return result
	}
	operation := protocol.TransportRequest{OperationID: id, RouteVersion: state.RouteVersion}
	prepared, err := s.callTransport(trial, protocol.TransportRequest{Action: protocol.TransportPrepare, OperationID: id,
		RouteVersion: state.RouteVersion, Transport: op.desired, AllocationRequest: &protocol.AllocationRequest{HostServerCSR: csr}})
	// A lost Prepare can leave a remote registration even if no child ran.
	// Clean up by operation ID, independently of the requesting HTTP stream.
	var data *frpRegistration
	cleanupPending := true
	defer func() {
		if result.data == nil {
			if data != nil {
				data.close()
			}
			if cleanupPending && ctx.Err() == nil && !result.uncertain {
				cleanup, cancel := context.WithTimeout(ctx, protocol.RequestTimeout)
				defer cancel()
				operation.Action, operation.AllocationRenew = protocol.TransportCancel, nil
				_, _ = s.callTransport(cleanup, operation)
			}
		}
	}()
	if err != nil {
		result.outcome.err = err
		return result
	}
	g := prepared.Grant
	if prepared.State != protocol.TransportPrepared || prepared.RouteVersion != state.RouteVersion || prepared.Active != state.Active ||
		prepared.ActiveAllocationID != state.ActiveAllocationID || g == nil || g.Transport == nil || g.GrantRevision != 1 ||
		g.Transport.Protocol != op.desired || g.Transport.OperationID != id || g.Transport.RouteVersion != state.RouteVersion ||
		g.AllocationID == state.ActiveAllocationID {
		result.outcome.err = protocol.Error{Code: protocol.StaleGeneration}
		return result
	}
	if err := s.validateFRPGrant(g, nil); err != nil {
		result.outcome.err = err
		return result
	}
	if err := trial.Err(); err != nil {
		result.outcome.err = err
		return result
	}
	deadline = minTime(deadline, time.Unix(g.Transport.Deadline, 0))
	result.outcome.stage = "handshake"
	data, err = s.startRegistration(ctx, dir, executable, key, *g)
	if err != nil {
		result.outcome.err = err
		return result
	}
	if err := trial.Err(); err != nil {
		result.outcome.err = err
		return result
	}
	operation.Action = protocol.TransportCommit
	operation.AllocationRenew = &protocol.AllocationRenew{AllocationID: g.AllocationID, GrantRevision: g.GrantRevision}
	commitCtx, stopCommit := context.WithDeadline(trial, deadline)
	commit, err := s.callTransport(commitCtx, operation)
	stopCommit()
	result.outcome = transportCommitFailure(err)
	resolved := err == nil && transportTerminal(commit.State)
	if !resolved && ctx.Err() == nil {
		commit, resolved = s.resolveTransportCommit(ctx, deadline, operation)
	}
	if !resolved {
		result.uncertain = ctx.Err() == nil
		return result
	}
	cleanupPending = false
	if commit.State != protocol.TransportCommitted {
		if err == nil && commit.State == protocol.TransportExpired {
			result.outcome = transportCommitFailure(protocol.Error{Code: protocol.Expired})
		}
		return result
	}
	if commit.RouteVersion != state.RouteVersion+1 || commit.Active != op.desired || commit.ActiveAllocationID != g.AllocationID || commit.Grant != nil {
		result.outcome = selectionOutcome{stage: "commit", err: protocol.Error{Code: protocol.StaleGeneration}}
		result.uncertain = true
		return result
	}
	result.grant, result.key, result.data = g, key, data
	result.outcome = selectionOutcome{stage: "commit"}
	return result
}

func transportTerminal(state protocol.TransportState) bool {
	return state == protocol.TransportCommitted || state == protocol.TransportCancelled || state == protocol.TransportExpired || state == protocol.TransportReleased
}

// Resolve the exact operation rather than retrying commit; cancelling an
// operation still prepared is also an authoritative answer if it wins the race.
func (s *connection) resolveTransportCommit(ctx context.Context, deadline time.Time, request protocol.TransportRequest) (protocol.TransportResult, bool) {
	resolve, cancel := context.WithDeadline(ctx, maxTime(deadline, time.Now()).Add(protocol.RequestTimeout))
	defer cancel()
	request.AllocationRenew = nil
	request.Action = protocol.TransportQuery
	for resolve.Err() == nil {
		result, err := s.callTransport(resolve, request)
		if err == nil && transportTerminal(result.State) {
			return result, result.Grant == nil
		}
		if request.Action == protocol.TransportQuery {
			request.Action = protocol.TransportCancel
			continue
		}
		request.Action = protocol.TransportQuery
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-resolve.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return protocol.TransportResult{}, false
}

func transportCommitFailure(err error) selectionOutcome {
	if err == nil {
		return selectionOutcome{stage: "commit", err: protocol.Error{Code: protocol.Unavailable}}
	}
	var wire protocol.Error
	if errors.As(err, &wire) {
		switch wire.Code {
		case protocol.Expired:
			return selectionOutcome{stage: "handshake", err: err}
		case protocol.UpstreamFailed:
			return selectionOutcome{stage: "probe", err: err}
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return selectionOutcome{stage: "handshake", err: err}
	}
	return selectionOutcome{stage: "commit", err: err}
}
