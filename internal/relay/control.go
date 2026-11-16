package relay

import (
	"context"
	"sync"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/xtaci/smux"
)

// The writer queue and pending map share the protocol's fixed budget. A slow
// control peer cannot turn user requests or ping traffic into unbounded memory.
const controlQueueSize = protocol.MaxPendingRequests

type outbound struct {
	message protocol.ControlMessage
	ctx     context.Context
	done    chan error
}

type pendingCall struct {
	want   protocol.ControlType
	ice    *protocol.ICERequest
	result chan protocol.ControlMessage
}

type controlRPC struct {
	session         *connection
	stream          *smux.Stream
	writes          chan outbound
	heartbeatCtx    context.Context
	heartbeatCancel context.CancelFunc
	heartbeatDone   chan struct{}
	mu              sync.Mutex
	pending         map[protocol.Token]pendingCall
	pings           map[protocol.Token]time.Time
	lastPong        time.Time
}

func newControlRPC(s *connection, stream *smux.Stream) *controlRPC {
	ctx, cancel := context.WithCancel(s.ctx)
	return &controlRPC{session: s, stream: stream, writes: make(chan outbound, controlQueueSize), heartbeatCtx: ctx, heartbeatCancel: cancel, heartbeatDone: make(chan struct{}), pending: make(map[protocol.Token]pendingCall), pings: make(map[protocol.Token]time.Time), lastPong: time.Now()}
}

func (r *controlRPC) start() {
	r.session.wg.Add(3)
	go func() { defer r.session.wg.Done(); r.writeLoop() }()
	go func() { defer r.session.wg.Done(); r.readLoop() }()
	go func() { defer r.session.wg.Done(); r.heartbeat() }()
}

func (r *controlRPC) message(kind protocol.ControlType, id protocol.Token) protocol.ControlMessage {
	return protocol.ControlMessage{Version: protocol.V1, Type: kind, RequestID: id, Scope: r.session.scope, ExpiresAt: time.Now().Add(protocol.ControlMessageTTL).Unix()}
}

func (r *controlRPC) send(ctx context.Context, message protocol.ControlMessage) error {
	if err := message.Validate(r.session.scope, time.Now()); err != nil {
		return err
	}
	item := outbound{message: message, ctx: ctx, done: make(chan error, 1)}
	select {
	case <-r.session.ctx.Done():
		return protocol.Error{Code: protocol.Unavailable}
	case <-ctx.Done():
		return ctx.Err()
	case r.writes <- item:
	default:
		return protocol.Error{Code: protocol.RateLimited}
	}
	select {
	case <-r.session.ctx.Done():
		return protocol.Error{Code: protocol.Unavailable}
	case <-ctx.Done():
		return ctx.Err()
	case err := <-item.done:
		return err
	}
}

func (r *controlRPC) writeLoop() {
	for {
		select {
		case <-r.session.ctx.Done():
			return
		case item := <-r.writes:
			if err := item.ctx.Err(); err != nil {
				item.done <- err
				continue
			}
			if err := item.message.Validate(r.session.scope, time.Now()); err != nil {
				item.done <- err
				continue
			}
			err := r.stream.SetWriteDeadline(time.Unix(item.message.ExpiresAt, 0))
			if err == nil {
				err = protocol.WriteFrame(r.stream, item.message, protocol.MaxControlBytes)
			}
			if err != nil {
				item.done <- protocol.Error{Code: protocol.Unavailable}
				r.session.stop(protocol.Error{Code: protocol.Unavailable})
				return
			}
			item.done <- nil
		}
	}
}

