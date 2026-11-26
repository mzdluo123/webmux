package relay

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/frphelper"
	"github.com/xtaci/smux"
)

func TestTransportCapabilityNegotiation(t *testing.T) {
	for name, echo := range map[string][]string{
		"legacy": nil, "selection": {protocol.TransportSelectionV1}, "unknown": {"selection.v2"},
		"duplicate": {protocol.TransportSelectionV1, protocol.TransportSelectionV1},
	} {
		t.Run(name, func(t *testing.T) {
			identity, key := newTestIdentity(t), testKey(t)
			a := testAssignment(t, identity)
			node, _ := nodeFixture(t, key, a)
			bundled := frphelper.Available() == nil
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				offer := r.Header.Values(protocol.TransportHeader)
				if bundled && (len(offer) != 1 || offer[0] != protocol.TransportSelectionV1) || !bundled && len(offer) != 0 {
					t.Error("host did not advertise its installed capability")
				}
				for _, v := range echo {
					w.Header().Add(protocol.TransportHeader, v)
				}
				node.ServeHTTP(w, r)
			})
			c := newTestClient(t, identity, tlsTestServer(t, identity, h).Listener.Addr().String())
			physical, err := c.dial(t.Context(), key, a)
			if err != nil {
				t.Fatal(err)
			}
			defer physical.close()
			if physical.selection != (bundled && name == "selection") {
				t.Fatalf("negotiated selection = %v", physical.selection)
			}
		})
	}
}

