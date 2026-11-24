package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- minimal CBOR encoder for test authenticators ---

func cborHead(major byte, n uint64) []byte {
	prefix := major << 5
	switch {
	case n < 24:
		return []byte{prefix | byte(n)}
	case n <= 0xff:
		return []byte{prefix | 24, byte(n)}
	case n <= 0xffff:
		out := make([]byte, 3)
		out[0] = prefix | 25
		binary.BigEndian.PutUint16(out[1:], uint16(n))
		return out
	default:
		out := make([]byte, 5)
		out[0] = prefix | 26
		binary.BigEndian.PutUint32(out[1:], uint32(n))
		return out
	}
}

func cborInt(v int64) []byte {
	if v >= 0 {
		return cborHead(0, uint64(v))
	}
	return cborHead(1, uint64(-1-v))
}

func cborBytes(b []byte) []byte {
	return append(cborHead(2, uint64(len(b))), b...)
}

func cborText(s string) []byte {
	return append(cborHead(3, uint64(len(s))), []byte(s)...)
}

type fakeKey struct {
	credID []byte
	alg    int64
	cose   []byte
	sign   func(msg []byte) ([]byte, error)
}

func newFakeES256(t *testing.T) fakeKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	x := make([]byte, 32)
	y := make([]byte, 32)
	priv.X.FillBytes(x)
	priv.Y.FillBytes(y)
	var cose []byte
	cose = append(cose, cborHead(5, 5)...)
	cose = append(cose, cborInt(1)...)
	cose = append(cose, cborInt(2)...) // EC2
	cose = append(cose, cborInt(3)...)
	cose = append(cose, cborInt(AlgES256)...)
	cose = append(cose, cborInt(-1)...)
	cose = append(cose, cborInt(1)...) // P-256
	cose = append(cose, cborInt(-2)...)
	cose = append(cose, cborBytes(x)...)
	cose = append(cose, cborInt(-3)...)
	cose = append(cose, cborBytes(y)...)
	credID := make([]byte, 16)
	_, _ = rand.Read(credID)
	return fakeKey{
		credID: credID,
		alg:    AlgES256,
		cose:   cose,
		sign: func(msg []byte) ([]byte, error) {
			d := sha256.Sum256(msg)
			return ecdsa.SignASN1(rand.Reader, priv, d[:])
		},
	}
}

func newFakeEd25519(t *testing.T) fakeKey {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var cose []byte
	cose = append(cose, cborHead(5, 4)...)
	cose = append(cose, cborInt(1)...)
	cose = append(cose, cborInt(1)...) // OKP
	cose = append(cose, cborInt(3)...)
	cose = append(cose, cborInt(AlgEdDSA)...)
	cose = append(cose, cborInt(-1)...)
	cose = append(cose, cborInt(6)...) // Ed25519
	cose = append(cose, cborInt(-2)...)
	cose = append(cose, cborBytes(pub)...)
	credID := make([]byte, 16)
	_, _ = rand.Read(credID)
	return fakeKey{
		credID: credID,
		alg:    AlgEdDSA,
		cose:   cose,
		sign: func(msg []byte) ([]byte, error) {
			return ed25519.Sign(priv, msg), nil
		},
	}
}

func newFakeRSA(t *testing.T) fakeKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	eBytes := []byte{0x01, 0x00, 0x01} // 65537
	var cose []byte
	cose = append(cose, cborHead(5, 4)...)
	cose = append(cose, cborInt(1)...)
	cose = append(cose, cborInt(3)...) // RSA
	cose = append(cose, cborInt(3)...)
	cose = append(cose, cborInt(AlgRS256)...)
	cose = append(cose, cborInt(-1)...)
	cose = append(cose, cborBytes(priv.N.Bytes())...)
	cose = append(cose, cborInt(-2)...)
	cose = append(cose, cborBytes(eBytes)...)
	credID := make([]byte, 16)
	_, _ = rand.Read(credID)
	return fakeKey{
		credID: credID,
		alg:    AlgRS256,
		cose:   cose,
		sign: func(msg []byte) ([]byte, error) {
			d := sha256.Sum256(msg)
			return rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, d[:])
		},
	}
}

func (k fakeKey) credIDB64() string {
	return base64.RawURLEncoding.EncodeToString(k.credID)
}

