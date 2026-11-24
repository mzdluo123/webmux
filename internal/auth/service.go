package auth

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/statefile"
)

const (
	setupCodeTTL       = 30 * time.Minute
	onboardingTicketTT = 10 * time.Minute
	challengeTTL       = 5 * time.Minute
	pairingCodeTTL     = 5 * time.Minute
	stepUpTTL          = 5 * time.Minute
	sessionTTL         = 30 * 24 * time.Hour
	sessionTouchPeriod = 15 * time.Minute
	maxPendingItems    = 64
	rateLimitWindow    = time.Minute
	rateLimitMaxHits   = 25
)

type setupEntry struct {
	code      string
	display   string
	expiresAt time.Time
}

type ticketEntry struct {
	expiresAt time.Time
}

type challengeKind string

const (
	challengeOnboarding challengeKind = "onboarding"
	challengeLogin      challengeKind = "login"
	challengePair       challengeKind = "pair"
	challengeStepUp     challengeKind = "step_up"
	challengeAddPasskey challengeKind = "add_passkey"
)

type challengeEntry struct {
	kind        challengeKind
	challenge   string
	rpID        string
	userHandle  string
	pairingCode string
	sessionID   string
	expiresAt   time.Time
}

type pairingEntry struct {
	display   string
	targetURL string
	expiresAt time.Time
}

type stepUpEntry struct {
	sessionID string
	expiresAt time.Time
}

type rateBucket struct {
	windowStart time.Time
	count       int
}

// Service owns Passkey authentication, revocable sessions, onboarding grants,
// recovery codes, and optional named API tokens for one WebMux instance.
type Service struct {
	mu         sync.Mutex
	opts       Options
	now        func() time.Time
	handle     *statefile.Handle[storedAuth]
	doc        storedAuth
	storageErr error
	setup      *setupEntry
	tickets    map[string]ticketEntry
	challenges map[string]challengeEntry
	pairings   map[string]pairingEntry
	stepUps    map[string]stepUpEntry
	rates      map[string]rateBucket
	lastTouch  map[string]time.Time
}

// Open opens <stateDir>/auth.json with a lifetime lock and returns a fail-closed
// authentication service. A missing file initializes cleanly into needs_onboarding.
func Open(stateDir string, opts Options) *Service {
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	if strings.TrimSpace(opts.RPDisplayName) == "" {
		opts.RPDisplayName = "WebMux"
	}
	if strings.TrimSpace(opts.UserName) == "" {
		opts.UserName = "webmux"
	}
	s := &Service{
		opts:       opts,
		now:        nowFn,
		doc:        emptyStoredAuth(),
		tickets:    map[string]ticketEntry{},
		challenges: map[string]challengeEntry{},
		pairings:   map[string]pairingEntry{},
		stepUps:    map[string]stepUpEntry{},
		rates:      map[string]rateBucket{},
		lastTouch:  map[string]time.Time{},
	}
	if opts.Disabled || opts.LegacyToken != "" {
		return s
	}
	if stateDir == "" {
		s.storageErr = errors.New("state directory is empty")
		return s
	}
	handle, doc, err := statefile.Open[storedAuth](stateDir, statefile.Auth, validateStoredAuth)
	s.handle = handle
	switch {
	case err == nil:
		s.doc = normalizeDoc(doc)
	case statefile.IsKind(err, statefile.Missing):
		s.doc = emptyStoredAuth()
		s.ensureSetupLocked()
	default:
		s.storageErr = err
	}
	if s.storageErr == nil && len(s.doc.Passkeys) == 0 && s.opts.LegacyToken == "" {
		s.ensureSetupLocked()
	}
	return s
}

func normalizeDoc(doc storedAuth) storedAuth {
	if doc.Passkeys == nil {
		doc.Passkeys = []PasskeyRecord{}
	}
	if doc.Sessions == nil {
		doc.Sessions = []SessionRecord{}
	}
	if doc.RecoveryCodes == nil {
		doc.RecoveryCodes = []RecoveryRecord{}
	}
	if doc.APITokens == nil {
		doc.APITokens = []APITokenRecord{}
	}
	return doc
}

// Close releases the auth.json lifetime lock.
func (s *Service) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handle != nil {
		s.handle.Close()
		s.handle = nil
	}
}

// StorageError returns the fail-closed storage error, or nil when healthy.
func (s *Service) StorageError() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageErr != nil {
		return s.storageErr
	}
	if s.handle != nil {
		return s.handle.Err()
	}
	return nil
}