// These lifecycle cases drive the authenticated control replies independently
// of node admission tests, but they launch and reap the real bundled frpc.
func TestTransportLegacyRecovery(t *testing.T) {
	f := connectTransportFixture(t, false, protocol.FRPTransportQUIC)
	select {
	case code := <-f.failed:
		if code != protocol.Incompatible {
			t.Fatalf("saved QUIC on legacy node: %s", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the saved QUIC attempt did not complete")
	}
	var unexpected protocol.ControlMessage
	if err := f.stream.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err := protocol.ReadFrame(f.stream, &unexpected, protocol.MaxControlBytes); err == nil {
		t.Fatalf("implicit fallback issued %s", unexpected.Type)
	}
	if err := f.stream.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	var saved atomic.Bool
	apply := f.apply(t.Context(), protocol.FRPTransportWSS, &saved)
	g := f.startLegacy(t)
	if status := f.client.TransportSelection(); saved.Load() || status.Preferred != protocol.FRPTransportQUIC || status.Active != "" || status.Pending != protocol.FRPTransportWSS {
		t.Fatalf("unproven legacy recovery: %+v, saved %v", status, saved.Load())
	}
	f.report(t, g, protocol.AllocationStatus{AllocationID: testToken(t), DataReady: true})
	if saved.Load() || f.client.TransportSelection().Active != "" {
		t.Fatal("a probe from another registration verified legacy recovery")
	}
	f.report(t, g, protocol.AllocationStatus{DataReady: true})
	if err := awaitManagement(t, apply); err != nil || !saved.Load() {
		t.Fatalf("verified legacy recovery: %v, saved %v", err, saved.Load())
	}
	if status := f.client.TransportSelection(); status.Preferred != protocol.FRPTransportWSS || status.Active != protocol.FRPTransportWSS || status.Pending != "" || status.Supported {
		t.Fatalf("legacy recovery status: %+v", status)
	}
}

func TestTransportCandidateReconciliation(t *testing.T) {
	f := connectTransportFixture(t, true, protocol.FRPTransportWSS)
	old := f.startLegacy(t)
	f.report(t, old, protocol.AllocationStatus{DataReady: true})
	awaitManagement(t, f.observed)
	stale := f.scope
	stale.ConnectorGeneration--
	if err := f.client.ApplyTransportInScope(t.Context(), stale, protocol.FRPTransportQUIC, func() error {
		t.Error("persist called from a stale generation")
		return nil
	}); protocolCode(err) != protocol.StaleGeneration || f.client.TransportSelection().Pending != "" {
		t.Fatalf("stale generation reserved a candidate: %v", err)
	}
	var saved atomic.Bool
	apply := f.apply(t.Context(), protocol.FRPTransportQUIC, &saved)
	candidate, commit := f.startQUIC(t, old.AllocationID)
	concurrent := f.client.ApplyTransportInScope(t.Context(), f.scope, protocol.FRPTransportWSS, func() error { t.Error("concurrent write"); return nil })
	if protocolCode(concurrent) != protocol.Conflict {
		t.Fatalf("second operation while testing: %v", concurrent)
	}
	f.reply(t, commit, nil, protocol.UpstreamFailed)
	query := f.read(t, protocol.ControlTransportRequest)
	assertTransportRequest(t, query, protocol.TransportQuery, candidate.Transport)
	f.reply(t, query, &protocol.TransportResult{OperationID: candidate.Transport.OperationID, RouteVersion: 1, Active: protocol.FRPTransportWSS,
		ActiveAllocationID: old.AllocationID, State: protocol.TransportExpired}, "")
	if err := awaitManagement(t, apply); protocolCode(err) != protocol.UpstreamFailed {
		t.Fatalf("candidate failure: %v", err)
	}
	status := f.client.TransportSelection()
	if saved.Load() || status.Preferred != protocol.FRPTransportWSS || status.Active != protocol.FRPTransportWSS || status.LastAttempt == nil || status.LastAttempt.Stage != "probe" {
		t.Fatalf("failed probe displaced WSS: %+v, saved %v", status, saved.Load())
	}

	// A cancelled HTTP waiter after the commit request cannot decide whether
	// the node accepted it. A query by the same operation ID decides instead.
	requestCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	apply = f.apply(requestCtx, protocol.FRPTransportQUIC, &saved)
	candidate, _ = f.startQUIC(t, old.AllocationID)
	cancel()
	query = f.read(t, protocol.ControlTransportRequest)
	assertTransportRequest(t, query, protocol.TransportQuery, candidate.Transport)
	if saved.Load() || f.client.TransportSelection().Preferred != protocol.FRPTransportWSS {
		t.Fatal("the host persisted a transport before receiving commit evidence")
	}
	f.reply(t, query, &protocol.TransportResult{OperationID: candidate.Transport.OperationID, RouteVersion: 2, State: protocol.TransportCommitted,
		Active: protocol.FRPTransportQUIC, ActiveAllocationID: candidate.AllocationID}, "")
	if err := awaitManagement(t, apply); err != nil || !saved.Load() {
		t.Fatalf("reconciled commit: %v, saved %v", err, saved.Load())
	}
	if before := awaitManagement(t, f.routeChanged); before != protocol.FRPTransportWSS {
		t.Fatalf("old observation was not cleared before publishing the new active protocol: %q", before)
	}
	if status := f.client.TransportSelection(); status.Preferred != protocol.FRPTransportQUIC || status.Active != protocol.FRPTransportQUIC || status.Pending != "" || status.LastAttempt == nil || status.LastAttempt.Error != "" {
		t.Fatalf("reconciled selection: %+v", status)
	}
	renew := f.read(t, protocol.ControlTransportRequest)
	assertTransportRequest(t, renew, protocol.TransportRenew, candidate.Transport)
	if ref := renew.TransportRequest.AllocationRenew; ref == nil || ref.AllocationID != candidate.AllocationID || ref.GrantRevision != candidate.GrantRevision {
		t.Fatalf("renewal did not keep the committed allocation reference: %+v", ref)
	}

	// Whole-scope revoke must reap the committed QUIC child and the WSS child
	// retained during the bounded drain before it acknowledges the node.
	revoke := protocol.ControlMessage{Version: protocol.V1, Type: protocol.ControlRevoke, RequestID: testToken(t), Scope: f.scope,
		ExpiresAt: time.Now().Add(10 * time.Second).Unix(), Revocation: &protocol.Revocation{ID: testToken(t), HostID: f.scope.HostID,
			BindingGeneration: f.scope.BindingGeneration, ConnectorGeneration: f.scope.ConnectorGeneration, Scope: "connector", Reason: "disconnected"}}
	if err := protocol.WriteFrame(f.stream, revoke, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	ack := f.read(t, protocol.ControlRevokeAck)
	if ack.RequestID != revoke.RequestID || ack.RevocationAck == nil || !ack.RevocationAck.AccessClosed {
		t.Fatal("revocation did not confirm closure")
	}
	awaitManagement(t, f.bridgeClosed)
	if status := f.client.TransportSelection(); status.Active != "" || status.Pending != "" {
		t.Fatalf("revoked connector retained its data selection: %+v", status)
	}
}

type transportFixture struct {
	client       *Client
	assignment   protocol.Assignment
	scope        protocol.Scope
	stream       *smux.Stream
	bridgeOpened <-chan struct{}
	bridgeClosed <-chan struct{}
	observed     <-chan struct{}
	routeChanged <-chan protocol.FRPTransport
	failed       <-chan protocol.Code
}

func connectTransportFixture(t *testing.T, negotiate bool, preferred protocol.FRPTransport) transportFixture {
	t.Helper()
	if frphelper.Available() != nil {
		t.Skip("requires the pinned bundled frpc")
	}
	identity, key := newTestIdentity(t), testKey(t)
	a := testAssignment(t, identity)
	node, nodes := nodeFixture(t, key, a)
	bridgeOpened, bridgeClosed := make(chan struct{}), make(chan struct{})
	var openOnce, closeOnce sync.Once
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/frp/") {
			ws, err := websocket.Accept(w, r, nil)
			if err != nil {
				t.Error(err)
				return
			}
			openOnce.Do(func() { close(bridgeOpened) })
			defer closeOnce.Do(func() { close(bridgeClosed) })
			defer ws.CloseNow()
			for {
				if _, _, err := ws.Read(r.Context()); err != nil {
					return
				}
			}
		}
		if negotiate {
			w.Header().Set(protocol.TransportHeader, protocol.TransportSelectionV1)
		}
		node.ServeHTTP(w, r)
	})
	c := newTestClient(t, identity, tlsTestServer(t, identity, h).Listener.Addr().String())
	c.assignment = &a
	c.SetPreferredTransport(preferred)
	ctx, cancel := context.WithCancel(t.Context())
	done, ready := make(chan struct{}), make(chan struct{})
	observed := make(chan struct{}, 1)
	routeChanged := make(chan protocol.FRPTransport, 1)
	failed := make(chan protocol.Code, 8)
	cb := testCallbacks()
	cb.Ready = func(protocol.Scope) { close(ready) }
	cb.TransportReady = func(_ protocol.Scope, isReady bool, code protocol.Code) {
		if !isReady {
			select {
			case failed <- code:
			default:
			}
		}
	}
	cb.DataObserved = func(_ protocol.Scope, observation DataObservation) {
		if observation.RouteChanged {
			routeChanged <- c.TransportSelection().Active
		}
		if observation.Ready {
			select {
			case observed <- struct{}{}:
			default:
			}
		}
	}
	go func() { defer close(done); _ = c.Connect(ctx, key, a, cb) }()
	t.Cleanup(func() { cancel(); awaitManagement(t, done) })
	n := awaitRole(t, nodes)
	stream, res := openNodeStream(t, n, protocol.StreamControl, protocol.RoleControl, n.scope)
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	awaitManagement(t, ready)
	return transportFixture{client: c, assignment: a, scope: n.scope, stream: stream, bridgeOpened: bridgeOpened, bridgeClosed: bridgeClosed,
		observed: observed, routeChanged: routeChanged, failed: failed}
}

