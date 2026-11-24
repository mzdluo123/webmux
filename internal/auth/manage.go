package auth

import (
	"fmt"
	"strings"
	"time"
)

// ListPasskeys returns all registered passkeys without key material.
func (s *Service) ListPasskeys() ([]PasskeyInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return nil, ErrStorageUnavailable
	}
	out := make([]PasskeyInfo, len(s.doc.Passkeys))
	for i, p := range s.doc.Passkeys {
		out[i] = toPasskeyInfo(p)
	}
	return out, nil
}

// RenamePasskey updates the display name of an existing passkey.
func (s *Service) RenamePasskey(id, name string) (PasskeyInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return PasskeyInfo{}, ErrStorageUnavailable
	}
	cleaned := sanitizeName(name, "", 80)
	if cleaned == "" {
		return PasskeyInfo{}, fmt.Errorf("%w: passkey name must not be empty", ErrInvalidInput)
	}
	idx := -1
	for i, p := range s.doc.Passkeys {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return PasskeyInfo{}, ErrNotFound
	}
	next := s.cloneDocLocked()
	next.Passkeys[idx].Name = cleaned
	if err := s.commitLocked(next); err != nil {
		return PasskeyInfo{}, err
	}
	return toPasskeyInfo(next.Passkeys[idx]), nil
}

// DeletePasskey removes a passkey. Deleting the final passkey is rejected
// unless allowLast is true (to prevent accidental lockout).
func (s *Service) DeletePasskey(id string, allowLast bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return ErrStorageUnavailable
	}
	idx := -1
	for i, p := range s.doc.Passkeys {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	if len(s.doc.Passkeys) <= 1 && !allowLast {
		return fmt.Errorf("%w: cannot remove the last passkey; add another passkey or use webmux auth reset", ErrInvalidInput)
	}
	next := s.cloneDocLocked()
	next.Passkeys = append(next.Passkeys[:idx], next.Passkeys[idx+1:]...)
	if len(next.Passkeys) == 0 {
		next.OnboardingCompleted = false
	}
	if err := s.commitLocked(next); err != nil {
		return err
	}
	if len(s.doc.Passkeys) == 0 {
		s.ensureSetupLocked()
	}
	return nil
}

// ListSessions returns all active (unexpired) browser sessions.
func (s *Service) ListSessions(currentSessionID string) ([]SessionInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return nil, ErrStorageUnavailable
	}
	now := s.now().UTC()
	out := make([]SessionInfo, 0, len(s.doc.Sessions))
	for _, sess := range s.doc.Sessions {
		if exp, err := time.Parse(time.RFC3339Nano, sess.ExpiresAt); err == nil && exp.After(now) {
			out = append(out, toSessionInfo(sess, currentSessionID))
		}
	}
	return out, nil
}

// Logout revokes the caller's own session token.
func (s *Service) Logout(sessionID string) error {
	if sessionID == "" {
		return nil
	}
	return s.RevokeSession(sessionID)
}

// RevokeSession removes one session by ID, immediately invalidating its token.
func (s *Service) RevokeSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return ErrStorageUnavailable
	}
	idx := -1
	for i, sess := range s.doc.Sessions {
		if sess.ID == sessionID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	next := s.cloneDocLocked()
	next.Sessions = append(next.Sessions[:idx], next.Sessions[idx+1:]...)
	if err := s.commitLocked(next); err != nil {
		return err
	}
	s.clearStepUpsForSessionLocked(sessionID)
	delete(s.lastTouch, sessionID)
	return nil
}

// RevokeOtherSessions revokes every active session except currentSessionID.
func (s *Service) RevokeOtherSessions(currentSessionID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return 0, ErrStorageUnavailable
	}
	next := s.cloneDocLocked()
	kept := make([]SessionRecord, 0, 1)
	removed := 0
	for _, sess := range next.Sessions {
		if sess.ID == currentSessionID {
			kept = append(kept, sess)
		} else {
			removed++
			s.clearStepUpsForSessionLocked(sess.ID)
			delete(s.lastTouch, sess.ID)
		}
	}
	if removed == 0 {
		return 0, nil
	}
	next.Sessions = kept
	if err := s.commitLocked(next); err != nil {
		return 0, err
	}
	return removed, nil
}

// LoginWithRecoveryCode consumes one unused recovery code and creates a session.
func (s *Service) LoginWithRecoveryCode(rawCode, deviceName, host string) (LoginResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return LoginResult{}, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) == 0 {
		return LoginResult{}, ErrNotInitialized
	}
	norm := normalizeCodeInput(rawCode)
	if len(norm) < 6 {
		return LoginResult{}, ErrInvalidRecovery
	}
	gotHash := hashSecret(norm)
	idx := -1
	for i, rec := range s.doc.RecoveryCodes {
		if rec.UsedAt == "" && constantTimeStringEqual(gotHash, rec.Hash) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return LoginResult{}, ErrInvalidRecovery
	}
	rpID, _ := ValidateRPID(host)
	now := s.now().UTC()
	next := s.cloneDocLocked()
	next.RecoveryCodes[idx].UsedAt = now.Format(time.RFC3339Nano)
	sessionToken, sess := s.appendSessionLocked(&next, deviceName, rpID, now)
	if err := s.commitLocked(next); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		SessionToken:           sessionToken,
		Session:                toSessionInfo(sess, sess.ID),
		RecoveryCodesRemaining: s.remainingRecoveryLocked(),
	}, nil
}

