package relay

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/xtaci/smux"
)

type nodeManagement struct {
	ws    *websocket.Conn
	mux   *smux.Session
	scope protocol.Scope
}

func (n *nodeManagement) close() { _ = n.ws.CloseNow(); _ = n.mux.Close() }

func nodeFixture(t *testing.T, key ed25519.PrivateKey, a protocol.Assignment) (http.Handler, <-chan *nodeManagement) {
	t.Helper()
	nodes := make(chan *nodeManagement, 1)
	epoch, nonce := testToken(t), testToken(t)
	public := protocol.PublicKey{Algorithm: protocol.HostAlgorithm, Key: base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey))}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != protocol.PathConnector {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{protocol.ConnectorSubprotocol}, CompressionMode: websocket.CompressionDisabled})
		if err != nil {
			t.Error(err)
			return
		}
		defer ws.CloseNow()
		ws.SetReadLimit(protocol.MaxAuthBytes)
		ctx, cancel := context.WithTimeout(r.Context(), protocol.AuthTimeout)
		defer cancel()
		var hello protocol.ConnectorHello
		if err := readAuth(ctx, ws, &hello); err != nil || hello.Validate() != nil || hello.HostID != a.Binding.HostID || hello.BindingGeneration != a.Binding.BindingGeneration || hello.Role != protocol.RoleControl {
			t.Error("unexpected management identity", err)
			return
		}
		generation := protocol.Counter(7)
		challenge := protocol.ConnectorChallenge{Version: protocol.V1, Audience: a.NodeAudience, HostID: hello.HostID, BindingGeneration: hello.BindingGeneration, ConnectorGeneration: generation, Role: protocol.RoleControl, Nonce: nonce, ExpiresAt: time.Now().Add(20 * time.Second).Unix(), ControlEpoch: a.ControlEpoch, NodeEpoch: epoch}
		if err := writeAuth(ctx, ws, challenge); err != nil {
			t.Error(err)
			return
		}
		var proof protocol.ConnectorProof
		if err := readAuth(ctx, ws, &proof); err != nil {
			t.Error(err)
			return
		}
		if err := protocol.VerifyConnector(public, challenge, proof, time.Now()); err != nil {
			t.Error(err)
			return
		}
		ready := protocol.ConnectorReady{Version: protocol.V1, HostID: hello.HostID, BindingGeneration: hello.BindingGeneration, ConnectorGeneration: generation, Role: protocol.RoleControl, ControlEpoch: a.ControlEpoch, NodeEpoch: epoch}
		if err := writeAuth(ctx, ws, ready); err != nil {
			t.Error(err)
			return
		}
		conn := websocket.NetConn(r.Context(), ws, websocket.MessageBinary)
		ws.SetReadLimit(maxBinaryMessage)
		mux, err := smux.Server(conn, smuxConfig())
		if err != nil {
			t.Error(err)
			return
		}
		defer mux.Close()
		node := &nodeManagement{ws: ws, mux: mux, scope: protocol.Scope{HostID: hello.HostID, BindingGeneration: hello.BindingGeneration, ConnectorGeneration: generation, ControlEpoch: a.ControlEpoch, NodeEpoch: epoch}}
		select {
		case nodes <- node:
		case <-t.Context().Done():
			return
		}
		if _, err := mux.AcceptStream(); err == nil {
			t.Error("host unexpectedly opened a node stream")
		}
	}), nodes
}

func awaitRole(t *testing.T, nodes <-chan *nodeManagement) *nodeManagement {
	t.Helper()
	return awaitManagement(t, nodes)
}

func awaitManagement[T any](t *testing.T, values <-chan T) T {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("management event timed out")
		var zero T
		return zero
	}
}

func openNodeStream(t *testing.T, node *nodeManagement, kind protocol.StreamKind, role protocol.Role, scope protocol.Scope) (*smux.Stream, protocol.StreamResult) {
	t.Helper()
	stream, err := node.mux.OpenStream()
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	header := protocol.StreamHeader{Version: protocol.V1, Kind: kind, RequestID: testToken(t), Scope: scope, Role: role}
	if err := protocol.WriteFrame(stream, header, protocol.MaxStreamHeaderBytes); err != nil {
		t.Fatal(err)
	}
	var result protocol.StreamResult
	if err := protocol.ReadFrame(stream, &result, protocol.MaxStreamHeaderBytes); err != nil {
		t.Fatal(err)
	}
	if result.Version != protocol.V1 || result.RequestID != header.RequestID {
		t.Fatal("invalid stream acceptance")
	}
	return stream, result
}

