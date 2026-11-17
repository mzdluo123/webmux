package relaysetting

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

type authTestCode struct {
	challenge string
	redirect  string
	grant     protocol.BrowserGrant
}

type authTestControl struct {
	mu                 sync.Mutex
	binding            AuthBinding
	codes              map[protocol.Token]authTestCode
	exchanges          int
	entered            chan struct{}
	proceed            chan struct{}
	ice                protocol.ICEConfig
	grants             map[protocol.Token]protocol.BrowserGrant
	resumes            int
	offline            bool
	resumeEntered      chan struct{}
	resumeProceed      chan struct{}
	resumeError        error
	resumeMutate       func(*protocol.BrowserGrant)
	resumeIgnoreCancel bool
}

func (c *authTestControl) Scope() (protocol.Scope, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.binding.Scope, !c.offline
}

func (c *authTestControl) snapshot() (AuthBinding, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.binding, !c.offline
}

// This in-memory control fixture performs the other side of the one-time PKCE
// exchange. Tests exercise Auth's real maps, locking, cookies and cancellation.
func (c *authTestControl) Exchange(ctx context.Context, request protocol.LoginExchange) (protocol.BrowserGrant, error) {
	c.mu.Lock()
	record, found := c.codes[request.Code]
	delete(c.codes, request.Code)
	c.exchanges++
	entered, proceed := c.entered, c.proceed
	c.mu.Unlock()
	if entered != nil {
		close(entered)
		select {
		case <-proceed:
		case <-ctx.Done():
			return protocol.BrowserGrant{}, ctx.Err()
		}
	}
	challenge := authDigest(request.Verifier)
	if !found || record.challenge != base64.RawURLEncoding.EncodeToString(challenge[:]) || record.redirect != request.RedirectURI {
		return protocol.BrowserGrant{}, protocol.Error{Code: protocol.Unauthenticated}
	}
	c.mu.Lock()
	c.grants[record.grant.ID] = record.grant
	c.mu.Unlock()
	return record.grant, nil
}

func (c *authTestControl) Resume(ctx context.Context, request protocol.LoginResume) (protocol.BrowserGrant, error) {
	c.mu.Lock()
	c.resumes++
	grant, found := c.grants[request.GrantID]
	grant.ConnectorGeneration = c.binding.Scope.ConnectorGeneration
	grant.ControlEpoch = c.binding.Scope.ControlEpoch
	grant.NodeEpoch = c.binding.Scope.NodeEpoch
	entered, proceed, err := c.resumeEntered, c.resumeProceed, c.resumeError
	ignoreCancel := c.resumeIgnoreCancel
	c.resumeEntered = nil
	if c.resumeMutate != nil {
		c.resumeMutate(&grant)
	}
	c.mu.Unlock()
	if entered != nil {
		close(entered)
	}
	if proceed != nil {
		if ignoreCancel {
			<-proceed
		} else {
			select {
			case <-proceed:
			case <-ctx.Done():
				return protocol.BrowserGrant{}, ctx.Err()
			}
		}
	}
	if err != nil {
		return protocol.BrowserGrant{}, err
	}
	if !found {
		return protocol.BrowserGrant{}, protocol.Error{Code: protocol.NotFound}
	}
	if ignoreCancel {
		return grant, nil
	}
	return grant, ctx.Err()
}

func (c *authTestControl) ICE(ctx context.Context, request protocol.ICERequest) (protocol.ICEConfig, error) {
	if err := ctx.Err(); err != nil {
		return protocol.ICEConfig{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	config := c.ice
	config.GrantID = request.GrantID
	config.PeerConnectionID = request.PeerConnectionID
	return config, nil
}

func authTestToken(b byte) protocol.Token {
	return protocol.Token(base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat(string([]byte{b}), 32))))
}

