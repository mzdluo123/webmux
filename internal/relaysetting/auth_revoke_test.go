package relaysetting

import (
	"context"
	"errors"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

func authTestRevocation(control *authTestControl, scope string, target protocol.Token) protocol.Revocation {
	binding, _ := control.snapshot()
	revocation := protocol.Revocation{ID: authTestToken(40), Scope: scope, HostID: binding.Scope.HostID, BindingGeneration: binding.Scope.BindingGeneration, Reason: "logout"}
	switch scope {
	case "connector":
		revocation.ConnectorGeneration = binding.Scope.ConnectorGeneration
	case "console-session":
		revocation.ConsoleSessionID = target
	case "grant":
		revocation.GrantID = target
	}
	return revocation
}

func TestAuthRevocationWaitsForAccessChildren(t *testing.T) {
	auth, control, _ := newAuthTest(t, AuthConfig{})
	finish := finishAuthLogin(t, auth, control, 10)
	binding, _ := control.snapshot()
	closing := make(chan struct{})
	done := make(chan struct{})
	// Always unblock teardown even if an assertion fails before the join.
	defer func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}()
	release, err := auth.RegisterAccess(finish.Principal, func() { close(closing) }, done)
	if err != nil {
		t.Fatal(err)
	}
	// A premature release must not allow the access-closed ACK to lie.
	release()
	revocation := authTestRevocation(control, "grant", finish.Principal.GrantID)
	result := make(chan error, 1)
	go func() { result <- auth.Revoke(context.Background(), revocation) }()
	select {
	case <-closing:
	case <-time.After(time.Second):
		t.Fatal("revocation did not interrupt access")
	}
	select {
	case <-finish.Principal.Context.Done():
	default:
		t.Fatal("revocation left session context live")
	}
	select {
	case err := <-result:
		t.Fatalf("acknowledged before access children exited: %v", err)
	default:
	}
	_, err = auth.Authenticate(finish.Cookie.Value, binding.Binding.PublicURL)
	requireAuthCode(t, err, protocol.Unauthenticated)
	_, err = auth.RegisterAccess(finish.Principal, func() {}, done)
	requireAuthCode(t, err, protocol.Unauthenticated)
	close(done)
	release()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("revocation did not join exited access")
	}
	if err := auth.Revoke(context.Background(), revocation); err != nil {
		t.Fatal("revocation replay must not resurrect access:", err)
	}
}

func TestAuthRevocationTimeoutIsNotAccessClosed(t *testing.T) {
	auth, control, _ := newAuthTest(t, AuthConfig{})
	finish := finishAuthLogin(t, auth, control, 10)
	done := make(chan struct{})
	defer func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}()
	closing := make(chan struct{})
	_, err := auth.RegisterAccess(finish.Principal, func() { close(closing) }, done)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	revocation := authTestRevocation(control, "grant", finish.Principal.GrantID)
	if err := auth.Revoke(ctx, revocation); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want incomplete cancellation", err)
	}
	select {
	case <-closing:
	case <-time.After(time.Second):
		t.Fatal("incomplete ACK did not still close access")
	}
	close(done)
	if err := auth.Revoke(context.Background(), revocation); err != nil {
		t.Fatal(err)
	}
}

func TestAuthRevocationScopeIsolation(t *testing.T) {
	tests := []struct {
		scope string
		both  bool
	}{
		{scope: "grant"}, {scope: "console-session", both: true}, {scope: "connector", both: true}, {scope: "binding", both: true},
	}
	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			auth, control, _ := newAuthTest(t, AuthConfig{})
			first := finishAuthLogin(t, auth, control, 10)
			second := finishAuthLogin(t, auth, control, 11)
			target := first.Principal.GrantID
			if tt.scope == "console-session" {
				target = authTestToken(5)
			}
			if err := auth.Revoke(context.Background(), authTestRevocation(control, tt.scope, target)); err != nil {
				t.Fatal(err)
			}
			select {
			case <-first.Principal.Context.Done():
			default:
				t.Fatal("target session not canceled")
			}
			select {
			case <-second.Principal.Context.Done():
				if !tt.both {
					t.Fatal("grant revoke canceled another session")
				}
			default:
				if tt.both {
					t.Fatal("scope revoke left matching session live")
				}
			}
		})
	}
}

