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

func TestConnectorRequiresAssignedPinEvenForBulkReconnect(t *testing.T) {
	id := newTestIdentity(t)
	var requests atomic.Int32
	remote := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}), false)
	c := newTestClient(t, id, remote.Listener.Addr().String(), "127.0.0.1:12345")
	key := testKey(t)
	a := testAssignment(t, id)
	scope := protocol.Scope{HostID: a.Binding.HostID, BindingGeneration: a.Binding.BindingGeneration, ConnectorGeneration: 1, ControlEpoch: a.ControlEpoch, NodeEpoch: testToken(t)}
	for _, tc := range []struct {
		name  string
		trust protocol.ServiceTrust
	}{
		{"wrong pin", protocol.ServiceTrust{Algorithm: protocol.ServiceAlgorithm, Pins: []string{string(testToken(t))}}},
		{"empty trust", protocol.ServiceTrust{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assignment := a
			assignment.Trust = tc.trust
			p, err := c.dial(t.Context(), t.Context(), key, assignment, protocol.RoleBulk, scope)
			if p != nil {
				p.close()
				t.Fatal("connector returned a connection without assigned trust")
			}
			if err == nil || requests.Load() != 0 {
				t.Fatalf("connector passed TLS without its assigned pin: requests=%d err=%v", requests.Load(), err)
			}
		})
	}
}

type nodeRole struct {
	ws         *websocket.Conn
	mux        *smux.Session
	scope      protocol.Scope
	role       protocol.Role
	generation protocol.Counter
}

func (n *nodeRole) close() { _ = n.ws.CloseNow(); _ = n.mux.Close() }