func newAuthTest(t *testing.T, config AuthConfig) (*Auth, *authTestControl, *atomic.Int64) {
	t.Helper()
	now := &atomic.Int64{}
	now.Store(time.Now().Unix())
	host := protocol.HostID("0123456789abcdef0123456789abcdef")
	binding := AuthBinding{Binding: protocol.Binding{HostID: host, OwnerID: "123", BindingGeneration: 2, PublicURL: "https://" + host.Label() + ".example.com"}, Scope: protocol.Scope{HostID: host, BindingGeneration: 2, ConnectorGeneration: 3, ControlEpoch: authTestToken(1), NodeEpoch: authTestToken(2)}, ControlOrigin: "https://control.example.com"}
	control := &authTestControl{binding: binding, codes: make(map[protocol.Token]authTestCode), grants: make(map[protocol.Token]protocol.BrowserGrant)}
	auth, err := NewAuth(config, AuthDependencies{Client: control, Snapshot: control.snapshot, Now: func() time.Time { return time.Unix(now.Load(), 0) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := auth.Close(); err != nil {
			t.Error(err)
		}
	})
	return auth, control, now
}

func prepareAuthLogin(t *testing.T, auth *Auth, control *authTestControl, grantByte byte) (AuthStart, string, string) {
	t.Helper()
	binding, _ := control.snapshot()
	start, err := auth.Start(context.Background(), binding.Binding.PublicURL, "")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(start.URL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if parsed.Scheme+"://"+parsed.Host+parsed.Path != binding.ControlOrigin+protocol.PathLoginAuthorize || query.Get("codeChallengeMethod") != "S256" || query.Get("hostId") != string(binding.Binding.HostID) {
		t.Fatal("authorization URL does not use frozen host PKCE contract")
	}
	grant := protocol.BrowserGrant{ID: authTestToken(grantByte), ConsoleSessionID: authTestToken(5), OwnerID: binding.Binding.OwnerID, HostID: binding.Scope.HostID, BindingGeneration: binding.Scope.BindingGeneration, ConnectorGeneration: binding.Scope.ConnectorGeneration, ControlEpoch: binding.Scope.ControlEpoch, NodeEpoch: binding.Scope.NodeEpoch, Origin: binding.Binding.PublicURL, ExpiresAt: auth.deps.Now().Add(2 * time.Hour).Unix()}
	code := authTestToken(grantByte + 64)
	control.mu.Lock()
	control.codes[code] = authTestCode{challenge: query.Get("codeChallenge"), redirect: query.Get("redirectUri"), grant: grant}
	control.mu.Unlock()
	return start, query.Get("state"), string(code)
}

func finishAuthLogin(t *testing.T, auth *Auth, control *authTestControl, grantByte byte) AuthFinish {
	t.Helper()
	start, state, code := prepareAuthLogin(t, auth, control, grantByte)
	binding, _ := control.snapshot()
	finish, err := auth.Finish(context.Background(), binding.Binding.PublicURL, start.Cookie.Value, state, code)
	if err != nil {
		t.Fatal(err)
	}
	return finish
}

func requireAuthCode(t *testing.T, err error, want protocol.Code) {
	t.Helper()
	var protocolErr protocol.Error
	if !errors.As(err, &protocolErr) || protocolErr.Code != want {
		t.Fatalf("got %v, want protocol code %s", err, want)
	}
}

func TestAuthTransactionConcurrentConsume(t *testing.T) {
	auth, control, _ := newAuthTest(t, AuthConfig{})
	start, state, code := prepareAuthLogin(t, auth, control, 10)
	binding, _ := control.snapshot()
	const attempts = 16
	results := make(chan error, attempts)
	var workers sync.WaitGroup
	for range attempts {
		workers.Go(func() {
			_, err := auth.Finish(context.Background(), binding.Binding.PublicURL, start.Cookie.Value, state, code)
			results <- err
		})
	}
	workers.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			requireAuthCode(t, err, protocol.Unauthenticated)
		}
	}
	control.mu.Lock()
	exchanges := control.exchanges
	control.mu.Unlock()
	if successes != 1 || exchanges != 1 {
		t.Fatalf("successes=%d exchanges=%d; want one atomic consume", successes, exchanges)
	}
}

