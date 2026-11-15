package relay

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/xtaci/smux"
)

type nodeRole struct {
	ws    *websocket.Conn
	mux   *smux.Session
	scope protocol.Scope
	role  protocol.Role
}

func (n *nodeRole) close() { _ = n.ws.CloseNow(); _ = n.mux.Close() }

func nodeFixture(t *testing.T, id testIdentity, key ed25519.PrivateKey, a protocol.Assignment) (http.Handler, <-chan *nodeRole, <-chan *nodeRole) {
	t.Helper()
	controls := make(chan *nodeRole, 1)
	bulks := make(chan *nodeRole, 4)
	epoch := testToken(t)
	public := protocol.PublicKey{Algorithm: protocol.HostAlgorithm, Key: base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey))}
	var mu sync.Mutex
	var roles []*nodeRole
	t.Cleanup(func() {
		mu.Lock()
		snapshot := append([]*nodeRole(nil), roles...)
		mu.Unlock()
		for _, role := range snapshot {
			role.close()
		}
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if err := readAuth(ctx, ws, &hello); err != nil {
			t.Error(err)
			return
		}
		if err := hello.Validate(); err != nil {
			t.Error(err)
			return
		}
		if hello.HostID != a.Binding.HostID || hello.BindingGeneration != a.Binding.BindingGeneration {
			t.Error("unexpected connector identity")
			return
		}
		generation := protocol.Counter(7)
		if hello.Role == protocol.RoleBulk && hello.ConnectorGeneration != generation {
			t.Error("bulk generation mismatch")
			return
		}
		challenge := protocol.ConnectorChallenge{Version: protocol.V1, Audience: a.NodeAudience, HostID: hello.HostID, BindingGeneration: hello.BindingGeneration, ConnectorGeneration: generation, Role: hello.Role, Nonce: testToken(t), ExpiresAt: time.Now().Add(20 * time.Second).Unix(), ControlEpoch: a.ControlEpoch, NodeEpoch: epoch}
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
		ready := protocol.ConnectorReady{Version: protocol.V1, HostID: hello.HostID, BindingGeneration: hello.BindingGeneration, ConnectorGeneration: generation, Role: hello.Role, ControlEpoch: a.ControlEpoch, NodeEpoch: epoch}
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
		node := &nodeRole{ws: ws, mux: mux, scope: protocol.Scope{HostID: hello.HostID, BindingGeneration: hello.BindingGeneration, ConnectorGeneration: generation, ControlEpoch: a.ControlEpoch, NodeEpoch: epoch}, role: hello.Role}
		mu.Lock()
		roles = append(roles, node)
		mu.Unlock()
		if hello.Role == protocol.RoleControl {
			controls <- node
		} else {
			bulks <- node
		}
		// smux reports transport EOF through AcceptStream, not CloseChan.
		// The node owns the session and must translate that terminal error
		// into Close, just as its production accept loop does.
		if _, err := mux.AcceptStream(); err == nil {
			t.Error("host unexpectedly opened a node stream")
		}
	})
	return handler, controls, bulks
}

func awaitRole(t *testing.T, roles <-chan *nodeRole) *nodeRole {
	t.Helper()
	select {
	case role := <-roles:
		return role
	case <-time.After(5 * time.Second):
		t.Fatal("connector authentication timed out")
		return nil
	}
}

