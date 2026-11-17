package relaysetting

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

func waitAuthCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("authentication transition did not complete")
		}
		time.Sleep(time.Millisecond)
	}
}

func recoverAuth(t *testing.T, auth *Auth, finish AuthFinish) AuthPrincipal {
	t.Helper()
	var principal AuthPrincipal
	waitAuthCondition(t, func() bool {
		var err error
		principal, err = auth.Authenticate(finish.Cookie.Value, finish.Principal.Session.grant.Origin)
		return err == nil
	})
	return principal
}

func nextAuthScope(control *authTestControl) {
	control.mu.Lock()
	control.offline = false
	control.binding.Scope.ConnectorGeneration++
	control.binding.Scope.ControlEpoch = authTestToken(70)
	control.binding.Scope.NodeEpoch = authTestToken(71)
	control.mu.Unlock()
}

func TestAuthSuspendResumePreservesCookieAndAbsoluteExpiry(t *testing.T) {
	auth, control, now := newAuthTest(t, AuthConfig{MaxSessions: 1, SessionTTL: time.Minute})
	finish := finishAuthLogin(t, auth, control, 10)
	origin := finish.Principal.Session.grant.Origin
	before, err := auth.Status(finish.Principal)
	if err != nil {
		t.Fatal(err)
	}
	authTestICE(t, auth, control)
	peerID := authTestToken(60)
	if _, err := auth.ICE(context.Background(), finish.Principal, peerID); err != nil {
		t.Fatal(err)
	}
	fingerprint := "sha-256 " + strings.TrimSuffix(strings.Repeat("AB:", 32), ":")
	peer, err := auth.AuthorizePeer(finish.Principal, peerID, fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	accessDone := make(chan struct{})
	release, err := auth.RegisterAccess(finish.Principal, func() { close(accessDone) }, accessDone)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	control.mu.Lock()
	control.offline = true
	control.mu.Unlock()
	if err := auth.SuspendAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if finish.Principal.Context.Err() == nil || peer.Context.Err() == nil {
		t.Fatal("disconnect left an access epoch or peer alive")
	}
	select {
	case <-accessDone:
	default:
		t.Fatal("disconnect did not close registered access")
	}
	status := auth.Probe(finish.Cookie.Value, origin)
	if status.Authenticated || !status.Relay || status.Reason != "reconnecting" || status.CSRFToken != "" || status.ExpiresAt != before.ExpiresAt {
		t.Fatalf("suspended status = %+v", status)
	}
	if status := auth.Probe(string(authTestToken(90)), origin); status.Reason != "unauthenticated" {
		t.Fatalf("unknown cookie exposed connector state: %+v", status)
	}
	now.Add(30)
	nextAuthScope(control)
	resumed := recoverAuth(t, auth, finish)
	after, err := auth.Status(resumed)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Session == finish.Principal.Session || resumed.Context == finish.Principal.Context || resumed.Context.Err() != nil || resumed.ID != finish.Principal.ID || resumed.GrantID != finish.Principal.GrantID || after.ExpiresAt != before.ExpiresAt || after.CSRFToken != before.CSRFToken {
		t.Fatal("resume changed browser identity/deadline or reused stale authority")
	}
	binding, _ := control.snapshot()
	if !authGrantMatches(resumed.Session.grant, binding) {
		t.Fatal("resumed grant retained old scope")
	}
	_, err = auth.Status(finish.Principal)
	requireAuthCode(t, err, protocol.Unauthenticated)
	requireAuthCode(t, auth.CheckMutation(finish.Principal, origin, before.CSRFToken), protocol.Unauthenticated)
	_, err = auth.RegisterAccess(finish.Principal, func() {}, accessDone)
	requireAuthCode(t, err, protocol.Unauthenticated)
	_, err = auth.ICE(context.Background(), finish.Principal, peerID)
	requireAuthCode(t, err, protocol.Unauthenticated)
	_, err = auth.LookupPeer(resumed, peerID, fingerprint)
	requireAuthCode(t, err, protocol.Unauthenticated)
	requireAuthCode(t, auth.Logout(context.Background(), finish.Principal), protocol.Unauthenticated)
	forgedEpoch := resumed
	forgedEpoch.Context = finish.Principal.Context
	_, err = auth.Status(forgedEpoch)
	requireAuthCode(t, err, protocol.Unauthenticated)
	now.Add(30)
	status = auth.Probe(finish.Cookie.Value, origin)
	if status.Authenticated || status.Reason != "expired" || resumed.Context.Err() == nil {
		t.Fatalf("resume extended absolute expiry: %+v", status)
	}
}

func TestAuthResumeSingleFlightAndJoinsOldAccess(t *testing.T) {
	auth, control, _ := newAuthTest(t, AuthConfig{})
	finish := finishAuthLogin(t, auth, control, 10)
	closing, done := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}()
	if _, err := auth.RegisterAccess(finish.Principal, func() { close(closing) }, done); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = auth.SuspendAll(ctx)
	<-closing
	nextAuthScope(control)
	entered, proceed := make(chan struct{}), make(chan struct{})
	defer close(proceed)
	control.mu.Lock()
	control.resumeEntered, control.resumeProceed = entered, proceed
	control.mu.Unlock()
	const requests = 32
	var workers sync.WaitGroup
	for range requests {
		workers.Go(func() {
			_, err := auth.Authenticate(finish.Cookie.Value, finish.Principal.Session.grant.Origin)
			if err == nil {
				t.Error("pending access was authorized")
			}
		})
	}
	workers.Wait()
	control.mu.Lock()
	resumes := control.resumes
	control.mu.Unlock()
	if resumes != 0 {
		t.Fatal("resume began before old access drained")
	}
	close(done)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("resume did not start after old access drained")
	}
	for range requests {
		_, err := auth.Authenticate(finish.Cookie.Value, finish.Principal.Session.grant.Origin)
		requireAuthCode(t, err, protocol.Unavailable)
	}
	control.mu.Lock()
	resumes = control.resumes
	control.mu.Unlock()
	if resumes != 1 {
		t.Fatalf("concurrent requests issued %d resumes", resumes)
	}
}

