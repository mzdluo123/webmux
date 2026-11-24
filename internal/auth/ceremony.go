package auth

import (
	"fmt"
	"log/slog"
	"time"
)

var supportedPubKeyAlgs = []int64{AlgES256, AlgEdDSA, AlgRS256}

// BeginOnboardingPasskey starts WebAuthn creation for the first Passkey.
func (s *Service) BeginOnboardingPasskey(ticket, host string) (RegistrationOptions, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return RegistrationOptions{}, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) > 0 {
		return RegistrationOptions{}, ErrAlreadyInitialized
	}
	now := s.now().UTC()
	t, ok := s.tickets[ticket]
	if !ok || now.After(t.expiresAt) {
		delete(s.tickets, ticket)
		return RegistrationOptions{}, ErrInvalidTicket
	}
	rpID, err := ValidateRPID(host)
	if err != nil {
		return RegistrationOptions{}, fmt.Errorf("%w: %s", ErrInvalidRPID, err.Error())
	}
	s.prunePendingLocked(now)
	userHandle := s.doc.UserHandle
	if userHandle == "" {
		userHandle = randomToken(32)
	}
	id := randomToken(16)
	challenge := randomToken(32)
	s.challenges[id] = challengeEntry{
		kind:       challengeOnboarding,
		challenge:  challenge,
		rpID:       rpID,
		userHandle: userHandle,
		expiresAt:  now.Add(challengeTTL),
	}
	return RegistrationOptions{
		ChallengeID: id,
		Challenge:   challenge,
		RPID:        rpID,
		RPName:      s.opts.RPDisplayName,
		UserID:      userHandle,
		UserName:    s.opts.UserName,
		DisplayName: "WebMux Owner",
		ExcludeIDs:  []string{},
		PubKeyAlgs:  append([]int64{}, supportedPubKeyAlgs...),
		TimeoutMS:   int(challengeTTL.Milliseconds()),
	}, nil
}

