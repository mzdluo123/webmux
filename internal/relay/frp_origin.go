package relay

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

const frpOriginReadyPath = "/_webmux/relay-origin/ready"

type frpAccess struct {
	ctx   context.Context
	ended atomic.Bool
	grant atomic.Int64
}

func (a *frpAccess) active(now time.Time) bool {
	return a.ctx.Err() == nil && !a.ended.Load() && now.UnixNano() < a.grant.Load()
}

func (a *frpAccess) end() { a.ended.Store(true) }

func newOriginCSR() (*ecdsa.PrivateKey, string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, "", err
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		return nil, "", err
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})), nil
}

func parseOriginCertificates(input string) ([]*x509.Certificate, error) {
	var out []*x509.Certificate
	for rest := []byte(input); len(rest) != 0; {
		block, next := pem.Decode(rest)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, protocol.Error{Code: protocol.Unauthenticated}
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, protocol.Error{Code: protocol.Unauthenticated}
		}
		out = append(out, cert)
		rest = next
	}
	if len(out) == 0 {
		return nil, protocol.Error{Code: protocol.Unauthenticated}
	}
	return out, nil
}

func originLeaf(leaf *x509.Certificate, uri string, eku x509.ExtKeyUsage) bool {
	key, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	return ok && key.Curve == elliptic.P256() && !leaf.IsCA && leaf.KeyUsage == x509.KeyUsageDigitalSignature &&
		len(leaf.ExtKeyUsage) == 1 && leaf.ExtKeyUsage[0] == eku && len(leaf.UnknownExtKeyUsage) == 0 &&
		len(leaf.IPAddresses)+len(leaf.EmailAddresses) == 0 && len(leaf.URIs) == 1 && leaf.URIs[0].String() == uri &&
		leaf.NotAfter.Sub(leaf.NotBefore) <= 24*time.Hour
}

func originTLS(scope protocol.Scope, key *ecdsa.PrivateKey, grant protocol.AllocationGrant, access *frpAccess) (*tls.Config, time.Time, error) {
	deny := protocol.Error{Code: protocol.Unauthenticated}
	roots, err := parseOriginCertificates(grant.Origin.TrustCertPEM)
	if err != nil {
		return nil, time.Time{}, deny
	}
	chain, err := parseOriginCertificates(grant.Origin.HostServerCertPEM)
	if err != nil {
		return nil, time.Time{}, deny
	}
	leaf := chain[0]
	name := protocol.FRPOriginServerName(grant.AllocationID)
	if !key.PublicKey.Equal(leaf.PublicKey) || !originLeaf(leaf, protocol.FRPOriginURI(scope, grant.AllocationID, "host").String(), x509.ExtKeyUsageServerAuth) ||
		len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != name {
		return nil, time.Time{}, deny
	}
	pool := x509.NewCertPool()
	expires := leaf.NotAfter
	for _, cert := range roots {
		if !cert.IsCA || !cert.BasicConstraintsValid || cert.KeyUsage&x509.KeyUsageCertSign == 0 {
			return nil, time.Time{}, deny
		}
		pool.AddCert(cert)
		expires = minTime(expires, cert.NotAfter)
	}
	der := make([][]byte, len(chain))
	intermediates := x509.NewCertPool()
	for i, cert := range chain {
		der[i] = cert.Raw
		if i > 0 {
			intermediates.AddCert(cert)
			expires = minTime(expires, cert.NotAfter)
		}
	}
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: name, Roots: pool, Intermediates: intermediates}); err != nil || time.Unix(grant.ExpiresAt, 0).After(expires) {
		return nil, time.Time{}, deny
	}
	var pin [sha256.Size]byte
	if n, err := hex.Decode(pin[:], []byte(grant.Origin.NodeClientSPKI)); err != nil || n != len(pin) {
		return nil, time.Time{}, deny
	}
	peerURI := protocol.FRPOriginURI(scope, grant.AllocationID, "node").String()
	return &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, NextProtos: []string{"http/1.1"}, SessionTicketsDisabled: true,
		Certificates: []tls.Certificate{{Certificate: der, PrivateKey: key, Leaf: leaf}}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool,
		VerifyConnection: func(state tls.ConnectionState) error {
			if !access.active(time.Now()) || state.ServerName != name || len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 {
				return deny
			}
			peer := state.PeerCertificates[0]
			if !originLeaf(peer, peerURI, x509.ExtKeyUsageClientAuth) || len(peer.DNSNames) != 0 ||
				sha256.Sum256(peer.RawSubjectPublicKeyInfo) != pin {
				return deny
			}
			return nil
		},
	}, expires, nil
}

type originConnKey struct{}

func originConnContext(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, originConnKey{}, c.(*tls.Conn).NetConn().(*frpConn))
}

func originHandler(publicURL string, origin http.Handler, access *frpAccess, originUntil time.Time) http.Handler {
	public, _ := url.Parse(publicURL) // Bound during Assignment validation.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != public.Host || r.URL.IsAbs() || r.Method == http.MethodConnect || r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		now := time.Now()
		if !access.active(now) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		peerUntil := originUntil
		for _, cert := range r.TLS.VerifiedChains[0] {
			peerUntil = minTime(peerUntil, cert.NotAfter)
		}
		if !now.Before(peerUntil) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if peerUntil.Before(originUntil) {
			c := r.Context().Value(originConnKey{}).(*frpConn)
			c.expireAt(peerUntil)
		}
		if r.URL.Path == frpOriginReadyPath {
			if r.Method != http.MethodHead || r.RequestURI != frpOriginReadyPath {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		origin.ServeHTTP(w, r)
	})
}

// The HTTP server no longer owns hijacked conns; retain the underlying TCP
// socket from Accept through close so cancellation also ends WebSocket access.
type frpListener struct {
	net.Listener
	max    int
	mu     sync.Mutex
	closed bool
	conns  map[*frpConn]struct{}
	work   sync.WaitGroup
}

type frpConn struct {
	net.Conn
	owner  *frpListener
	expiry *time.Timer // Protected by owner.mu.
}

func listenFRP(maxConns int) (*frpListener, error) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	return &frpListener{Listener: ln, max: maxConns, conns: make(map[*frpConn]struct{})}, nil
}

func (l *frpListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		l.mu.Lock()
		if !l.closed && len(l.conns) < l.max {
			tracked := &frpConn{Conn: c, owner: l}
			l.conns[tracked] = struct{}{}
			l.mu.Unlock()
			return tracked, nil
		}
		l.mu.Unlock()
		_ = c.Close()
	}
}

func (l *frpListener) track(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			return
		}
		l.work.Add(1)
		l.mu.Unlock()
		defer l.work.Done()
		h.ServeHTTP(w, r)
	})
}

func (l *frpListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	err := l.Listener.Close()
	for c := range l.conns {
		if c.expiry != nil {
			c.expiry.Stop()
		}
		_ = c.Conn.Close()
	}
	clear(l.conns)
	return err
}

func (c *frpConn) Close() error {
	err := c.Conn.Close()
	c.owner.mu.Lock()
	if c.expiry != nil {
		c.expiry.Stop()
	}
	delete(c.owner.conns, c)
	c.owner.mu.Unlock()
	return err
}

func (c *frpConn) expireAt(at time.Time) {
	c.owner.mu.Lock()
	defer c.owner.mu.Unlock()
	if _, tracked := c.owner.conns[c]; tracked && c.expiry == nil {
		c.expiry = time.AfterFunc(time.Until(at), func() { _ = c.Close() })
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