func (f transportFixture) apply(ctx context.Context, desired protocol.FRPTransport, saved *atomic.Bool) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- f.client.ApplyTransportInScope(ctx, f.scope, desired, func() error { saved.Store(true); return nil })
	}()
	return done
}

func (f transportFixture) read(t *testing.T, want protocol.ControlType) protocol.ControlMessage {
	t.Helper()
	if err := f.stream.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		var m protocol.ControlMessage
		if err := protocol.ReadFrame(f.stream, &m, protocol.MaxControlBytes); err != nil {
			t.Fatal(err)
		}
		if m.Type == protocol.ControlPing {
			m.Type = protocol.ControlPong
			if err := protocol.WriteFrame(f.stream, m, protocol.MaxControlBytes); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if m.Type != want || m.Validate(f.scope, time.Now()) != nil {
			t.Fatalf("invalid management request of type %s, wanted %s", m.Type, want)
		}
		return m
	}
}

func (f transportFixture) reply(t *testing.T, m protocol.ControlMessage, result *protocol.TransportResult, code protocol.Code) {
	t.Helper()
	m.Type, m.TransportRequest, m.TransportResult = protocol.ControlTransportResult, nil, result
	if code != "" {
		m.Type, m.Error = protocol.ControlFailure, &protocol.Error{Code: code}
	}
	if err := protocol.WriteFrame(f.stream, m, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
}

func (f transportFixture) report(t *testing.T, g protocol.AllocationGrant, observation protocol.AllocationStatus) {
	t.Helper()
	if observation.AllocationID == "" {
		observation.AllocationID = g.AllocationID
	}
	observation.GrantRevision, observation.CheckedAt = g.GrantRevision, time.Now().UnixMilli()
	m := protocol.ControlMessage{Version: protocol.V1, Type: protocol.ControlFRPStatus, Scope: f.scope, RequestID: testToken(t),
		ExpiresAt: time.Now().Add(time.Second * 5).Unix(), AllocationStatus: &observation}
	if err := protocol.WriteFrame(f.stream, m, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
}

func (f transportFixture) startLegacy(t *testing.T) protocol.AllocationGrant {
	t.Helper()
	m := f.read(t, protocol.ControlFRPAllocate)
	g := f.grant(t, m.AllocationRequest.HostServerCSR, nil)
	m.Type, m.AllocationRequest, m.AllocationGrant = protocol.ControlFRPGrant, nil, &g
	if err := protocol.WriteFrame(f.stream, m, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	awaitManagement(t, f.bridgeOpened)
	return g
}

func (f transportFixture) startQUIC(t *testing.T, old protocol.Token) (protocol.AllocationGrant, protocol.ControlMessage) {
	t.Helper()
	state := f.read(t, protocol.ControlTransportRequest)
	if state.TransportRequest.Action != protocol.TransportStateAction {
		t.Fatal("candidate did not read the route version")
	}
	f.reply(t, state, &protocol.TransportResult{RouteVersion: 1, State: protocol.TransportIdle, Active: protocol.FRPTransportWSS, ActiveAllocationID: old}, "")
	prepare := f.read(t, protocol.ControlTransportRequest)
	p := prepare.TransportRequest
	if p.Action != protocol.TransportPrepare || p.Transport != protocol.FRPTransportQUIC || p.RouteVersion != 1 {
		t.Fatal("prepare did not bind the requested protocol and existing route version")
	}
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	leaf := newTestIdentity(t).server.Leaf
	pin := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	g := f.grant(t, p.AllocationRequest.HostServerCSR, &protocol.TransportGrant{Protocol: p.Transport, OperationID: p.OperationID, RouteVersion: p.RouteVersion,
		Deadline: time.Now().Add(30 * time.Second).Unix(), Endpoint: &protocol.QUICEndpoint{Address: udp.LocalAddr().String(), ServerName: "relay.example",
			SPKI: protocol.Digest(hex.EncodeToString(pin[:])), CertificatePEM: certPEM(leaf.Raw)}})
	f.reply(t, prepare, &protocol.TransportResult{OperationID: p.OperationID, RouteVersion: 1, State: protocol.TransportPrepared, Active: protocol.FRPTransportWSS,
		ActiveAllocationID: old, Grant: &g}, "")
	commit := f.read(t, protocol.ControlTransportRequest)
	assertTransportRequest(t, commit, protocol.TransportCommit, g.Transport)
	if ref := commit.TransportRequest.AllocationRenew; ref == nil || ref.AllocationID != g.AllocationID || ref.GrantRevision != g.GrantRevision {
		t.Fatal("commit did not refer to the prepared allocation")
	}
	if err := udp.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var packet [1500]byte
	if n, _, err := udp.ReadFromUDP(packet[:]); err != nil || n < 1200 {
		t.Fatalf("stock frpc did not send a QUIC initial datagram: bytes %d, %v", n, err)
	}
	return g, commit
}

func assertTransportRequest(t *testing.T, message protocol.ControlMessage, action protocol.TransportAction, grant *protocol.TransportGrant) {
	t.Helper()
	if r := message.TransportRequest; r == nil || r.Action != action || r.OperationID != grant.OperationID || r.RouteVersion != grant.RouteVersion {
		t.Fatalf("transport %s did not reference the original operation", action)
	}
}

func certPEM(der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func (f transportFixture) grant(t *testing.T, csrPEM string, transport *protocol.TransportGrant) protocol.AllocationGrant {
	t.Helper()
	block, _ := pem.Decode([]byte(csrPEM))
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil || csr.CheckSignature() != nil {
		t.Fatalf("invalid host CSR: %v", err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageCertSign, BasicConstraintsValid: true, IsCA: true}
	root, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	id := testToken(t)
	host := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{protocol.FRPOriginServerName(id)},
		URIs: []*url.URL{protocol.FRPOriginURI(f.scope, id, "host")}}
	leaf, err := x509.CreateCertificate(rand.Reader, host, ca, csr.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(csr.RawSubjectPublicKeyInfo)
	g := protocol.AllocationGrant{AllocationID: id, GrantRevision: 1, ProxyName: "fixture", RemotePort: protocol.FRPPortMin, Transport: transport,
		IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), RenewAfter: now.Add(45 * time.Second).Unix(),
		Origin: protocol.AllocationOrigin{HostServerCertPEM: certPEM(leaf), TrustCertPEM: certPEM(root), NodeClientSPKI: protocol.Digest(hex.EncodeToString(pin[:]))}}
	if g.Protocol() == protocol.FRPTransportWSS {
		g.TransportURL, g.BridgeKey = "wss://relay.example/v1/frp/"+string(id), testToken(t)
	} else {
		g.ExpiresAt, g.RenewAfter = now.Add(25*time.Second).Unix(), now.Add(15*time.Second).Unix()
	}
	claims := protocol.FRPClaims{Issuer: "https://control.example/v1/frp/oidc", Audience: protocol.FRPAudience(f.assignment.NodeID), Subject: string(id), ID: testToken(t),
		IssuedAt: g.IssuedAt, NotBefore: g.NotBefore, ExpiresAt: g.ExpiresAt, Scope: f.scope, NodeID: f.assignment.NodeID, AllocationID: id,
		GrantRevision: g.GrantRevision, Role: protocol.FRPRole, ProxyName: g.ProxyName, RemotePort: g.RemotePort, Transport: transport}
	body, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	g.JWT = "e30." + base64.RawURLEncoding.EncodeToString(body) + ".c2ln"
	return g
}
