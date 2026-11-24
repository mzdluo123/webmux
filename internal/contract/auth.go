package contract

// Auth states returned by GET /api/auth/status and /local/v1/status.
const (
	AuthNeedsOnboarding = "needs_onboarding"
	AuthReady           = "ready"
	AuthStorageError    = "storage_error"
	AuthDisabled        = "disabled"
	AuthLegacyToken     = "legacy_token"
)

// AuthSummary is the non-sensitive auth status embedded in /local/v1/status
// and `webmux auth status --json`.
type AuthSummary struct {
	State                  string   `json:"state"`
	OnboardingCompleted    bool     `json:"onboardingCompleted"`
	PasskeyCount           int      `json:"passkeyCount"`
	RPIDs                  []string `json:"rpIds,omitempty"`
	SessionCount           int      `json:"sessionCount"`
	RecoveryCodesRemaining int      `json:"recoveryCodesRemaining"`
	APITokenCount          int      `json:"apiTokenCount"`
	Error                  string   `json:"error,omitempty"`
}

// AuthStatusResponse is returned by GET /api/auth/status (public).
type AuthStatusResponse struct {
	State                  string `json:"state"`
	RPID                   string `json:"rpId,omitempty"`
	RPIDValid              bool   `json:"rpIdValid"`
	RPIDError              string `json:"rpIdError,omitempty"`
	HasPasskeyForRPID      bool   `json:"hasPasskeyForRpId"`
	OnboardingCompleted    bool   `json:"onboardingCompleted"`
	PasskeyCount           int    `json:"passkeyCount"`
	SessionCount           int    `json:"sessionCount"`
	RecoveryCodesRemaining int    `json:"recoveryCodesRemaining"`
	Hostname               string `json:"hostname,omitempty"`
	Error                  string `json:"error,omitempty"`
}

// OnboardingVerifyRequest is the body of POST /api/auth/onboarding/verify.
type OnboardingVerifyRequest struct {
	Code string `json:"code"`
}

// OnboardingVerifyResponse returns a short-lived ticket for registering the first passkey.
type OnboardingVerifyResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresAt string `json:"expiresAt"`
}

// OnboardingBeginRequest is the body of POST /api/auth/onboarding/passkey/begin.
type OnboardingBeginRequest struct {
	Ticket string `json:"ticket"`
}

// WebAuthnCreationOptions is the browser-facing WebAuthn create parameters.
type WebAuthnCreationOptions struct {
	ChallengeID string   `json:"challengeId"`
	Challenge   string   `json:"challenge"`
	RPID        string   `json:"rpId"`
	RPName      string   `json:"rpName"`
	UserID      string   `json:"userId"`
	UserName    string   `json:"userName"`
	DisplayName string   `json:"displayName"`
	ExcludeIDs  []string `json:"excludeIds"`
	PubKeyAlgs  []int64  `json:"pubKeyAlgs"`
	TimeoutMS   int      `json:"timeoutMs"`
}

// WebAuthnAssertionOptions is the browser-facing WebAuthn get parameters.
type WebAuthnAssertionOptions struct {
	ChallengeID string   `json:"challengeId"`
	Challenge   string   `json:"challenge"`
	RPID        string   `json:"rpId"`
	AllowIDs    []string `json:"allowIds"`
	TimeoutMS   int      `json:"timeoutMs"`
}

// OnboardingFinishRequest is the body of POST /api/auth/onboarding/passkey/finish.
type OnboardingFinishRequest struct {
	Ticket            string   `json:"ticket"`
	ChallengeID       string   `json:"challengeId"`
	Name              string   `json:"name"`
	ClientDataJSON    string   `json:"clientDataJson"`
	AttestationObject string   `json:"attestationObject"`
	Transports        []string `json:"transports,omitempty"`
	DeviceName        string   `json:"deviceName,omitempty"`
}

// OnboardingFinishResponse returns the initial session token and one-time recovery codes.
type OnboardingFinishResponse struct {
	SessionToken  string      `json:"sessionToken"`
	Session       AuthSession `json:"session"`
	Passkey       AuthPasskey `json:"passkey"`
	RecoveryCodes []string    `json:"recoveryCodes"`
}

// AssertionFinishRequest is the body of POST /api/auth/login/finish and step-up/finish.
type AssertionFinishRequest struct {
	ChallengeID       string `json:"challengeId"`
	CredentialID      string `json:"credentialId"`
	ClientDataJSON    string `json:"clientDataJson"`
	AuthenticatorData string `json:"authenticatorData"`
	Signature         string `json:"signature"`
	DeviceName        string `json:"deviceName,omitempty"`
}

// AuthLoginResponse is returned by login, pair, and recovery login endpoints.
type AuthLoginResponse struct {
	SessionToken           string       `json:"sessionToken"`
	Session                AuthSession  `json:"session"`
	Passkey                *AuthPasskey `json:"passkey,omitempty"`
	RecoveryCodesRemaining int          `json:"recoveryCodesRemaining"`
}

