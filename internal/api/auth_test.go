package api_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/auth"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
)

func testCBORHead(major byte, n uint64) []byte {
	prefix := major << 5
	switch {
	case n < 24:
		return []byte{prefix | byte(n)}
	case n <= 0xff:
		return []byte{prefix | 24, byte(n)}
	default:
		out := make([]byte, 3)
		out[0] = prefix | 25
		binary.BigEndian.PutUint16(out[1:], uint16(n))
		return out
	}
}

func testCBORInt(v int64) []byte {
	if v >= 0 {
		return testCBORHead(0, uint64(v))
	}
	return testCBORHead(1, uint64(-1-v))
}

func testCBORBytes(b []byte) []byte { return append(testCBORHead(2, uint64(len(b))), b...) }
func testCBORText(s string) []byte  { return append(testCBORHead(3, uint64(len(s))), []byte(s)...) }

type apiTestKey struct {
	credID []byte
	priv   *ecdsa.PrivateKey
	cose   []byte
}

func newAPITestKey(t *testing.T) apiTestKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	x, y := make([]byte, 32), make([]byte, 32)
	priv.X.FillBytes(x)
	priv.Y.FillBytes(y)
	var cose []byte
	cose = append(cose, testCBORHead(5, 5)...)
	cose = append(cose, testCBORInt(1)...)
	cose = append(cose, testCBORInt(2)...)
	cose = append(cose, testCBORInt(3)...)
	cose = append(cose, testCBORInt(auth.AlgES256)...)
	cose = append(cose, testCBORInt(-1)...)
	cose = append(cose, testCBORInt(1)...)
	cose = append(cose, testCBORInt(-2)...)
	cose = append(cose, testCBORBytes(x)...)
	cose = append(cose, testCBORInt(-3)...)
	cose = append(cose, testCBORBytes(y)...)
	credID := make([]byte, 16)
	_, _ = rand.Read(credID)
	return apiTestKey{credID: credID, priv: priv, cose: cose}
}

func (k apiTestKey) attest(rpID, origin, challenge string) (string, string) {
	cd, _ := json.Marshal(map[string]any{"type": "webauthn.create", "challenge": challenge, "origin": origin})
	rpHash := sha256.Sum256([]byte(rpID))
	var ad []byte
	ad = append(ad, rpHash[:]...)
	ad = append(ad, 0x45) // UP | UV | AT
	ad = append(ad, 0, 0, 0, 0)
	ad = append(ad, make([]byte, 16)...)
	var lb [2]byte
	binary.BigEndian.PutUint16(lb[:], uint16(len(k.credID)))
	ad = append(ad, lb[:]...)
	ad = append(ad, k.credID...)
	ad = append(ad, k.cose...)
	var att []byte
	att = append(att, testCBORHead(5, 3)...)
	att = append(att, testCBORText("fmt")...)
	att = append(att, testCBORText("none")...)
	att = append(att, testCBORText("attStmt")...)
	att = append(att, testCBORHead(5, 0)...)
	att = append(att, testCBORText("authData")...)
	att = append(att, testCBORBytes(ad)...)
	return base64.RawURLEncoding.EncodeToString(cd), base64.RawURLEncoding.EncodeToString(att)
}