func readRPC(stream io.ReadWriter, scope protocol.Scope) (protocol.ControlMessage, error) {
	for {
		var message protocol.ControlMessage
		if err := protocol.ReadFrame(stream, &message, protocol.MaxControlBytes); err != nil {
			return message, err
		}
		if err := message.Validate(scope, time.Now()); err != nil {
			return message, err
		}
		switch message.Type {
		case protocol.ControlFRPAllocate:
			// These tests exercise management with no data-plane node. This
			// also works when run in a build that embeds the stock companion.
			message.Type = protocol.ControlFailure
			message.AllocationRequest = nil
			message.Error = &protocol.Error{Code: protocol.Unavailable}
		case protocol.ControlPing:
			message.Type = protocol.ControlPong
		default:
			return message, nil
		}
		if err := protocol.WriteFrame(stream, message, protocol.MaxControlBytes); err != nil {
			return message, err
		}
	}
}

func readManagement(t *testing.T, stream io.ReadWriter, scope protocol.Scope) protocol.ControlMessage {
	t.Helper()
	message, err := readRPC(stream, scope)
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func TestManagementRequiresAssignedPin(t *testing.T) {
	id, key := newTestIdentity(t), testKey(t)
	var requests atomic.Int32
	remote := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	c := newTestClient(t, id, remote.Listener.Addr().String())
	a := testAssignment(t, id)
	a.Trust.Pins = []string{string(testToken(t))}
	c.assignment = &a
	if err := c.Connect(t.Context(), key, a, testCallbacks()); protocolCode(err) != protocol.Unauthenticated || requests.Load() != 0 {
		t.Fatalf("management sent requests before verifying assigned pin: requests=%d err=%v", requests.Load(), err)
	}
}

func TestManagementRejectsExtraAndRetiredStreams(t *testing.T) {
	id, key := newTestIdentity(t), testKey(t)
	a := testAssignment(t, id)
	h, nodes := nodeFixture(t, key, a)
	c := newTestClient(t, id, tlsTestServer(t, id, h).Listener.Addr().String())
	c.assignment = &a
	gotReady := make(chan protocol.Scope, 1)
	cb := testCallbacks()
	cb.Ready = func(s protocol.Scope) { gotReady <- s }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Connect(ctx, key, a, cb) }()
	node := awaitRole(t, nodes)
	if _, ok := c.Scope(); ok {
		t.Fatal("authentication alone published control readiness")
	}
	old, result := openNodeStream(t, node, protocol.StreamKind("http"), protocol.Role("bulk"), node.scope)
	defer old.Close()
	if result.Error == nil {
		t.Fatal("retired stream accepted")
	}
	stale := node.scope
	stale.ConnectorGeneration++
	_, result = openNodeStream(t, node, protocol.StreamControl, protocol.RoleControl, stale)
	if result.Error == nil {
		t.Fatal("stale scope accepted")
	}
	_, result = openNodeStream(t, node, protocol.StreamControl, protocol.RoleControl, node.scope)
	if result.Error != nil || awaitManagement(t, gotReady) != node.scope {
		t.Fatal("valid control stream did not become ready")
	}
	_, result = openNodeStream(t, node, protocol.StreamControl, protocol.RoleControl, node.scope)
	if result.Error == nil || result.Error.Code != protocol.Conflict {
		t.Fatal("duplicate control stream did not return conflict")
	}
	node.close()
	_ = awaitManagement(t, done)
	if _, ok := c.Scope(); ok {
		t.Fatal("lost management left scope ready")
	}
}

