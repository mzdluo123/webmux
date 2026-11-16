package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"testing"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// TestRelayIntegrationHost is a subprocess entrypoint for webmux-relay's real
// two-repository regression suite. Configuration is an ephemeral private file.
func TestRelayIntegrationHost(t *testing.T) {
	path := os.Getenv("WEBMUX_RELAY_HOST_FIXTURE")
	if path == "" {
		t.Skip("entrypoint for relay cross-repository integration")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Assignment  protocol.Assignment
		Key         []byte
		Certificate []byte
		Target      string
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(fixture.Certificate)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	id := newTestIdentity(t)
	local := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hold" {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		_, _ = io.WriteString(w, strings.Repeat("asset", 256))
	}), true)
	c, err := New(Config{ControlEndpoint: "https://example.com", ControlAudience: "test-control", LoopbackAddress: local.Listener.Addr().String(), TLSConfig: &tls.Config{RootCAs: roots}, LoopbackTLSConfig: id.localTLS()})
	if err != nil {
		t.Fatal(err)
	}
	c.dialContext = newTestClient(t, id, fixture.Target, local.Listener.Addr().String()).dialContext
	c.assignment = &fixture.Assignment
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	err = c.Connect(ctx, ed25519.PrivateKey(fixture.Key), fixture.Assignment, testCallbacks())
	if ctx.Err() == nil {
		t.Fatalf("connector exited before test shutdown: %v", err)
	}
}
