package auth

import (
	"fmt"
	"strings"
	"time"
)

// CreatePairingCode generates a 5-minute single-use code for registering a
// Passkey on a new device or a new domain (RP ID).
func (s *Service) CreatePairingCode(targetURL string) (PairingGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opts.Disabled {
		return PairingGrant{}, ErrDisabled
	}
	if s.storageFailedLocked() {
		return PairingGrant{}, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) == 0 {
		return PairingGrant{}, ErrNotInitialized
	}
	if len(s.doc.Passkeys) >= maxPasskeys {
		return PairingGrant{}, ErrLimitReached
	}
	now := s.now().UTC()
	s.prunePendingLocked(now)
	code := randomToken(18)
	display := formatDisplayCode(8)
	exp := now.Add(pairingCodeTTL)
	s.pairings[code] = pairingEntry{
		display:   display,
		targetURL: strings.TrimRight(strings.TrimSpace(targetURL), "/"),
		expiresAt: exp,
	}
	return PairingGrant{
		Code:        code,
		DisplayCode: display,
		TargetURL:   s.pairings[code].targetURL,
		ExpiresAt:   exp,
	}, nil
}

// BeginPair starts Passkey registration for a new device/domain using a pairing code.
func (s *Service) BeginPair(rawCode, host string) (RegistrationOptions, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return RegistrationOptions{}, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) == 0 {
		return RegistrationOptions{}, ErrNotInitialized
	}
	if len(s.doc.Passkeys) >= maxPasskeys {
		return RegistrationOptions{}, ErrLimitReached
	}
	now := s.now().UTC()
	pairKey, ok := s.matchPairingCodeLocked(rawCode, now)
	if !ok {
		return RegistrationOptions{}, ErrInvalidPairingCode
	}
	rpID, err := ValidateRPID(host)
	if err != nil {
		return RegistrationOptions{}, fmt.Errorf("%w: %s", ErrInvalidRPID, err.Error())
	}
	s.prunePendingLocked(now)
	id := randomToken(16)
	challenge := randomToken(32)
	s.challenges[id] = challengeEntry{
		kind:        challengePair,
		challenge:   challenge,
		rpID:        rpID,
		userHandle:  s.doc.UserHandle,
		pairingCode: pairKey,
		expiresAt:   now.Add(challengeTTL),
	}
	return RegistrationOptions{
		ChallengeID: id,
		Challenge:   challenge,
		RPID:        rpID,
		RPName:      s.opts.RPDisplayName,
		UserID:      s.doc.UserHandle,
		UserName:    s.opts.UserName,
		DisplayName: "WebMux Owner",
		ExcludeIDs:  s.credentialIDsForRPIDLocked(rpID),
		PubKeyAlgs:  append([]int64{}, supportedPubKeyAlgs...),
		TimeoutMS:   int(challengeTTL.Milliseconds()),
	}, nil
}

// FinishPair verifies the new Passkey, consumes the pairing code, and signs in.
func (s *Service) FinishPair(in PasskeyFinishInput) (LoginResult, PasskeyInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return LoginResult{}, PasskeyInfo{}, ErrStorageUnavailable
	}
	now := s.now().UTC()
	ch, ok := s.challenges[in.ChallengeID]
	delete(s.challenges, in.ChallengeID)
	if !ok || ch.kind != challengePair || now.After(ch.expiresAt) {
		return LoginResult{}, PasskeyInfo{}, ErrInvalidChallenge
	}
	pEntry, ok := s.pairings[ch.pairingCode]
	if !ok || now.After(pEntry.expiresAt) {
		delete(s.pairings, ch.pairingCode)
		return LoginResult{}, PasskeyInfo{}, ErrInvalidPairingCode
	}
	if len(s.doc.Passkeys) >= maxPasskeys {
		return LoginResult{}, PasskeyInfo{}, ErrLimitReached
	}
	verified, err := VerifyRegistration(RegistrationVerifyInput{
		ExpectedChallenge: ch.challenge,
		ExpectedOrigin:    in.Origin,
		ExpectedRPID:      ch.rpID,
		ClientDataJSON:    in.ClientDataJSON,
		AttestationObject: in.AttestationObject,
	})
	if err != nil {
		return LoginResult{}, PasskeyInfo{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	if s.hasCredentialIDLocked(verified.CredentialID) {
		return LoginResult{}, PasskeyInfo{}, fmt.Errorf("%w: credential already registered", ErrInvalidInput)
	}
	next := s.cloneDocLocked()
	pk := PasskeyRecord{
		ID:             verified.CredentialID,
		RPID:           ch.rpID,
		Name:           sanitizeName(in.Name, defaultPasskeyName(ch.rpID), 80),
		PublicKeyCOSE:  verified.PublicKeyCOSE,
		Alg:            verified.Alg,
		SignCount:      verified.SignCount,
		Transports:     sanitizeTransports(in.Transports),
		BackupEligible: verified.BackupEligible,
		BackupState:    verified.BackupState,
		CreatedAt:      now.Format(time.RFC3339Nano),
		LastUsedAt:     now.Format(time.RFC3339Nano),
	}
	next.Passkeys = append(next.Passkeys, pk)
	sessionToken, sess := s.appendSessionLocked(&next, in.DeviceName, ch.rpID, now)
	if err := s.commitLocked(next); err != nil {
		return LoginResult{}, PasskeyInfo{}, err
	}
	delete(s.pairings, ch.pairingCode)
	return LoginResult{
		SessionToken:           sessionToken,
		Session:                toSessionInfo(sess, sess.ID),
		RecoveryCodesRemaining: s.remainingRecoveryLocked(),
	}, toPasskeyInfo(pk), nil
}

// BeginStepUp starts a secondary Passkey verification for a sensitive action.
func (s *Service) BeginStepUp(sessionID, host string) (AssertionOptions, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return AssertionOptions{}, ErrStorageUnavailable
	}
	rpID, err := ValidateRPID(host)
	if err != nil {
		return AssertionOptions{}, fmt.Errorf("%w: %s", ErrInvalidRPID, err.Error())
	}
	allowIDs := s.credentialIDsForRPIDLocked(rpID)
	if len(allowIDs) == 0 {
		return AssertionOptions{}, ErrNoPasskeyForRPID
	}
	now := s.now().UTC()
	s.prunePendingLocked(now)
	id := randomToken(16)
	challenge := randomToken(32)
	s.challenges[id] = challengeEntry{
		kind:      challengeStepUp,
		challenge: challenge,
		rpID:      rpID,
		sessionID: sessionID,
		expiresAt: now.Add(challengeTTL),
	}
	return AssertionOptions{
		ChallengeID: id,
		Challenge:   challenge,
		RPID:        rpID,
		AllowIDs:    allowIDs,
		TimeoutMS:   int(challengeTTL.Milliseconds()),
	}, nil
}