// Ready reports whether Passkey authentication (or an explicit legacy token) is
// initialized and healthy enough to expose remote entry points.
func (s *Service) Ready() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opts.Disabled || s.storageFailedLocked() {
		return false
	}
	return len(s.doc.Passkeys) > 0 || s.opts.LegacyToken != ""
}

// State reports the overall auth lifecycle state without host-specific details.
func (s *Service) State() string {
	if s == nil {
		return StateDisabled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateLocked()
}

func (s *Service) stateLocked() string {
	if s.opts.Disabled {
		return StateDisabled
	}
	if s.storageFailedLocked() {
		return StateStorageError
	}
	if len(s.doc.Passkeys) > 0 {
		return StateReady
	}
	if s.opts.LegacyToken != "" {
		return StateLegacyToken
	}
	return StateNeedsOnboarding
}

func (s *Service) storageFailedLocked() bool {
	if s.storageErr != nil {
		return true
	}
	return s.handle != nil && s.handle.Err() != nil
}

func (s *Service) storageErrStringLocked() string {
	if s.storageErr != nil {
		return s.storageErr.Error()
	}
	if s.handle != nil && s.handle.Err() != nil {
		return s.handle.Err().Error()
	}
	return ""
}

// Summary returns the non-sensitive live status summary for /local/v1/status.
func (s *Service) Summary() InspectSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.stateLocked()
	if state == StateStorageError {
		return InspectSummary{State: StateStorageError, Error: s.storageErrStringLocked()}
	}
	s.pruneExpiredSessionsLocked(s.now().UTC())
	return InspectSummary{
		State:                  state,
		OnboardingCompleted:    s.doc.OnboardingCompleted,
		PasskeyCount:           len(s.doc.Passkeys),
		RPIDs:                  uniqueRPIDs(s.doc.Passkeys),
		SessionCount:           len(s.doc.Sessions),
		RecoveryCodesRemaining: s.remainingRecoveryLocked(),
		APITokenCount:          len(s.doc.APITokens),
	}
}

// Status returns the public /api/auth/status payload scoped to the request Host.
func (s *Service) Status(host string) StatusSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.stateLocked()
	snap := StatusSnapshot{
		State:                  state,
		OnboardingCompleted:    s.doc.OnboardingCompleted,
		PasskeyCount:           len(s.doc.Passkeys),
		RecoveryCodesRemaining: s.remainingRecoveryLocked(),
	}
	if state == StateStorageError {
		snap.Error = s.storageErrStringLocked()
		return snap
	}
	s.pruneExpiredSessionsLocked(s.now().UTC())
	snap.SessionCount = len(s.doc.Sessions)
	if rpID, err := ValidateRPID(host); err == nil {
		snap.RPID = rpID
		snap.RPIDValid = true
		for _, p := range s.doc.Passkeys {
			if p.RPID == rpID {
				snap.HasPasskeyForRPID = true
				break
			}
		}
	} else {
		snap.RPIDError = err.Error()
	}
	return snap
}

// AllowPublic applies a lightweight per-key sliding rate limit to public auth
// endpoints (setup verify, login, pair, recovery).
func (s *Service) AllowPublic(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if len(s.rates) > 512 {
		for k, b := range s.rates {
			if now.Sub(b.windowStart) > rateLimitWindow {
				delete(s.rates, k)
			}
		}
	}
	b := s.rates[key]
	if b.windowStart.IsZero() || now.Sub(b.windowStart) > rateLimitWindow {
		s.rates[key] = rateBucket{windowStart: now, count: 1}
		return true
	}
	if b.count >= rateLimitMaxHits {
		return false
	}
	b.count++
	s.rates[key] = b
	return true
}

// SetupInfo returns the active in-memory setup code for needs_onboarding,
// refreshing it automatically if expired.
func (s *Service) SetupInfo() (SetupGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opts.Disabled {
		return SetupGrant{}, ErrDisabled
	}
	if s.storageFailedLocked() {
		return SetupGrant{}, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) > 0 {
		return SetupGrant{}, ErrAlreadyInitialized
	}
	s.ensureSetupLocked()
	return SetupGrant{Code: s.setup.code, DisplayCode: s.setup.display, ExpiresAt: s.setup.expiresAt}, nil
}

// RefreshSetupCode rotates the in-memory setup code while in needs_onboarding.
func (s *Service) RefreshSetupCode() (SetupGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opts.Disabled {
		return SetupGrant{}, ErrDisabled
	}
	if s.storageFailedLocked() {
		return SetupGrant{}, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) > 0 {
		return SetupGrant{}, ErrAlreadyInitialized
	}
	s.setup = nil
	s.ensureSetupLocked()
	return SetupGrant{Code: s.setup.code, DisplayCode: s.setup.display, ExpiresAt: s.setup.expiresAt}, nil
}

