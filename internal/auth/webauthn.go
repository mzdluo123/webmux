package auth

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"strings"
)

// COSE algorithm identifiers supported for WebAuthn Passkeys.
const (
	AlgES256 int64 = -7   // ECDSA w/ SHA-256 (P-256)
	AlgEdDSA int64 = -8   // EdDSA (Ed25519)
	AlgRS256 int64 = -257 // RSASSA-PKCS1-v1_5 w/ SHA-256
)

const (
	flagUserPresent    byte = 1 << 0
	flagUserVerified   byte = 1 << 2
	flagBackupEligible byte = 1 << 3
	flagBackupState    byte = 1 << 4
	flagAttestedData   byte = 1 << 6
)

// ValidateRPID extracts and validates a WebAuthn Relying Party ID from a host
// header or hostname. Browsers prohibit IP literals (including 127.0.0.1 and
// ::1) as RP IDs; localhost and valid non-IP DNS hostnames are accepted.
func ValidateRPID(authority string) (string, error) {
	raw := strings.TrimSpace(authority)
	if raw == "" {
		return "", errors.New("missing host for WebAuthn RP ID")
	}
	u, err := url.Parse("http://" + raw)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid host %q for WebAuthn RP ID", authority)
	}
	host := strings.TrimRight(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return "", fmt.Errorf("invalid hostname %q for WebAuthn RP ID", authority)
	}
	if net.ParseIP(host) != nil {
		return "", fmt.Errorf("WebAuthn Passkeys require localhost or a domain name, not IP address %q", host)
	}
	if host == "localhost" {
		return host, nil
	}
	if len(host) > 253 {
		return "", errors.New("WebAuthn RP ID hostname is too long")
	}
	for label := range strings.SplitSeq(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid DNS label in WebAuthn RP ID %q", host)
		}
		for _, ch := range label {
			if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
				return "", fmt.Errorf("invalid character in WebAuthn RP ID %q", host)
			}
		}
	}
	return host, nil
}

type clientData struct {
	Type        string `json:"type"`
	Challenge   string `json:"challenge"`
	Origin      string `json:"origin"`
	CrossOrigin bool   `json:"crossOrigin"`
}

// RegistrationVerifyInput holds the raw browser attestation response fields.
type RegistrationVerifyInput struct {
	ExpectedChallenge string
	ExpectedOrigin    string
	ExpectedRPID      string
	ClientDataJSON    string // base64url
	AttestationObject string // base64url
}

// VerifiedCredential is the extracted credential metadata after verification.
type VerifiedCredential struct {
	CredentialID   string // base64url
	PublicKeyCOSE  string // base64url raw COSE_Key bytes
	Alg            int64
	SignCount      uint32
	BackupEligible bool
	BackupState    bool
}

// VerifyRegistration validates a WebAuthn registration response (attestation=none).
func VerifyRegistration(in RegistrationVerifyInput) (VerifiedCredential, error) {
	if _, err := verifyClientData(in.ClientDataJSON, "webauthn.create", in.ExpectedChallenge, in.ExpectedOrigin); err != nil {
		return VerifiedCredential{}, err
	}
	attBytes, err := decodeBase64URL(in.AttestationObject)
	if err != nil {
		return VerifiedCredential{}, fmt.Errorf("invalid attestationObject encoding: %w", err)
	}
	val, consumed, err := decodeCBOR(attBytes)
	if err != nil {
		return VerifiedCredential{}, fmt.Errorf("invalid attestationObject CBOR: %w", err)
	}
	if consumed != len(attBytes) {
		return VerifiedCredential{}, errors.New("trailing bytes in attestationObject")
	}
	attMap, ok := val.(cborMap)
	if !ok {
		return VerifiedCredential{}, errors.New("attestationObject must be a CBOR map")
	}
	rawAuthData, ok := attMap.stringKey("authData")
	if !ok {
		return VerifiedCredential{}, errors.New("attestationObject missing authData")
	}
	authData, ok := rawAuthData.([]byte)
	if !ok {
		return VerifiedCredential{}, errors.New("attestationObject authData must be bytes")
	}
	return parseAttestedAuthData(authData, in.ExpectedRPID)
}

