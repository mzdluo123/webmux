package relay

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

type PublicKey struct {
	Algorithm string `json:"algorithm"`
	Key       string `json:"key"`
}

func (p PublicKey) Parse() (ed25519.PublicKey, error) {
	if p.Algorithm != HostAlgorithm {
		return nil, Error{Code: Invalid}
	}
	b, err := decodeFixed(p.Key, ed25519.PublicKeySize)
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(b), nil
}

type ServiceTrust struct {
	Algorithm string   `json:"algorithm"`
	Pins      []string `json:"pins"`
}

func (t ServiceTrust) Validate() error {
	if t.Algorithm != ServiceAlgorithm || len(t.Pins) < 1 || len(t.Pins) > MaxPins {
		return Error{Code: Invalid}
	}
	for i, pin := range t.Pins {
		if _, err := decodeFixed(pin, sha256.Size); err != nil {
			return err
		}
		for _, prior := range t.Pins[:i] {
			if prior == pin {
				return Error{Code: Invalid}
			}
		}
	}
	return nil
}

// VerifyCertificatePin supplements, NEVER replaces, normal TLS verification.
// The transport must first verify chain, hostname, time and TLS >= 1.3.
func VerifyCertificatePin(cert *x509.Certificate, trust ServiceTrust) error {
	if err := trust.Validate(); err != nil {
		return err
	}
	if cert == nil {
		return Error{Code: Unauthenticated}
	}
	key, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok || key.Curve != elliptic.P256() {
		return Error{Code: Unauthenticated}
	}
	digest := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	pin := base64.RawURLEncoding.EncodeToString(digest[:])
	for _, allowed := range trust.Pins {
		if pin == allowed {
			return nil
		}
	}
	return Error{Code: Unauthenticated}
}

type ConnectorHello struct {
	Version             Version `json:"version"`
	HostID              HostID  `json:"hostId"`
	BindingGeneration   Counter `json:"bindingGeneration"`
	ConnectorGeneration Counter `json:"connectorGeneration"`
	Role                Role    `json:"role"`
}

func (h ConnectorHello) Validate() error {
	if h.Version != V1 {
		return Error{Code: Incompatible}
	}
	if err := h.HostID.Validate(); err != nil {
		return err
	}
	if h.BindingGeneration == 0 || !h.Role.Valid() || h.Role == RoleBulk && h.ConnectorGeneration == 0 {
		return Error{Code: Invalid}
	}
	return nil
}

type ConnectorChallenge struct {
	Version             Version `json:"version"`
	Audience            string  `json:"audience"`
	HostID              HostID  `json:"hostId"`
	BindingGeneration   Counter `json:"bindingGeneration"`
	ConnectorGeneration Counter `json:"connectorGeneration"`
	Role                Role    `json:"role"`
	Nonce               Token   `json:"nonce"`
	ExpiresAt           int64   `json:"expiresAt"`
	ControlEpoch        Token   `json:"controlEpoch"`
	NodeEpoch           Token   `json:"nodeEpoch"`
}

func (c ConnectorChallenge) Validate(now time.Time) error {
	if c.Version != V1 {
		return Error{Code: Incompatible}
	}
	if !audience(c.Audience) {
		return Error{Code: Invalid}
	}
	if err := c.HostID.Validate(); err != nil {
		return err
	}
	if c.BindingGeneration == 0 || c.ConnectorGeneration == 0 || !c.Role.Valid() {
		return Error{Code: Invalid}
	}
	if err := c.Nonce.Validate(); err != nil {
		return err
	}
	if err := c.ControlEpoch.Validate(); err != nil {
		return err
	}
	if err := c.NodeEpoch.Validate(); err != nil {
		return err
	}
	if !validUntil(now, c.ExpiresAt, NonceTTL) {
		return Error{Code: Expired}
	}
	return nil
}

// ConnectorTranscript implements the approved newline layout exactly: UTF-8,
// decimal generations without leading zeroes, canonical base64url nonce, no
// trailing newline. Epoch/deadline are bound by the server's nonce record.
func ConnectorTranscript(c ConnectorChallenge) ([]byte, error) {
	// Time validity is checked by the signer/verifier, not byte construction.
	if err := c.Validate(time.Unix(c.ExpiresAt-1, 0)); err != nil {
		return nil, err
	}
	return []byte(strings.Join([]string{
		"WebMux-Relay-v1", c.Audience, string(c.HostID),
		strconv.FormatUint(uint64(c.BindingGeneration), 10),
		strconv.FormatUint(uint64(c.ConnectorGeneration), 10), string(c.Role), string(c.Nonce),
	}, "\n")), nil
}

type ConnectorProof struct {
	Version   Version `json:"version"`
	Algorithm string  `json:"algorithm"`
	Nonce     Token   `json:"nonce"`
	Signature string  `json:"signature"`
}