// RegenerateRecoveryCodes replaces all recovery codes with a fresh set.
func (s *Service) RegenerateRecoveryCodes() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return nil, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) == 0 {
		return nil, ErrNotInitialized
	}
	plain, records := generateRecoveryBatch()
	next := s.cloneDocLocked()
	next.RecoveryCodes = records
	if err := s.commitLocked(next); err != nil {
		return nil, err
	}
	return plain, nil
}

// ListAPITokens lists all named personal access tokens (metadata only).
func (s *Service) ListAPITokens() ([]APITokenInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return nil, ErrStorageUnavailable
	}
	out := make([]APITokenInfo, len(s.doc.APITokens))
	for i, tok := range s.doc.APITokens {
		out[i] = toAPITokenInfo(tok)
	}
	return out, nil
}

// CreateAPIToken creates a named personal access token for scripts/automation.
// expiresDays <= 0 creates a non-expiring token; max 365 days when set.
func (s *Service) CreateAPIToken(name string, expiresDays int) (CreatedAPIToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return CreatedAPIToken{}, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) == 0 {
		return CreatedAPIToken{}, ErrNotInitialized
	}
	if len(s.doc.APITokens) >= maxAPITokens {
		return CreatedAPIToken{}, ErrLimitReached
	}
	cleaned := sanitizeName(name, "", 80)
	if cleaned == "" {
		return CreatedAPIToken{}, fmt.Errorf("%w: token name is required", ErrInvalidInput)
	}
	if expiresDays < 0 || expiresDays > 365 {
		return CreatedAPIToken{}, fmt.Errorf("%w: expiresDays must be between 0 and 365", ErrInvalidInput)
	}
	now := s.now().UTC()
	secret := "wm_pat_" + randomToken(32)
	rec := APITokenRecord{
		ID:        "pat_" + randomToken(10),
		Name:      cleaned,
		TokenHash: hashSecret(secret),
		CreatedAt: now.Format(time.RFC3339Nano),
	}
	if expiresDays > 0 {
		rec.ExpiresAt = now.Add(time.Duration(expiresDays) * 24 * time.Hour).Format(time.RFC3339Nano)
	}
	next := s.cloneDocLocked()
	next.APITokens = append(next.APITokens, rec)
	if err := s.commitLocked(next); err != nil {
		return CreatedAPIToken{}, err
	}
	return CreatedAPIToken{Token: secret, Info: toAPITokenInfo(rec)}, nil
}

// RevokeAPIToken deletes one personal access token by ID.
func (s *Service) RevokeAPIToken(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return ErrStorageUnavailable
	}
	idx := -1
	for i, tok := range s.doc.APITokens {
		if tok.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	next := s.cloneDocLocked()
	next.APITokens = append(next.APITokens[:idx], next.APITokens[idx+1:]...)
	if err := s.commitLocked(next); err != nil {
		return err
	}
	delete(s.lastTouch, "pat:"+id)
	return nil
}

func (s *Service) clearStepUpsForSessionLocked(sessionID string) {
	for token, entry := range s.stepUps {
		if entry.sessionID == sessionID {
			delete(s.stepUps, token)
		}
	}
}

func toPasskeyInfo(p PasskeyRecord) PasskeyInfo {
	return PasskeyInfo{
		ID:             p.ID,
		RPID:           p.RPID,
		Name:           p.Name,
		Transports:     append([]string{}, p.Transports...),
		BackupEligible: p.BackupEligible,
		BackupState:    p.BackupState,
		CreatedAt:      p.CreatedAt,
		LastUsedAt:     p.LastUsedAt,
	}
}

func toSessionInfo(sess SessionRecord, currentID string) SessionInfo {
	return SessionInfo{
		ID:         sess.ID,
		Name:       sess.Name,
		RPID:       sess.RPID,
		Current:    currentID != "" && sess.ID == currentID,
		CreatedAt:  sess.CreatedAt,
		LastUsedAt: sess.LastUsedAt,
		ExpiresAt:  sess.ExpiresAt,
	}
}

func toAPITokenInfo(tok APITokenRecord) APITokenInfo {
	return APITokenInfo{
		ID:         tok.ID,
		Name:       strings.TrimSpace(tok.Name),
		CreatedAt:  tok.CreatedAt,
		LastUsedAt: tok.LastUsedAt,
		ExpiresAt:  tok.ExpiresAt,
	}
}