func (k apiTestKey) assert(t *testing.T, rpID, origin, challenge string) (string, string, string) {
	t.Helper()
	cd, _ := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": challenge, "origin": origin})
	rpHash := sha256.Sum256([]byte(rpID))
	var ad []byte
	ad = append(ad, rpHash[:]...)
	ad = append(ad, 0x05) // UP | UV
	ad = append(ad, 0, 0, 0, 0)
	cdHash := sha256.Sum256(cd)
	signed := append(append([]byte{}, ad...), cdHash[:]...)
	digest := sha256.Sum256(signed)
	sig, err := ecdsa.SignASN1(rand.Reader, k.priv, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(cd),
		base64.RawURLEncoding.EncodeToString(ad),
		base64.RawURLEncoding.EncodeToString(sig)
}

func doAuthJSON(t *testing.T, h http.Handler, method, path, bearer, stepUp string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, "http://localhost:7681"+path, &buf)
	req.Host = "localhost:7681"
	req.Header.Set("Origin", "http://localhost:7681")
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if stepUp != "" {
		req.Header.Set("X-WebMux-Step-Up", stepUp)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPasskeyOnboardingAndStepUpAPI(t *testing.T) {
	dir := t.TempDir()
	authSvc := auth.Open(dir, auth.Options{RPDisplayName: "WebMux", UserName: "alice@host"})
	t.Cleanup(authSvc.Close)
	m := session.NewManager(session.Options{})
	h := api.New(api.Deps{
		Config:  config.Config{Listen: "127.0.0.1:7681"},
		Auth:    authSvc,
		Manager: m,
	})

	// Unauthenticated /api/health returns 401 while needs_onboarding.
	if rec := doAuthJSON(t, h, http.MethodGet, "/api/health", "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/health before onboarding = %d, want 401", rec.Code)
	}
	// Public /api/auth/status reports needs_onboarding.
	rec := doAuthJSON(t, h, http.MethodGet, "/api/auth/status", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/auth/status = %d", rec.Code)
	}
	var st contract.AuthStatusResponse
	_ = json.NewDecoder(rec.Body).Decode(&st)
	if st.State != contract.AuthNeedsOnboarding || !st.RPIDValid || st.RPID != "localhost" {
		t.Fatalf("unexpected auth status: %+v", st)
	}

	// Verify setup code -> ticket -> register first passkey.
	grant, err := authSvc.SetupInfo()
	if err != nil {
		t.Fatal(err)
	}
	rec = doAuthJSON(t, h, http.MethodPost, "/api/auth/onboarding/verify", "", "", contract.OnboardingVerifyRequest{Code: grant.DisplayCode})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify setup code = %d: %s", rec.Code, rec.Body.String())
	}
	var verified contract.OnboardingVerifyResponse
	_ = json.NewDecoder(rec.Body).Decode(&verified)

	rec = doAuthJSON(t, h, http.MethodPost, "/api/auth/onboarding/passkey/begin", "", "", contract.OnboardingBeginRequest{Ticket: verified.Ticket})
	if rec.Code != http.StatusOK {
		t.Fatalf("onboarding begin = %d: %s", rec.Code, rec.Body.String())
	}
	var creation contract.WebAuthnCreationOptions
	_ = json.NewDecoder(rec.Body).Decode(&creation)

	key := newAPITestKey(t)
	cd, att := key.attest("localhost", "http://localhost:7681", creation.Challenge)
	rec = doAuthJSON(t, h, http.MethodPost, "/api/auth/onboarding/passkey/finish", "", "", contract.OnboardingFinishRequest{
		Ticket:            verified.Ticket,
		ChallengeID:       creation.ChallengeID,
		Name:              "Test Browser",
		ClientDataJSON:    cd,
		AttestationObject: att,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("onboarding finish = %d: %s", rec.Code, rec.Body.String())
	}
	var finished contract.OnboardingFinishResponse
	_ = json.NewDecoder(rec.Body).Decode(&finished)
	if finished.SessionToken == "" || len(finished.RecoveryCodes) == 0 {
		t.Fatalf("unexpected finish response: %+v", finished)
	}

	// Authenticated /api/health now returns 200.
	if rec := doAuthJSON(t, h, http.MethodGet, "/api/health", finished.SessionToken, "", nil); rec.Code != http.StatusOK {
		t.Fatalf("GET /api/health after onboarding = %d", rec.Code)
	}

	// Pair creation without Step-Up must return 403 step_up_required.
	rec = doAuthJSON(t, h, http.MethodPost, "/api/auth/pair/create", finished.SessionToken, "", contract.PairCreateRequest{})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("pair/create without step-up = %d, want 403", rec.Code)
	}

	// Complete Step-Up and create a pairing code + API token.
	rec = doAuthJSON(t, h, http.MethodPost, "/api/auth/step-up/begin", finished.SessionToken, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("step-up begin = %d", rec.Code)
	}
	var suOpts contract.WebAuthnAssertionOptions
	_ = json.NewDecoder(rec.Body).Decode(&suOpts)
	cd, ad, sig := key.assert(t, "localhost", "http://localhost:7681", suOpts.Challenge)
	rec = doAuthJSON(t, h, http.MethodPost, "/api/auth/step-up/finish", finished.SessionToken, "", contract.AssertionFinishRequest{
		ChallengeID:       suOpts.ChallengeID,
		CredentialID:      base64.RawURLEncoding.EncodeToString(key.credID),
		ClientDataJSON:    cd,
		AuthenticatorData: ad,
		Signature:         sig,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("step-up finish = %d: %s", rec.Code, rec.Body.String())
	}
	var suRes contract.StepUpFinishResponse
	_ = json.NewDecoder(rec.Body).Decode(&suRes)

	rec = doAuthJSON(t, h, http.MethodPost, "/api/auth/pair/create", finished.SessionToken, suRes.StepUpToken, contract.PairCreateRequest{TargetURL: "https://mybox.ts.net"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("pair/create with step-up = %d: %s", rec.Code, rec.Body.String())
	}

	// Logout revokes the session immediately.
	if rec := doAuthJSON(t, h, http.MethodPost, "/api/auth/logout", finished.SessionToken, "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d", rec.Code)
	}
	if rec := doAuthJSON(t, h, http.MethodGet, "/api/health", finished.SessionToken, "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/health after logout = %d, want 401", rec.Code)
	}
}