func SignConnector(key ed25519.PrivateKey, c ConnectorChallenge, now time.Time) (ConnectorProof, error) {
	if len(key) != ed25519.PrivateKeySize {
		return ConnectorProof{}, Error{Code: Invalid}
	}
	if err := c.Validate(now); err != nil {
		return ConnectorProof{}, err
	}
	b, err := ConnectorTranscript(c)
	if err != nil {
		return ConnectorProof{}, err
	}
	return ConnectorProof{Version: V1, Algorithm: HostAlgorithm, Nonce: c.Nonce, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, b))}, nil
}

// VerifyConnector verifies a proof against the immutable challenge stored for
// THIS physical connection. The caller must atomically consume that challenge
// before invoking this helper (including failed attempts), and separately check
// expected audience, current binding, epochs and role before replacing sessions.
func VerifyConnector(key PublicKey, c ConnectorChallenge, p ConnectorProof, now time.Time) error {
	if p.Version != V1 {
		return Error{Code: Incompatible}
	}
	if err := c.Validate(now); err != nil {
		return err
	}
	if p.Algorithm != HostAlgorithm || p.Nonce != c.Nonce {
		return Error{Code: Unauthenticated}
	}
	pub, err := key.Parse()
	if err != nil {
		return err
	}
	sig, err := decodeFixed(p.Signature, ed25519.SignatureSize)
	if err != nil {
		return Error{Code: InvalidSignature}
	}
	b, err := ConnectorTranscript(c)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, b, sig) {
		return Error{Code: InvalidSignature}
	}
	return nil
}

type HostPurpose string

const (
	PurposeAssignment HostPurpose = "assignment"
	PurposeDelete     HostPurpose = "delete-binding"
)

type HostChallengeRequest struct {
	Version Version     `json:"version"`
	HostID  HostID      `json:"hostId"`
	Purpose HostPurpose `json:"purpose"`
}

// HostChallenge uses an independent domain from connector authentication.
type HostChallenge struct {
	Version           Version     `json:"version"`
	Audience          string      `json:"audience"`
	HostID            HostID      `json:"hostId"`
	BindingGeneration Counter     `json:"bindingGeneration"`
	Purpose           HostPurpose `json:"purpose"`
	Nonce             Token       `json:"nonce"`
	ExpiresAt         int64       `json:"expiresAt"`
	ControlEpoch      Token       `json:"controlEpoch"`
}

func (c HostChallenge) Validate(now time.Time) error {
	if c.Version != V1 {
		return Error{Code: Incompatible}
	}
	if err := c.HostID.Validate(); err != nil {
		return err
	}
	if !audience(c.Audience) || c.BindingGeneration == 0 || c.Purpose != PurposeAssignment && c.Purpose != PurposeDelete {
		return Error{Code: Invalid}
	}
	if err := c.Nonce.Validate(); err != nil {
		return err
	}
	if err := c.ControlEpoch.Validate(); err != nil {
		return err
	}
	if !validUntil(now, c.ExpiresAt, NonceTTL) {
		return Error{Code: Expired}
	}
	return nil
}

func HostTranscript(c HostChallenge) ([]byte, error) {
	if err := c.Validate(time.Unix(c.ExpiresAt-1, 0)); err != nil {
		return nil, err
	}
	return []byte(strings.Join([]string{"WebMux-Relay-v1/" + string(c.Purpose), c.Audience, string(c.HostID), strconv.FormatUint(uint64(c.BindingGeneration), 10), string(c.Nonce)}, "\n")), nil
}

type HostProof struct {
	Version   Version `json:"version"`
	HostID    HostID  `json:"hostId"`
	Algorithm string  `json:"algorithm"`
	Nonce     Token   `json:"nonce"`
	Signature string  `json:"signature"`
}

func SignHost(key ed25519.PrivateKey, c HostChallenge, now time.Time) (HostProof, error) {
	if len(key) != ed25519.PrivateKeySize {
		return HostProof{}, Error{Code: Invalid}
	}
	if err := c.Validate(now); err != nil {
		return HostProof{}, err
	}
	b, err := HostTranscript(c)
	if err != nil {
		return HostProof{}, err
	}
	return HostProof{Version: V1, HostID: c.HostID, Algorithm: HostAlgorithm, Nonce: c.Nonce, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, b))}, nil
}

// VerifyHost has the same atomic nonce-consumption requirement as VerifyConnector.
func VerifyHost(key PublicKey, c HostChallenge, p HostProof, now time.Time) error {
	if p.Version != V1 {
		return Error{Code: Incompatible}
	}
	if err := c.Validate(now); err != nil {
		return err
	}
	if p.HostID != c.HostID || p.Nonce != c.Nonce || p.Algorithm != HostAlgorithm {
		return Error{Code: Unauthenticated}
	}
	pub, err := key.Parse()
	if err != nil {
		return err
	}
	sig, err := decodeFixed(p.Signature, ed25519.SignatureSize)
	if err != nil {
		return Error{Code: InvalidSignature}
	}
	b, err := HostTranscript(c)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, b, sig) {
		return Error{Code: InvalidSignature}
	}
	return nil
}