func (k fakeKey) makeAttestation(t *testing.T, rpID, origin, challenge string, signCount uint32, synced bool) (clientDataB64, attObjB64 string) {
	t.Helper()
	cdJSON, _ := json.Marshal(map[string]any{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    origin,
	})
	rpHash := sha256.Sum256([]byte(rpID))
	flags := flagUserPresent | flagUserVerified | flagAttestedData
	if synced {
		flags |= flagBackupEligible | flagBackupState
	}
	var authData []byte
	authData = append(authData, rpHash[:]...)
	authData = append(authData, flags)
	var cnt [4]byte
	binary.BigEndian.PutUint32(cnt[:], signCount)
	authData = append(authData, cnt[:]...)
	authData = append(authData, make([]byte, 16)...) // AAGUID
	var lenBuf [2]byte
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(k.credID)))
	authData = append(authData, lenBuf[:]...)
	authData = append(authData, k.credID...)
	authData = append(authData, k.cose...)

	var attObj []byte
	attObj = append(attObj, cborHead(5, 3)...)
	attObj = append(attObj, cborText("fmt")...)
	attObj = append(attObj, cborText("none")...)
	attObj = append(attObj, cborText("attStmt")...)
	attObj = append(attObj, cborHead(5, 0)...)
	attObj = append(attObj, cborText("authData")...)
	attObj = append(attObj, cborBytes(authData)...)

	return base64.RawURLEncoding.EncodeToString(cdJSON), base64.RawURLEncoding.EncodeToString(attObj)
}

func (k fakeKey) makeAssertion(t *testing.T, rpID, origin, challenge string, signCount uint32) (cdB64, authDataB64, sigB64 string) {
	t.Helper()
	cdJSON, _ := json.Marshal(map[string]any{
		"type":      "webauthn.get",
		"challenge": challenge,
		"origin":    origin,
	})
	rpHash := sha256.Sum256([]byte(rpID))
	var authData []byte
	authData = append(authData, rpHash[:]...)
	authData = append(authData, flagUserPresent|flagUserVerified)
	var cnt [4]byte
	binary.BigEndian.PutUint32(cnt[:], signCount)
	authData = append(authData, cnt[:]...)

	cdHash := sha256.Sum256(cdJSON)
	signed := append(append([]byte{}, authData...), cdHash[:]...)
	sig, err := k.sign(signed)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(cdJSON),
		base64.RawURLEncoding.EncodeToString(authData),
		base64.RawURLEncoding.EncodeToString(sig)
}

func TestValidateRPID(t *testing.T) {
	valid := []struct{ in, want string }{
		{"localhost:7681", "localhost"},
		{"LOCALHOST", "localhost"},
		{"mybox.tail123.ts.net", "mybox.tail123.ts.net"},
		{"term.example.com:8443", "term.example.com"},
	}
	for _, tc := range valid {
		got, err := ValidateRPID(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("ValidateRPID(%q) = (%q, %v), want (%q, nil)", tc.in, got, err, tc.want)
		}
	}
	invalid := []string{"", "127.0.0.1:7681", "[::1]:7681", "192.168.1.50:7681", "bad host", "-bad.example"}
	for _, in := range invalid {
		if got, err := ValidateRPID(in); err == nil {
			t.Fatalf("ValidateRPID(%q) unexpectedly succeeded with %q", in, got)
		}
	}
}