func parseAttestedAuthData(authData []byte, expectedRPID string) (VerifiedCredential, error) {
	if len(authData) < 37 {
		return VerifiedCredential{}, errors.New("authenticatorData too short")
	}
	wantHash := sha256.Sum256([]byte(expectedRPID))
	if !bytes.Equal(authData[:32], wantHash[:]) {
		return VerifiedCredential{}, errors.New("authenticatorData rpIdHash mismatch")
	}
	flags := authData[32]
	if flags&flagUserPresent == 0 {
		return VerifiedCredential{}, errors.New("authenticatorData user presence flag not set")
	}
	if flags&flagAttestedData == 0 {
		return VerifiedCredential{}, errors.New("authenticatorData missing attested credential data")
	}
	signCount := binary.BigEndian.Uint32(authData[33:37])
	// Attested credential data: 16-byte AAGUID + 2-byte credID len + credID + COSE key
	if len(authData) < 37+16+2 {
		return VerifiedCredential{}, errors.New("attested credential data truncated")
	}
	credLen := int(binary.BigEndian.Uint16(authData[53:55]))
	if credLen == 0 || credLen > 1023 {
		return VerifiedCredential{}, errors.New("invalid credential ID length")
	}
	credEnd := 55 + credLen
	if len(authData) <= credEnd {
		return VerifiedCredential{}, errors.New("attested credential data missing public key")
	}
	credID := authData[55:credEnd]
	coseTail := authData[credEnd:]
	coseVal, consumed, err := decodeCBOR(coseTail)
	if err != nil {
		return VerifiedCredential{}, fmt.Errorf("invalid COSE public key: %w", err)
	}
	coseBytes := coseTail[:consumed]
	alg, _, err := parseCOSEPublicKey(coseVal)
	if err != nil {
		return VerifiedCredential{}, err
	}
	return VerifiedCredential{
		CredentialID:   base64.RawURLEncoding.EncodeToString(credID),
		PublicKeyCOSE:  base64.RawURLEncoding.EncodeToString(coseBytes),
		Alg:            alg,
		SignCount:      signCount,
		BackupEligible: flags&flagBackupEligible != 0,
		BackupState:    flags&flagBackupState != 0,
	}, nil
}

// AssertionVerifyInput holds the raw browser assertion response fields.
type AssertionVerifyInput struct {
	ExpectedChallenge string
	ExpectedOrigin    string
	ExpectedRPID      string
	StoredPublicKey   string // base64url COSE_Key
	StoredSignCount   uint32
	ClientDataJSON    string // base64url
	AuthenticatorData string // base64url
	Signature         string // base64url
}

// VerifiedAssertion is the verified signCount and flags from a login/step-up.
type VerifiedAssertion struct {
	SignCount      uint32
	UserVerified   bool
	BackupEligible bool
	BackupState    bool
}

// VerifyAssertion validates a WebAuthn assertion signature and anti-clone state.
func VerifyAssertion(in AssertionVerifyInput) (VerifiedAssertion, error) {
	clientBytes, err := verifyClientData(in.ClientDataJSON, "webauthn.get", in.ExpectedChallenge, in.ExpectedOrigin)
	if err != nil {
		return VerifiedAssertion{}, err
	}
	authData, err := decodeBase64URL(in.AuthenticatorData)
	if err != nil {
		return VerifiedAssertion{}, fmt.Errorf("invalid authenticatorData encoding: %w", err)
	}
	if len(authData) < 37 {
		return VerifiedAssertion{}, errors.New("authenticatorData too short")
	}
	wantHash := sha256.Sum256([]byte(in.ExpectedRPID))
	if !bytes.Equal(authData[:32], wantHash[:]) {
		return VerifiedAssertion{}, errors.New("authenticatorData rpIdHash mismatch")
	}
	flags := authData[32]
	if flags&flagUserPresent == 0 {
		return VerifiedAssertion{}, errors.New("authenticatorData user presence flag not set")
	}
	signCount := binary.BigEndian.Uint32(authData[33:37])
	// WebAuthn §7.2 step 21: if either counter is non-zero, the new counter
	// must strictly increase; 0 -> 0 is standard for synced passkeys.
	if (in.StoredSignCount > 0 || signCount > 0) && signCount <= in.StoredSignCount {
		return VerifiedAssertion{}, ErrSignCountAnomaly
	}
	sig, err := decodeBase64URL(in.Signature)
	if err != nil || len(sig) == 0 {
		return VerifiedAssertion{}, errors.New("invalid assertion signature encoding")
	}
	coseBytes, err := decodeBase64URL(in.StoredPublicKey)
	if err != nil {
		return VerifiedAssertion{}, errors.New("corrupt stored public key")
	}
	coseVal, _, err := decodeCBOR(coseBytes)
	if err != nil {
		return VerifiedAssertion{}, errors.New("corrupt stored COSE key")
	}
	alg, pub, err := parseCOSEPublicKey(coseVal)
	if err != nil {
		return VerifiedAssertion{}, err
	}
	clientHash := sha256.Sum256(clientBytes)
	signed := make([]byte, 0, len(authData)+len(clientHash))
	signed = append(signed, authData...)
	signed = append(signed, clientHash[:]...)
	if err := verifySignature(alg, pub, signed, sig); err != nil {
		return VerifiedAssertion{}, err
	}
	return VerifiedAssertion{
		SignCount:      signCount,
		UserVerified:   flags&flagUserVerified != 0,
		BackupEligible: flags&flagBackupEligible != 0,
		BackupState:    flags&flagBackupState != 0,
	}, nil
}