func nodeFixture(t *testing.T, id testIdentity, key ed25519.PrivateKey, a protocol.Assignment) (http.Handler, <-chan *nodeRole, <-chan *nodeRole) {
	t.Helper()
	controls := make(chan *nodeRole, protocol.MaxRoleConnections)
	bulks := make(chan *nodeRole, protocol.MaxRoleConnections)
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
		if (hello.Role == protocol.RoleBulk || hello.ConnectorGeneration > 0) && hello.ConnectorGeneration != generation {
			t.Error("auxiliary generation mismatch")
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
		node := &nodeRole{ws: ws, mux: mux, scope: protocol.Scope{HostID: hello.HostID, BindingGeneration: hello.BindingGeneration, ConnectorGeneration: generation, ControlEpoch: a.ControlEpoch, NodeEpoch: epoch}, role: hello.Role, generation: hello.ConnectorGeneration}
		mu.Lock()
		roles = append(roles, node)
		mu.Unlock()
		if hello.Role != protocol.RoleBulk {
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
	c.config.BulkConnections = 1
	// Equivalent to the authenticated HTTPS Assignment response, without a
	// fake authorization service in this transport-only fixture.
	owned := cloneAssignment(a)
	c.assignment = &owned
	ready := make(chan protocol.Scope, 1)
	bulkState := make(chan bool, 8)
	lastBulk := false
	var disconnected atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- c.Connect(ctx, key, a, Callbacks{Ready: func(scope protocol.Scope) { ready <- scope }, PoolsReady: func(_ bool, value bool) {
			if value != lastBulk {
				lastBulk = value
				bulkState <- value
			}
		}, Disconnected: func() { disconnected.Store(true) }, Revoke: func(context.Context, protocol.Revocation) error { return nil }})
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
	interactive := awaitRole(t, controls)
	for _, role := range []*nodeRole{interactive, replacement} {
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
	for _, role := range []protocol.Role{protocol.RoleControl, protocol.RoleBulk} {
		give := protocol.ConnectorChallenge{Version: protocol.V1, Audience: a.NodeAudience, HostID: expected.HostID, BindingGeneration: 1, ConnectorGeneration: expected.ConnectorGeneration, Role: role, Nonce: testToken(t), ExpiresAt: time.Now().Add(20 * time.Second).Unix(), ControlEpoch: expected.ControlEpoch, NodeEpoch: expected.NodeEpoch}
		if err := checkChallenge(give, a, role, expected, time.Now()); err != nil {
			t.Fatal(err)
		}
		tests := []func(*protocol.ConnectorChallenge){func(c *protocol.ConnectorChallenge) {
			c.Role = protocol.RoleControl
			if role == protocol.RoleControl {
				c.Role = protocol.RoleBulk
			}
		}, func(c *protocol.ConnectorChallenge) { c.ConnectorGeneration++ }, func(c *protocol.ConnectorChallenge) { c.Audience = "other.example" }, func(c *protocol.ConnectorChallenge) { c.NodeEpoch = testToken(t) }, func(c *protocol.ConnectorChallenge) { c.ControlEpoch = testToken(t) }, func(c *protocol.ConnectorChallenge) { c.BindingGeneration++ }}
		for index, change := range tests {
			t.Run(fmt.Sprintf("%s/%d", role, index), func(t *testing.T) {
				changed := give
				change(&changed)
				if err := checkChallenge(changed, a, role, expected, time.Now()); err == nil {
					t.Fatal("cross-scope signing allowed")
				}
			})
		}
	}
}

type poolConnector struct {
	client     *Client
	primary    *nodeRole
	bulk       *nodeRole
	management *smux.Stream
	controls   <-chan *nodeRole
	bulks      <-chan *nodeRole
	bulkState  chan bool
	cancel     context.CancelFunc
	done       chan struct{}
	err        error
}

func startPoolConnector(t *testing.T, id testIdentity, loopback string, wrap func(http.Handler) http.Handler) *poolConnector {
	t.Helper()
	key := testKey(t)
	a := testAssignment(t, id)
	handler, controls, bulks := nodeFixture(t, id, key, a)
	if wrap != nil {
		handler = wrap(handler)
	}
	remote := tlsTestServer(t, id, handler, false)
	c := newTestClient(t, id, remote.Listener.Addr().String(), loopback)
	owned := cloneAssignment(a)
	c.assignment = &owned
	ctx, cancel := context.WithCancel(context.Background())
	h := &poolConnector{client: c, controls: controls, bulks: bulks, bulkState: make(chan bool, 32), cancel: cancel, done: make(chan struct{})}
	callbacks := testCallbacks()
	lastBulk := false
	callbacks.PoolsReady = func(_ bool, ready bool) {
		if ready != lastBulk {
			lastBulk = ready
			h.bulkState <- ready
		}
	}
	go func() {
		h.err = c.Connect(ctx, key, a, callbacks)
		close(h.done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-h.done:
		case <-time.After(5 * time.Second):
			t.Error("pooled connector did not join all workers")
		}
	})
	h.primary = awaitRole(t, controls)
	var result protocol.StreamResult
	h.management, result = openNodeStream(t, h.primary, protocol.StreamControl, h.primary.scope)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	if err := h.management.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	h.bulk = awaitRole(t, bulks)
	select {
	case ready := <-h.bulkState:
		if !ready {
			t.Fatal("initial bulk was not ready")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("initial bulk readiness absent")
	}
	return h
}

func holdPoolHTTP(t *testing.T, node *nodeRole) (*smux.Stream, *http.Response) {
	t.Helper()
	stream, accepted := openNodeStream(t, node, protocol.StreamHTTP, node.scope)
	if accepted.Error != nil {
		t.Fatal(accepted.Error)
	}
	if _, err := io.WriteString(stream, "GET /hold HTTP/1.1\r\nHost: original.example\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(stream), nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("held HTTP status = %d", response.StatusCode)
	}
	return stream, response
}

func awaitPoolClosed(t *testing.T, h *poolConnector, roles ...*nodeRole) {
	t.Helper()
	select {
	case <-h.done:
		if h.err == nil {
			t.Fatal("connector ended without its lifecycle error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pooled connector did not stop")
	}
	if _, ok := h.client.Scope(); ok {
		t.Fatal("stopped pool retained an active scope")
	}
	for _, role := range roles {
		select {
		case <-role.mux.CloseChan():
		case <-time.After(time.Second):
			t.Fatal("physical socket survived logical shutdown")
		}
	}
}

func TestAuxiliaryLossPreservesPrimaryAndOtherRole(t *testing.T) {
	id := newTestIdentity(t)
	cancelled := make(chan struct{}, 2)
	local := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		cancelled <- struct{}{}
	}), true)
	h := startPoolConnector(t, id, local.Listener.Addr().String(), nil)
	auxControl := awaitRole(t, h.controls)
	auxBulk := awaitRole(t, h.bulks)
	awaitRole(t, h.controls)
	for range protocol.DefaultBulkConnections - 2 {
		awaitRole(t, h.bulks)
	}
	lostStream, lostResponse := holdPoolHTTP(t, auxControl)
	survivingStream, _ := holdPoolHTTP(t, auxBulk)
	auxControl.close()
	_ = lostStream.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := lostResponse.Body.Read(make([]byte, 1)); err == nil {
		t.Fatal("lost auxiliary stream remained readable")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("lost auxiliary left loopback access open")
	}
	replacementControl := awaitRole(t, h.controls)
	h.bulk.close()
	replacementBulk := awaitRole(t, h.bulks)
	select {
	case ready := <-h.bulkState:
		t.Fatalf("bulk readiness changed to %v while another bulk socket remained", ready)
	case <-time.After(30 * time.Millisecond):
	}
	if scope, ok := h.client.Scope(); !ok || scope != h.primary.scope {
		t.Fatal("auxiliary loss revoked the primary scope")
	}
	select {
	case <-survivingStream.GetDieCh():
		t.Fatal("control auxiliary loss closed the other role's stream")
	default:
	}
	h.cancel()
	awaitPoolClosed(t, h, h.primary, auxBulk, replacementControl, replacementBulk)
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("cancelled pool retained the surviving ordinary stream")
	}
}

