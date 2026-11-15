package relay

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

type testIdentity struct {
	roots  *x509.CertPool
	server tls.Certificate
	client tls.Certificate
	trust  protocol.ServiceTrust
}

func newTestIdentity(t *testing.T) testIdentity {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Relay test CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	issue := func(serial int64, client bool) tls.Certificate {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		cert := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, DNSNames: []string{"relay.example", "webmux-relay-loopback"}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		if client {
			cert.DNSNames = nil
			cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		}
		der, err := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		pair, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
		if err != nil {
			t.Fatal(err)
		}
		pair.Leaf, err = x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return pair
	}
	server := issue(2, false)
	pin := sha256.Sum256(server.Leaf.RawSubjectPublicKeyInfo)
	return testIdentity{roots: roots, server: server, client: issue(3, true), trust: protocol.ServiceTrust{Algorithm: protocol.ServiceAlgorithm, Pins: []string{base64.RawURLEncoding.EncodeToString(pin[:])}}}
}

func (id testIdentity) localTLS() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: id.roots, ServerName: "webmux-relay-loopback", Certificates: []tls.Certificate{id.client}}
}

func testBinding() protocol.Binding {
	return protocol.Binding{HostID: "0123456789abcdef0123456789abcdef", OwnerID: "123", BindingGeneration: 1, PublicURL: "https://h-0123456789abcdef0123456789abcdef.webmux.example"}
}

func testToken(t *testing.T) protocol.Token {
	t.Helper()
	token, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func testKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func testAssignment(t *testing.T, id testIdentity) protocol.Assignment {
	return protocol.Assignment{Version: protocol.V1, Revision: 1, ControlEpoch: testToken(t), ControlAudience: "control.example", Source: "authenticated-control", ExpiresAt: time.Now().Add(25 * time.Second).Unix(), Binding: testBinding(), NodeID: "test-node", Capabilities: protocol.Capabilities{protocol.CapabilityWSS}, ConfigRevision: 1, ConnectorURL: "wss://relay.example/v1/connector", DataOrigin: "https://relay.example", NodeAudience: "node.example", Trust: id.trust}
}

func newTestClient(t *testing.T, id testIdentity, target, loopback string) *Client {
	t.Helper()
	c, err := New(Config{ControlEndpoint: "https://relay.example", ControlAudience: "control.example", LoopbackAddress: loopback, InitialTrust: id.trust, TLSConfig: &tls.Config{RootCAs: id.roots}, LoopbackTLSConfig: id.localTLS()})
	if err != nil {
		t.Fatal(err)
	}
	// Only tests redirect network dialing to isolated ephemeral listeners.
	// Production URLs, TLS hostname, chain and SPKI verification remain real.
	c.dialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, target)
	}
	c.http.CloseIdleConnections()
	c.http = c.httpClient(c.config.InitialTrust)
	t.Cleanup(c.http.CloseIdleConnections)
	return c
}

func tlsTestServer(t *testing.T, id testIdentity, handler http.Handler, local bool) *httptest.Server {
	t.Helper()
	s := httptest.NewUnstartedServer(handler)
	s.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{id.server}}
	if local {
		s.TLS.ClientAuth = tls.RequireAndVerifyClientCert
		s.TLS.ClientCAs = id.roots
	}
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}

func mustJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Error(err)
	}
}