// VerifySetupCode validates the one-time setup code (either raw or hyphenated
// display form) and issues a short-lived onboarding ticket.
func (s *Service) VerifySetupCode(raw string) (OnboardingTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return OnboardingTicket{}, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) > 0 {
		return OnboardingTicket{}, ErrAlreadyInitialized
	}
	now := s.now().UTC()
	if s.setup == nil || now.After(s.setup.expiresAt) {
		return OnboardingTicket{}, ErrInvalidSetupCode
	}
	normalized := normalizeCodeInput(raw)
	if normalized == "" {
		return OnboardingTicket{}, ErrInvalidSetupCode
	}
	matchRaw := constantTimeStringEqual(strings.TrimSpace(raw), s.setup.code)
	matchDisplay := constantTimeStringEqual(normalized, normalizeCodeInput(s.setup.display))
	if !matchRaw && !matchDisplay {
		return OnboardingTicket{}, ErrInvalidSetupCode
	}
	s.prunePendingLocked(now)
	ticket := randomToken(24)
	exp := now.Add(onboardingTicketTT)
	s.tickets[ticket] = ticketEntry{expiresAt: exp}
	return OnboardingTicket{Ticket: ticket, ExpiresAt: exp}, nil
}

// CompleteOnboarding marks the optional post-passkey onboarding steps finished.
func (s *Service) CompleteOnboarding() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) == 0 {
		return ErrNotInitialized
	}
	if s.doc.OnboardingCompleted {
		return nil
	}
	next := s.cloneDocLocked()
	next.OnboardingCompleted = true
	if err := s.commitLocked(next); err != nil {
		return err
	}
	return nil
}

// AuthenticateBearer validates a Bearer or WebSocket subprotocol token against
// active Passkey sessions, user-created API tokens, and optional legacy config token.
func (s *Service) AuthenticateBearer(token string) (Principal, bool) {
	if s == nil {
		return Principal{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opts.Disabled {
		return Principal{Kind: PrincipalDisabled}, true
	}
	if s.storageFailedLocked() {
		return Principal{}, false
	}
	if token == "" {
		return Principal{}, false
	}
	gotHash := hashSecret(token)
	now := s.now().UTC()
	for i := range s.doc.Sessions {
		sess := s.doc.Sessions[i]
		exp, err := time.Parse(time.RFC3339Nano, sess.ExpiresAt)
		if err != nil || !exp.After(now) {
			continue
		}
		if constantTimeStringEqual(gotHash, sess.TokenHash) {
			s.touchSessionLocked(i, now)
			return Principal{Kind: PrincipalSession, SessionID: sess.ID, RPID: sess.RPID}, true
		}
	}
	for i := range s.doc.APITokens {
		tok := s.doc.APITokens[i]
		if tok.ExpiresAt != "" {
			if exp, err := time.Parse(time.RFC3339Nano, tok.ExpiresAt); err != nil || !exp.After(now) {
				continue
			}
		}
		if constantTimeStringEqual(gotHash, tok.TokenHash) {
			s.touchAPITokenLocked(i, now)
			return Principal{Kind: PrincipalAPIToken, TokenID: tok.ID}, true
		}
	}
	if s.opts.LegacyToken != "" && constantTimeStringEqual(gotHash, hashSecret(s.opts.LegacyToken)) {
		return Principal{Kind: PrincipalLegacyToken}, true
	}
	return Principal{}, false
}

// Reset clears all persisted passkeys, sessions, recovery codes, and API tokens
// on a running instance and generates a fresh setup code for re-onboarding.
func (s *Service) Reset() (SetupGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opts.Disabled {
		return SetupGrant{}, ErrDisabled
	}
	if s.handle == nil {
		return SetupGrant{}, ErrStorageUnavailable
	}
	if err := s.handle.ForceRemove(); err != nil {
		return SetupGrant{}, err
	}
	s.storageErr = nil
	s.doc = emptyStoredAuth()
	s.tickets = map[string]ticketEntry{}
	s.challenges = map[string]challengeEntry{}
	s.pairings = map[string]pairingEntry{}
	s.stepUps = map[string]stepUpEntry{}
	s.lastTouch = map[string]time.Time{}
	s.setup = nil
	s.ensureSetupLocked()
	return SetupGrant{Code: s.setup.code, DisplayCode: s.setup.display, ExpiresAt: s.setup.expiresAt}, nil
}