func TestAuxiliaryAuthenticationOutlivesDiscoveryExpiry(t *testing.T) {
	id := newTestIdentity(t)
	key := testKey(t)
	a := testAssignment(t, id)
	handler, controls, bulks := nodeFixture(t, id, key, a)
	remote := tlsTestServer(t, id, handler, false)
	c := newTestClient(t, id, remote.Listener.Addr().String(), "127.0.0.1:12345")
	primary, err := c.dial(t.Context(), t.Context(), key, a, protocol.RoleControl, protocol.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	defer primary.close()
	awaitRole(t, controls)
	a.ExpiresAt = time.Now().Add(-time.Second).Unix()
	for _, role := range []protocol.Role{protocol.RoleInteractive, protocol.RoleBulk} {
		auxiliary, err := c.dial(t.Context(), t.Context(), key, a, role, primary.scope)
		if err != nil {
			t.Fatal(err)
		}
		defer auxiliary.close()
		roles := controls
		if role == protocol.RoleBulk {
			roles = bulks
		}
		node := awaitRole(t, roles)
		if node.generation != primary.scope.ConnectorGeneration || node.scope != primary.scope {
			t.Fatal("expired discovery auxiliary failed to authenticate its exact scope")
		}
	}
	if p, err := c.dial(t.Context(), t.Context(), key, a, protocol.RoleControl, protocol.Scope{}); err == nil {
		p.close()
		t.Fatal("expired discovery created a new primary")
	}
}

func TestPoolSmuxBudgetIsBoundedPerPhysicalSocket(t *testing.T) {
	cfg := smuxConfig()
	if cfg.MaxReceiveBuffer > connectionBuffer || cfg.MaxReceiveBuffer < 1 {
		t.Fatal("physical receive windows multiply the role budget")
	}
	if cfg.MaxStreamBuffer > cfg.MaxReceiveBuffer || cfg.MaxStreamBuffer > streamBuffer || cfg.MaxFrameSize > cfg.MaxStreamBuffer {
		t.Fatal("physical stream buffer violates its receive window or frame constraints")
	}
}

func TestConnectorRevocationClosesEveryPooledPhysical(t *testing.T) {
	for _, finish := range []string{"peer-fin", "cancel", "deadline"} {
		t.Run(finish, func(t *testing.T) {
			id := newTestIdentity(t)
			local := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}), true)
			h := startPoolConnector(t, id, local.Listener.Addr().String(), nil)
			control := awaitRole(t, h.controls)
			bulk := awaitRole(t, h.bulks)
			holdPoolHTTP(t, control)
			holdPoolHTTP(t, bulk)
			revocation := protocol.Revocation{ID: testToken(t), Scope: "connector", HostID: h.primary.scope.HostID, BindingGeneration: h.primary.scope.BindingGeneration, ConnectorGeneration: h.primary.scope.ConnectorGeneration, Reason: "logout"}
			expires := time.Now().Add(protocol.ControlMessageTTL)
			if finish == "deadline" {
				expires = time.Now().Add(2 * time.Second)
			}
			message := protocol.ControlMessage{Version: protocol.V1, Type: protocol.ControlRevoke, RequestID: testToken(t), Scope: h.primary.scope, ExpiresAt: expires.Unix(), Revocation: &revocation}
			if err := protocol.WriteFrame(h.management, message, protocol.MaxControlBytes); err != nil {
				t.Fatal(err)
			}
			var ack protocol.ControlMessage
			if err := protocol.ReadFrame(h.management, &ack, protocol.MaxControlBytes); err != nil {
				t.Fatal(err)
			}
			if err := ack.Validate(h.primary.scope, time.Now()); err != nil {
				t.Fatal(err)
			}
			if ack.Type != protocol.ControlRevokeAck || ack.RequestID != message.RequestID || ack.RevocationAck.ID != revocation.ID || !ack.RevocationAck.AccessClosed {
				t.Fatal("whole connector revocation was not acknowledged after closing access")
			}
			var trailing [1]byte
			if n, err := h.management.Read(trailing[:]); n != 0 || err != io.EOF {
				t.Fatalf("revocation ACK was not followed by management FIN: n=%d err=%v", n, err)
			}
			select {
			case <-h.done:
				t.Fatal("connector closed before its peer finished draining the ACK")
			default:
			}
			h.client.mu.Lock()
			active := h.client.active
			h.client.mu.Unlock()
			if active == nil {
				t.Fatal("connector stopped before management drain")
			}
			active.mu.Lock()
			denied := active.denied
			active.mu.Unlock()
			if !denied {
				t.Fatal("management drain reopened revoked access")
			}
			switch finish {
			case "peer-fin":
				if err := h.management.CloseWrite(); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				h.cancel()
			}
			awaitPoolClosed(t, h, h.primary, h.bulk, control, bulk)
			if protocolCode(h.err) != protocol.Forbidden {
				t.Fatalf("revocation ended as a reconnectable failure: %v", h.err)
			}
		})
	}
}

