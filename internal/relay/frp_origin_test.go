package relay

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

func TestOriginRequiresScopedMTLSAndFreshDataStatus(t *testing.T) {
	id := testToken(t)
	binding := testBinding()
	scope := protocol.Scope{HostID: binding.HostID, BindingGeneration: binding.BindingGeneration, ConnectorGeneration: 2, ControlEpoch: testToken(t), NodeEpoch: testToken(t)}
	key, csrPEM, err := newOriginCSR()
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(csrPEM))
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil || csr.CheckSignature() != nil || !key.PublicKey.Equal(csr.PublicKey) {
		t.Fatalf("host invalid proof of key possession: %v", err)
	}
	newKey := func() *ecdsa.PrivateKey {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return key
	}
	issue := func(template, parent *x509.Certificate, pub, signingKey *ecdsa.PrivateKey) (*x509.Certificate, string) {
		der, err := x509.CreateCertificate(rand.Reader, template, parent, &pub.PublicKey, signingKey)
		if err != nil {
			t.Fatal(err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return cert, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	now := time.Now()
	start, end := now.Add(-time.Minute), now.Add(time.Minute)
	caKey := newKey()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), KeyUsage: x509.KeyUsageCertSign, NotBefore: start, NotAfter: end, IsCA: true, BasicConstraintsValid: true}
	ca, caPEM := issue(ca, ca, caKey, caKey)
	name := protocol.FRPOriginServerName(id)
	_, hostPEM := issue(&x509.Certificate{SerialNumber: big.NewInt(2), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		NotBefore: start, NotAfter: end, DNSNames: []string{name}, URIs: []*url.URL{protocol.FRPOriginURI(scope, id, "host")}}, ca, key, caKey)
	nodeKey := newKey()
	issuePeer := func(serial int64, scope protocol.Scope, peerKey *ecdsa.PrivateKey) tls.Certificate {
		leaf, _ := issue(&x509.Certificate{SerialNumber: big.NewInt(serial), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			NotBefore: start, NotAfter: end, URIs: []*url.URL{protocol.FRPOriginURI(scope, id, "node")}}, ca, peerKey, caKey)
		return tls.Certificate{Certificate: [][]byte{leaf.Raw}, Leaf: leaf, PrivateKey: peerKey}
	}
	node := issuePeer(3, scope, nodeKey)
	pin := sha256.Sum256(node.Leaf.RawSubjectPublicKeyInfo)
	grant := protocol.AllocationGrant{AllocationID: id, ExpiresAt: now.Add(30 * time.Second).Unix(), Origin: protocol.AllocationOrigin{HostServerCertPEM: hostPEM, TrustCertPEM: caPEM, NodeClientSPKI: protocol.Digest(hex.EncodeToString(pin[:]))}}
	access := &frpAccess{ctx: t.Context()}
	access.grant.Store(time.Unix(grant.ExpiresAt, 0).UnixNano())
	if _, _, err := originTLS(scope, newKey(), grant, access); protocolCode(err) != protocol.Unauthenticated {
		t.Fatal("accepted host certificate not issued to the local CSR key")
	}
	secure, until, err := originTLS(scope, key, grant, access)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := listenFRP(8)
	if err != nil {
		t.Fatal(err)
	}
	var served atomic.Int32
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.VerifiedChains) != 1 || !r.TLS.PeerCertificates[0].Equal(node.Leaf) {
			t.Error("missing authenticated origin peer")
		}
		served.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	server := &http.Server{Handler: originHandler(binding.PublicURL, h, access, until), ConnContext: originConnContext, TLSConfig: secure, ReadHeaderTimeout: 2 * time.Second}
	go func() { _ = server.Serve(tls.NewListener(ln, secure)) }()
	defer ln.Close()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	target := "https://" + ln.Addr().String()
	publicHost := strings.TrimPrefix(binding.PublicURL, "https://")
	request := func(peer []tls.Certificate, method, host, path string) int {
		t.Helper()
		tr := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: name, Certificates: peer}, DisableKeepAlives: true}
		defer tr.CloseIdleConnections()
		client := &http.Client{Transport: tr, Timeout: 2 * time.Second}
		req, err := http.NewRequestWithContext(t.Context(), method, target+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = host
		res, err := client.Do(req)
		if err != nil {
			return 0 // TLS should fail before any HTTP is admitted.
		}
		_ = res.Body.Close()
		return res.StatusCode
	}
	wrongScope := scope
	wrongScope.ConnectorGeneration++
	for name, peer := range map[string][]tls.Certificate{"none": nil, "different key": {issuePeer(4, scope, newKey())}, "different scope": {issuePeer(5, wrongScope, nodeKey)}} {
		if status := request(peer, http.MethodHead, publicHost, frpOriginReadyPath); status != 0 {
			t.Fatalf("peer %s reached origin HTTP: status=%d", name, status)
		}
	}
	peer := []tls.Certificate{node}
	if got := request(peer, http.MethodHead, publicHost, frpOriginReadyPath); got != http.StatusNoContent {
		t.Fatalf("valid mTLS probe got %d", got)
	}
	if got := request(peer, http.MethodGet, publicHost, frpOriginReadyPath); got != http.StatusNotFound {
		t.Fatalf("non-probe method got %d", got)
	}
	if got := request(peer, http.MethodGet, publicHost, "/api/sessions"); got != http.StatusServiceUnavailable {
		t.Fatalf("unverified application request got %d", got)
	}
	access.status.Store(time.Now().Add(frpStatusTTL).UnixNano())
	if got := request(peer, http.MethodGet, "wrong.example", "/api/sessions"); got != http.StatusNotFound {
		t.Fatalf("wrong public Host got %d", got)
	}
	if got := request(peer, http.MethodGet, publicHost, "/api/sessions"); got != http.StatusOK || served.Load() != 1 {
		t.Fatalf("authenticated application got %d; served %d", got, served.Load())
	}
	access.status.Store(time.Now().Add(-time.Second).UnixNano())
	if got := request(peer, http.MethodGet, publicHost, "/api/sessions"); got != http.StatusServiceUnavailable {
		t.Fatalf("expired status left application open: %d", got)
	}
	access.end()
	if got := request(peer, http.MethodHead, publicHost, frpOriginReadyPath); got != 0 {
		t.Fatalf("ended allocation still accepted origin handshake: %d", got)
	}
}

