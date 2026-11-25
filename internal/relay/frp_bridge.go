package relay

import (
	"crypto/sha1" // WebSocket's standard Sec-WebSocket-Accept digest.
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

const (
	frpStockPath = "/~!frp"
	// One frp control socket, 64 independently admitted origin requests, and
	// room for stock frps's work queue and handshakes. Not a business quota.
	frpBridgeSockets = 96
	frpBridgeBurst   = 32
	frpBridgeRate    = 2 * frpBridgeSockets
)

type frpBridge struct {
	access     *frpAccess
	transport  *http.Transport
	target     string
	targetHost string
	bridgeKey  string
	localHost  string
	login      atomic.Bool
	lost       chan struct{}
	badTLS     chan struct{}
	mu         sync.Mutex
	last       time.Time
	tokens     float64
}

func newFRPBridge(c *Client, trust *protocol.ServiceTrust, g protocol.AllocationGrant, access *frpAccess, localHost string) *frpBridge {
	// The authenticated grant validator checked this exact URL. RoundTrip never
	// follows redirects; a new TLS handshake verifies the pin on every socket.
	target, _ := url.Parse(g.TransportURL)
	target.Scheme = "https"
	transport := c.httpClient(trust).Transport.(*http.Transport)
	transport.DisableKeepAlives = true
	transport.MaxConnsPerHost = frpBridgeSockets
	return &frpBridge{access: access, transport: transport, target: target.String(), targetHost: target.Host,
		bridgeKey: "Bearer " + string(g.BridgeKey), localHost: localHost, lost: make(chan struct{}), badTLS: make(chan struct{}, 1), tokens: frpBridgeBurst}
}

func (b *frpBridge) take(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.last.IsZero() {
		b.tokens = min(frpBridgeBurst, b.tokens+now.Sub(b.last).Seconds()*frpBridgeRate)
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func hasUpgrade(h http.Header) bool {
	for _, header := range h.Values("Connection") {
		for token := range strings.SplitSeq(header, ",") {
			if strings.EqualFold(strings.TrimSpace(token), "upgrade") {
				return true
			}
		}
	}
	return false
}

func (b *frpBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// RequestURI enforces the literal stock path, not an escaped variant and
	// not an absolute URI, even if net/http would normalize them equivalently.
	if r.Method != http.MethodGet || r.RequestURI != frpStockPath || r.Host != b.localHost {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	decoded, err := base64.StdEncoding.Strict().DecodeString(key)
	if r.Proto != "HTTP/1.1" || r.ContentLength != 0 || len(r.TransferEncoding) != 0 || !hasUpgrade(r.Header) ||
		!strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || r.Header.Get("Sec-WebSocket-Version") != "13" ||
		len(r.Header.Values("Sec-WebSocket-Key")) != 1 || err != nil || len(decoded) != 16 ||
		r.Header.Get("Sec-WebSocket-Protocol") != "" || r.Header.Get("Sec-WebSocket-Extensions") != "" ||
		r.Header.Get("Origin") != "http://"+b.localHost {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	now := time.Now()
	if !b.access.active(now) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if !b.take(now) {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	// Construct a new head from an allowlist. In particular, no client-supplied
	// authorization, forwarding, Cookie, Origin or proxy headers survive.
	out, err := http.NewRequestWithContext(r.Context(), http.MethodGet, b.target, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	out.Header.Set("Connection", "Upgrade")
	out.Header.Set("Upgrade", "websocket")
	out.Header.Set("Sec-WebSocket-Version", "13")
	out.Header.Set("Sec-WebSocket-Key", key)
	out.Header.Set("Origin", "https://"+b.targetHost)
	out.Header.Set("Authorization", b.bridgeKey)
	res, err := b.transport.RoundTrip(out)
	if err != nil {
		if protocolCode(transportError(r.Context(), err)) == protocol.Unauthenticated {
			select {
			case b.badTLS <- struct{}{}:
			default:
			}
		}
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusSwitchingProtocols {
		switch res.StatusCode {
		case http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusServiceUnavailable:
			w.WriteHeader(res.StatusCode)
		default:
			w.WriteHeader(http.StatusBadGateway)
		}
		return
	}
	accept := res.Header.Get("Sec-WebSocket-Accept")
	digest := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	back, ok := res.Body.(io.ReadWriteCloser)
	if !ok || !hasUpgrade(res.Header) || !strings.EqualFold(res.Header.Get("Upgrade"), "websocket") ||
		accept != base64.StdEncoding.EncodeToString(digest[:]) || res.Header.Get("Sec-WebSocket-Protocol") != "" || res.Header.Get("Sec-WebSocket-Extensions") != "" {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !b.access.active(time.Now()) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	// Stock frpc cannot open work sockets before its first Upgrade has carried
	// Login and registered the proxy. After that socket is lost, deny all new
	// sockets before its one-use Login token could be retried.
	if b.login.CompareAndSwap(false, true) {
		defer func() {
			b.access.end()
			close(b.lost)
		}()
	}
	front, buffered, err := http.NewResponseController(w).Hijack()
	if err != nil {
		return
	}
	defer func() { _ = front.Close() }()
	// Match the HTTP/1.1 Upgrade, then relay only the negotiated bytes. The
	// buffered reader includes any optimistic bytes sent with the handshake.
	_ = front.SetWriteDeadline(time.Now().Add(protocol.AuthTimeout))
	_, err = fmt.Fprintf(buffered, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
	if err != nil || buffered.Flush() != nil {
		return
	}
	_ = front.SetDeadline(time.Time{})
	finished := make(chan struct{})
	go func() {
		_, _ = io.Copy(back, buffered.Reader)
		_ = back.Close()
		close(finished)
	}()
	_, _ = io.Copy(front, back)
	_ = front.Close()
	_ = back.Close()
	<-finished
}
