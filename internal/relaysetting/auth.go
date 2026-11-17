package relaysetting

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

const (
	AuthCookieName            = "__Host-webmux-relay"
	AuthTransactionCookieName = "__Host-webmux-relay-tx"
)

// AuthClient is implemented by the authenticated connector control channel.
type AuthClient interface {
	Exchange(context.Context, protocol.LoginExchange) (protocol.BrowserGrant, error)
	Resume(context.Context, protocol.LoginResume) (protocol.BrowserGrant, error)
	ICE(context.Context, protocol.ICERequest) (protocol.ICEConfig, error)
	Scope() (protocol.Scope, bool)
}

type AuthBinding struct {
	Binding       protocol.Binding
	Scope         protocol.Scope
	ControlOrigin string
}

type AuthConfig struct {
	TransactionTTL     time.Duration
	SessionTTL         time.Duration
	MaxTransactions    int
	MaxSessions        int
	MaxPeersPerSession int
}

type AuthDependencies struct {
	Client   AuthClient
	// Snapshot may retain binding identity while its readiness result is false.
	Snapshot func() (AuthBinding, bool)
	Now      func() time.Time
	Random   io.Reader
}

// AuthCookie carries only authentication material and its absolute lifetime.
// The controller supplies the fixed host-only browser security attributes.
type AuthCookie struct {
	Name    string
	Value   string
	Expires time.Time
	MaxAge  int
}

type AuthStart struct {
	URL    string
	Cookie AuthCookie
}

type AuthFinish struct {
	Cookie    AuthCookie
	Principal AuthPrincipal
	ReturnTo  string
}

type authTransaction struct {
	state      [32]byte
	verifier   string
	binding    AuthBinding
	expires    time.Time
	generation uint64
	returnTo   string
}

// Auth owns only ephemeral browser authority. Closing it never terminates PTYs.
type Auth struct {
	mu           sync.Mutex
	randomMu     sync.Mutex
	workers      sync.WaitGroup
	config       AuthConfig
	deps         AuthDependencies
	csrfKey      [32]byte
	transactions map[[32]byte]authTransaction
	sessions     map[[32]byte]*AuthSession
	failures     map[[32]byte]authFailure
	generation   uint64
	closed       bool
	stop         chan struct{}
	done         chan struct{}
}

