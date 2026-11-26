package relay

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"testing"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/frphelper"
)

// TestRelayIntegrationHost is a subprocess entrypoint for webmux-relay's real
// two-repository regression suite. Configuration is an ephemeral private file.
func TestRelayIntegrationHost(t *testing.T) {
	path := os.Getenv("WEBMUX_RELAY_HOST_FIXTURE")
	if path == "" {
		t.Skip("entrypoint for relay cross-repository integration")
	}
	if err := frphelper.Available(); err != nil {
		t.Fatal("cross-repository host fixture requires the embedded stock frpc")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Assignment      protocol.Assignment
		Key             []byte
		Certificate     []byte
		Target          string
		SelectTransport bool
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
	callbacks := testCallbacks()
	var revoked bool
	callbacks.Revoke = func(_ context.Context, r protocol.Revocation) error {
		revoked = r.Scope == "connector" && r.Reason == "disconnected"
		return nil
	}
	callbacks.Origin = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
			t.Error("frp request did not arrive through the authenticated origin")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Path == "/hold" {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		_, _ = io.WriteString(w, strings.Repeat("asset", 256))
	})
	c, err := New(Config{ControlEndpoint: "https://example.com", ControlAudience: "test-control", TLSConfig: &tls.Config{RootCAs: roots}})
	if err != nil {
		t.Fatal(err)
	}
	c.dialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, fixture.Target)
	}
	c.assignment = &fixture.Assignment
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if fixture.SelectTransport {
		results := os.NewFile(3, "transport-results")
		defer results.Close()
		go func() {
			in := bufio.NewScanner(os.Stdin)
			for in.Scan() {
				want := protocol.FRPTransport(in.Text())
				var persisted bool
				scope, _ := c.Scope()
				err := c.ApplyTransportInScope(ctx, scope, want, func() error { persisted = true; return nil })
				result := struct {
					Status    TransportSelection
					Persisted bool
					Code      protocol.Code
				}{Status: c.TransportSelection(), Persisted: persisted}
				if err != nil {
					result.Code = protocolCode(err)
				}
				if json.NewEncoder(results).Encode(result) != nil {
					return
				}
			}
		}()
	}
	err = c.Connect(ctx, ed25519.PrivateKey(fixture.Key), fixture.Assignment, callbacks)
	if ctx.Err() == nil && (!revoked || protocolCode(err) != protocol.Forbidden) {
		t.Fatalf("connector exited before test shutdown or revocation: %v", err)
	}
}
