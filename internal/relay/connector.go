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
	maxHTTPStreams   = 64
	// Includes stream headers still being read; malformed peers cannot spawn
	// unbounded goroutines while withholding their headers.
	maxStreamWorkers = maxHTTPStreams + 1
	maxBinaryMessage = protocol.MaxControlBytes
	maxBulkBackoff   = 15 * time.Second
)

type Callbacks struct {
	Ready        func(protocol.Scope)
	BulkReady    func(bool)
	Disconnected func()
	// Revoke returns only after the targeted HTTP/WS/P2P/forward access is closed.
	Revoke func(context.Context, protocol.Revocation) error
}

type physical struct {
	ws        *websocket.Conn
	mux       *smux.Session
	scope     protocol.Scope
	ordinary  int // guarded by connection.mu
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
	target    int
	growUntil time.Time
	wake      chan struct{}
	paused    bool
}

type httpStream struct {
	physical *physical
	conn     net.Conn
}

type connection struct {
	client       *Client
	assignment   protocol.Assignment
	key          ed25519.PrivateKey
	callbacks    Callbacks
	ctx          context.Context
	cancel       context.CancelFunc
	control      *physical
	scope        protocol.Scope
	wg           sync.WaitGroup
	workers      chan struct{}
	ready        chan struct{}
	eventMu      sync.Mutex
	stopped      bool
	isReady      bool
	bulkReady    bool
	err          error
	shutdownDone chan struct{}

	mu             sync.Mutex
	rpc            *controlRPC
	controlClaimed bool
	controlPool    connectionPool
	bulkPool       connectionPool
	denied         bool
	httpStreams    map[*smux.Stream]httpStream
}

// Connect runs one logical generation until loss or cancellation. Its caller
// owns primary control reconnect policy; auxiliary sockets stay in its scope.
func (c *Client) Connect(ctx context.Context, key ed25519.PrivateKey, a protocol.Assignment, callbacks Callbacks) error {
	if callbacks.Ready == nil || callbacks.BulkReady == nil || callbacks.Disconnected == nil || callbacks.Revoke == nil {
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
	s := &connection{client: c, assignment: cloneAssignment(a), key: key, callbacks: callbacks, ctx: ctx, cancel: cancel, control: p, scope: p.scope, workers: make(chan struct{}, maxStreamWorkers), ready: make(chan struct{}), shutdownDone: make(chan struct{}), httpStreams: make(map[*smux.Stream]httpStream)}
	s.controlPool = connectionPool{physicals: map[*physical]struct{}{p: {}}, target: 1, wake: make(chan struct{}, 1)}
	s.bulkPool = connectionPool{physicals: make(map[*physical]struct{}), target: 1, wake: make(chan struct{}, 1)}
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
		s.wg.Add(2)
		go func() { defer s.wg.Done(); s.runPool(protocol.RoleControl) }()
		go func() { defer s.wg.Done(); s.runPool(protocol.RoleBulk) }()
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

func (s *connection) publishBulk() {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	s.mu.Lock()
	ready := len(s.bulkPool.physicals) > 0
	s.mu.Unlock()
	if s.stopped || !s.isReady || s.bulkReady == ready {
		return
	}
	s.bulkReady = ready
	s.callbacks.BulkReady(ready)
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
	physicals := make([]*physical, 0, len(s.controlPool.physicals)+len(s.bulkPool.physicals))
	for p := range s.controlPool.physicals {
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
	if role == protocol.RoleControl {
		return &s.controlPool
	}
	return &s.bulkPool
}

func wakePool(pool *connectionPool) {
	select {
	case pool.wake <- struct{}{}:
	default:
	}
}

func (s *connection) grow(demand protocol.ConnectionDemand) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.denied || s.ctx.Err() != nil {
		return
	}
	pool := s.pool(demand.Role)
	target := int(demand.Connections)
	if target <= len(pool.physicals) {
		return
	}
	now := time.Now()
	if pool.growUntil.IsZero() || !now.Before(pool.growUntil) {
		pool.target = max(1, len(pool.physicals))
		pool.growUntil = now.Add(protocol.StreamAdmissionTimeout)
	} else if target <= pool.target {
		// Duplicate demand neither starts another dial nor extends its budget.
		return
	}
	pool.target = max(pool.target, target)
	pool.paused = false
	wakePool(pool)
}

// One long-lived worker per role serializes authentication and bounds retries.
// Bulk's first socket retains its original reconnect policy; extra sockets are
// attempted only within the current demand's admission budget.
func (s *connection) runPool(role protocol.Role) {
	pool := s.pool(role)
	backoff := time.Duration(0)
	for {
		s.mu.Lock()
		if s.denied || s.ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		now := time.Now()
		if !pool.growUntil.IsZero() && !now.Before(pool.growUntil) {
			pool.growUntil = time.Time{}
			pool.target = max(1, len(pool.physicals))
		}
		until := pool.growUntil
		growing := !until.IsZero()
		baseline := role == protocol.RoleBulk && len(pool.physicals) == 0
		needed := len(pool.physicals) < pool.target && (growing || baseline && !pool.paused)
		if needed {
			select {
			case <-pool.wake:
			default:
			}
		}
		s.mu.Unlock()
		if !needed {
			backoff = 0
			select {
			case <-s.ctx.Done():
				return
			case <-pool.wake:
				continue
			}
		}
		authCtx := s.ctx
		cancel := func() {}
		if growing {
			authCtx, cancel = context.WithDeadline(s.ctx, until)
		}
		p, err := s.client.dial(s.ctx, authCtx, s.key, s.assignment, role, s.scope)
		cancel()
		if err == nil {
			backoff = 0
			if !s.attach(p, role) {
				p.close()
				return
			}
			continue
		}
		if s.ctx.Err() != nil {
			return
		}
		s.mu.Lock()
		pool.paused = permanentError(err)
		s.mu.Unlock()
		limit := maxBulkBackoff
		if growing {
			limit = time.Second
		}
		if backoff == 0 {
			backoff = time.Second
			if growing {
				backoff = 100 * time.Millisecond
			}
		}
		wait := backoff
		if growing {
			wait = min(wait, max(time.Duration(0), time.Until(until)))
		}
		timer := time.NewTimer(wait)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-pool.wake:
			timer.Stop()
		case <-timer.C:
		}
		backoff = min(backoff*2, limit)
	}
}

func (s *connection) attach(p *physical, role protocol.Role) bool {
	s.mu.Lock()
	pool := s.pool(role)
	if s.denied || s.ctx.Err() != nil || len(pool.physicals) >= protocol.MaxRoleConnections {
		s.mu.Unlock()
		return false
	}
	pool.physicals[p] = struct{}{}
	pool.paused = false
	s.wg.Add(1)
	s.mu.Unlock()
	if role == protocol.RoleBulk {
		s.publishBulk()
	}
	go func() {
		defer s.wg.Done()
		_ = s.accept(p, role)
		p.close()
		s.mu.Lock()
		delete(pool.physicals, p)
		for stream, access := range s.httpStreams {
			if access.physical == p {
				if access.conn != nil {
					abortConn(access.conn)
				}
				_ = stream.SetDeadline(time.Now())
			}
		}
		wakePool(pool)
		s.mu.Unlock()
		if role == protocol.RoleBulk {
			s.publishBulk()
		}
	}()
	return true
}

func permanentError(err error) bool {
	var p protocol.Error
	return errors.As(err, &p) && p.Code != protocol.Unavailable && p.Code != protocol.RateLimited
}