// FinishOnboardingPasskey verifies the first Passkey, commits auth.json, clears
// all setup grants/tickets, and returns a session token + recovery codes.
func (s *Service) FinishOnboardingPasskey(ticket string, in PasskeyFinishInput) (OnboardingCompleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return OnboardingCompleteResult{}, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) > 0 {
		return OnboardingCompleteResult{}, ErrAlreadyInitialized
	}
	now := s.now().UTC()
	t, ok := s.tickets[ticket]
	if !ok || now.After(t.expiresAt) {
		delete(s.tickets, ticket)
		return OnboardingCompleteResult{}, ErrInvalidTicket
	}
	ch, ok := s.challenges[in.ChallengeID]
	delete(s.challenges, in.ChallengeID)
	if !ok || ch.kind != challengeOnboarding || now.After(ch.expiresAt) {
		return OnboardingCompleteResult{}, ErrInvalidChallenge
	}
	verified, err := VerifyRegistration(RegistrationVerifyInput{
		ExpectedChallenge: ch.challenge,
		ExpectedOrigin:    in.Origin,
		ExpectedRPID:      ch.rpID,
		ClientDataJSON:    in.ClientDataJSON,
		AttestationObject: in.AttestationObject,
	})
	if err != nil {
		return OnboardingCompleteResult{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	next := s.cloneDocLocked()
	next.V = storeVersion
	next.UserHandle = ch.userHandle
	passkey := PasskeyRecord{
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
	next.Passkeys = append(next.Passkeys, passkey)
	plainCodes, recCodes := generateRecoveryBatch()
	next.RecoveryCodes = recCodes
	sessionToken, sess := s.appendSessionLocked(&next, in.DeviceName, ch.rpID, now)
	if err := s.commitLocked(next); err != nil {
		return OnboardingCompleteResult{}, err
	}
	// Ownership established: destroy all setup codes and onboarding tickets.
	s.setup = nil
	s.tickets = map[string]ticketEntry{}
	return OnboardingCompleteResult{
		SessionToken:  sessionToken,
		Session:       toSessionInfo(sess, sess.ID),
		Passkey:       toPasskeyInfo(passkey),
		RecoveryCodes: plainCodes,
	}, nil
}

// BeginLogin creates a WebAuthn assertion challenge for credentials registered
// on the request's RP ID.
func (s *Service) BeginLogin(host string) (AssertionOptions, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return AssertionOptions{}, ErrStorageUnavailable
	}
	if len(s.doc.Passkeys) == 0 {
		return AssertionOptions{}, ErrNotInitialized
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
		kind:      challengeLogin,
		challenge: challenge,
		rpID:      rpID,
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

// FinishLogin verifies a Passkey assertion and issues a new per-device session.
func (s *Service) FinishLogin(in AssertionFinishInput) (LoginResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return LoginResult{}, ErrStorageUnavailable
	}
	now := s.now().UTC()
	ch, ok := s.challenges[in.ChallengeID]
	delete(s.challenges, in.ChallengeID)
	if !ok || ch.kind != challengeLogin || now.After(ch.expiresAt) {
		return LoginResult{}, ErrInvalidChallenge
	}
	idx := s.findPasskeyLocked(in.CredentialID, ch.rpID)
	if idx < 0 {
		return LoginResult{}, ErrInvalidAssertion
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
		if err == ErrSignCountAnomaly {
			slog.Warn("passkey signCount anomaly", "passkeyId", pk.ID, "rpId", pk.RPID, "stored", pk.SignCount)
		}
		return LoginResult{}, err
	}
	next := s.cloneDocLocked()
	next.Passkeys[idx].SignCount = verified.SignCount
	next.Passkeys[idx].BackupEligible = verified.BackupEligible
	next.Passkeys[idx].BackupState = verified.BackupState
	next.Passkeys[idx].LastUsedAt = now.Format(time.RFC3339Nano)
	sessionToken, sess := s.appendSessionLocked(&next, in.DeviceName, ch.rpID, now)
	if err := s.commitLocked(next); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		SessionToken:           sessionToken,
		Session:                toSessionInfo(sess, sess.ID),
		RecoveryCodesRemaining: s.remainingRecoveryLocked(),
	}, nil
}

// BeginAddPasskey starts registering an additional Passkey for the current RP ID.
func (s *Service) BeginAddPasskey(sessionID, host string) (RegistrationOptions, error) {
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
	rpID, err := ValidateRPID(host)
	if err != nil {
		return RegistrationOptions{}, fmt.Errorf("%w: %s", ErrInvalidRPID, err.Error())
	}
	now := s.now().UTC()
	s.prunePendingLocked(now)
	id := randomToken(16)
	challenge := randomToken(32)
	s.challenges[id] = challengeEntry{
		kind:       challengeAddPasskey,
		challenge:  challenge,
		rpID:       rpID,
		userHandle: s.doc.UserHandle,
		sessionID:  sessionID,
		expiresAt:  now.Add(challengeTTL),
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

// FinishAddPasskey completes adding a new Passkey to the current instance.
func (s *Service) FinishAddPasskey(sessionID string, in PasskeyFinishInput) (PasskeyInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageFailedLocked() {
		return PasskeyInfo{}, ErrStorageUnavailable
	}
	now := s.now().UTC()
	ch, ok := s.challenges[in.ChallengeID]
	delete(s.challenges, in.ChallengeID)
	if !ok || ch.kind != challengeAddPasskey || ch.sessionID != sessionID || now.After(ch.expiresAt) {
		return PasskeyInfo{}, ErrInvalidChallenge
	}
	if len(s.doc.Passkeys) >= maxPasskeys {
		return PasskeyInfo{}, ErrLimitReached
	}
	verified, err := VerifyRegistration(RegistrationVerifyInput{
		ExpectedChallenge: ch.challenge,
		ExpectedOrigin:    in.Origin,
		ExpectedRPID:      ch.rpID,
		ClientDataJSON:    in.ClientDataJSON,
		AttestationObject: in.AttestationObject,
	})
	if err != nil {
		return PasskeyInfo{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	if s.hasCredentialIDLocked(verified.CredentialID) {
		return PasskeyInfo{}, fmt.Errorf("%w: credential already registered", ErrInvalidInput)
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
	}
	next.Passkeys = append(next.Passkeys, pk)
	if err := s.commitLocked(next); err != nil {
		return PasskeyInfo{}, err
	}
	return toPasskeyInfo(pk), nil
}

func (s *Service) credentialIDsForRPIDLocked(rpID string) []string {
	ids := make([]string, 0, len(s.doc.Passkeys))
	for _, p := range s.doc.Passkeys {
		if p.RPID == rpID {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

func (s *Service) findPasskeyLocked(credID, rpID string) int {
	for i, p := range s.doc.Passkeys {
		if p.ID == credID && p.RPID == rpID {
			return i
		}
	}
	return -1
}

func (s *Service) hasCredentialIDLocked(credID string) bool {
	for _, p := range s.doc.Passkeys {
		if p.ID == credID {
			return true
		}
	}
	return false
}

func defaultPasskeyName(rpID string) string {
	return "Passkey (" + rpID + ")"
}

func sanitizeTransports(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, t := range raw {
		switch t {
		case "internal", "hybrid", "usb", "nfc", "ble", "smart-card":
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}
