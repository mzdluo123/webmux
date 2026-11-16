package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	s.err = err
	relayDebug("connection-stop", protocol.RoleControl, err)
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

// ctx owns the physical lifetime; authCtx may impose a shorter growth budget.
func (c *Client) dial(ctx, authCtx context.Context, key ed25519.PrivateKey, a protocol.Assignment, role protocol.Role, expected protocol.Scope) (_ *physical, err error) {
	stage := "assignment-validation"
	defer func() { relayDebug(stage, role, err) }()
	// Only a new primary depends on discovery expiry. Every auxiliary is
	// independently authorized within its exact, still-live logical scope.
	if role == protocol.RoleControl && expected.ConnectorGeneration == 0 {
		if err := a.Validate(time.Now()); err != nil {
			return nil, err
		}
	} else if err := expected.Validate(); err != nil {
		return nil, err
	}
	authCtx, cancel := context.WithTimeout(authCtx, protocol.AuthTimeout)
	defer cancel()
	httpClient := c.httpClient(&a.Trust)
	defer httpClient.CloseIdleConnections()
	stage = "wss-dial"
	ws, response, err := websocket.Dial(authCtx, a.ConnectorURL, &websocket.DialOptions{HTTPClient: httpClient, Subprotocols: []string{protocol.ConnectorSubprotocol}, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		relayDebug(stage, role, err)
		if response != nil && response.StatusCode >= 300 && response.StatusCode < 400 {
			return nil, protocol.Error{Code: protocol.Forbidden}
		}
		return nil, transportError(authCtx, err)
	}
	defer func() {
		if err != nil {
			_ = ws.CloseNow()
		}
	}()
	if ws.Subprotocol() != protocol.ConnectorSubprotocol {
		return nil, protocol.Error{Code: protocol.Incompatible}
	}
	ws.SetReadLimit(protocol.MaxAuthBytes)
	hello := protocol.ConnectorHello{Version: protocol.V1, HostID: a.Binding.HostID, BindingGeneration: a.Binding.BindingGeneration, Role: role}
	if expected.ConnectorGeneration > 0 {
		hello.ConnectorGeneration = expected.ConnectorGeneration
	}
	stage = "hello-write"
	if err := writeAuth(authCtx, ws, hello); err != nil {
		return nil, err
	}
	var challenge protocol.ConnectorChallenge
	stage = "challenge-read"
	if err := readAuth(authCtx, ws, &challenge); err != nil {
		return nil, err
	}
	stage = "challenge-validation"
	if err := checkChallenge(challenge, a, role, expected, time.Now()); err != nil {
		return nil, err
	}
	proof, err := protocol.SignConnector(key, challenge, time.Now())
	if err != nil {
		return nil, err
	}
	stage = "proof-write"
	if err := writeAuth(authCtx, ws, proof); err != nil {
		return nil, err
	}
	var ready protocol.ConnectorReady
	stage = "ready-read"
	if err := readAuth(authCtx, ws, &ready); err != nil {
		return nil, err
	}
	stage = "ready-validation"
	scope := protocol.Scope{HostID: challenge.HostID, BindingGeneration: challenge.BindingGeneration, ConnectorGeneration: challenge.ConnectorGeneration, ControlEpoch: challenge.ControlEpoch, NodeEpoch: challenge.NodeEpoch}
	if ready.Version != protocol.V1 || ready.HostID != scope.HostID || ready.BindingGeneration != scope.BindingGeneration || ready.ConnectorGeneration != scope.ConnectorGeneration || ready.ControlEpoch != scope.ControlEpoch || ready.NodeEpoch != scope.NodeEpoch || ready.Role != role {
		return nil, protocol.Error{Code: protocol.StaleGeneration}
	}
	conn := websocket.NetConn(ctx, ws, websocket.MessageBinary)
	// NetConn disables this limit, so restore it AFTER creating the adapter.
	ws.SetReadLimit(maxBinaryMessage)
	stage = "smux-start"
	mux, err := smux.Client(conn, smuxConfig())
	if err != nil {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	stage = "authenticated"
	return &physical{ws: ws, mux: mux, scope: scope}, nil
}

func checkChallenge(challenge protocol.ConnectorChallenge, a protocol.Assignment, role protocol.Role, expected protocol.Scope, now time.Time) error {
	if err := challenge.Validate(now); err != nil {
		return err
	}
	if challenge.Audience != a.NodeAudience || challenge.HostID != a.Binding.HostID || challenge.BindingGeneration != a.Binding.BindingGeneration || challenge.Role != role || challenge.ControlEpoch != a.ControlEpoch {
		return protocol.Error{Code: protocol.WrongAudience}
	}
	if expected.ConnectorGeneration > 0 && (challenge.HostID != expected.HostID || challenge.BindingGeneration != expected.BindingGeneration || challenge.ConnectorGeneration != expected.ConnectorGeneration || challenge.NodeEpoch != expected.NodeEpoch || challenge.ControlEpoch != expected.ControlEpoch) {
		return protocol.Error{Code: protocol.StaleGeneration}
	}
	return nil
}

func smuxConfig() *smux.Config {
	cfg := smux.DefaultConfig()
	cfg.Version = 2
	cfg.MaxReceiveBuffer = max(1, connectionBuffer/protocol.MaxRoleConnections)
	cfg.MaxStreamBuffer = min(streamBuffer, cfg.MaxReceiveBuffer)
	cfg.KeepAliveInterval = protocol.KeepAliveInterval
	cfg.KeepAliveTimeout = protocol.ControlLease
	return cfg
}

func writeAuth(ctx context.Context, ws *websocket.Conn, value any) error {
	data, err := json.Marshal(value)
	if err != nil || len(data) > protocol.MaxAuthBytes {
		return protocol.Error{Code: protocol.Invalid}
	}
	if err := ws.Write(ctx, websocket.MessageText, data); err != nil {
		return transportError(ctx, err)
	}
	return nil
}

func readAuth(ctx context.Context, ws *websocket.Conn, value any) error {
	kind, data, err := ws.Read(ctx)
	if err != nil {
		relayDebug("auth-read", "", err)
		return transportError(ctx, err)
	}
	if kind != websocket.MessageText {
		return protocol.Error{Code: protocol.Invalid}
	}
	if err := protocol.Decode(data, value, protocol.MaxAuthBytes); err == nil {
		return nil
	}
	var failure protocol.Error
	if protocol.Decode(data, &failure, protocol.MaxAuthBytes) == nil && failure.Code.Valid() {
		return protocol.Error{Code: failure.Code, RetryAfterSeconds: failure.RetryAfterSeconds}
	}
	return protocol.Error{Code: protocol.Invalid}
}

func newToken() (protocol.Token, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", protocol.Error{Code: protocol.Unavailable}
	}
	return protocol.Token(base64.RawURLEncoding.EncodeToString(b[:])), nil
}