func NewAuth(config AuthConfig, deps AuthDependencies) (*Auth, error) {
	if config.TransactionTTL == 0 {
		config.TransactionTTL = protocol.BrowserTransactionTTL
	}
	if config.SessionTTL == 0 {
		config.SessionTTL = protocol.BrowserSessionTTL
	}
	if config.MaxTransactions == 0 {
		config.MaxTransactions = 256
	}
	if config.MaxSessions == 0 {
		config.MaxSessions = 128
	}
	if config.MaxPeersPerSession == 0 {
		config.MaxPeersPerSession = 8
	}
	if deps.Client == nil || deps.Snapshot == nil || config.TransactionTTL < time.Second || config.TransactionTTL > protocol.BrowserTransactionTTL || config.SessionTTL < time.Second || config.SessionTTL > protocol.BrowserSessionTTL || config.MaxTransactions < 1 || config.MaxSessions < 1 || config.MaxPeersPerSession < 1 {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Random == nil {
		deps.Random = rand.Reader
	}
	a := &Auth{config: config, deps: deps, transactions: make(map[[32]byte]authTransaction), sessions: make(map[[32]byte]*AuthSession), failures: make(map[[32]byte]authFailure), stop: make(chan struct{}), done: make(chan struct{})}
	if _, err := io.ReadFull(deps.Random, a.csrfKey[:]); err != nil {
		return nil, fmt.Errorf("initialize relay browser authentication: %w", err)
	}
	a.workers.Go(a.cleanTransactions)
	return a, nil
}

// PublicOrigin is binding identity, not authorization or transport readiness.
// Only a trusted connector ingress may use it when the control link is down.
func (a *Auth) PublicOrigin() string {
	binding, _ := a.deps.Snapshot()
	if binding.Binding.Validate() != nil {
		return ""
	}
	return binding.Binding.PublicURL
}

func (a *Auth) snapshot() (AuthBinding, error) {
	binding, ok := a.deps.Snapshot()
	if !ok {
		return AuthBinding{}, protocol.Error{Code: protocol.Unavailable}
	}
	scope, ok := a.deps.Client.Scope()
	if !ok || scope != binding.Scope || binding.Binding.Validate() != nil || scope.Validate() != nil || scope.HostID != binding.Binding.HostID || scope.BindingGeneration != binding.Binding.BindingGeneration || !authOrigin(binding.ControlOrigin) {
		return AuthBinding{}, protocol.Error{Code: protocol.Unavailable}
	}
	return binding, nil
}

func authOrigin(origin string) bool {
	u, err := url.Parse(origin)
	return err == nil && len(origin) <= protocol.MaxURLBytes && u.Scheme == "https" && u.Hostname() != "" && (u.Port() == "" || u.Port() == "443") && u.User == nil && u.Path == "" && !u.ForceQuery && u.RawQuery == "" && u.Fragment == "" && u.Opaque == "" && u.String() == origin
}

func (a *Auth) token() (string, error) {
	var raw [32]byte
	a.randomMu.Lock()
	_, err := io.ReadFull(a.deps.Random, raw[:])
	a.randomMu.Unlock()
	if err != nil {
		return "", fmt.Errorf("generate relay browser authentication material: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func authDigest(value string) [32]byte { return sha256.Sum256([]byte(value)) }

func authCookie(name, value string, now, expires time.Time) AuthCookie {
	return AuthCookie{Name: name, Value: value, Expires: expires, MaxAge: int(expires.Sub(now) / time.Second)}
}

// ClearAuthCookie also serves the transaction cookie by passing its name.
func ClearAuthCookie(name string) AuthCookie {
	return AuthCookie{Name: name, MaxAge: -1, Expires: time.Unix(1, 0)}
}

var authAppRoute = regexp.MustCompile(`^(/|/app|/(s|code|handoff)/[a-z2-7]{8}|/git(/[A-Za-z0-9_-]{1,64}(/worktrees/[A-Za-z0-9_-]{1,64})?)?|/new|/settings(/(relay|tailscale|tunnel))?)$`)

// authReturnTo accepts only application routes, never credentials or endpoints.
func authReturnTo(destination string) (string, error) {
	if destination == "" {
		return "/app", nil
	}
	u, err := url.Parse(destination)
	if err != nil || len(destination) > protocol.MaxURLBytes || u.IsAbs() || u.Host != "" || u.User != nil || u.Opaque != "" || u.RawPath != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(destination, "\\\x00\r\n\t") || !authAppRoute.MatchString(u.Path) {
		return "", protocol.Error{Code: protocol.Invalid}
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query) > 0 && u.Path != "/git" && !strings.HasPrefix(u.Path, "/git/") {
		return "", protocol.Error{Code: protocol.Invalid}
	}
	for key, values := range query {
		switch key {
		case "view", "scope", "branch", "commit", "q", "author", "since", "until", "path", "returnSession":
		default:
			return "", protocol.Error{Code: protocol.Invalid}
		}
		if len(values) != 1 || strings.ContainsAny(values[0], "\x00\r\n") {
			return "", protocol.Error{Code: protocol.Invalid}
		}
	}
	return destination, nil
}

func (a *Auth) Start(ctx context.Context, publicOrigin, returnTo string) (AuthStart, error) {
	if err := ctx.Err(); err != nil {
		return AuthStart{}, err
	}
	returnTo, err := authReturnTo(returnTo)
	if err != nil {
		return AuthStart{}, err
	}
	binding, err := a.snapshot()
	if err != nil {
		return AuthStart{}, err
	}
	if publicOrigin != binding.Binding.PublicURL {
		return AuthStart{}, protocol.Error{Code: protocol.Forbidden}
	}
	handle, err := a.token()
	if err != nil {
		return AuthStart{}, err
	}
	state, err := a.token()
	if err != nil {
		return AuthStart{}, err
	}
	verifier, err := a.token()
	if err != nil {
		return AuthStart{}, err
	}
	now := a.deps.Now()
	expires := now.Add(a.config.TransactionTTL)
	challenge := authDigest(verifier)
	query := url.Values{"version": {strconv.Itoa(int(protocol.V1))}, "hostId": {string(binding.Binding.HostID)}, "state": {state}, "redirectUri": {publicOrigin + protocol.HostLoginCallback}, "codeChallenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "codeChallengeMethod": {"S256"}}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pruneTransactionsLocked(now)
	if a.closed {
		return AuthStart{}, protocol.Error{Code: protocol.Unavailable}
	}
	if len(a.transactions) >= a.config.MaxTransactions {
		return AuthStart{}, protocol.Error{Code: protocol.RateLimited}
	}
	key := authDigest(handle)
	if _, exists := a.transactions[key]; exists {
		return AuthStart{}, protocol.Error{Code: protocol.Conflict}
	}
	a.transactions[key] = authTransaction{state: authDigest(state), verifier: verifier, binding: binding, expires: expires, generation: a.generation, returnTo: returnTo}
	return AuthStart{URL: binding.ControlOrigin + protocol.PathLoginAuthorize + "?" + query.Encode(), Cookie: authCookie(AuthTransactionCookieName, handle, now, expires)}, nil
}

// Finish consumes the transaction before validating any callback material. A
// failed exchange is never retried, because its one-time code may be consumed.
func (a *Auth) Finish(ctx context.Context, publicOrigin, transactionCookie, state, code string) (AuthFinish, error) {
	key := authDigest(transactionCookie)
	a.mu.Lock()
	tx, found := a.transactions[key]
	delete(a.transactions, key)
	closed := a.closed
	a.mu.Unlock()
	if closed {
		return AuthFinish{}, protocol.Error{Code: protocol.Unavailable}
	}
	if !found || protocol.Token(transactionCookie).Validate() != nil || protocol.Token(state).Validate() != nil || !hmac.Equal(tx.state[:], authDigestBytes(state)) {
		return AuthFinish{}, protocol.Error{Code: protocol.Unauthenticated}
	}
	if !a.deps.Now().Before(tx.expires) {
		return AuthFinish{}, protocol.Error{Code: protocol.Expired}
	}
	binding, err := a.snapshot()
	if err != nil {
		return AuthFinish{}, err
	}
	if binding != tx.binding || publicOrigin != binding.Binding.PublicURL {
		return AuthFinish{}, protocol.Error{Code: protocol.Forbidden}
	}
	exchange := protocol.LoginExchange{Code: protocol.Token(code), Verifier: tx.verifier, RedirectURI: publicOrigin + protocol.HostLoginCallback}
	if err := exchange.Validate(); err != nil {
		return AuthFinish{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, protocol.RequestTimeout)
	defer cancel()
	grant, err := a.deps.Client.Exchange(requestCtx, exchange)
	if err != nil {
		return AuthFinish{}, err
	}
	now := a.deps.Now()
	if err := grant.Validate(now); err != nil {
		return AuthFinish{}, err
	}
	if !authGrantMatches(grant, binding) {
		return AuthFinish{}, protocol.Error{Code: protocol.Forbidden}
	}
	current, err := a.snapshot()
	if err != nil {
		return AuthFinish{}, err
	}
	if current != binding {
		return AuthFinish{}, protocol.Error{Code: protocol.StaleGeneration}
	}
	if err := ctx.Err(); err != nil {
		return AuthFinish{}, err
	}
	handle, err := a.token()
	if err != nil {
		return AuthFinish{}, err
	}
	finish, err := a.newSession(handle, grant, binding, tx.generation, now)
	finish.ReturnTo = tx.returnTo
	return finish, err
}

func authDigestBytes(value string) []byte {
	digest := authDigest(value)
	return digest[:]
}

func authGrantMatches(grant protocol.BrowserGrant, binding AuthBinding) bool {
	scope := binding.Scope
	return grant.OwnerID == binding.Binding.OwnerID && grant.HostID == scope.HostID && grant.BindingGeneration == scope.BindingGeneration && grant.ConnectorGeneration == scope.ConnectorGeneration && grant.ControlEpoch == scope.ControlEpoch && grant.NodeEpoch == scope.NodeEpoch && grant.Origin == binding.Binding.PublicURL
}

func (a *Auth) pruneTransactionsLocked(now time.Time) {
	for key, tx := range a.transactions {
		if !now.Before(tx.expires) {
			delete(a.transactions, key)
		}
	}
}

func (a *Auth) cleanTransactions() {
	defer close(a.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.stop:
			return
		case <-ticker.C:
			a.mu.Lock()
			a.pruneTransactionsLocked(a.deps.Now())
			a.pruneSessionsLocked(a.deps.Now())
			a.mu.Unlock()
		}
	}
}