func TestAuthResumeCannotRaceRevocationOrDisconnect(t *testing.T) {
	for _, operation := range []string{"grant", "console-session", "binding", "connector", "invalidate", "logout", "browser logout", "expiry", "disconnect"} {
		t.Run(operation, func(t *testing.T) {
			auth, control, now := newAuthTest(t, AuthConfig{SessionTTL: time.Minute})
			finish := finishAuthLogin(t, auth, control, 10)
			if err := auth.SuspendAll(context.Background()); err != nil {
				t.Fatal(err)
			}
			nextAuthScope(control)
			entered, proceed := make(chan struct{}), make(chan struct{})
			defer func() {
				select {
				case <-proceed:
				default:
					close(proceed)
				}
			}()
			control.mu.Lock()
			control.resumeEntered, control.resumeProceed, control.resumeIgnoreCancel = entered, proceed, true
			control.mu.Unlock()
			_, err := auth.Authenticate(finish.Cookie.Value, finish.Principal.Session.grant.Origin)
			requireAuthCode(t, err, protocol.Unavailable)
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("resume did not enter control plane")
			}
			switch operation {
			case "invalidate":
				err = auth.InvalidateAll(context.Background())
			case "browser logout":
				err = auth.LogoutBrowser(context.Background(), finish.Cookie.Value, finish.Principal.Session.grant.Origin, auth.csrfToken(finish.Principal.Session.key))
			case "logout":
				err = auth.Logout(context.Background(), finish.Principal)
			case "expiry":
				now.Add(60)
				err = nil
			case "disconnect":
				err = auth.SuspendAll(context.Background())
			default:
				target := finish.Principal.GrantID
				if operation == "console-session" {
					target = finish.Principal.Session.grant.ConsoleSessionID
				}
				revocation := authTestRevocation(control, operation, target)
				if operation == "connector" {
					revocation.ConnectorGeneration = finish.Principal.Session.grant.ConnectorGeneration
				}
				err = auth.Revoke(context.Background(), revocation)
			}
			if err != nil {
				t.Fatal(err)
			}
			close(proceed)
			waitAuthCondition(t, func() bool {
				auth.mu.Lock()
				defer auth.mu.Unlock()
				return !finish.Principal.Session.resuming
			})
			auth.mu.Lock()
			current := auth.sessions[authDigest(finish.Cookie.Value)]
			if current != nil && current != finish.Principal.Session {
				t.Error("late control reply restored fenced authority")
			}
			auth.mu.Unlock()
			if operation == "disconnect" {
				nextAuthScope(control)
				recoverAuth(t, auth, finish)
				return
			}
			status := auth.Probe(finish.Cookie.Value, finish.Principal.Session.grant.Origin)
			want := "revoked"
			if operation == "expiry" {
				want = "expired"
			}
			if status.Authenticated || status.Reason != want {
				t.Fatalf("late reply status = %+v, want %s", status, want)
			}
		})
	}
}