func (r *controlRPC) readLoop() {
	for {
		var message protocol.ControlMessage
		if err := protocol.ReadFrame(r.stream, &message, protocol.MaxControlBytes); err != nil {
			r.session.stop(protocol.Error{Code: protocol.Unavailable})
			return
		}
		if err := message.Validate(r.session.scope, time.Now()); err != nil {
			r.session.stop(err)
			return
		}
		switch message.Type {
		case protocol.ControlPing:
			if err := r.send(r.session.ctx, r.message(protocol.ControlPong, message.RequestID)); err != nil {
				r.session.stop(err)
				return
			}
		case protocol.ControlPong:
			r.mu.Lock()
			sent, found := r.pings[message.RequestID]
			delete(r.pings, message.RequestID)
			if found && time.Since(sent) <= protocol.ControlLease {
				r.lastPong = time.Now()
			}
			r.mu.Unlock()
			if !found {
				r.session.stop(protocol.Error{Code: protocol.Invalid})
				return
			}
		case protocol.ControlRevoke:
			if err := r.revoke(message); err != nil {
				r.session.stop(err)
				return
			}
		case protocol.ControlGrant, protocol.ControlICEConfig, protocol.ControlFailure:
			r.mu.Lock()
			pending, found := r.pending[message.RequestID]
			if found {
				delete(r.pending, message.RequestID)
			}
			r.mu.Unlock()
			// A timed-out non-idempotent operation is never replayed. Its late
			// result must not be delivered to another request or revive a session.
			if !found {
				continue
			}
			if err := r.validateReply(message, pending); err != nil {
				r.session.stop(err)
				return
			}
			pending.result <- message
		default:
			r.session.stop(protocol.Error{Code: protocol.Invalid})
			return
		}
	}
}

func (r *controlRPC) validateReply(m protocol.ControlMessage, pending pendingCall) error {
	if m.Type == protocol.ControlFailure {
		return nil
	}
	if m.Type != pending.want {
		return protocol.Error{Code: protocol.Invalid}
	}
	if m.Grant != nil && (m.Grant.OwnerID != r.session.assignment.Binding.OwnerID || m.Grant.Origin != r.session.assignment.Binding.PublicURL) {
		return protocol.Error{Code: protocol.Forbidden}
	}
	if m.ICEConfig != nil && (pending.ice == nil || m.ICEConfig.GrantID != pending.ice.GrantID || m.ICEConfig.PeerConnectionID != pending.ice.PeerConnectionID) {
		return protocol.Error{Code: protocol.Forbidden}
	}
	return nil
}

func (r *controlRPC) heartbeat() {
	defer close(r.heartbeatDone)
	defer r.heartbeatCancel()
	ticker := time.NewTicker(protocol.KeepAliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.heartbeatCtx.Done():
			return
		case now := <-ticker.C:
			r.mu.Lock()
			expired := now.Sub(r.lastPong) >= protocol.ControlLease
			for id, sent := range r.pings {
				if now.Sub(sent) >= protocol.ControlLease {
					delete(r.pings, id)
				}
			}
			r.mu.Unlock()
			if expired {
				r.session.stop(protocol.Error{Code: protocol.Expired})
				return
			}
			id, err := newToken()
			if err != nil {
				r.session.stop(err)
				return
			}
			r.mu.Lock()
			r.pings[id] = now
			r.mu.Unlock()
			if err := r.send(r.heartbeatCtx, r.message(protocol.ControlPing, id)); err != nil {
				if r.heartbeatCtx.Err() != nil {
					return
				}
				r.session.stop(err)
				return
			}
		}
	}
}