func openNodeStream(t *testing.T, node *nodeRole, kind protocol.StreamKind, scope protocol.Scope) (*smux.Stream, protocol.StreamResult) {
	t.Helper()
	stream, err := node.mux.OpenStream()
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	header := protocol.StreamHeader{Version: protocol.V1, Kind: kind, RequestID: testToken(t), Scope: scope, Role: node.role}
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

func TestConnectorReadinessBulkSeparationAndSynchronousDisconnect(t *testing.T) {
	id := newTestIdentity(t)
	key := testKey(t)
	a := testAssignment(t, id)
	var forwarded atomic.Int32
	chunks := make(chan chan struct{}, 2)
	local := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.VerifiedChains) == 0 {
			t.Error("loopback client was not authenticated")
		}
		if r.Host != "original.example" || r.Header.Get("Origin") != "https://original.example" {
			t.Error("raw HTTP origin changed")
		}
		forwarded.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "first\n")
		w.(http.Flusher).Flush()
		release := make(chan struct{})
		chunks <- release
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, "last\n")
	}), true)
	handler, controls, bulks := nodeFixture(t, id, key, a)
	remote := tlsTestServer(t, id, handler, false)
	c := newTestClient(t, id, remote.Listener.Addr().String(), local.Listener.Addr().String())
	// Equivalent to the authenticated HTTPS Assignment response, without a
	// fake authorization service in this transport-only fixture.
	owned := cloneAssignment(a)
	c.assignment = &owned
	ready := make(chan protocol.Scope, 1)
	bulkState := make(chan bool, 8)
	var disconnected atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- c.Connect(ctx, key, a, Callbacks{Ready: func(scope protocol.Scope) { ready <- scope }, BulkReady: func(value bool) { bulkState <- value }, Disconnected: func() { disconnected.Store(true) }, Revoke: func(context.Context, protocol.Revocation) error { return nil }})
	}()
	control := awaitRole(t, controls)
	select {
	case <-ready:
		t.Fatal("ready before control stream accepted")
	case <-time.After(30 * time.Millisecond):
	}
	stream, result := openNodeStream(t, control, protocol.StreamControl, control.scope)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	select {
	case scope := <-ready:
		if scope != control.scope {
			t.Fatal("wrong active scope")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ready callback absent")
	}
	bulk := awaitRole(t, bulks)
	select {
	case value := <-bulkState:
		if !value {
			t.Fatal("initial bulk not ready")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("bulk callback absent")
	}
	// A stale scope is rejected before any fixed-loopback connection is made.
	stale := control.scope
	stale.ConnectorGeneration++
	_, refused := openNodeStream(t, control, protocol.StreamHTTP, stale)
	if refused.Error == nil || refused.Error.Code != protocol.StaleGeneration || forwarded.Load() != 0 {
		t.Fatal("stale stream reached loopback")
	}
	// Failed bulk is replaced in the SAME scope, without control replacement.
	bulk.close()
	select {
	case value := <-bulkState:
		if value {
			t.Fatal("bulk loss not reported")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("bulk loss callback absent")
	}
	if disconnected.Load() {
		t.Fatal("bulk loss revoked control")
	}
	replacement := awaitRole(t, bulks)
	if replacement.scope != control.scope {
		t.Fatal("bulk reconnect changed generation")
	}
	if scope, ok := c.Scope(); !ok || scope != control.scope {
		t.Fatal("control scope lost with bulk")
	}
	for _, role := range []*nodeRole{control, replacement} {
		httpStream, accepted := openNodeStream(t, role, protocol.StreamHTTP, role.scope)
		if accepted.Error != nil {
			t.Fatal(accepted.Error)
		}
		if _, err := io.WriteString(httpStream, "GET /stream HTTP/1.1\r\nHost: original.example\r\nOrigin: https://original.example\r\nConnection: close\r\n\r\n"); err != nil {
			t.Fatal(err)
		}
		response, err := http.ReadResponse(bufio.NewReader(httpStream), nil)
		if err != nil {
			t.Fatal(err)
		}
		var first [6]byte
		if _, err := io.ReadFull(response.Body, first[:]); err != nil || string(first[:]) != "first\n" {
			t.Fatalf("unflushed response = %q, %v", first, err)
		}
		select {
		case release := <-chunks:
			close(release)
		case <-time.After(time.Second):
			t.Fatal("streaming handler did not flush")
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusAccepted || string(body) != "last\n" {
			t.Fatalf("streaming response = %q, %v", body, err)
		}
	}
	if forwarded.Load() != 2 {
		t.Fatal("unexpected forwarding count")
	}
	// Keep an RPC pending at control loss. It must terminate, not replay on bulk.
	rpcDone := make(chan error, 1)
	go func() {
		_, err := c.Exchange(ctx, protocol.LoginExchange{Code: testToken(t), Verifier: strings.Repeat("a", 43), RedirectURI: a.Binding.PublicURL + protocol.HostLoginCallback})
		rpcDone <- err
	}()
	var request protocol.ControlMessage
	if err := protocol.ReadFrame(stream, &request, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	if request.Type != protocol.ControlExchange {
		t.Fatal("missing exchange RPC")
	}
	control.close()
	select {
	case err := <-done:
		if err == nil || !disconnected.Load() {
			t.Fatal("Connect returned before synchronous disconnect")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("control loss did not join connector")
	}
	select {
	case err := <-rpcDone:
		if err == nil {
			t.Fatal("orphan RPC succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("RPC survived control loss")
	}
	if _, ok := c.Scope(); ok {
		t.Fatal("closed generation remains active")
	}
	select {
	case <-replacement.mux.CloseChan():
	case <-time.After(time.Second):
		t.Fatal("bulk survived control loss")
	}
}

func TestRevocationACKWaitsForClosedAccess(t *testing.T) {
	id := newTestIdentity(t)
	key := testKey(t)
	a := testAssignment(t, id)
	handler, controls, bulks := nodeFixture(t, id, key, a)
	remote := tlsTestServer(t, id, handler, false)
	c := newTestClient(t, id, remote.Listener.Addr().String(), "127.0.0.1:12345")
	owned := cloneAssignment(a)
	c.assignment = &owned
	entered := make(chan struct{})
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	callbacks := testCallbacks()
	callbacks.Revoke = func(ctx context.Context, r protocol.Revocation) error {
		close(entered)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	go func() { done <- c.Connect(ctx, key, a, callbacks) }()
	control := awaitRole(t, controls)
	stream, result := openNodeStream(t, control, protocol.StreamControl, control.scope)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	awaitRole(t, bulks)
	revocation := protocol.Revocation{ID: testToken(t), Scope: "grant", HostID: a.Binding.HostID, BindingGeneration: a.Binding.BindingGeneration, GrantID: testToken(t), Reason: "logout"}
	message := protocol.ControlMessage{Version: protocol.V1, Type: protocol.ControlRevoke, RequestID: testToken(t), Scope: control.scope, ExpiresAt: time.Now().Add(9 * time.Second).Unix(), Revocation: &revocation}
	if err := protocol.WriteFrame(stream, message, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("revocation callback absent")
	}
	ack := make(chan protocol.ControlMessage, 1)
	readErr := make(chan error, 1)
	go func() {
		var m protocol.ControlMessage
		if err := protocol.ReadFrame(stream, &m, protocol.MaxControlBytes); err != nil {
			readErr <- err
			return
		}
		ack <- m
	}()
	select {
	case <-ack:
		t.Fatal("ACK preceded access closure")
	case err := <-readErr:
		t.Fatal(err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	select {
	case m := <-ack:
		if err := m.Validate(control.scope, time.Now()); err != nil {
			t.Fatal(err)
		}
		if m.Type != protocol.ControlRevokeAck || m.RequestID != message.RequestID || m.RevocationAck.ID != revocation.ID || !m.RevocationAck.AccessClosed {
			t.Fatal("incorrect revocation ACK")
		}
	case err := <-readErr:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("ACK missing after access closed")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("connector leaked on cancellation")
	}
}

func TestConnectorChallengeScopeAndRole(t *testing.T) {
	id := newTestIdentity(t)
	a := testAssignment(t, id)
	expected := protocol.Scope{HostID: a.Binding.HostID, BindingGeneration: 1, ConnectorGeneration: 9, ControlEpoch: a.ControlEpoch, NodeEpoch: testToken(t)}
	give := protocol.ConnectorChallenge{Version: protocol.V1, Audience: a.NodeAudience, HostID: expected.HostID, BindingGeneration: 1, ConnectorGeneration: expected.ConnectorGeneration, Role: protocol.RoleBulk, Nonce: testToken(t), ExpiresAt: time.Now().Add(20 * time.Second).Unix(), ControlEpoch: expected.ControlEpoch, NodeEpoch: expected.NodeEpoch}
	if err := checkChallenge(give, a, protocol.RoleBulk, expected, time.Now()); err != nil {
		t.Fatal(err)
	}
	tests := []func(*protocol.ConnectorChallenge){func(c *protocol.ConnectorChallenge) { c.Role = protocol.RoleControl }, func(c *protocol.ConnectorChallenge) { c.ConnectorGeneration++ }, func(c *protocol.ConnectorChallenge) { c.Audience = "other.example" }, func(c *protocol.ConnectorChallenge) { c.NodeEpoch = testToken(t) }, func(c *protocol.ConnectorChallenge) { c.ControlEpoch = testToken(t) }, func(c *protocol.ConnectorChallenge) { c.BindingGeneration++ }}
	for index, change := range tests {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			changed := give
			change(&changed)
			if err := checkChallenge(changed, a, protocol.RoleBulk, expected, time.Now()); err == nil {
				t.Fatal("cross-scope signing allowed")
			}
		})
	}
}