// FinishStepUp validates a step-up Passkey assertion and mints a 5-minute token.
func (s *Service) FinishStepUp(sessionID string, in AssertionFinishInput) (StepUpResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return StepUpResult{}, ErrStorageUnavailable
	}
	now := s.now().UTC()
	ch, ok := s.challenges[in.ChallengeID]
	delete(s.challenges, in.ChallengeID)
	if !ok || ch.kind != challengeStepUp || ch.sessionID != sessionID || now.After(ch.expiresAt) {
		return StepUpResult{}, ErrInvalidChallenge
	}
	idx := s.findPasskeyLocked(in.CredentialID, ch.rpID)
	if idx < 0 {
		return StepUpResult{}, ErrInvalidAssertion
	}
	pk := s.doc.Passkeys[idx]
	verified, err := VerifyAssertion(AssertionVerifyInput{
		ExpectedChallenge: ch.challenge,
		ExpectedOrigin:    in.Origin,
		ExpectedRPID:      ch.rpID,
		StoredPublicKey:   pk.PublicKeyCOSE,
		StoredSignCount:   pk.SignCount,
		ClientDataJSON:    in.ClientDataJSON,
		AuthenticatorData: in.AuthenticatorData,
		Signature:         in.Signature,
	})
	if err != nil {
		return StepUpResult{}, err
	}
	next := s.cloneDocLocked()
	next.Passkeys[idx].SignCount = verified.SignCount
	next.Passkeys[idx].BackupEligible = verified.BackupEligible
	next.Passkeys[idx].BackupState = verified.BackupState
	next.Passkeys[idx].LastUsedAt = now.Format(time.RFC3339Nano)
	if err := s.commitLocked(next); err != nil {
		return StepUpResult{}, err
	}
	token := randomToken(24)
	exp := now.Add(stepUpTTL)
	s.stepUps[token] = stepUpEntry{sessionID: sessionID, expiresAt: exp}
	return StepUpResult{StepUpToken: token, ExpiresAt: exp}, nil
}

// VerifyStepUp reports whether token is a valid unexpired step-up proof for sessionID.
func (s *Service) VerifyStepUp(sessionID, token string) bool {
	if s == nil || token == "" || sessionID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.stepUps[token]
	if !ok {
		return false
	}
	if s.now().UTC().After(entry.expiresAt) {
		delete(s.stepUps, token)
		return false
	}
	return entry.sessionID == sessionID
}

func (s *Service) matchPairingCodeLocked(raw string, now time.Time) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	norm := normalizeCodeInput(raw)
	if trimmed == "" || norm == "" {
		return "", false
	}
	for key, entry := range s.pairings {
		if now.After(entry.expiresAt) {
			delete(s.pairings, key)
			continue
		}
		if constantTimeStringEqual(trimmed, key) || constantTimeStringEqual(norm, normalizeCodeInput(entry.display)) {
			return key, true
		}
	}
	return "", false
}