func TestAuthResumeRejectsChangedAuthority(t *testing.T) {
	for _, change := range []struct {
		name   string
		mutate func(*protocol.BrowserGrant)
	}{
		{"grant", func(g *protocol.BrowserGrant) { g.ID = authTestToken(80) }},
		{"console", func(g *protocol.BrowserGrant) { g.ConsoleSessionID = authTestToken(81) }},
		{"owner", func(g *protocol.BrowserGrant) { g.OwnerID = "456" }},
		{"expiry", func(g *protocol.BrowserGrant) { g.ExpiresAt++ }},
		{"scope", func(g *protocol.BrowserGrant) { g.ConnectorGeneration++ }},
		{"binding", func(g *protocol.BrowserGrant) { g.BindingGeneration++ }},
		{"control epoch", func(g *protocol.BrowserGrant) { g.ControlEpoch = authTestToken(82) }},
		{"node epoch", func(g *protocol.BrowserGrant) { g.NodeEpoch = authTestToken(83) }},
		{"origin", func(g *protocol.BrowserGrant) { g.Origin = "https://" + g.HostID.Label() + ".elsewhere.example" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			auth, control, _ := newAuthTest(t, AuthConfig{})
			finish := finishAuthLogin(t, auth, control, 10)
			if err := auth.SuspendAll(context.Background()); err != nil {
				t.Fatal(err)
			}
			nextAuthScope(control)
			control.mu.Lock()
			control.resumeMutate = change.mutate
			control.mu.Unlock()
			waitAuthCondition(t, func() bool {
				return auth.Probe(finish.Cookie.Value, finish.Principal.Session.grant.Origin).Reason == "revoked"
			})
		})
	}
}

func TestAuthSuspendedSessionAndFailureMetadataRemainBounded(t *testing.T) {
	auth, control, now := newAuthTest(t, AuthConfig{MaxSessions: 1, SessionTTL: time.Minute})
	finish := finishAuthLogin(t, auth, control, 10)
	if err := auth.SuspendAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	start, state, code := prepareAuthLogin(t, auth, control, 11)
	binding, _ := control.snapshot()
	_, err := auth.Finish(context.Background(), binding.Binding.PublicURL, start.Cookie.Value, state, code)
	requireAuthCode(t, err, protocol.RateLimited)
	now.Add(60)
	auth.mu.Lock()
	auth.pruneSessionsLocked(auth.deps.Now())
	if len(auth.sessions) != 0 || len(auth.failures) != 1 {
		t.Errorf("expired suspended records: sessions=%d failures=%d", len(auth.sessions), len(auth.failures))
	}
	auth.mu.Unlock()
	if status := auth.Probe(finish.Cookie.Value, binding.Binding.PublicURL); status.Reason != "expired" {
		t.Fatalf("expired record status = %+v", status)
	}
	second := finishAuthLogin(t, auth, control, 12)
	if err := auth.Logout(context.Background(), second.Principal); err != nil {
		t.Fatal(err)
	}
	auth.mu.Lock()
	if len(auth.failures) != 1 {
		t.Error("failure metadata exceeded session bound")
	}
	now.Add(301)
	auth.pruneSessionsLocked(auth.deps.Now())
	if len(auth.failures) != 0 {
		t.Error("failure metadata did not expire")
	}
	auth.mu.Unlock()
}

func TestAuthResumeTransientFailureIsRateBounded(t *testing.T) {
	auth, control, now := newAuthTest(t, AuthConfig{})
	finish := finishAuthLogin(t, auth, control, 10)
	if err := auth.SuspendAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	nextAuthScope(control)
	control.mu.Lock()
	control.resumeError = protocol.Error{Code: protocol.Unavailable}
	control.mu.Unlock()
	_, err := auth.Authenticate(finish.Cookie.Value, finish.Principal.Session.grant.Origin)
	requireAuthCode(t, err, protocol.Unavailable)
	waitAuthCondition(t, func() bool {
		auth.mu.Lock()
		defer auth.mu.Unlock()
		return !finish.Principal.Session.resuming
	})
	for range 32 {
		if status := auth.Probe(finish.Cookie.Value, finish.Principal.Session.grant.Origin); status.Reason != "reconnecting" {
			t.Fatalf("transient failure became permanent: %+v", status)
		}
	}
	control.mu.Lock()
	if control.resumes != 1 {
		t.Errorf("failed resumes were not rate limited: %d", control.resumes)
	}
	control.resumeError = nil
	control.mu.Unlock()
	now.Add(1)
	recoverAuth(t, auth, finish)
}

func TestAuthResumeRetiresControlRejection(t *testing.T) {
	for _, code := range []protocol.Code{protocol.Unauthenticated, protocol.Forbidden, protocol.NotFound, protocol.Expired} {
		t.Run(string(code), func(t *testing.T) {
			auth, control, _ := newAuthTest(t, AuthConfig{})
			finish := finishAuthLogin(t, auth, control, 10)
			if err := auth.SuspendAll(context.Background()); err != nil {
				t.Fatal(err)
			}
			nextAuthScope(control)
			control.mu.Lock()
			control.resumeError = protocol.Error{Code: code}
			control.mu.Unlock()
			want := "revoked"
			if code == protocol.Expired {
				want = "expired"
			}
			waitAuthCondition(t, func() bool {
				return auth.Probe(finish.Cookie.Value, finish.Principal.Session.grant.Origin).Reason == want
			})
		})
	}
}
