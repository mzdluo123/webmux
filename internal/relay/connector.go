package relay

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/xtaci/smux"
)

const (
	connectionBuffer = 4 << 20
	streamBuffer     = 1 << 20
	maxBinaryMessage = protocol.MaxControlBytes
)

type Callbacks struct {
	Ready        func(protocol.Scope)
	DataReady    func(ready bool, reason protocol.Code)
	Disconnected func()
	// Revoke returns only after the targeted HTTP/WS/P2P/forward access is closed.
	Revoke func(context.Context, protocol.Revocation) error
	// Origin handles requests only after scope-bound mTLS and data readiness.
	Origin http.Handler
}

type physical struct {
	ws        *websocket.Conn
	mux       *smux.Session
	scope     protocol.Scope
	closeOnce sync.Once
}

func (p *physical) close() {
	p.closeOnce.Do(func() {
		// Immediate socket closure also unblocks smux's serialized writer; no
		// graceful websocket close can delay local access revocation.
		_ = p.ws.CloseNow()
		_ = p.mux.Close()
	})
}

type connection struct {
	client     *Client
	assignment protocol.Assignment
	callbacks  Callbacks
	ctx        context.Context
	cancel     context.CancelFunc
	control    *physical
	scope      protocol.Scope
	wg         sync.WaitGroup
	ready      chan struct{}
	dataCancel context.CancelFunc
	dataDone   chan struct{}
	status     chan timedFRPStatus

	eventMu      sync.Mutex
	stopped      bool
	err          error
	shutdownDone chan struct{}
	mu           sync.Mutex
	rpc          *controlRPC
	denied       bool
}

// Connect runs one authenticated management generation until loss or cancellation.
func (c *Client) Connect(ctx context.Context, key ed25519.PrivateKey, a protocol.Assignment, callbacks Callbacks) error {
	if callbacks.Ready == nil || callbacks.DataReady == nil || callbacks.Disconnected == nil || callbacks.Revoke == nil || callbacks.Origin == nil {
		return protocol.Error{Code: protocol.InvalidConfig}
	}
	if len(key) != ed25519.PrivateKeySize {
		return protocol.Error{Code: protocol.Invalid}
	}
	if err := a.Validate(time.Now()); err != nil {
		return err
	}
	c.mu.Lock()
	if c.connecting {
		c.mu.Unlock()
		return protocol.Error{Code: protocol.Conflict}
	}
	if c.assignment == nil || !sameAssignment(a, *c.assignment) {
		c.mu.Unlock()
		return protocol.Error{Code: protocol.Forbidden}
	}
	c.connecting = true
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.connecting = false; c.mu.Unlock() }()

	a = cloneAssignment(a)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p, err := c.dial(ctx, key, a)
	if err != nil {
		callbacks.Disconnected()
		return err
	}
	dataCtx, stopData := context.WithCancel(ctx)
	s := &connection{client: c, assignment: a, callbacks: callbacks, ctx: ctx, cancel: cancel, control: p, scope: p.scope,
		ready: make(chan struct{}), shutdownDone: make(chan struct{}), dataCancel: stopData, dataDone: make(chan struct{}), status: make(chan timedFRPStatus, 16)}
	go func() {
		defer close(s.dataDone)
		select {
		case <-s.ready:
			s.runFRP(dataCtx)
		case <-dataCtx.Done():
		}
	}()
	defer func() { s.stop(protocol.Error{Code: protocol.Unavailable}); s.wg.Wait() }()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.acceptControl(); err != nil {
			s.stop(err)
		}
	}()
	startup := time.NewTimer(protocol.StreamOpenTimeout)
	defer startup.Stop()
	select {
	case <-s.ready:
	case <-ctx.Done():
		s.stop(ctx.Err())
	case <-startup.C:
		s.stop(protocol.Error{Code: protocol.Unavailable})
	}
	<-ctx.Done()
	s.stop(ctx.Err())
	s.eventMu.Lock()
	err = s.err
	s.eventMu.Unlock()
	return err
}

func (c *Client) Scope() (protocol.Scope, bool) {
	c.mu.Lock()
	s := c.active
	c.mu.Unlock()
	if s == nil {
		return protocol.Scope{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.denied || s.ctx.Err() != nil {
		return protocol.Scope{}, false
	}
	return s.scope, true
}

func (s *connection) publishReady(rpc *controlRPC) bool {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	if s.stopped || s.ctx.Err() != nil {
		return false
	}
	s.mu.Lock()
	if s.denied {
		s.mu.Unlock()
		return false
	}
	s.rpc = rpc
	s.mu.Unlock()
	s.client.mu.Lock()
	s.client.active = s
	s.client.mu.Unlock()
	s.callbacks.Ready(s.scope)
	close(s.ready)
	return true
}

func (s *connection) publishData(ready bool, reason protocol.Code) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	if s.stopped || s.ctx.Err() != nil {
		return
	}
	s.mu.Lock()
	denied := s.denied
	s.mu.Unlock()
	if !ready || !denied {
		s.callbacks.DataReady(ready, reason)
	}
}

func (s *connection) denyData() {
	s.eventMu.Lock()
	s.mu.Lock()
	s.denied = true
	s.mu.Unlock()
	s.dataCancel()
	s.eventMu.Unlock()
	<-s.dataDone
}

func (s *connection) stop(err error) {
	s.eventMu.Lock()
	if s.stopped {
		s.eventMu.Unlock()
		<-s.shutdownDone
		return
	}
	s.stopped = true
	s.mu.Lock()
	if s.denied {
		// Whole-scope revocation has already removed access. A concurrent
		// transport failure or cancellation must not make it reconnectable.
		err = protocol.Error{Code: protocol.Forbidden}
	}
	s.mu.Unlock()
	s.err = err
	s.client.mu.Lock()
	if s.client.active == s {
		s.client.active = nil
	}
	s.client.mu.Unlock()
	s.cancel()
	s.eventMu.Unlock()

	s.control.close()
	<-s.dataDone
	s.callbacks.Disconnected()
	close(s.shutdownDone)
}
