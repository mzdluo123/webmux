package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// TestRelayIntegrationEnrollment is invoked by Relay's actual-control fixture.
// Only dialing is redirected; enrollment requests and response validation use
// the production Client with verified TLS and the shared wire protocol.
func TestRelayIntegrationEnrollment(t *testing.T) {
	path := os.Getenv("WEBMUX_RELAY_ENROLLMENT_FIXTURE")
	if path == "" {
		t.Skip("entrypoint for relay cross-repository integration")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Certificate []byte
		Target      string
		MachineID   protocol.MachineID
		WantCode    protocol.Code
		Output      string
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(fixture.Certificate)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	id := newTestIdentity(t)
	c, err := New(Config{ControlEndpoint: "https://example.com", ControlAudience: "test-control", LoopbackAddress: "127.0.0.1:12345", TLSConfig: &tls.Config{RootCAs: roots}, LoopbackTLSConfig: id.localTLS()})
	if err != nil {
		t.Fatal(err)
	}
	c.dialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, fixture.Target)
	}
	c.http.CloseIdleConnections()
	c.http = c.httpClient(nil)
	t.Cleanup(c.http.CloseIdleConnections)
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start, err := c.Enroll(ctx, key, fixture.MachineID)
	if err != nil {
		t.Fatal(err)
	}
	var result protocol.EnrollmentResult
	for range 10 {
		result, err = c.Poll(ctx, start.Handle)
		var remote protocol.Error
		if errors.As(err, &remote) && remote.Code == protocol.Pending {
			continue
		}
		break
	}
	if fixture.WantCode != "" {
		var remote protocol.Error
		if !errors.As(err, &remote) || remote.Code != fixture.WantCode {
			t.Fatalf("got %v; want %s", err, fixture.WantCode)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(fixture.Output, data, 0600); err != nil {
		t.Fatal(err)
	}
}
