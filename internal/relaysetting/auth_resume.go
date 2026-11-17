package relaysetting

import (
	"context"
	"errors"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// Failure metadata has no grant or credential and is independently bounded.
// It lets a recently expired or revoked cookie explain why login is required.
type authFailure struct {
	origin string
	reason string
	until  time.Time
}

func (a *Auth) rememberFailureLocked(s *AuthSession, reason string) {
	if len(a.failures) >= a.config.MaxSessions {
		var oldest [32]byte
		var until time.Time
		for key, failure := range a.failures {
			if until.IsZero() || failure.until.Before(until) {
				oldest, until = key, failure.until
			}
		}
		delete(a.failures, oldest)
	}
	a.failures[s.key] = authFailure{origin: s.grant.Origin, reason: reason, until: a.deps.Now().Add(5 * time.Minute)}
}

func (a *Auth) retireLocked(s *AuthSession, reason string) {
	if !s.revoked {
		s.revoked = true
		a.rememberFailureLocked(s, reason)
	}
	s.cancel()
	if s.resumeCancel != nil {
		s.resumeCancel()
	}
	select {
	case <-s.done:
		if a.sessions[s.key] == s {
			delete(a.sessions, s.key)
		}
	default:
	}
}

func (a *Auth) pruneSessionsLocked(now time.Time) {
	for key, failure := range a.failures {
		if !now.Before(failure.until) {
			delete(a.failures, key)
		}
	}
	for _, s := range a.sessions {
		if !now.Before(s.expires) {
			a.retireLocked(s, "expired")
		}
	}
}

func (a *Auth) suspendLocked(s *AuthSession) {
	if !s.revoked && !s.suspended {
		s.suspended = true
		s.cancel()
	}
}

// SuspendAll immediately denies authority and joins derived access, not PTYs.
// The bounded session records retain their original absolute expiry only.
func (a *Auth) SuspendAll(ctx context.Context) error {
	a.mu.Lock()
	a.generation++
	clear(a.transactions)
	sessions := make([]*AuthSession, 0, len(a.sessions))
	for _, s := range a.sessions {
		a.suspendLocked(s)
		if s.resumeCancel != nil {
			s.resumeCancel()
		}
		sessions = append(sessions, s)
	}
	a.mu.Unlock()
	return authWait(ctx, sessions)
}

// Probe is used only after the API has authenticated the connector ingress and
// matched its public origin. Unknown cookies never reveal control availability.
func (a *Auth) Probe(cookie, publicOrigin string) AuthSessionStatus {
	principal, err := a.Authenticate(cookie, publicOrigin)
	if err == nil {
		if status, statusErr := a.Status(principal); statusErr == nil {
			return status
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.deps.Now()
	key := authDigest(cookie)
	status := AuthSessionStatus{Relay: true, Reason: "unauthenticated"}
	if failure, ok := a.failures[key]; ok && failure.origin == publicOrigin && now.Before(failure.until) {
		status.Reason = failure.reason
	} else if s := a.sessions[key]; s != nil && !s.revoked && s.grant.Origin == publicOrigin {
		if !now.Before(s.expires) {
			a.retireLocked(s, "expired")
			status.Reason = "expired"
		} else {
			status.Reason = "reconnecting"
			status.ExpiresAt = s.expires.Unix()
		}
	}
	return status
}

func (a *Auth) resumeLocked(s *AuthSession, binding AuthBinding, now time.Time) {
	if s.resuming || s.revoked || a.closed || now.Before(s.retryAfter) {
		return
	}
	// Rebinding, rekeying or a different control authority requires new login.
	if binding.Binding != s.binding.Binding || binding.ControlOrigin != s.binding.ControlOrigin {
		a.retireLocked(s, "revoked")
		return
	}
	s.resuming = true
	generation := a.generation
	ctx, cancel := context.WithTimeout(context.Background(), protocol.RequestTimeout)
	s.resumeCancel = cancel
	a.workers.Go(func() { a.resumeSession(ctx, cancel, s, binding, generation) })
}

func (a *Auth) resumeSession(ctx context.Context, cancel context.CancelFunc, s *AuthSession, binding AuthBinding, generation uint64) {
	defer cancel()
	// No fresh context may coexist with old HTTP/WS/peer children. This runs
	// outside Ready and the control reader, so RPC startup and revoke can proceed.
	select {
	case <-s.done:
	case <-ctx.Done():
	}
	grant := protocol.BrowserGrant{}
	err := ctx.Err()
	if err == nil {
		grant, err = a.deps.Client.Resume(ctx, protocol.LoginResume{GrantID: s.grant.ID})
	}
	if err == nil {
		err = grant.Validate(a.deps.Now())
	}
	if err == nil && (!authGrantMatches(grant, binding) || grant.ID != s.grant.ID || grant.ConsoleSessionID != s.grant.ConsoleSessionID || grant.OwnerID != s.grant.OwnerID || grant.HostID != s.grant.HostID || grant.Origin != s.grant.Origin || grant.ExpiresAt != s.grant.ExpiresAt) {
		err = protocol.Error{Code: protocol.Forbidden}
	}
	current, snapshotErr := a.snapshot()
	a.mu.Lock()
	defer a.mu.Unlock()
	s.resuming = false
	s.resumeCancel = nil
	if a.closed || a.sessions[s.key] != s || s.revoked || generation != a.generation {
		return
	}
	now := a.deps.Now()
	if ctx.Err() != nil {
		s.retryAfter = now.Add(time.Second)
		return
	}
	if !now.Before(s.expires) {
		a.retireLocked(s, "expired")
		return
	}
	if err != nil {
		var remote protocol.Error
		if errors.As(err, &remote) {
			switch remote.Code {
			case protocol.Expired:
				a.retireLocked(s, "expired")
				return
			case protocol.Unavailable, protocol.RateLimited, protocol.StaleGeneration:
			default:
				a.retireLocked(s, "revoked")
				return
			}
		}
	}
	if err != nil || snapshotErr != nil || current != binding {
		s.retryAfter = now.Add(time.Second)
		return
	}
	accessCtx, accessCancel := context.WithCancel(context.Background())
	resumed := &AuthSession{owner: a, key: s.key, id: s.id, grant: grant, binding: binding, csrf: s.csrf, expires: s.expires, ctx: accessCtx, cancel: accessCancel, done: make(chan struct{}), accesses: make(map[*authAccess]struct{}), peers: make(map[protocol.Token]*authPeer)}
	a.sessions[s.key] = resumed
	a.workers.Go(func() { a.sessionLifetime(resumed) })
}