func TestManagementLeaseExpiryFencesCurrentScope(t *testing.T) {
	id, key := newTestIdentity(t), testKey(t)
	a := testAssignment(t, id)
	h, nodes := nodeFixture(t, key, a)
	c := newTestClient(t, id, tlsTestServer(t, id, h).Listener.Addr().String())
	c.assignment = &a
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cb := testCallbacks()
	ready := make(chan protocol.Scope, 1)
	disconnected := make(chan struct{})
	cb.Ready = func(scope protocol.Scope) { ready <- scope }
	cb.Disconnected = func() { close(disconnected) }
	done := make(chan error, 1)
	go func() { done <- c.Connect(ctx, key, a, cb) }()
	node := awaitRole(t, nodes)
	_, result := openNodeStream(t, node, protocol.StreamControl, protocol.RoleControl, node.scope)
	if result.Error != nil || awaitManagement(t, ready) != node.scope {
		t.Fatal("management did not become ready")
	}
	rpc, err := c.activeRPC()
	if err != nil {
		t.Fatal(err)
	}
	// Start the watchdog with its last pong at the hard lease boundary.
	watchdog := newControlRPC(rpc.session, rpc.stream)
	watchdog.lastPong = time.Now().Add(-protocol.ControlLease)
	go watchdog.heartbeat()
	awaitManagement(t, disconnected)
	if _, ok := c.Scope(); ok || rpc.session.ctx.Err() == nil {
		t.Fatal("lease expiry retained the active scope or origin context")
	}
	if err := awaitManagement(t, done); protocolCode(err) != protocol.Expired {
		t.Fatalf("management lease returned %v", err)
	}
	awaitManagement(t, watchdog.heartbeatDone)
}

func TestRevocationACKWaitsForClosedAccess(t *testing.T) {
	id, key := newTestIdentity(t), testKey(t)
	a := testAssignment(t, id)
	h, nodes := nodeFixture(t, key, a)
	c := newTestClient(t, id, tlsTestServer(t, id, h).Listener.Addr().String())
	c.assignment = &a
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	ready := make(chan struct{})
	cb := testCallbacks()
	cb.Ready = func(protocol.Scope) { close(ready) }
	cb.Revoke = func(ctx context.Context, _ protocol.Revocation) error {
		close(entered)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done := make(chan error, 1)
	go func() { done <- c.Connect(ctx, key, a, cb) }()
	node := awaitRole(t, nodes)
	stream, result := openNodeStream(t, node, protocol.StreamControl, protocol.RoleControl, node.scope)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	awaitManagement(t, ready)
	rpc, err := c.activeRPC()
	if err != nil {
		t.Fatal(err)
	}
	message := protocol.ControlMessage{Version: protocol.V1, Type: protocol.ControlRevoke, RequestID: testToken(t), Scope: node.scope,
		ExpiresAt: time.Now().Add(9 * time.Second).Unix(), Revocation: &protocol.Revocation{ID: testToken(t), Scope: "connector", HostID: a.Binding.HostID,
			BindingGeneration: a.Binding.BindingGeneration, ConnectorGeneration: node.scope.ConnectorGeneration, Reason: "disconnected"}}
	if err := protocol.WriteFrame(stream, message, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	awaitManagement(t, entered)
	select {
	case <-rpc.session.dataDone:
	default:
		t.Fatal("revocation callback preceded data worker shutdown")
	}
	if _, ok := c.Scope(); ok {
		t.Fatal("revoked scope still ready before ACK")
	}
	type receipt struct {
		message protocol.ControlMessage
		err     error
	}
	ack := make(chan receipt, 1)
	go func() { m, err := readRPC(stream, node.scope); ack <- receipt{m, err} }()
	select {
	case got := <-ack:
		t.Fatalf("ACK preceded access closure: %v", got.err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	got := awaitManagement(t, ack)
	m := got.message
	if got.err != nil || m.Type != protocol.ControlRevokeAck || m.RequestID != message.RequestID || m.RevocationAck.ID != message.Revocation.ID || !m.RevocationAck.AccessClosed {
		t.Fatalf("incorrect revocation ACK: %v", got.err)
	}
	_ = stream.Close()
	if err := awaitManagement(t, done); protocolCode(err) != protocol.Forbidden {
		t.Fatalf("revoked connector returned %v", err)
	}
}
