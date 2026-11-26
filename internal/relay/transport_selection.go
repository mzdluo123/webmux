package relay

import (
	"context"
	"errors"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// TransportSelection separates the saved preference from a verified live route.
type TransportSelection struct {
	Supported   bool                  `json:"supported"`
	Preferred   protocol.FRPTransport `json:"preferred"`
	Active      protocol.FRPTransport `json:"active,omitempty"`
	Pending     protocol.FRPTransport `json:"pending,omitempty"`
	LastAttempt *TransportAttempt     `json:"lastAttempt,omitempty"`
}

// TransportAttempt reports only redacted candidate diagnostics.
type TransportAttempt struct {
	NodeID      string                `json:"nodeId"`
	Protocol    protocol.FRPTransport `json:"protocol"`
	Stage       string                `json:"stage"`
	Error       string                `json:"error"`
	DurationMs  int64                 `json:"durationMs"`
	CompletedAt time.Time             `json:"completedAt"`
}

type transportOperation struct {
	session *connection
	ctx     context.Context
	desired protocol.FRPTransport
	started time.Time
	done    chan selectionOutcome // nil for a saved preference attempt at startup
}

type selectionOutcome struct {
	stage string
	err   error
}

func (c *Client) TransportSelection() TransportSelection {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.selection
	out.Preferred = c.preferredTransportLocked()
	if c.active == nil || c.active.dataCtx.Err() != nil {
		out.Supported, out.Active = false, ""
	} else if c.transportOp != nil && c.transportOp.session == c.active {
		out.Pending = c.transportOp.desired
	}
	if out.LastAttempt != nil {
		attempt := *out.LastAttempt
		out.LastAttempt = &attempt
	}
	return out
}

func (c *Client) preferredTransportLocked() protocol.FRPTransport {
	if c.preferred == protocol.FRPTransportQUIC {
		return c.preferred
	}
	return protocol.FRPTransportWSS
}

// SetPreferredTransport accepts the saved preference before the connector runs.
func (c *Client) SetPreferredTransport(preferred protocol.FRPTransport) {
	c.mu.Lock()
	c.preferred = preferred
	c.mu.Unlock()
}

// ApplyTransportInScope offers the requested protocol on the specified
// authenticated connector and persists it only after the node commits.
// Cancellation before commit abandons the candidate. Once commit is issued,
// its authoritative outcome decides whether persistence is needed.
func (c *Client) ApplyTransportInScope(ctx context.Context, scope protocol.Scope, desired protocol.FRPTransport, persist func() error) error {
	if !desired.Valid() || persist == nil {
		return protocol.Error{Code: protocol.Invalid}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	s := c.active
	if s == nil || s.dataCtx.Err() != nil {
		c.mu.Unlock()
		return protocol.Error{Code: protocol.Unavailable}
	}
	if s.scope != scope {
		c.mu.Unlock()
		return protocol.Error{Code: protocol.StaleGeneration}
	}
	if c.transportOp != nil && c.transportOp.session == s {
		c.mu.Unlock()
		return protocol.Error{Code: protocol.Conflict}
	}
	if desired == c.selection.Active && desired == c.preferredTransportLocked() {
		c.mu.Unlock()
		return nil
	}
	if !c.selection.Supported && desired != protocol.FRPTransportWSS {
		c.mu.Unlock()
		return protocol.Error{Code: protocol.Incompatible}
	}
	op := &transportOperation{session: s, ctx: ctx, desired: desired, started: time.Now(), done: make(chan selectionOutcome, 1)}
	c.transportOp = op
	c.mu.Unlock()

	select {
	case s.transport <- op:
	case <-ctx.Done():
		return c.recordTransportAttempt(op, selectionOutcome{stage: "prepare", err: ctx.Err()}, false)
	case <-s.dataDone:
		return c.recordTransportAttempt(op, unavailableSelection(), false)
	}
	outcome := <-op.done
	if outcome.err != nil {
		return outcome.err
	}
	// The actor can close transports immediately on revoke even if the local
	// settings write blocks. A retired generation cannot start another write.
	c.mu.Lock()
	live := c.active == s && s.dataCtx.Err() == nil && c.transportOp == op
	c.mu.Unlock()
	if !live {
		return c.recordTransportAttempt(op, unavailableSelection(), false)
	}
	if err := persist(); err != nil {
		return c.recordTransportAttempt(op, selectionOutcome{stage: "commit", err: err}, false)
	}
	return c.recordTransportAttempt(op, outcome, true)
}

func unavailableSelection() selectionOutcome {
	return selectionOutcome{stage: "prepare", err: protocol.Error{Code: protocol.Unavailable}}
}

func (s *connection) beginSavedTransport() *transportOperation {
	c := s.client
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active != s || c.preferredTransportLocked() != protocol.FRPTransportQUIC || c.transportOp != nil && c.transportOp.session == s {
		return nil
	}
	op := &transportOperation{session: s, ctx: s.dataCtx, desired: protocol.FRPTransportQUIC, started: time.Now()}
	c.transportOp = op
	return op
}

func (s *connection) setActiveTransport(p protocol.FRPTransport) {
	c := s.client
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == s && s.dataCtx.Err() == nil {
		c.selection.Active = p
	}
}

func (s *connection) finishTransportAttempt(op *transportOperation, outcome selectionOutcome) {
	if op.done == nil || outcome.err != nil {
		_ = s.client.recordTransportAttempt(op, outcome, false)
	}
	if op.done != nil {
		op.done <- outcome
	}
}

func (c *Client) recordTransportAttempt(op *transportOperation, outcome selectionOutcome, saved bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.transportOp != op {
		return outcome.err
	}
	c.transportOp = nil
	if saved {
		c.preferred = op.desired
	}
	now := time.Now()
	var failure string
	if outcome.err != nil {
		var wire protocol.Error
		switch {
		case errors.As(outcome.err, &wire):
			failure = string(wire.Code)
		case errors.Is(outcome.err, context.Canceled):
			failure = "cancelled"
		case errors.Is(outcome.err, context.DeadlineExceeded):
			failure = string(protocol.Expired)
		default:
			failure = string(protocol.Unavailable)
		}
	}
	c.selection.LastAttempt = &TransportAttempt{NodeID: string(op.session.assignment.NodeID), Protocol: op.desired, Stage: outcome.stage,
		Error: failure, CompletedAt: now.UTC(), DurationMs: now.Sub(op.started).Milliseconds()}
	return outcome.err
}
