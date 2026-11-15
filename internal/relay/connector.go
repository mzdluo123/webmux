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
	client       *Client
	assignment   protocol.Assignment
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
	bulk           *physical
	denied         bool
	httpStreams    map[*smux.Stream]net.Conn
}

// Connect runs one logical generation until loss or cancellation. Its caller
// owns control reconnect policy; only bulk is retried here, in the same scope.
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p, err := c.dial(ctx, key, a, protocol.RoleControl, protocol.Scope{})
	if err != nil {
		callbacks.Disconnected()
		return err
	}
	s := &connection{client: c, assignment: cloneAssignment(a), callbacks: callbacks, ctx: ctx, cancel: cancel, control: p, scope: p.scope, workers: make(chan struct{}, maxStreamWorkers), ready: make(chan struct{}), shutdownDone: make(chan struct{}), httpStreams: make(map[*smux.Stream]net.Conn)}
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
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.runBulk(key) }()
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

func (s *connection) publishBulk(ready bool) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
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
	bulk := s.bulk
	s.mu.Unlock()
	var closing sync.WaitGroup
	closing.Add(1)
	go func() {
		defer closing.Done()
		s.control.close()
	}()
	if bulk != nil {
		closing.Add(1)
		go func() {
			defer closing.Done()
			bulk.close()
		}()
	}
	s.callbacks.Disconnected()
	closing.Wait()
	close(s.shutdownDone)
}

func (s *connection) runBulk(key ed25519.PrivateKey) {
	backoff := time.Second
	for s.ctx.Err() == nil {
		p, err := s.client.dial(s.ctx, key, s.assignment, protocol.RoleBulk, s.scope)
		if err == nil {
			s.mu.Lock()
			if s.denied || s.ctx.Err() != nil {
				s.mu.Unlock()
				p.close()
				return
			}
			s.bulk = p
			s.mu.Unlock()
			s.publishBulk(true)
			err = s.accept(p, protocol.RoleBulk)
			p.close()
			s.mu.Lock()
			if s.bulk == p {
				s.bulk = nil
			}
			s.mu.Unlock()
			s.publishBulk(false)
			backoff = time.Second
		}
		if s.ctx.Err() != nil || permanentError(err) {
			return
		}
		// Bounded cryptographic jitter, without changing endpoints/trust/role.
		var jitter [1]byte
		if _, err := rand.Read(jitter[:]); err != nil {
			return
		}
		wait := backoff/2 + time.Duration(jitter[0])*backoff/512
		timer := time.NewTimer(wait)
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
	return errors.As(err, &p) && p.Code != protocol.Unavailable && p.Code != protocol.RateLimited
}

func (c *Client) dial(ctx context.Context, key ed25519.PrivateKey, a protocol.Assignment, role protocol.Role, expected protocol.Scope) (_ *physical, err error) {
	// Discovery expiry governs new control connections. A bulk reconnect is
	// authorized only by its still-live control generation and online node proof.
	if role == protocol.RoleControl {
		if err := a.Validate(time.Now()); err != nil {
			return nil, err
		}
	} else if err := expected.Validate(); err != nil {
		return nil, err
	}
	authCtx, cancel := context.WithTimeout(ctx, protocol.AuthTimeout)
	defer cancel()
	httpClient := c.httpClient(a.Trust)
	defer httpClient.CloseIdleConnections()
	ws, response, err := websocket.Dial(authCtx, a.ConnectorURL, &websocket.DialOptions{HTTPClient: httpClient, Subprotocols: []string{protocol.ConnectorSubprotocol}, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
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
	if role == protocol.RoleBulk {
		hello.ConnectorGeneration = expected.ConnectorGeneration
	}
	if err := writeAuth(authCtx, ws, hello); err != nil {
		return nil, err
	}
	var challenge protocol.ConnectorChallenge
	if err := readAuth(authCtx, ws, &challenge); err != nil {
		return nil, err
	}
	if err := checkChallenge(challenge, a, role, expected, time.Now()); err != nil {
		return nil, err
	}
	proof, err := protocol.SignConnector(key, challenge, time.Now())
	if err != nil {
		return nil, err
	}
	if err := writeAuth(authCtx, ws, proof); err != nil {
		return nil, err
	}
	var ready protocol.ConnectorReady
	if err := readAuth(authCtx, ws, &ready); err != nil {
		return nil, err
	}
	scope := protocol.Scope{HostID: challenge.HostID, BindingGeneration: challenge.BindingGeneration, ConnectorGeneration: challenge.ConnectorGeneration, ControlEpoch: challenge.ControlEpoch, NodeEpoch: challenge.NodeEpoch}
	if ready.Version != protocol.V1 || ready.HostID != scope.HostID || ready.BindingGeneration != scope.BindingGeneration || ready.ConnectorGeneration != scope.ConnectorGeneration || ready.ControlEpoch != scope.ControlEpoch || ready.NodeEpoch != scope.NodeEpoch || ready.Role != role {
		return nil, protocol.Error{Code: protocol.StaleGeneration}
	}
	conn := websocket.NetConn(ctx, ws, websocket.MessageBinary)
	// NetConn disables this limit, so restore it AFTER creating the adapter.
	ws.SetReadLimit(maxBinaryMessage)
	mux, err := smux.Client(conn, smuxConfig())
	if err != nil {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	return &physical{ws: ws, mux: mux, scope: scope}, nil
}

func checkChallenge(challenge protocol.ConnectorChallenge, a protocol.Assignment, role protocol.Role, expected protocol.Scope, now time.Time) error {
	if err := challenge.Validate(now); err != nil {
		return err
	}
	if challenge.Audience != a.NodeAudience || challenge.HostID != a.Binding.HostID || challenge.BindingGeneration != a.Binding.BindingGeneration || challenge.Role != role || challenge.ControlEpoch != a.ControlEpoch {
		return protocol.Error{Code: protocol.WrongAudience}
	}
	if role == protocol.RoleBulk && (challenge.ConnectorGeneration != expected.ConnectorGeneration || challenge.NodeEpoch != expected.NodeEpoch || challenge.ControlEpoch != expected.ControlEpoch) {
		return protocol.Error{Code: protocol.StaleGeneration}
	}
	return nil
}

func smuxConfig() *smux.Config {
	cfg := smux.DefaultConfig()
	cfg.Version = 2
	cfg.MaxReceiveBuffer = connectionBuffer
	cfg.MaxStreamBuffer = streamBuffer
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
