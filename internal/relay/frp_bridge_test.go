package relay

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

func bridgeTestDial(t *testing.T, address, rawPath, tail string) (net.Conn, *bufio.Reader, int) {
	t.Helper()
	conn, err := net.DialTimeout("tcp4", address, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\nOrigin: http://%s\r\nAuthorization: Basic injected\r\nForwarded: for=spoof\r\nCookie: injected\r\n\r\n%s", rawPath, address, address, tail)
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	res, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	return conn, reader, res.StatusCode
}

func TestFRPBridgePinsEachActualWorkSocketAndRelaysBufferedBytes(t *testing.T) {
	id, unpinned := newTestIdentity(t), newTestIdentity(t)
	// Both leaves pass ordinary chain and hostname validation at relay.example.
	id.roots.AddCert(unpinned.server.Leaf)
	allocation, bridgeKey := testToken(t), testToken(t)
	path := "/v1/frp/" + string(allocation)
	var attempted atomic.Int32
	bad := tlsTestServer(t, unpinned, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempted.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	good := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "relay.example" || r.TLS.ServerName != "relay.example" || r.RequestURI != path ||
			r.Header.Get("Authorization") != "Bearer "+string(bridgeKey) || r.Header.Get("Origin") != "https://relay.example" ||
			r.Header.Get("Forwarded") != "" || r.Header.Get("Cookie") != "" {
			t.Error("bridge did not rewrite endpoint or strip untrusted headers")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		front, buffered, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer front.Close()
		_, err = buffered.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n\r\n")
		if err != nil || buffered.Flush() != nil {
			t.Error("cannot send Upgrade")
			return
		}
		var data [4]byte
		if _, err := io.ReadFull(buffered.Reader, data[:]); err != nil || string(data[:]) != "ping" {
			t.Error("the bridge lost buffered client bytes", err)
			return
		}
		_, _ = io.WriteString(front, "pong")
		_, _ = io.Copy(io.Discard, buffered.Reader)
	}))
	c := newTestClient(t, id, bad.Listener.Addr().String())
	probe, err := c.http.Get("https://relay.example")
	if err != nil {
		t.Fatalf("fixture unpinned certificate must pass ordinary TLS verification: %v", err)
	}
	_ = probe.Body.Close()
	if attempted.Swap(0) != 1 {
		t.Fatal("unpinned fixture did not receive the CA-only probe")
	}
	var dialTarget atomic.Value
	dialTarget.Store(good.Listener.Addr().String())
	c.dialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, dialTarget.Load().(string))
	}
	ln, err := listenFRP(frpBridgeSockets)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	access := &frpAccess{ctx: t.Context()}
	access.grant.Store(time.Now().Add(time.Minute).UnixNano())
	bridge := newFRPBridge(c, &id.trust, protocol.AllocationGrant{TransportURL: "wss://relay.example" + path, BridgeKey: bridgeKey}, access, ln.Addr().String())
	server := &http.Server{Handler: ln.track(bridge), ReadHeaderTimeout: 2 * time.Second}
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = server.Close() })

	for _, rejected := range []string{"/~%21frp", "/~!frp?x", "http://" + ln.Addr().String() + frpStockPath} {
		_, _, status := bridgeTestDial(t, ln.Addr().String(), rejected, "")
		if status != http.StatusNotFound {
			t.Fatalf("unexpected stock path variant returned %d", status)
		}
	}
	control, reader, status := bridgeTestDial(t, ln.Addr().String(), frpStockPath, "ping")
	if status != http.StatusSwitchingProtocols {
		t.Fatalf("valid pinned Upgrade returned %d", status)
	}
	var echo [4]byte
	if _, err := io.ReadFull(reader, echo[:]); err != nil || string(echo[:]) != "pong" {
		t.Fatalf("bridged bytes %q: %v", echo, err)
	}
	dialTarget.Store(bad.Listener.Addr().String())
	_, _, status = bridgeTestDial(t, ln.Addr().String(), frpStockPath, "")
	if status != http.StatusBadGateway || attempted.Load() != 0 {
		t.Fatalf("unpinned work Upgrade status=%d, actual upstream requests=%d", status, attempted.Load())
	}
	awaitManagement(t, bridge.badTLS)
	if !access.active(time.Now()) {
		t.Fatal("one bad work connection killed the healthy control connection")
	}
	_ = control.Close()
	awaitManagement(t, bridge.lost)
	if access.active(time.Now()) {
		t.Fatal("dead frp control socket left the bridge open to another Login")
	}
}