func TestFixedSlotsRetryAuthenticationWithoutGrowth(t *testing.T) {
	id := newTestIdentity(t)
	var attempts atomic.Int32
	wrap := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempt := attempts.Add(1)
			if attempt == 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if attempt == 3 || attempt == 4 {
				ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{protocol.ConnectorSubprotocol}})
				if err != nil {
					t.Error(err)
					return
				}
				defer ws.CloseNow()
				var hello protocol.ConnectorHello
				if err := readAuth(r.Context(), ws, &hello); err != nil {
					t.Error(err)
					return
				}
				code := protocol.StaleGeneration
				if attempt == 4 {
					code = protocol.Expired
				}
				if err := writeAuth(r.Context(), ws, protocol.Error{Code: code}); err != nil {
					t.Error(err)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	h := startPoolConnector(t, id, "127.0.0.1:12345", wrap)
	roles := []*nodeRole{h.primary, h.bulk}
	for range protocol.DefaultInteractiveConnections {
		roles = append(roles, awaitRole(t, h.controls))
	}
	for range protocol.DefaultBulkConnections - 1 {
		roles = append(roles, awaitRole(t, h.bulks))
	}
	for _, role := range roles {
		if role.scope != h.primary.scope {
			t.Fatal("fixed slot retry changed scope")
		}
	}
	if got := attempts.Load(); got != 1+protocol.DefaultInteractiveConnections+protocol.DefaultBulkConnections+3 {
		t.Fatalf("unexpected authentication attempts: %d", got)
	}
	h.cancel()
	awaitPoolClosed(t, h, roles...)
}