func TestAuthFailedCallbackConsumesTransaction(t *testing.T) {
	tests := []struct {
		name    string
		failure string
		want    protocol.Code
	}{
		{name: "wrong state", failure: "state", want: protocol.Unauthenticated},
		{name: "invalid code", failure: "code", want: protocol.Invalid},
		{name: "wrong origin", failure: "origin", want: protocol.Forbidden},
		{name: "expired transaction", failure: "expired", want: protocol.Expired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, control, now := newAuthTest(t, AuthConfig{})
			start, state, code := prepareAuthLogin(t, auth, control, 10)
			binding, _ := control.snapshot()
			giveState, giveCode, origin := state, code, binding.Binding.PublicURL
			switch tt.failure {
			case "state":
				giveState = string(authTestToken(90))
			case "code":
				giveCode = "invalid"
			case "origin":
				origin = "https://another.example.com"
			case "expired":
				now.Add(int64(protocol.BrowserTransactionTTL / time.Second))
			}
			_, err := auth.Finish(context.Background(), origin, start.Cookie.Value, giveState, giveCode)
			requireAuthCode(t, err, tt.want)
			_, err = auth.Finish(context.Background(), binding.Binding.PublicURL, start.Cookie.Value, state, code)
			requireAuthCode(t, err, protocol.Unauthenticated)
		})
	}
}

func TestAuthSessionOriginCSRFAndCookies(t *testing.T) {
	auth, control, _ := newAuthTest(t, AuthConfig{})
	finish := finishAuthLogin(t, auth, control, 10)
	binding, _ := control.snapshot()
	cookie := finish.Cookie
	if cookie.Name != AuthCookieName || protocol.Token(cookie.Value).Validate() != nil || cookie.MaxAge <= 0 || cookie.MaxAge > int(protocol.BrowserSessionTTL/time.Second) || !cookie.Expires.After(auth.deps.Now()) {
		t.Fatal("invalid session cookie material or lifetime")
	}
	principal, err := auth.Authenticate(cookie.Value, binding.Binding.PublicURL)
	if err != nil {
		t.Fatal(err)
	}
	status, err := auth.Status(principal)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Authenticated || status.CSRFToken == "" || status.ExpiresAt != cookie.Expires.Unix() {
		t.Fatal("invalid authenticated session status")
	}
	statusAgain, err := auth.Status(principal)
	if err != nil {
		t.Fatal(err)
	}
	if statusAgain.CSRFToken != status.CSRFToken {
		t.Fatal("CSRF token changed without session rotation")
	}
	if err := auth.CheckMutation(principal, binding.Binding.PublicURL, status.CSRFToken); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"", "null", "https://another.example.com", binding.Binding.PublicURL + "/", binding.Binding.PublicURL + ":443"} {
		requireAuthCode(t, auth.CheckMutation(principal, origin, status.CSRFToken), protocol.Forbidden)
	}
	requireAuthCode(t, auth.CheckMutation(principal, binding.Binding.PublicURL, string(authTestToken(12))), protocol.Forbidden)
	_, err = auth.Authenticate(cookie.Value, "https://local.example.com")
	requireAuthCode(t, err, protocol.Forbidden)
	if principal.ID == cookie.Value || principal.ID == string(principal.GrantID) {
		t.Fatal("diagnostic ID leaks an authentication handle")
	}
}

func TestAuthTransactionCapacityAndAbsoluteExpiry(t *testing.T) {
	auth, control, now := newAuthTest(t, AuthConfig{MaxTransactions: 1, SessionTTL: time.Minute})
	start, state, code := prepareAuthLogin(t, auth, control, 10)
	binding, _ := control.snapshot()
	_, err := auth.Start(context.Background(), binding.Binding.PublicURL, "")
	requireAuthCode(t, err, protocol.RateLimited)
	finish, err := auth.Finish(context.Background(), binding.Binding.PublicURL, start.Cookie.Value, state, code)
	if err != nil {
		t.Fatal(err)
	}
	now.Add(59)
	if _, err := auth.Authenticate(finish.Cookie.Value, binding.Binding.PublicURL); err != nil {
		t.Fatal(err)
	}
	now.Add(1)
	_, err = auth.Authenticate(finish.Cookie.Value, binding.Binding.PublicURL)
	requireAuthCode(t, err, protocol.Expired)
	select {
	case <-finish.Principal.Context.Done():
	default:
		t.Fatal("expired session context remains live")
	}
}
