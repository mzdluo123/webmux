package relaysetting

import (
	"context"
	"strings"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

type authPeer struct {
	id          protocol.Token
	host        protocol.ICEEndpoint
	fingerprint string
	used        bool
	pending     bool
	expires     time.Time
	ctx         context.Context
	cancel      context.CancelFunc
	wake        chan struct{}
	done        chan struct{}
}

// AuthPeer binds the negotiated DTLS identity to an authenticated owner session.
// Context must govern the actual PeerConnection, not merely its signaling call.
type AuthPeer struct {
	PeerConnectionID protocol.Token
	Fingerprint      string
	Host             protocol.ICEEndpoint
	Context          context.Context
	Session          *AuthSession
}

func cloneAuthEndpoint(endpoint protocol.ICEEndpoint) protocol.ICEEndpoint {
	copyEndpoint := protocol.ICEEndpoint{Role: endpoint.Role, ExpiresAt: endpoint.ExpiresAt, Servers: make([]protocol.ICEServer, len(endpoint.Servers))}
	for i, server := range endpoint.Servers {
		copyEndpoint.Servers[i] = protocol.ICEServer{URLs: append([]string(nil), server.URLs...), Username: server.Username, Credential: server.Credential}
	}
	return copyEndpoint
}

// ICE never accepts a grant or an owner identity from the browser. The caller
// applies CheckMutation before invoking this method at the HTTP boundary.
func (a *Auth) ICE(ctx context.Context, principal AuthPrincipal, peerID protocol.Token) (protocol.ICEEndpoint, error) {
	if err := peerID.Validate(); err != nil {
		return protocol.ICEEndpoint{}, err
	}
	binding, err := a.snapshot()
	if err != nil {
		return protocol.ICEEndpoint{}, err
	}
	a.mu.Lock()
	s, err := a.validLocked(principal, binding, a.deps.Now())
	if err != nil {
		a.mu.Unlock()
		return protocol.ICEEndpoint{}, err
	}
	peer := s.peers[peerID]
	if peer != nil && (peer.pending || peer.ctx.Err() != nil) {
		a.mu.Unlock()
		return protocol.ICEEndpoint{}, protocol.Error{Code: protocol.Conflict}
	}
	if peer == nil {
		if len(s.peers) >= a.config.MaxPeersPerSession {
			a.mu.Unlock()
			return protocol.ICEEndpoint{}, protocol.Error{Code: protocol.RateLimited}
		}
		peerCtx, cancel := context.WithCancel(s.ctx)
		// One coalescing notification is sufficient: the watcher rereads the
		// latest deadline under the auth lock on every wake.
		peer = &authPeer{id: peerID, ctx: peerCtx, cancel: cancel, expires: a.deps.Now().Add(protocol.RequestTimeout), wake: make(chan struct{}, 1), done: make(chan struct{})}
		s.peers[peerID] = peer
		a.workers.Go(func() { a.peerLifetime(s, peer) })
	}
	peer.pending = true
	a.mu.Unlock()
	requestCtx, cancel := context.WithTimeout(ctx, protocol.RequestTimeout)
	stop := context.AfterFunc(peer.ctx, cancel)
	defer func() { stop(); cancel() }()
	config, requestErr := a.deps.Client.ICE(requestCtx, protocol.ICERequest{GrantID: principal.GrantID, PeerConnectionID: peerID})
	if requestErr == nil {
		requestErr = config.Validate(a.deps.Now())
	}
	if requestErr == nil && (config.GrantID != principal.GrantID || config.PeerConnectionID != peerID) {
		requestErr = protocol.Error{Code: protocol.Forbidden}
	}
	current, snapshotErr := a.snapshot()
	if requestErr == nil {
		requestErr = snapshotErr
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if requestErr == nil {
		_, requestErr = a.validLocked(principal, current, a.deps.Now())
	}
	if requestErr == nil && (s.peers[peerID] != peer || peer.ctx.Err() != nil) {
		requestErr = protocol.Error{Code: protocol.Expired}
	}
	peer.pending = false
	if requestErr != nil {
		// Failed refresh revokes the affected peer immediately; old TURN
		// credentials must not keep application access alive.
		peer.cancel()
		return protocol.ICEEndpoint{}, requestErr
	}
	peer.host = cloneAuthEndpoint(config.Host)
	peer.expires = time.Unix(config.ExpiresAt, 0)
	if s.expires.Before(peer.expires) {
		peer.expires = s.expires
	}
	select {
	case peer.wake <- struct{}{}:
	default:
	}
	return cloneAuthEndpoint(config.Browser), nil
}

func (a *Auth) peerLifetime(s *AuthSession, peer *authPeer) {
	defer close(peer.done)
	timer := time.NewTimer(protocol.RequestTimeout)
	defer timer.Stop()
	for {
		a.mu.Lock()
		remaining := peer.expires.Sub(a.deps.Now())
		if remaining <= 0 {
			peer.cancel()
		}
		a.mu.Unlock()
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(max(remaining, time.Nanosecond))
		select {
		case <-peer.ctx.Done():
			a.mu.Lock()
			if s.peers[peer.id] == peer {
				delete(s.peers, peer.id)
			}
			a.mu.Unlock()
			return
		case <-peer.wake:
		case <-timer.C:
		}
	}
}

func validAuthFingerprint(fingerprint string) bool {
	const prefix = "sha-256 "
	if len(fingerprint) != len(prefix)+32*2+31 || !strings.HasPrefix(fingerprint, prefix) {
		return false
	}
	for i := len(prefix); i < len(fingerprint); i++ {
		if (i-len(prefix))%3 == 2 {
			if fingerprint[i] != ':' {
				return false
			}
			continue
		}
		c := fingerprint[i]
		if (c < '0' || c > '9') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// AuthorizePeer atomically consumes the peer's one-time signaling authority.
// The fingerprint must come from the parsed offer's SHA-256 DTLS fingerprint,
// never a separate unverified browser field. A failed negotiation needs a new ID.
func (a *Auth) AuthorizePeer(principal AuthPrincipal, peerID protocol.Token, fingerprint string) (AuthPeer, error) {
	if peerID.Validate() != nil || !validAuthFingerprint(fingerprint) {
		return AuthPeer{}, protocol.Error{Code: protocol.Invalid}
	}
	binding, err := a.snapshot()
	if err != nil {
		return AuthPeer{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.validLocked(principal, binding, a.deps.Now())
	if err != nil {
		return AuthPeer{}, err
	}
	peer := s.peers[peerID]
	if peer == nil || peer.ctx.Err() != nil || !a.deps.Now().Before(peer.expires) || peer.host.Role != "host" {
		return AuthPeer{}, protocol.Error{Code: protocol.Unauthenticated}
	}
	if peer.used {
		return AuthPeer{}, protocol.Error{Code: protocol.Replay}
	}
	peer.used = true
	peer.fingerprint = fingerprint
	return AuthPeer{PeerConnectionID: peerID, Fingerprint: fingerprint, Host: cloneAuthEndpoint(peer.host), Context: peer.ctx, Session: s}, nil
}

// LookupPeer rechecks current session, exact peer, fingerprint and expiry. It
// cannot create a new signaling authorization or transfer one across sessions.
func (a *Auth) LookupPeer(principal AuthPrincipal, peerID protocol.Token, fingerprint string) (AuthPeer, error) {
	binding, err := a.snapshot()
	if err != nil {
		return AuthPeer{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.validLocked(principal, binding, a.deps.Now())
	if err != nil {
		return AuthPeer{}, err
	}
	peer := s.peers[peerID]
	if peer == nil || !peer.used || peer.fingerprint != fingerprint || peer.ctx.Err() != nil || !a.deps.Now().Before(peer.expires) {
		return AuthPeer{}, protocol.Error{Code: protocol.Unauthenticated}
	}
	return AuthPeer{PeerConnectionID: peerID, Fingerprint: fingerprint, Host: cloneAuthEndpoint(peer.host), Context: peer.ctx, Session: s}, nil
}
