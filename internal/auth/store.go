package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/statefile"
)

const (
	storeVersion      = 1
	maxPasskeys       = 32
	maxSessions       = 64
	maxAPITokens      = 16
	maxRecoveryCodes  = 10
	recoveryCodeCount = 6
)

type storedAuth struct {
	V                   int              `json:"v"`
	OnboardingCompleted bool             `json:"onboardingCompleted"`
	UserHandle          string           `json:"userHandle"`
	Passkeys            []PasskeyRecord  `json:"passkeys"`
	Sessions            []SessionRecord  `json:"sessions"`
	RecoveryCodes       []RecoveryRecord `json:"recoveryCodes"`
	APITokens           []APITokenRecord `json:"apiTokens"`
}

// PasskeyRecord is one persisted WebAuthn credential bound to a specific RP ID.
type PasskeyRecord struct {
	ID             string   `json:"id"`
	RPID           string   `json:"rpId"`
	Name           string   `json:"name"`
	PublicKeyCOSE  string   `json:"publicKeyCose"`
	Alg            int64    `json:"alg"`
	SignCount      uint32   `json:"signCount"`
	Transports     []string `json:"transports,omitempty"`
	BackupEligible bool     `json:"backupEligible"`
	BackupState    bool     `json:"backupState"`
	CreatedAt      string   `json:"createdAt"`
	LastUsedAt     string   `json:"lastUsedAt,omitempty"`
}

// SessionRecord is one revocable browser login session (token hash only).
type SessionRecord struct {
	ID         string `json:"id"`
	TokenHash  string `json:"tokenHash"`
	Name       string `json:"name"`
	RPID       string `json:"rpId"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt"`
	ExpiresAt  string `json:"expiresAt"`
}

// RecoveryRecord is one single-use recovery code hash.
type RecoveryRecord struct {
	ID     string `json:"id"`
	Hash   string `json:"hash"`
	UsedAt string `json:"usedAt,omitempty"`
}

// APITokenRecord is one optional user-created personal access token (hash only).
type APITokenRecord struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TokenHash  string `json:"tokenHash"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
}

func emptyStoredAuth() storedAuth {
	return storedAuth{
		V:             storeVersion,
		Passkeys:      []PasskeyRecord{},
		Sessions:      []SessionRecord{},
		RecoveryCodes: []RecoveryRecord{},
		APITokens:     []APITokenRecord{},
	}
}

func validateStoredAuth(doc storedAuth) error {
	if doc.V != storeVersion {
		return fmt.Errorf("unsupported auth store version %d", doc.V)
	}
	if len(doc.Passkeys) > maxPasskeys {
		return fmt.Errorf("too many passkeys (%d > %d)", len(doc.Passkeys), maxPasskeys)
	}
	if len(doc.Sessions) > maxSessions {
		return fmt.Errorf("too many sessions (%d > %d)", len(doc.Sessions), maxSessions)
	}
	if len(doc.APITokens) > maxAPITokens {
		return fmt.Errorf("too many API tokens (%d > %d)", len(doc.APITokens), maxAPITokens)
	}
	if len(doc.RecoveryCodes) > maxRecoveryCodes {
		return fmt.Errorf("too many recovery codes (%d > %d)", len(doc.RecoveryCodes), maxRecoveryCodes)
	}
	if len(doc.Passkeys) > 0 && strings.TrimSpace(doc.UserHandle) == "" {
		return errors.New("auth store has passkeys without userHandle")
	}
	seenPasskeys := make(map[string]bool, len(doc.Passkeys))
	for _, p := range doc.Passkeys {
		if p.ID == "" || seenPasskeys[p.ID] {
			return errors.New("invalid or duplicate passkey ID")
		}
		seenPasskeys[p.ID] = true
		if _, err := ValidateRPID(p.RPID); err != nil {
			return fmt.Errorf("passkey %s: %w", p.ID, err)
		}
		if p.PublicKeyCOSE == "" || p.CreatedAt == "" {
			return fmt.Errorf("passkey %s missing required fields", p.ID)
		}
	}
	seenSessions := make(map[string]bool, len(doc.Sessions))
	for _, s := range doc.Sessions {
		if s.ID == "" || seenSessions[s.ID] || s.TokenHash == "" || s.ExpiresAt == "" {
			return errors.New("invalid session record")
		}
		if _, err := time.Parse(time.RFC3339Nano, s.ExpiresAt); err != nil {
			return fmt.Errorf("session %s has invalid expiresAt", s.ID)
		}
		seenSessions[s.ID] = true
	}
	seenTokens := make(map[string]bool, len(doc.APITokens))
	for _, tok := range doc.APITokens {
		if tok.ID == "" || seenTokens[tok.ID] || tok.TokenHash == "" || strings.TrimSpace(tok.Name) == "" {
			return errors.New("invalid API token record")
		}
		if tok.ExpiresAt != "" {
			if _, err := time.Parse(time.RFC3339Nano, tok.ExpiresAt); err != nil {
				return fmt.Errorf("API token %s has invalid expiresAt", tok.ID)
			}
		}
		seenTokens[tok.ID] = true
	}
	return nil
}

// Inspect reads auth.json without creating directories, acquiring locks, or
// mutating files. Used by offline `webmux config show` and `webmux auth status`.
func Inspect(stateDir string) (InspectSummary, error) {
	doc, err := statefile.ReadSnapshot[storedAuth](stateDir, statefile.Auth, validateStoredAuth)
	if statefile.IsKind(err, statefile.Missing) {
		return InspectSummary{State: StateNeedsOnboarding}, nil
	}
	if err != nil {
		return InspectSummary{State: StateStorageError, Error: err.Error()}, err
	}
	state := StateNeedsOnboarding
	if len(doc.Passkeys) > 0 {
		state = StateReady
	}
	now := time.Now().UTC()
	activeSessions := 0
	for _, s := range doc.Sessions {
		if exp, parseErr := time.Parse(time.RFC3339Nano, s.ExpiresAt); parseErr == nil && exp.After(now) {
			activeSessions++
		}
	}
	remainingRecovery := 0
	for _, r := range doc.RecoveryCodes {
		if r.UsedAt == "" {
			remainingRecovery++
		}
	}
	rpIDs := uniqueRPIDs(doc.Passkeys)
	return InspectSummary{
		State:                  state,
		OnboardingCompleted:    doc.OnboardingCompleted,
		PasskeyCount:           len(doc.Passkeys),
		RPIDs:                  rpIDs,
		SessionCount:           activeSessions,
		RecoveryCodesRemaining: remainingRecovery,
		APITokenCount:          len(doc.APITokens),
	}, nil
}

// ResetOffline resets auth.json when no server instance holds its lock.
func ResetOffline(stateDir string) error {
	handle, _, err := statefile.Open[storedAuth](stateDir, statefile.Auth, validateStoredAuth)
	if handle != nil {
		defer handle.Close()
	}
	if err != nil && !statefile.IsKind(err, statefile.Missing) {
		if statefile.IsKind(err, statefile.Locked) {
			return ErrLocked
		}
		// Even if the previous file was corrupt, once we hold the lock we may
		// remove the broken auth.json explicitly when the operator runs reset.
		if handle == nil || handle.Path() == "" {
			return err
		}
		return handle.ForceRemove()
	}
	if handle == nil {
		return errors.New("auth storage unavailable")
	}
	return handle.ForceRemove()
}

func uniqueRPIDs(passkeys []PasskeyRecord) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(passkeys))
	for _, p := range passkeys {
		if !seen[p.RPID] {
			seen[p.RPID] = true
			out = append(out, p.RPID)
		}
	}
	return out
}