func TestFRPGrantBindingAndImmutableRenewal(t *testing.T) {
	a := testAssignment(t, newTestIdentity(t))
	scope := protocol.Scope{HostID: a.Binding.HostID, BindingGeneration: a.Binding.BindingGeneration, ConnectorGeneration: 1, ControlEpoch: a.ControlEpoch, NodeEpoch: testToken(t)}
	s := &connection{assignment: a, scope: scope}
	id := testToken(t)
	now := time.Now().Unix()
	initial := protocol.AllocationGrant{AllocationID: id, GrantRevision: 1, ProxyName: "fixture", RemotePort: protocol.FRPPortMin, BridgeKey: testToken(t),
		TransportURL: "wss://relay.example/v1/frp/" + string(id), IssuedAt: now, NotBefore: now, ExpiresAt: now + 120, RenewAfter: now + 60}
	claim := func(g protocol.AllocationGrant, scope protocol.Scope) string {
		jti := testToken(t)
		claims := protocol.FRPClaims{Issuer: "https://control.example/v1/frp/oidc", Audience: protocol.FRPAudience(a.NodeID), Subject: string(g.AllocationID), ID: jti,
			IssuedAt: g.IssuedAt, NotBefore: g.NotBefore, ExpiresAt: g.ExpiresAt, Scope: scope, NodeID: a.NodeID, AllocationID: g.AllocationID,
			GrantRevision: g.GrantRevision, Role: protocol.FRPRole, ProxyName: g.ProxyName, RemotePort: g.RemotePort}
		body, err := json.Marshal(claims)
		if err != nil {
			t.Fatal(err)
		}
		return "e30." + base64.RawURLEncoding.EncodeToString(body) + ".c2ln"
	}
	initial.JWT = claim(initial, scope)
	if err := s.validateFRPGrant(&initial, nil); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*protocol.AllocationGrant){
		"foreign endpoint":       func(g *protocol.AllocationGrant) { g.TransportURL = "wss://other.example/v1/frp/" + string(id) },
		"stale revision":         func(g *protocol.AllocationGrant) { g.GrantRevision = 1; g.JWT = claim(*g, scope) },
		"different proxy":        func(g *protocol.AllocationGrant) { g.ProxyName += "-new"; g.JWT = claim(*g, scope) },
		"different origin trust": func(g *protocol.AllocationGrant) { g.Origin.TrustCertPEM = "different" },
		"wrong scope": func(g *protocol.AllocationGrant) {
			stale := scope
			stale.ConnectorGeneration++
			g.JWT = claim(*g, stale)
		},
	} {
		g := initial
		g.GrantRevision++
		g.JWT = claim(g, scope)
		if err := s.validateFRPGrant(&g, &initial); err != nil {
			t.Fatal(err)
		}
		change(&g)
		if err := s.validateFRPGrant(&g, &initial); err == nil {
			t.Fatal("accepted renewal with " + name)
		}
	}
}

func TestFRPTokenReplacedAtomicallyWithRestrictedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := replaceFRPToken(path, "old"); err != nil {
		t.Fatal(err)
	}
	old, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if err := replaceFRPToken(path, "new"); err != nil {
		t.Fatal(err)
	}
	previous, prevErr := io.ReadAll(old)
	current, currErr := os.ReadFile(path)
	info, statErr := os.Stat(path)
	if prevErr != nil || currErr != nil || statErr != nil || string(previous) != "old" || string(current) != "new" || info.Mode().Perm() != 0600 {
		t.Fatal(fmt.Errorf("token was not atomically replaced with restricted permissions: old=%v new=%v stat=%v", prevErr, currErr, statErr))
	}
}