func TestAuthRevocationFencesInflightExchange(t *testing.T) {
	auth, control, _ := newAuthTest(t, AuthConfig{})
	start, state, code := prepareAuthLogin(t, auth, control, 10)
	binding, _ := control.snapshot()
	entered, proceed := make(chan struct{}), make(chan struct{})
	control.mu.Lock()
	control.entered, control.proceed = entered, proceed
	control.mu.Unlock()
	defer func() {
		select {
		case <-proceed:
		default:
			close(proceed)
		}
	}()
	result := make(chan error, 1)
	go func() {
		_, err := auth.Finish(context.Background(), binding.Binding.PublicURL, start.Cookie.Value, state, code)
		result <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("exchange did not start")
	}
	if err := auth.Revoke(context.Background(), authTestRevocation(control, "console-session", authTestToken(5))); err != nil {
		t.Fatal(err)
	}
	close(proceed)
	select {
	case err := <-result:
		requireAuthCode(t, err, protocol.Unavailable)
	case <-time.After(time.Second):
		t.Fatal("exchange did not finish")
	}
}

func TestAuthInvalidateAndLogoutKeepOldCookiesRevoked(t *testing.T) {
	auth, control, _ := newAuthTest(t, AuthConfig{})
	first := finishAuthLogin(t, auth, control, 10)
	second := finishAuthLogin(t, auth, control, 11)
	binding, _ := control.snapshot()
	if err := auth.Logout(context.Background(), first.Principal); err != nil {
		t.Fatal(err)
	}
	_, err := auth.Authenticate(first.Cookie.Value, binding.Binding.PublicURL)
	requireAuthCode(t, err, protocol.Unauthenticated)
	if _, err := auth.Authenticate(second.Cookie.Value, binding.Binding.PublicURL); err != nil {
		t.Fatal(err)
	}
	if err := auth.InvalidateAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = auth.Authenticate(second.Cookie.Value, binding.Binding.PublicURL)
	requireAuthCode(t, err, protocol.Unauthenticated)
	third := finishAuthLogin(t, auth, control, 12)
	if _, err := auth.Authenticate(third.Cookie.Value, binding.Binding.PublicURL); err != nil {
		t.Fatal(err)
	}
}

func TestAuthRejectsGrantScopeMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*protocol.BrowserGrant)
	}{
		{name: "owner", mutate: func(g *protocol.BrowserGrant) { g.OwnerID = "456" }},
		{name: "binding", mutate: func(g *protocol.BrowserGrant) { g.BindingGeneration++ }},
		{name: "connector", mutate: func(g *protocol.BrowserGrant) { g.ConnectorGeneration++ }},
		{name: "control epoch", mutate: func(g *protocol.BrowserGrant) { g.ControlEpoch = authTestToken(30) }},
		{name: "node epoch", mutate: func(g *protocol.BrowserGrant) { g.NodeEpoch = authTestToken(31) }},
		{name: "origin", mutate: func(g *protocol.BrowserGrant) { g.Origin = "https://" + g.HostID.Label() + ".another.example.com" }},
		{name: "host", mutate: func(g *protocol.BrowserGrant) {
			g.HostID = "ffffffffffffffffffffffffffffffff"
			g.Origin = "https://" + g.HostID.Label() + ".example.com"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, control, _ := newAuthTest(t, AuthConfig{})
			start, state, code := prepareAuthLogin(t, auth, control, 10)
			binding, _ := control.snapshot()
			control.mu.Lock()
			record := control.codes[protocol.Token(code)]
			tt.mutate(&record.grant)
			control.codes[protocol.Token(code)] = record
			control.mu.Unlock()
			_, err := auth.Finish(context.Background(), binding.Binding.PublicURL, start.Cookie.Value, state, code)
			requireAuthCode(t, err, protocol.Forbidden)
		})
	}
}
