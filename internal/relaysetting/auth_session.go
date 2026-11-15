package relaysetting

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"sync"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// AuthSession is an opaque, non-bearer reference; callers cannot mint authority
// by constructing it. All mutable fields are guarded by its owning Auth's lock.
type AuthSession struct {
	owner    *Auth
	key      [32]byte
	id       string
	grant    protocol.BrowserGrant
	binding  AuthBinding
	csrf     [32]byte
	expires  time.Time
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	accesses map[*authAccess]struct{}
	peers    map[protocol.Token]*authPeer
}

type AuthPrincipal struct {
	ID      string
	GrantID protocol.Token
	Context context.Context
	Session *AuthSession
}

type AuthSessionStatus struct {
	Authenticated bool
	CSRFToken     string
	ExpiresAt     int64
}

type authAccess struct {
	close func()
	done  <-chan struct{}
	once  sync.Once
}

func (a *Auth) principal(s *AuthSession) AuthPrincipal {
	return AuthPrincipal{ID: s.id, GrantID: s.grant.ID, Context: s.ctx, Session: s}
}

// CSRF values are reproducible only with the in-memory per-Auth secret. The
// session stores their digest, never the raw value or the browser Cookie.
func (a *Auth) csrfToken(key [32]byte) string {
	mac := hmac.New(sha256.New, a.csrfKey[:])
	_, _ = mac.Write([]byte("WebMux-Relay-v1/host-csrf\x00"))
	_, _ = mac.Write(key[:])
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *Auth) newSession(handle string, grant protocol.BrowserGrant, binding AuthBinding, generation uint64, now time.Time) (AuthFinish, error) {
	expires := time.Unix(grant.ExpiresAt, 0)
	if limit := now.Add(a.config.SessionTTL).Truncate(time.Second); limit.Before(expires) {
		expires = limit
	}
	if !now.Before(expires) {
		return AuthFinish{}, protocol.Error{Code: protocol.Expired}
	}
	key := authDigest(handle)
	ctx, cancel := context.WithCancel(context.Background())
	s := &AuthSession{owner: a, key: key, id: base64.RawURLEncoding.EncodeToString(key[:12]), grant: grant, binding: binding, csrf: authDigest(a.csrfToken(key)), expires: expires, ctx: ctx, cancel: cancel, done: make(chan struct{}), accesses: make(map[*authAccess]struct{}), peers: make(map[protocol.Token]*authPeer)}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || generation != a.generation {
		cancel()
		return AuthFinish{}, protocol.Error{Code: protocol.Unavailable}
	}
	if len(a.sessions) >= a.config.MaxSessions {
		cancel()
		return AuthFinish{}, protocol.Error{Code: protocol.RateLimited}
	}
	if _, exists := a.sessions[key]; exists {
		cancel()
		return AuthFinish{}, protocol.Error{Code: protocol.Conflict}
	}
	for _, existing := range a.sessions {
		if existing.grant.ID == grant.ID {
			cancel()
			return AuthFinish{}, protocol.Error{Code: protocol.Replay}
		}
	}
	a.sessions[key] = s
	a.workers.Go(func() { a.sessionLifetime(s, expires.Sub(now)) })
	return AuthFinish{Cookie: authCookie(AuthCookieName, handle, now, expires), Principal: a.principal(s)}, nil
}

func (a *Auth) sessionLifetime(s *AuthSession, lifetime time.Duration) {
	timer := time.NewTimer(lifetime)
	defer timer.Stop()
	select {
	case <-timer.C:
		s.cancel()
	case <-s.ctx.Done():
	}
	a.mu.Lock()
	accesses := make([]*authAccess, 0, len(s.accesses))
	for access := range s.accesses {
		accesses = append(accesses, access)
	}
	peers := make([]*authPeer, 0, len(s.peers))
	for _, peer := range s.peers {
		peer.cancel()
		peers = append(peers, peer)
	}
	clear(s.peers)
	a.mu.Unlock()
	for _, access := range accesses {
		access.once.Do(access.close)
	}
	for _, access := range accesses {
		<-access.done
	}
	for _, peer := range peers {
		<-peer.done
	}
	a.mu.Lock()
	delete(a.sessions, s.key)
	close(s.done)
	a.mu.Unlock()
}

func (a *Auth) validLocked(principal AuthPrincipal, binding AuthBinding, now time.Time) (*AuthSession, error) {
	s := principal.Session
	if s == nil || s.owner != a || a.sessions[s.key] != s || principal.GrantID != s.grant.ID || principal.ID != s.id || s.ctx.Err() != nil || a.closed {
		return nil, protocol.Error{Code: protocol.Unauthenticated}
	}
	if !now.Before(s.expires) {
		s.cancel()
		return nil, protocol.Error{Code: protocol.Expired}
	}
	if binding != s.binding {
		s.cancel()
		return nil, protocol.Error{Code: protocol.Unauthenticated}
	}
	return s, nil
}