func TestPhysicalBackoffStabilityJitterAndRetryAfter(t *testing.T) {
	b := newPhysicalBackoff(0)
	if b.threshold != protocol.ControlLease {
		t.Fatalf("threshold = %v, want %v", b.threshold, protocol.ControlLease)
	}
	// Short-lived disconnects continue exponential backoff up to maxBulkBackoff.
	var prev time.Duration
	for step := range 10 {
		b.observeConnected(50 * time.Millisecond)
		low := b.current
		got := b.nextDelay(nil, 128)
		high := low + 255*low/512
		if got < low || got > high {
			t.Fatalf("step %d delay = %v, want in [%v, %v]", step, got, low, high)
		}
		if step > 0 && low < maxBulkBackoff && low <= prev {
			t.Fatalf("step %d base %v did not advance beyond %v", step, low, prev)
		}
		prev = low
	}
	if b.current != maxBulkBackoff {
		t.Fatalf("capped current = %v, want %v", b.current, maxBulkBackoff)
	}

	// Stable uptime at or above threshold resets backoff to 100ms.
	b.observeConnected(protocol.ControlLease)
	if got := b.nextDelay(nil, 0); got != minPhysicalBackoff {
		t.Fatalf("after stable run delay = %v, want %v", got, minPhysicalBackoff)
	}

	// RetryAfterSeconds acts as a strict lower bound with bounded jitter.
	retryErr := protocol.Error{Code: protocol.RateLimited, RetryAfterSeconds: 6}
	if got := b.nextDelay(retryErr, 0); got != 6*time.Second {
		t.Fatalf("retryAfter delay without jitter = %v, want 6s", got)
	}
	if got := b.nextDelay(retryErr, 200); got < 6*time.Second || got > 9*time.Second {
		t.Fatalf("retryAfter delay with jitter = %v, want in [6s, 9s]", got)
	}
}

func TestFlappingPhysicalSlotBacksOffUntilStable(t *testing.T) {
	id := newTestIdentity(t)
	key := testKey(t)
	a := testAssignment(t, id)
	handler, controls, bulks := nodeFixture(t, id, key, a)
	remote := tlsTestServer(t, id, handler, false)
	c := newTestClient(t, id, remote.Listener.Addr().String(), "127.0.0.1:12345")
	c.config.InteractiveConnections = 1
	c.config.BulkConnections = 1
	c.stableThreshold = 250 * time.Millisecond
	owned := cloneAssignment(a)
	c.assignment = &owned

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan protocol.Scope, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.Connect(ctx, key, a, Callbacks{
			Ready:        func(scope protocol.Scope) { ready <- scope },
			PoolsReady:   func(bool, bool) {},
			Disconnected: func() {},
			Revoke:       func(context.Context, protocol.Revocation) error { return nil },
		})
	}()
	primary := awaitRole(t, controls)
	if _, res := openNodeStream(t, primary, protocol.StreamControl, primary.scope); res.Error != nil {
		t.Fatal(res.Error)
	}
	<-ready
	_ = awaitRole(t, controls) // interactive slot

	// Flap bulk slot three times immediately (< 250ms threshold).
	for i := range 3 {
		b := awaitRole(t, bulks)
		closedAt := time.Now()
		b.close()
		if i == 2 {
			bNext := awaitRole(t, bulks)
			gap := time.Since(closedAt)
			if gap < 350*time.Millisecond {
				t.Fatalf("flapping bulk slot did not back off: reconnect gap = %v, want >= 350ms", gap)
			}
			// Keep bNext alive past stableThreshold, then close and verify reset.
			time.Sleep(300 * time.Millisecond)
			resetAt := time.Now()
			bNext.close()
			bAfterStable := awaitRole(t, bulks)
			resetGap := time.Since(resetAt)
			if resetGap > 350*time.Millisecond {
				t.Fatalf("stable bulk slot did not reset backoff: reconnect gap = %v", resetGap)
			}
			bAfterStable.close()
		}
	}
	cancel()
	<-done
}
