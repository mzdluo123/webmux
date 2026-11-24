package auth

import (
	"errors"
	"time"
)

// Auth lifecycle states reported to clients and CLI diagnostics.
const (
	StateNeedsOnboarding = "needs_onboarding"
	StateReady           = "ready"
	StateStorageError    = "storage_error"
	StateDisabled        = "disabled"
	StateLegacyToken     = "legacy_token"
)

// PrincipalKind identifies how a Bearer / WS subprotocol credential authenticated.
const (
	PrincipalSession     = "session"
	PrincipalAPIToken    = "api_token"
	PrincipalLegacyToken = "legacy_token"
	PrincipalDisabled    = "disabled"
)

// Sentinel errors mapped to HTTP statuses in internal/api.
var (
	ErrStorageUnavailable = errors.New("auth storage unavailable")
	ErrLocked             = errors.New("auth store is locked by another running instance")
	ErrInvalidSetupCode   = errors.New("invalid or expired setup code")
	ErrInvalidTicket      = errors.New("invalid or expired onboarding ticket")
	ErrInvalidChallenge   = errors.New("invalid or expired WebAuthn challenge")
	ErrInvalidAssertion   = errors.New("invalid WebAuthn signature")
	ErrSignCountAnomaly   = errors.New("passkey signature counter anomaly detected")
	ErrAlreadyInitialized = errors.New("WebMux authentication is already initialized")
	ErrNotInitialized     = errors.New("WebMux onboarding has not been completed")
	ErrInvalidPairingCode = errors.New("invalid or expired pairing code")
	ErrInvalidRecovery    = errors.New("invalid or already used recovery code")
	ErrInvalidRPID        = errors.New("invalid WebAuthn RP ID")
	ErrNoPasskeyForRPID   = errors.New("no passkey registered for this domain")
	ErrStepUpRequired     = errors.New("step-up passkey verification required")
	ErrNotFound           = errors.New("auth resource not found")
	ErrLimitReached       = errors.New("resource limit reached")
	ErrInvalidInput       = errors.New("invalid auth input")
	ErrRateLimited        = errors.New("too many authentication attempts; try again shortly")
	ErrDisabled           = errors.New("authentication is disabled on this instance")
)

// Options configures Service.Open.
type Options struct {
	// Disabled turns off TCP authentication (explicit loopback-only --insecure-no-auth).
	Disabled bool
	// LegacyToken keeps an explicitly configured static token working for
	// backwards compatibility and automated test suites.
	LegacyToken string
	// RPDisplayName is shown by authenticators during registration (e.g. "WebMux (host)").
	RPDisplayName string
	// UserName is the WebAuthn user.name label (e.g. "alice@host").
	UserName string
	// Now overrides time.Now for deterministic tests.
	Now func() time.Time
}

// InspectSummary is returned by offline Inspect and live status diagnostics.
type InspectSummary struct {
	State                  string
	OnboardingCompleted    bool
	PasskeyCount           int
	RPIDs                  []string
	SessionCount           int
	RecoveryCodesRemaining int
	APITokenCount          int
	Error                  string
}

// StatusSnapshot is returned by GET /api/auth/status.
type StatusSnapshot struct {
	State                  string
	RPID                   string
	RPIDValid              bool
	RPIDError              string
	HasPasskeyForRPID      bool
	OnboardingCompleted    bool
	PasskeyCount           int
	SessionCount           int
	RecoveryCodesRemaining int
	Error                  string
}

// SetupGrant describes the active in-memory one-time setup code for onboarding.
type SetupGrant struct {
	Code        string
	DisplayCode string
	ExpiresAt   time.Time
}

// OnboardingTicket authorizes registering the first Passkey after claiming ownership.
type OnboardingTicket struct {
	Ticket    string
	ExpiresAt time.Time
}

// PairingGrant is a short-lived single-use code to register a Passkey on another
// device or domain (RP ID).
type PairingGrant struct {
	Code        string
	DisplayCode string
	TargetURL   string
	ExpiresAt   time.Time
}

// Principal describes the authenticated caller on an HTTP or WebSocket request.
type Principal struct {
	Kind      string
	SessionID string
	TokenID   string
	RPID      string
}

// RegistrationOptions holds the WebAuthn creation challenge parameters.
type RegistrationOptions struct {
	ChallengeID string
	Challenge   string
	RPID        string
	RPName      string
	UserID      string
	UserName    string
	DisplayName string
	ExcludeIDs  []string
	PubKeyAlgs  []int64
	TimeoutMS   int
}

// AssertionOptions holds the WebAuthn get challenge parameters.
type AssertionOptions struct {
	ChallengeID string
	Challenge   string
	RPID        string
	AllowIDs    []string
	TimeoutMS   int
}

// PasskeyFinishInput carries the browser's registration payload.
type PasskeyFinishInput struct {
	ChallengeID       string
	Name              string
	Origin            string
	ClientDataJSON    string
	AttestationObject string
	Transports        []string
	DeviceName        string
}

// AssertionFinishInput carries the browser's assertion payload.
type AssertionFinishInput struct {
	ChallengeID       string
	CredentialID      string
	Origin            string
	ClientDataJSON    string
	AuthenticatorData string
	Signature         string
	DeviceName        string
}

// OnboardingCompleteResult returns the new session token and one-time recovery codes.
type OnboardingCompleteResult struct {
	SessionToken  string
	Session       SessionInfo
	Passkey       PasskeyInfo
	RecoveryCodes []string
}

// LoginResult returns the new session token and session metadata.
type LoginResult struct {
	SessionToken           string
	Session                SessionInfo
	RecoveryCodesRemaining int
}

// StepUpResult returns a short-lived step-up verification token.
type StepUpResult struct {
	StepUpToken string
	ExpiresAt   time.Time
}

// PasskeyInfo is the client-visible projection of a registered Passkey.
type PasskeyInfo struct {
	ID             string
	RPID           string
	Name           string
	Transports     []string
	BackupEligible bool
	BackupState    bool
	CreatedAt      string
	LastUsedAt     string
}

// SessionInfo is the client-visible projection of an active browser session.
type SessionInfo struct {
	ID         string
	Name       string
	RPID       string
	Current    bool
	CreatedAt  string
	LastUsedAt string
	ExpiresAt  string
}

// APITokenInfo is the client-visible metadata of a named personal access token.
type APITokenInfo struct {
	ID         string
	Name       string
	CreatedAt  string
	LastUsedAt string
	ExpiresAt  string
}

// CreatedAPIToken includes the one-time plaintext secret returned only on creation.
type CreatedAPIToken struct {
	Token string
	Info  APITokenInfo
}