func (a *Auth) validatePrincipal(principal AuthPrincipal) (*AuthSession, error) {
	binding, err := a.snapshot()
	if err != nil {
		if s := principal.Session; s != nil && s.owner == a {
			s.cancel()
		}
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.validLocked(principal, binding, a.deps.Now())
}

func (a *Auth) Authenticate(cookie, publicOrigin string) (AuthPrincipal, error) {
	if protocol.Token(cookie).Validate() != nil {
		return AuthPrincipal{}, protocol.Error{Code: protocol.Unauthenticated}
	}
	binding, err := a.snapshot()
	if err != nil {
		return AuthPrincipal{}, err
	}
	if publicOrigin != binding.Binding.PublicURL {
		return AuthPrincipal{}, protocol.Error{Code: protocol.Forbidden}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.sessions[authDigest(cookie)]
	if s == nil {
		return AuthPrincipal{}, protocol.Error{Code: protocol.Unauthenticated}
	}
	principal := a.principal(s)
	if _, err := a.validLocked(principal, binding, a.deps.Now()); err != nil {
		return AuthPrincipal{}, err
	}
	return principal, nil
}

func (a *Auth) Status(principal AuthPrincipal) (AuthSessionStatus, error) {
	s, err := a.validatePrincipal(principal)
	if err != nil {
		return AuthSessionStatus{}, err
	}
	return AuthSessionStatus{Authenticated: true, CSRFToken: a.csrfToken(s.key), ExpiresAt: s.expires.Unix()}, nil
}

func (a *Auth) CheckOrigin(principal AuthPrincipal, origin string) error {
	s, err := a.validatePrincipal(principal)
	if err != nil {
		return err
	}
	if origin != s.binding.Binding.PublicURL {
		return protocol.Error{Code: protocol.Forbidden}
	}
	return nil
}

func (a *Auth) CheckMutation(principal AuthPrincipal, origin, csrf string) error {
	if err := a.CheckOrigin(principal, origin); err != nil {
		return err
	}
	if protocol.Token(csrf).Validate() != nil || !hmac.Equal(principal.Session.csrf[:], authDigestBytes(csrf)) {
		return protocol.Error{Code: protocol.Forbidden}
	}
	return nil
}

// RegisterAccess must run before an authenticated handler starts work. close
// must promptly interrupt its HTTP/WS/P2P/forward access, not the persistent PTY.
// done closes after all that access's children exit. release is called only
// after done closes; a concurrent revocation still joins its captured access.
func (a *Auth) RegisterAccess(principal AuthPrincipal, closeAccess func(), done <-chan struct{}) (func(), error) {
	if closeAccess == nil || done == nil {
		return nil, protocol.Error{Code: protocol.Invalid}
	}
	binding, err := a.snapshot()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.validLocked(principal, binding, a.deps.Now())
	if err != nil {
		return nil, err
	}
	access := &authAccess{close: closeAccess, done: done}
	s.accesses[access] = struct{}{}
	return func() {
		// A premature release cannot hide still-running access from an ACK.
		select {
		case <-done:
		default:
			return
		}
		a.mu.Lock()
		delete(s.accesses, access)
		a.mu.Unlock()
	}, nil
}

func (a *Auth) Logout(ctx context.Context, principal AuthPrincipal) error {
	s := principal.Session
	a.mu.Lock()
	if s == nil || s.owner != a || principal.GrantID != s.grant.ID || principal.ID != s.id {
		a.mu.Unlock()
		return protocol.Error{Code: protocol.Unauthenticated}
	}
	s.cancel()
	a.mu.Unlock()
	return authWait(ctx, []*AuthSession{s})
}

func authWait(ctx context.Context, sessions []*AuthSession) error {
	for _, s := range sessions {
		select {
		case <-s.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (a *Auth) Revoke(ctx context.Context, revocation protocol.Revocation) error {
	if err := revocation.Validate(); err != nil {
		return err
	}
	a.mu.Lock()
	a.generation++ // Fence an exchange already in flight, including console revocation.
	for key, tx := range a.transactions {
		if tx.binding.Scope.HostID == revocation.HostID && tx.binding.Scope.BindingGeneration == revocation.BindingGeneration {
			delete(a.transactions, key)
		}
	}
	var sessions []*AuthSession
	for _, s := range a.sessions {
		g := s.grant
		if g.HostID != revocation.HostID || g.BindingGeneration != revocation.BindingGeneration {
			continue
		}
		matches := revocation.Scope == "binding" || revocation.Scope == "connector" && g.ConnectorGeneration == revocation.ConnectorGeneration || revocation.Scope == "console-session" && g.ConsoleSessionID == revocation.ConsoleSessionID || revocation.Scope == "grant" && g.ID == revocation.GrantID
		if matches {
			s.cancel()
			sessions = append(sessions, s)
		}
	}
	a.mu.Unlock()
	return authWait(ctx, sessions)
}

func (a *Auth) InvalidateAll(ctx context.Context) error {
	a.mu.Lock()
	a.generation++
	clear(a.transactions)
	sessions := make([]*AuthSession, 0, len(a.sessions))
	for _, s := range a.sessions {
		s.cancel()
		sessions = append(sessions, s)
	}
	a.mu.Unlock()
	return authWait(ctx, sessions)
}

func (a *Auth) Close() error {
	a.mu.Lock()
	if !a.closed {
		a.closed = true
		close(a.stop)
	}
	a.mu.Unlock()
	err := a.InvalidateAll(context.Background())
	<-a.done
	a.workers.Wait()
	return err
}