func (r *controlRPC) revoke(m protocol.ControlMessage) error {
	revocation := *m.Revocation
	if revocation.Scope == "connector" && revocation.ConnectorGeneration != r.session.scope.ConnectorGeneration {
		return protocol.Error{Code: protocol.StaleGeneration}
	}
	whole := revocation.Scope == "binding" || revocation.Scope == "connector"
	if whole {
		// Deny new streams before closing access or invoking consumer callbacks.
		r.session.closeHTTP()
		// Once access is denied, stop and join heartbeat writes before the
		// final ACK/FIN. No ping may write to the half-closed management stream.
		r.heartbeatCancel()
		<-r.heartbeatDone
	}
	ctx, cancel := context.WithDeadline(r.session.ctx, time.Unix(m.ExpiresAt, 0))
	defer cancel()
	if err := r.session.callbacks.Revoke(ctx, revocation); err != nil {
		return protocol.Error{Code: protocol.Unavailable}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	ack := r.message(protocol.ControlRevokeAck, m.RequestID)
	ack.RevocationAck = &protocol.RevocationAck{ID: revocation.ID, HostID: r.session.scope.HostID, BindingGeneration: r.session.scope.BindingGeneration, ConnectorGeneration: r.session.scope.ConnectorGeneration, AccessClosed: true}
	if err := r.send(ctx, ack); err != nil {
		return err
	}
	if whole {
		// A completed smux write only proves transport delivery, not that the
		// peer consumed the ACK. Half-close and let its reader drain before
		// closing the primary socket; access stays denied throughout.
		done := make(chan struct{})
		go func() {
			defer close(done)
			if err := r.stream.CloseWrite(); err != nil {
				return
			}
			var buffer [1024]byte
			for {
				if _, err := r.stream.Read(buffer[:]); err != nil {
					return
				}
			}
		}()
		select {
		case <-done:
		case <-ctx.Done():
			// Publish the terminal revocation before closing sockets: their
			// accept loops must not turn deadline expiry into a reconnectable
			// transport failure. Socket closure interrupts FIN and read.
			r.session.stop(protocol.Error{Code: protocol.Forbidden})
			<-done
		}
		return protocol.Error{Code: protocol.Forbidden}
	}
	return nil
}

func (c *Client) activeRPC() (*controlRPC, error) {
	c.mu.Lock()
	s := c.active
	c.mu.Unlock()
	if s == nil {
		return nil, protocol.Error{Code: protocol.Unavailable}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.denied || s.ctx.Err() != nil || s.rpc == nil {
		return nil, protocol.Error{Code: protocol.Unavailable}
	}
	return s.rpc, nil
}

func (r *controlRPC) call(ctx context.Context, message protocol.ControlMessage, want protocol.ControlType, ice *protocol.ICERequest) (protocol.ControlMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, protocol.RequestTimeout)
	defer cancel()
	pending := pendingCall{want: want, ice: ice, result: make(chan protocol.ControlMessage, 1)}
	r.mu.Lock()
	if len(r.pending) >= protocol.MaxPendingRequests {
		r.mu.Unlock()
		return protocol.ControlMessage{}, protocol.Error{Code: protocol.RateLimited}
	}
	r.pending[message.RequestID] = pending
	r.mu.Unlock()
	defer func() { r.mu.Lock(); delete(r.pending, message.RequestID); r.mu.Unlock() }()
	if err := r.send(ctx, message); err != nil {
		return protocol.ControlMessage{}, err
	}
	select {
	case <-ctx.Done():
		return protocol.ControlMessage{}, ctx.Err()
	case <-r.session.ctx.Done():
		return protocol.ControlMessage{}, protocol.Error{Code: protocol.Unavailable}
	case response := <-pending.result:
		if scope, valid := r.session.client.Scope(); !valid || scope != r.session.scope {
			return protocol.ControlMessage{}, protocol.Error{Code: protocol.Unavailable}
		}
		if response.Error != nil {
			return protocol.ControlMessage{}, protocol.Error{Code: response.Error.Code, RetryAfterSeconds: response.Error.RetryAfterSeconds}
		}
		return response, nil
	}
}

func (c *Client) Exchange(ctx context.Context, exchange protocol.LoginExchange) (protocol.BrowserGrant, error) {
	if err := exchange.Validate(); err != nil {
		return protocol.BrowserGrant{}, err
	}
	rpc, err := c.activeRPC()
	if err != nil {
		return protocol.BrowserGrant{}, err
	}
	if exchange.RedirectURI != rpc.session.assignment.Binding.PublicURL+protocol.HostLoginCallback {
		return protocol.BrowserGrant{}, protocol.Error{Code: protocol.Forbidden}
	}
	id, err := newToken()
	if err != nil {
		return protocol.BrowserGrant{}, err
	}
	message := rpc.message(protocol.ControlExchange, id)
	message.Exchange = &exchange
	response, err := rpc.call(ctx, message, protocol.ControlGrant, nil)
	if err != nil {
		return protocol.BrowserGrant{}, err
	}
	return *response.Grant, nil
}

func (c *Client) ICE(ctx context.Context, request protocol.ICERequest) (protocol.ICEConfig, error) {
	if err := request.GrantID.Validate(); err != nil {
		return protocol.ICEConfig{}, err
	}
	if err := request.PeerConnectionID.Validate(); err != nil {
		return protocol.ICEConfig{}, err
	}
	rpc, err := c.activeRPC()
	if err != nil {
		return protocol.ICEConfig{}, err
	}
	id, err := newToken()
	if err != nil {
		return protocol.ICEConfig{}, err
	}
	message := rpc.message(protocol.ControlICERequest, id)
	message.ICERequest = &request
	response, err := rpc.call(ctx, message, protocol.ControlICEConfig, &request)
	if err != nil {
		return protocol.ICEConfig{}, err
	}
	return *response.ICEConfig, nil
}