func verifyClientData(encoded, wantType, wantChallenge, wantOrigin string) ([]byte, error) {
	raw, err := decodeBase64URL(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid clientDataJSON encoding: %w", err)
	}
	var cd clientData
	if err := json.Unmarshal(raw, &cd); err != nil {
		return nil, fmt.Errorf("invalid clientDataJSON: %w", err)
	}
	if cd.Type != wantType {
		return nil, fmt.Errorf("unexpected clientData type %q", cd.Type)
	}
	if cd.Challenge != wantChallenge {
		return nil, errors.New("clientData challenge mismatch")
	}
	if cd.Origin != wantOrigin {
		return nil, fmt.Errorf("clientData origin %q does not match %q", cd.Origin, wantOrigin)
	}
	if cd.CrossOrigin {
		return nil, errors.New("cross-origin WebAuthn ceremonies are not allowed")
	}
	return raw, nil
}

func parseCOSEPublicKey(val any) (int64, crypto.PublicKey, error) {
	m, ok := val.(cborMap)
	if !ok {
		return 0, nil, errors.New("COSE key must be a CBOR map")
	}
	kty, ok := intField(m, 1)
	if !ok {
		return 0, nil, errors.New("COSE key missing kty")
	}
	alg, ok := intField(m, 3)
	if !ok {
		return 0, nil, errors.New("COSE key missing alg")
	}
	switch alg {
	case AlgES256:
		if kty != 2 {
			return 0, nil, errors.New("ES256 requires EC2 key type")
		}
		crv, ok := intField(m, -1)
		if !ok || crv != 1 { // P-256
			return 0, nil, errors.New("ES256 requires P-256 curve")
		}
		xBytes, ok1 := bytesField(m, -2)
		yBytes, ok2 := bytesField(m, -3)
		if !ok1 || !ok2 || len(xBytes) != 32 || len(yBytes) != 32 {
			return 0, nil, errors.New("ES256 key missing valid P-256 coordinates")
		}
		uncompressed := make([]byte, 1+len(xBytes)+len(yBytes))
		uncompressed[0] = 0x04
		copy(uncompressed[1:], xBytes)
		copy(uncompressed[1+len(xBytes):], yBytes)
		pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), uncompressed)
		if err != nil {
			return 0, nil, errors.New("ES256 public key is not on P-256")
		}
		return alg, pub, nil
	case AlgEdDSA:
		if kty != 1 { // OKP
			return 0, nil, errors.New("EdDSA requires OKP key type")
		}
		crv, ok := intField(m, -1)
		if !ok || crv != 6 { // Ed25519
			return 0, nil, errors.New("EdDSA requires Ed25519 curve")
		}
		xBytes, ok := bytesField(m, -2)
		if !ok || len(xBytes) != ed25519.PublicKeySize {
			return 0, nil, errors.New("EdDSA key missing valid Ed25519 bytes")
		}
		return alg, ed25519.PublicKey(xBytes), nil
	case AlgRS256:
		if kty != 3 { // RSA
			return 0, nil, errors.New("RS256 requires RSA key type")
		}
		nBytes, ok1 := bytesField(m, -1)
		eBytes, ok2 := bytesField(m, -2)
		if !ok1 || !ok2 || len(nBytes) < 256 || len(eBytes) == 0 || len(eBytes) > 8 {
			return 0, nil, errors.New("RS256 key missing valid RSA parameters")
		}
		n := new(big.Int).SetBytes(nBytes)
		eBig := new(big.Int).SetBytes(eBytes)
		if !eBig.IsInt64() || eBig.Int64() < 3 {
			return 0, nil, errors.New("RS256 key has invalid exponent")
		}
		return alg, &rsa.PublicKey{N: n, E: int(eBig.Int64())}, nil
	default:
		return 0, nil, fmt.Errorf("unsupported COSE algorithm %d", alg)
	}
}

func verifySignature(alg int64, pub crypto.PublicKey, signed, sig []byte) error {
	switch alg {
	case AlgES256:
		key, ok := pub.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("key type mismatch for ES256")
		}
		digest := sha256.Sum256(signed)
		if !ecdsa.VerifyASN1(key, digest[:], sig) {
			return ErrInvalidAssertion
		}
		return nil
	case AlgEdDSA:
		key, ok := pub.(ed25519.PublicKey)
		if !ok {
			return errors.New("key type mismatch for EdDSA")
		}
		if !ed25519.Verify(key, signed, sig) {
			return ErrInvalidAssertion
		}
		return nil
	case AlgRS256:
		key, ok := pub.(*rsa.PublicKey)
		if !ok {
			return errors.New("key type mismatch for RS256")
		}
		digest := sha256.Sum256(signed)
		if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
			return ErrInvalidAssertion
		}
		return nil
	default:
		return fmt.Errorf("unsupported COSE algorithm %d", alg)
	}
}

func intField(m cborMap, key int64) (int64, bool) {
	v, ok := m.intKey(key)
	if !ok {
		return 0, false
	}
	n, ok := v.(int64)
	return n, ok
}

func bytesField(m cborMap, key int64) ([]byte, bool) {
	v, ok := m.intKey(key)
	if !ok {
		return nil, false
	}
	b, ok := v.([]byte)
	return b, ok
}

func decodeBase64URL(encoded string) ([]byte, error) {
	s := strings.TrimRight(strings.TrimSpace(encoded), "=")
	if s == "" {
		return nil, errors.New("empty base64url value")
	}
	return base64.RawURLEncoding.DecodeString(s)
}