func TestOnboardingLoginPairStepUpRecoveryAndPAT(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 24, 1, 0, 0, 0, time.UTC)
	svc := Open(dir, Options{RPDisplayName: "WebMux Test", UserName: "alice@dev", Now: func() time.Time { return now }})
	defer svc.Close()

	// Fresh install enters needs_onboarding.
	st := svc.Status("localhost:7681")
	if st.State != StateNeedsOnboarding || !st.RPIDValid || st.RPID != "localhost" || st.HasPasskeyForRPID {
		t.Fatalf("unexpected initial status: %+v", st)
	}
	// IP literal reports RPIDValid=false.
	ipSt := svc.Status("127.0.0.1:7681")
	if ipSt.RPIDValid {
		t.Fatalf("expected 127.0.0.1 to have RPIDValid=false")
	}

	grant, err := svc.SetupInfo()
	if err != nil || grant.Code == "" || grant.DisplayCode == "" {
		t.Fatalf("SetupInfo failed: %+v, %v", grant, err)
	}
	if _, err := svc.VerifySetupCode("WRONG-CODE"); !errors.Is(err, ErrInvalidSetupCode) {
		t.Fatalf("expected ErrInvalidSetupCode, got %v", err)
	}
	// Verify via hyphenated display code.
	ticket, err := svc.VerifySetupCode(grant.DisplayCode)
	if err != nil {
		t.Fatalf("VerifySetupCode failed: %v", err)
	}

	// Register first passkey (ES256, synced 0->0 counter).
	esKey := newFakeES256(t)
	regOpts, err := svc.BeginOnboardingPasskey(ticket.Ticket, "localhost:7681")
	if err != nil {
		t.Fatalf("BeginOnboardingPasskey failed: %v", err)
	}
	cd, att := esKey.makeAttestation(t, "localhost", "http://localhost:7681", regOpts.Challenge, 0, true)
	res, err := svc.FinishOnboardingPasskey(ticket.Ticket, PasskeyFinishInput{
		ChallengeID:       regOpts.ChallengeID,
		Name:              "MacBook Chrome",
		Origin:            "http://localhost:7681",
		ClientDataJSON:    cd,
		AttestationObject: att,
		Transports:        []string{"internal", "hybrid"},
		DeviceName:        "Chrome on macOS",
	})
	if err != nil {
		t.Fatalf("FinishOnboardingPasskey failed: %v", err)
	}
	if res.SessionToken == "" || len(res.RecoveryCodes) != recoveryCodeCount || !res.Passkey.BackupState {
		t.Fatalf("unexpected onboarding result: %+v", res)
	}

	// Setup code must now be invalid / already initialized.
	if _, err := svc.VerifySetupCode(grant.Code); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("expected ErrAlreadyInitialized after onboarding, got %v", err)
	}

	// Complete onboarding.
	if err := svc.CompleteOnboarding(); err != nil {
		t.Fatalf("CompleteOnboarding failed: %v", err)
	}
	st = svc.Status("localhost:7681")
	if st.State != StateReady || !st.OnboardingCompleted || !st.HasPasskeyForRPID || st.PasskeyCount != 1 {
		t.Fatalf("unexpected ready status: %+v", st)
	}

	// Bearer token authenticates as session principal.
	p, ok := svc.AuthenticateBearer(res.SessionToken)
	if !ok || p.Kind != PrincipalSession || p.SessionID != res.Session.ID {
		t.Fatalf("AuthenticateBearer failed: %+v, %v", p, ok)
	}

	// Step-up verification.
	suOpts, err := svc.BeginStepUp(p.SessionID, "localhost:7681")
	if err != nil {
		t.Fatalf("BeginStepUp failed: %v", err)
	}
	cd, ad, sig := esKey.makeAssertion(t, "localhost", "http://localhost:7681", suOpts.Challenge, 0)
	suRes, err := svc.FinishStepUp(p.SessionID, AssertionFinishInput{
		ChallengeID:       suOpts.ChallengeID,
		CredentialID:      esKey.credIDB64(),
		Origin:            "http://localhost:7681",
		ClientDataJSON:    cd,
		AuthenticatorData: ad,
		Signature:         sig,
	})
	if err != nil {
		t.Fatalf("FinishStepUp failed: %v", err)
	}
	if !svc.VerifyStepUp(p.SessionID, suRes.StepUpToken) {
		t.Fatalf("VerifyStepUp returned false")
	}

	// Cross-RP-ID pairing for https://mybox.ts.net with Ed25519 key (non-zero signCount).
	pairGrant, err := svc.CreatePairingCode("https://mybox.ts.net")
	if err != nil {
		t.Fatalf("CreatePairingCode failed: %v", err)
	}
	edKey := newFakeEd25519(t)
	pairOpts, err := svc.BeginPair(pairGrant.DisplayCode, "mybox.ts.net")
	if err != nil {
		t.Fatalf("BeginPair failed: %v", err)
	}
	cd, att = edKey.makeAttestation(t, "mybox.ts.net", "https://mybox.ts.net", pairOpts.Challenge, 1, false)
	pairLogin, pairPK, err := svc.FinishPair(PasskeyFinishInput{
		ChallengeID:       pairOpts.ChallengeID,
		Name:              "iPhone Safari",
		Origin:            "https://mybox.ts.net",
		ClientDataJSON:    cd,
		AttestationObject: att,
		DeviceName:        "Safari on iOS",
	})
	if err != nil {
		t.Fatalf("FinishPair failed: %v", err)
	}
	if pairPK.RPID != "mybox.ts.net" || pairLogin.SessionToken == "" {
		t.Fatalf("unexpected pair result: %+v %+v", pairPK, pairLogin)
	}
	// Pairing code must be single-use.
	if _, err := svc.BeginPair(pairGrant.Code, "mybox.ts.net"); !errors.Is(err, ErrInvalidPairingCode) {
		t.Fatalf("expected ErrInvalidPairingCode on reuse, got %v", err)
	}

	// Passkey login on mybox.ts.net with incremented signCount succeeds.
	loginOpts, err := svc.BeginLogin("mybox.ts.net")
	if err != nil {
		t.Fatalf("BeginLogin failed: %v", err)
	}
	cd, ad, sig = edKey.makeAssertion(t, "mybox.ts.net", "https://mybox.ts.net", loginOpts.Challenge, 2)
	if _, err := svc.FinishLogin(AssertionFinishInput{
		ChallengeID:       loginOpts.ChallengeID,
		CredentialID:      edKey.credIDB64(),
		Origin:            "https://mybox.ts.net",
		ClientDataJSON:    cd,
		AuthenticatorData: ad,
		Signature:         sig,
		DeviceName:        "Safari on iOS",
	}); err != nil {
		t.Fatalf("FinishLogin failed: %v", err)
	}

	// SignCount rollback (2 -> 2) must be rejected as clone anomaly.
	loginOpts, _ = svc.BeginLogin("mybox.ts.net")
	cd, ad, sig = edKey.makeAssertion(t, "mybox.ts.net", "https://mybox.ts.net", loginOpts.Challenge, 2)
	if _, err := svc.FinishLogin(AssertionFinishInput{
		ChallengeID:       loginOpts.ChallengeID,
		CredentialID:      edKey.credIDB64(),
		Origin:            "https://mybox.ts.net",
		ClientDataJSON:    cd,
		AuthenticatorData: ad,
		Signature:         sig,
	}); !errors.Is(err, ErrSignCountAnomaly) {
		t.Fatalf("expected ErrSignCountAnomaly, got %v", err)
	}

	// Add RS256 passkey on localhost.
	rsaKey := newFakeRSA(t)
	addOpts, err := svc.BeginAddPasskey(p.SessionID, "localhost:7681")
	if err != nil {
		t.Fatalf("BeginAddPasskey failed: %v", err)
	}
	cd, att = rsaKey.makeAttestation(t, "localhost", "http://localhost:7681", addOpts.Challenge, 0, false)
	addedPK, err := svc.FinishAddPasskey(p.SessionID, PasskeyFinishInput{
		ChallengeID:       addOpts.ChallengeID,
		Name:              "YubiKey RSA",
		Origin:            "http://localhost:7681",
		ClientDataJSON:    cd,
		AttestationObject: att,
	})
	if err != nil {
		t.Fatalf("FinishAddPasskey RS256 failed: %v", err)
	}
	// Login with RS256 key.
	loginOpts, _ = svc.BeginLogin("localhost:7681")
	cd, ad, sig = rsaKey.makeAssertion(t, "localhost", "http://localhost:7681", loginOpts.Challenge, 0)
	if _, err := svc.FinishLogin(AssertionFinishInput{
		ChallengeID:       loginOpts.ChallengeID,
		CredentialID:      rsaKey.credIDB64(),
		Origin:            "http://localhost:7681",
		ClientDataJSON:    cd,
		AuthenticatorData: ad,
		Signature:         sig,
	}); err != nil {
		t.Fatalf("RS256 FinishLogin failed: %v", err)
	}

	// Recovery code login consumes the code once.
	recRes, err := svc.LoginWithRecoveryCode(res.RecoveryCodes[0], "Recovery Browser", "localhost:7681")
	if err != nil || recRes.RecoveryCodesRemaining != recoveryCodeCount-1 {
		t.Fatalf("LoginWithRecoveryCode failed: %+v, %v", recRes, err)
	}
	if _, err := svc.LoginWithRecoveryCode(res.RecoveryCodes[0], "Replay", "localhost:7681"); !errors.Is(err, ErrInvalidRecovery) {
		t.Fatalf("expected ErrInvalidRecovery on replay, got %v", err)
	}

	// Named API Token (PAT) create, authenticate, revoke.
	pat, err := svc.CreateAPIToken("ci-script", 7)
	if err != nil || pat.Token == "" {
		t.Fatalf("CreateAPIToken failed: %+v, %v", pat, err)
	}
	patPrincipal, ok := svc.AuthenticateBearer(pat.Token)
	if !ok || patPrincipal.Kind != PrincipalAPIToken || patPrincipal.TokenID != pat.Info.ID {
		t.Fatalf("PAT auth failed: %+v, %v", patPrincipal, ok)
	}
	if err := svc.RevokeAPIToken(pat.Info.ID); err != nil {
		t.Fatalf("RevokeAPIToken failed: %v", err)
	}
	if _, ok := svc.AuthenticateBearer(pat.Token); ok {
		t.Fatalf("revoked PAT still authenticated")
	}

	// Revoke other sessions invalidates phone session but keeps desktop session.
	removed, err := svc.RevokeOtherSessions(p.SessionID)
	if err != nil || removed < 2 {
		t.Fatalf("RevokeOtherSessions = (%d, %v)", removed, err)
	}
	if _, ok := svc.AuthenticateBearer(pairLogin.SessionToken); ok {
		t.Fatalf("revoked phone session still authenticated")
	}
	if _, ok := svc.AuthenticateBearer(res.SessionToken); !ok {
		t.Fatalf("current session unexpectedly revoked")
	}

	// Rename and delete passkey.
	if _, err := svc.RenamePasskey(addedPK.ID, "Office YubiKey"); err != nil {
		t.Fatalf("RenamePasskey failed: %v", err)
	}
	if err := svc.DeletePasskey(addedPK.ID, false); err != nil {
		t.Fatalf("DeletePasskey failed: %v", err)
	}

	// Reopen from disk and verify persistence survived restart.
	svc.Close()
	reopened := Open(dir, Options{Now: func() time.Time { return now }})
	defer reopened.Close()
	if reopened.State() != StateReady {
		t.Fatalf("reopened state = %q, want ready", reopened.State())
	}
	if _, ok := reopened.AuthenticateBearer(res.SessionToken); !ok {
		t.Fatalf("session did not persist across restart")
	}

	// Reset clears everything and returns to needs_onboarding.
	resetGrant, err := reopened.Reset()
	if err != nil || resetGrant.Code == "" || reopened.State() != StateNeedsOnboarding {
		t.Fatalf("Reset failed: %+v, %v, state=%q", resetGrant, err, reopened.State())
	}
	if _, ok := reopened.AuthenticateBearer(res.SessionToken); ok {
		t.Fatalf("old session survived Reset")
	}
}

func TestFailClosedOnCorruptAuthStore(t *testing.T) {
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(authPath, []byte("{corrupt json"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := Open(dir, Options{})
	defer svc.Close()

	if svc.State() != StateStorageError || svc.Ready() {
		t.Fatalf("expected fail-closed storage_error, got state=%q ready=%v", svc.State(), svc.Ready())
	}
	if _, err := svc.SetupInfo(); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("corrupt store must not allow onboarding setup, got %v", err)
	}
	if _, ok := svc.AuthenticateBearer("anything"); ok {
		t.Fatalf("corrupt store must reject all bearer tokens")
	}
	// Original corrupt file must still be preserved on disk.
	raw, err := os.ReadFile(authPath)
	if err != nil || string(raw) != "{corrupt json" {
		t.Fatalf("corrupt file was modified or removed: %q, %v", string(raw), err)
	}
	svc.Close()

	// Offline reset removes corrupt store so instance can re-onboard cleanly.
	if err := ResetOffline(dir); err != nil {
		t.Fatalf("ResetOffline failed: %v", err)
	}
	insp, err := Inspect(dir)
	if err != nil || insp.State != StateNeedsOnboarding {
		t.Fatalf("Inspect after ResetOffline = (%+v, %v)", insp, err)
	}
}