// StepUpFinishResponse is returned by POST /api/auth/step-up/finish.
type StepUpFinishResponse struct {
	StepUpToken string `json:"stepUpToken"`
	ExpiresAt   string `json:"expiresAt"`
}

// PairCreateRequest is the body of POST /api/auth/pair/create and /local/v1/auth/pair.
type PairCreateRequest struct {
	TargetURL string `json:"targetUrl,omitempty"`
}

// PairCreateResponse returns a 5-minute single-use pairing code and optional URL.
type PairCreateResponse struct {
	Code        string `json:"code"`
	DisplayCode string `json:"displayCode"`
	PairURL     string `json:"pairUrl,omitempty"`
	ExpiresAt   string `json:"expiresAt"`
}

// PairBeginRequest is the body of POST /api/auth/pair/begin.
type PairBeginRequest struct {
	Code string `json:"code"`
}

// PasskeyFinishRequest is the body of POST /api/auth/pair/finish and /api/auth/passkeys/finish.
type PasskeyFinishRequest struct {
	ChallengeID       string   `json:"challengeId"`
	Name              string   `json:"name"`
	ClientDataJSON    string   `json:"clientDataJson"`
	AttestationObject string   `json:"attestationObject"`
	Transports        []string `json:"transports,omitempty"`
	DeviceName        string   `json:"deviceName,omitempty"`
}

// RecoveryLoginRequest is the body of POST /api/auth/recovery.
type RecoveryLoginRequest struct {
	Code       string `json:"code"`
	DeviceName string `json:"deviceName,omitempty"`
}

// RecoveryCodesResponse is returned by POST /api/auth/recovery-codes/regenerate.
type RecoveryCodesResponse struct {
	RecoveryCodes []string `json:"recoveryCodes"`
}

// AuthPasskey describes one registered Passkey.
type AuthPasskey struct {
	ID             string   `json:"id"`
	RPID           string   `json:"rpId"`
	Name           string   `json:"name"`
	Transports     []string `json:"transports,omitempty"`
	BackupEligible bool     `json:"backupEligible"`
	BackupState    bool     `json:"backupState"`
	CreatedAt      string   `json:"createdAt"`
	LastUsedAt     string   `json:"lastUsedAt,omitempty"`
}

// PasskeyListResponse is returned by GET /api/auth/passkeys.
type PasskeyListResponse struct {
	Passkeys []AuthPasskey `json:"passkeys"`
}

// RenamePasskeyRequest is the body of PATCH /api/auth/passkeys/{id}.
type RenamePasskeyRequest struct {
	Name string `json:"name"`
}

// AuthSession describes one active browser session.
type AuthSession struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	RPID       string `json:"rpId"`
	Current    bool   `json:"current"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt"`
	ExpiresAt  string `json:"expiresAt"`
}

// SessionListResponse is returned by GET /api/auth/sessions.
type SessionListResponse struct {
	Sessions []AuthSession `json:"sessions"`
}

// RevokeOthersResponse is returned by POST /api/auth/sessions/revoke-others.
type RevokeOthersResponse struct {
	Revoked int `json:"revoked"`
}

// AuthAPIToken describes one named personal access token without secret.
type AuthAPIToken struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
}

// APITokenListResponse is returned by GET /api/auth/tokens.
type APITokenListResponse struct {
	Tokens []AuthAPIToken `json:"tokens"`
}

// CreateAPITokenRequest is the body of POST /api/auth/tokens.
type CreateAPITokenRequest struct {
	Name        string `json:"name"`
	ExpiresDays int    `json:"expiresDays,omitempty"`
}

// CreateAPITokenResponse returns the one-time plaintext token on creation.
type CreateAPITokenResponse struct {
	Token string       `json:"token"`
	Info  AuthAPIToken `json:"info"`
}

// LocalAuthSetupResponse is returned by POST /local/v1/auth/setup and reset.
type LocalAuthSetupResponse struct {
	Code        string `json:"code"`
	DisplayCode string `json:"displayCode"`
	SetupURL    string `json:"setupUrl"`
	ExpiresAt   string `json:"expiresAt"`
}

// LocalAuthResetRequest is the body of POST /local/v1/auth/reset.
type LocalAuthResetRequest struct {
	Confirm bool `json:"confirm"`
}

// LocalAuthSetupRequest is the optional body of POST /local/v1/auth/setup.
type LocalAuthSetupRequest struct {
	Refresh bool `json:"refresh,omitempty"`
}

// AuthReasonError is an error response with a machine-readable reason (e.g. step_up_required).
type AuthReasonError struct {
	Error  string `json:"error"`
	Reason string `json:"reason,omitempty"`
}
