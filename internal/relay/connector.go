package relay

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/coder/websocket"
	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/xtaci/smux"
)

const (
	connectionBuffer = 4 << 20
	streamBuffer     = 1 << 20
	// Bounds header parsing only; established HTTP streams are admitted by relay.
	maxPendingHeaders = 128
	maxBinaryMessage  = protocol.MaxControlBytes
	maxBulkBackoff    = 15 * time.Second
)

type Callbacks struct {
	Ready        func(protocol.Scope)
	PoolsReady   func(interactive, bulk bool)
	Disconnected func()
	// Revoke returns only after the targeted HTTP/WS/P2P/forward access is closed.
	Revoke func(context.Context, protocol.Revocation) error
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

type connectionPool struct {
	physicals map[*physical]struct{}
}

type httpStream struct {
	physical *physical
	conn     net.Conn
}

type connection struct {
	client           *Client
	assignment       protocol.Assignment
	key              ed25519.PrivateKey
	callbacks        Callbacks
	ctx              context.Context
	cancel           context.CancelFunc
	control          *physical
	scope            protocol.Scope
	wg               sync.WaitGroup
	headerSlots      chan struct{}
	ready            chan struct{}
	eventMu          sync.Mutex
	stopped          bool
	isReady          bool
	bulkReady        bool
	interactiveReady bool
	err              error
	shutdownDone     chan struct{}

	mu              sync.Mutex
	rpc             *controlRPC
	controlClaimed  bool
	interactivePool connectionPool
	bulkPool        connectionPool
	denied          bool
	httpStreams     map[*smux.Stream]httpStream
}

// Connect runs one logical generation until loss or cancellation. Its caller
// owns primary control reconnect policy; auxiliary sockets stay in its scope.
func (c *Client) Connect(ctx context.Context, key ed25519.PrivateKey, a protocol.Assignment, callbacks Callbacks) error {
	if callbacks.Ready == nil || callbacks.PoolsReady == nil || callbacks.Disconnected == nil || callbacks.Revoke == nil {
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

	// Retain a private, memory-only copy until every physical worker is joined.
	key = append(ed25519.PrivateKey(nil), key...)
	defer clear(key)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p, err := c.dial(ctx, ctx, key, a, protocol.RoleControl, protocol.Scope{})
	if err != nil {
		callbacks.Disconnected()
		return err
	}
	s := &connection{client: c, assignment: cloneAssignment(a), key: key, callbacks: callbacks, ctx: ctx, cancel: cancel, control: p, scope: p.scope, headerSlots: make(chan struct{}, maxPendingHeaders), ready: make(chan struct{}), shutdownDone: make(chan struct{}), httpStreams: make(map[*smux.Stream]httpStream)}
	s.interactivePool = connectionPool{physicals: make(map[*physical]struct{})}
	s.bulkPool = connectionPool{physicals: make(map[*physical]struct{})}
	defer func() { s.stop(protocol.Error{Code: protocol.Unavailable}); s.wg.Wait() }()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.accept(p, protocol.RoleControl); err != nil {
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
	if ctx.Err() == nil {
		for role, count := range map[protocol.Role]int{protocol.RoleInteractive: c.config.InteractiveConnections, protocol.RoleBulk: c.config.BulkConnections} {
			for range count {
				s.wg.Add(1)
				go func() { defer s.wg.Done(); s.runPhysical(role) }()
			}
		}
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
	s.rpc = rpc
	s.mu.Unlock()
	s.client.mu.Lock()
	s.client.active = s
	s.client.mu.Unlock()
	s.isReady = true
	s.callbacks.Ready(s.scope)
	close(s.ready)
	return true
}

func (s *connection) publishPools() {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	s.mu.Lock()
	interactive := len(s.interactivePool.physicals) > 0
	bulk := len(s.bulkPool.physicals) > 0
	s.mu.Unlock()
	if s.stopped || !s.isReady || s.bulkReady == bulk && s.interactiveReady == interactive {
		return
	}
	s.interactiveReady, s.bulkReady = interactive, bulk
	s.callbacks.PoolsReady(interactive, bulk)
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

	// Deny new access and interrupt loopback I/O before joining either role.
	// A slow control close must never leave bulk open behind it.
	s.closeHTTP()
	s.mu.Lock()
	physicals := []*physical{s.control}
	for p := range s.interactivePool.physicals {
		physicals = append(physicals, p)
	}
	for p := range s.bulkPool.physicals {
		physicals = append(physicals, p)
	}
	s.mu.Unlock()
	var closing sync.WaitGroup
	for _, p := range physicals {
		closing.Add(1)
		go func() {
			defer closing.Done()
			p.close()
		}()
	}
	s.callbacks.Disconnected()
	closing.Wait()
	close(s.shutdownDone)
}

func (s *connection) pool(role protocol.Role) *connectionPool {
	switch role {
	case protocol.RoleInteractive:
		return &s.interactivePool
	default:
		return &s.bulkPool
	}
}

// Each fixed slot owns its socket and reconnect loop. Six default workers can
// authenticate concurrently without creating demand messages or pool counters.
func (s *connection) runPhysical(role protocol.Role) {
	backoff := 100 * time.Millisecond
	for s.ctx.Err() == nil {
		p, err := s.client.dial(s.ctx, s.ctx, s.key, s.assignment, role, s.scope)
		if err == nil {
			s.mu.Lock()
			if s.denied || s.ctx.Err() != nil {
				s.mu.Unlock()
				p.close()
				return
			}
			s.pool(role).physicals[p] = struct{}{}
			s.mu.Unlock()
			s.publishPools()
			backoff = 100 * time.Millisecond
			_ = s.accept(p, role)
			p.close()
			s.mu.Lock()
			delete(s.pool(role).physicals, p)
			for stream, access := range s.httpStreams {
				if access.physical == p {
					if access.conn != nil {
						abortConn(access.conn)
					}
					_ = stream.SetDeadline(time.Now())
				}
			}
			s.mu.Unlock()
			s.publishPools()
		} else if permanentError(err) {
			s.stop(err)
			return
		}
		timer := time.NewTimer(backoff)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(backoff*2, maxBulkBackoff)
	}
}

func permanentError(err error) bool {
	var p protocol.Error
	if !errors.As(err, &p) {
		return false
	}
	switch p.Code {
	case protocol.Forbidden, protocol.Unauthenticated, protocol.WrongAudience, protocol.InvalidSignature, protocol.Incompatible, protocol.InvalidConfig:
		return true
	default:
		return false
	}
}
