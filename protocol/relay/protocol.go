// Package relay defines the WebMux Relay v1 wire contract. It contains no
// transport, persistence, authorization service, or production trust defaults.
package relay

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"time"
)

type Version uint16

const (
	V1                            Version = 1
	HostAlgorithm                         = "Ed25519"
	ServiceAlgorithm                      = "ECDSA-P256"
	ConnectorSubprotocol                  = "webmux-relay.v2"
	NodeSubprotocol                       = "webmux-relay-node.v1"
	MaxAuthBytes                          = 4096
	MaxControlBytes                       = 65536
	MaxSnapshotBytes                      = 1048576
	MaxStreamHeaderBytes                  = 1024
	MaxURLBytes                           = 2048
	MaxPins                               = 4
	MaxTURNURLs                           = 8
	MaxBindings                           = 4096
	MaxPendingRequests                    = 32
	AuthTimeout                           = 10 * time.Second
	NonceTTL                              = 30 * time.Second
	RequestTimeout                        = 10 * time.Second
	ControlMessageTTL                     = 60 * time.Second
	StreamAdmissionTimeout                = 15 * time.Second
	MaxRoleConnections                    = 16
	DefaultInteractiveConnections         = 2
	DefaultBulkConnections                = 4
	MaxPendingStreams                     = 128
	StreamOpenTimeout                     = 10 * time.Second
	KeepAliveInterval                     = 10 * time.Second
	ControlLease                          = 30 * time.Second
	LoginCodeTTL                          = 60 * time.Second
	BrowserTransactionTTL                 = 5 * time.Minute
	BrowserSessionTTL                     = 12 * time.Hour
	MaxICETTL                             = 10 * time.Minute
)

type Code string

const (
	Invalid          Code = "invalid_request"
	Incompatible     Code = "incompatible_version"
	Unauthenticated  Code = "unauthenticated"
	Forbidden        Code = "forbidden"
	NotFound         Code = "not_found"
	Conflict         Code = "conflict"
	Expired          Code = "expired"
	Replay           Code = "replay"
	RateLimited      Code = "rate_limited"
	Unavailable      Code = "unavailable"
	UpstreamFailed   Code = "upstream_failed"
	Pending          Code = "pending"
	InvalidSignature Code = "invalid_signature"
	WrongAudience    Code = "wrong_audience"
	StaleGeneration  Code = "stale_generation"
	InvalidConfig    Code = "invalid_config"
)

// Error deliberately contains no user-controlled description or secret material.
// RequestID is correlation only; it does not make operations retryable.
type Error struct {
	Code              Code   `json:"code"`
	RequestID         string `json:"requestId,omitempty"`
	RetryAfterSeconds uint32 `json:"retryAfterSeconds,omitempty"`
}

func (e Error) Error() string {
	if !e.Code.Valid() {
		return "relay: internal_error"
	}
	return "relay: " + string(e.Code)
}

func (c Code) Valid() bool {
	switch c {
	case Invalid, Incompatible, Unauthenticated, Forbidden, NotFound, Conflict,
		Expired, Replay, RateLimited, Unavailable, UpstreamFailed, Pending,
		InvalidSignature, WrongAudience, StaleGeneration, InvalidConfig:
		return true
	default:
		return false
	}
}

// HTTPStatus is the controller mapping, not a business-service error wrapper.
func HTTPStatus(c Code) int {
	switch c {
	case Invalid, InvalidConfig:
		return 400
	case Incompatible:
		return 426
	case Unauthenticated, InvalidSignature:
		return 401
	case Forbidden, WrongAudience:
		return 403
	case NotFound:
		return 404
	case Conflict, Replay, StaleGeneration:
		return 409
	case Expired:
		return 410
	case RateLimited:
		return 429
	case UpstreamFailed:
		return 502
	case Unavailable:
		return 503
	case Pending:
		return 202
	default:
		return 500
	}
}

type Capability string

const (
	CapabilityWSS  Capability = "wss"
	CapabilityTURN Capability = "turn"
)

type Capabilities []Capability

func (c Capabilities) Has(want Capability) bool {
	for _, v := range c {
		if v == want {
			return true
		}
	}
	return false
}

func (c Capabilities) Validate() error {
	if len(c) < 1 || len(c) > 2 {
		return Error{Code: InvalidConfig}
	}
	for i, v := range c {
		if v != CapabilityWSS && v != CapabilityTURN {
			return Error{Code: InvalidConfig}
		}
		if i > 0 && c[i-1] >= v {
			return Error{Code: InvalidConfig}
		}
	}
	return nil
}

type Role string

const (
	RoleControl     Role = "control"
	RoleBulk        Role = "bulk"
	RoleInteractive Role = "interactive"
)

func (r Role) Valid() bool { return r == RoleControl || r == RoleBulk || r == RoleInteractive }

// MachineID is a random installation identifier, not an authentication secret.
// Only an independently authenticated owner may use it to match a binding.
type MachineID string

func (id MachineID) Validate() error {
	var data [32]byte
	if len(id) != base64.RawURLEncoding.EncodedLen(len(data)) {
		return Error{Code: Invalid}
	}
	n, err := base64.RawURLEncoding.Strict().Decode(data[:], []byte(id))
	if err != nil || n != len(data) {
		return Error{Code: Invalid}
	}
	return nil
}

// HostID is exactly 16 random bytes encoded as 32 lowercase hexadecimal digits.
type HostID string

func (h HostID) Validate() error {
	if len(h) != 32 {
		return Error{Code: Invalid}
	}
	for _, c := range h {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return Error{Code: Invalid}
		}
	}
	return nil
}

func (h HostID) Label() string { return "h-" + string(h) }

// NodeID is a stable administrator-assigned opaque identifier, not a hostname.
type NodeID string

func (n NodeID) Validate() error {
	if !identifier(string(n), 64) {
		return Error{Code: Invalid}
	}
	return nil
}

func identifier(s string, max int) bool {
	if len(s) == 0 || len(s) > max {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func audience(s string) bool {
	if len(s) == 0 || len(s) > 255 {
		return false
	}
	for _, c := range s {
		if c < 0x21 || c > 0x7e {
			return false
		}
	}
	return true
}

// Token is unpadded canonical base64url of exactly 32 cryptographically random
// bytes. The same encoding is used for nonce, epoch, request ID and grant handles.
type Token string

func (t Token) Validate() error {
	_, err := decodeFixed(string(t), 32)
	return err
}

func decodeFixed(s string, size int) ([]byte, error) {
	if len(s) != base64.RawURLEncoding.EncodedLen(size) {
		return nil, Error{Code: Invalid}
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil || len(b) != size {
		return nil, Error{Code: Invalid}
	}
	return b, nil
}

type Digest string

func (d Digest) Validate() error {
	if len(d) != 64 {
		return Error{Code: InvalidConfig}
	}
	b, err := hex.DecodeString(string(d))
	if err != nil || hex.EncodeToString(b) != string(d) {
		return Error{Code: InvalidConfig}
	}
	return nil
}

// Counter is a positive decimal uint64 encoded as a JSON string, avoiding JS
// precision loss. Zero is reserved for absent/not-yet-applied state.
type Counter uint64

func (c Counter) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(strconv.FormatUint(uint64(c), 10))), nil
}
func (c *Counter) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) != nil {
		return Error{Code: Invalid}
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || strconv.FormatUint(n, 10) != s {
		return Error{Code: Invalid}
	}
	*c = Counter(n)
	return nil
}

// Decode rejects unknown fields, duplicate object keys, trailing JSON and
// oversized input. Call the DTO's Validate method after decoding. Readers must
// enforce the same limit before allocating the complete input.
func Decode(data []byte, dst any, max int) error {
	if max <= 0 || len(data) > max {
		return Error{Code: Invalid}
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueValue(d, 0); err != nil {
		return Error{Code: Invalid}
	}
	if _, err := d.Token(); err != io.EOF {
		return Error{Code: Invalid}
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return Error{Code: Invalid}
	}
	return nil
}

func uniqueValue(d *json.Decoder, depth int) error {
	if depth > 32 {
		return Error{Code: Invalid}
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok {
				return Error{Code: Invalid}
			}
			if _, found := seen[s]; found {
				return Error{Code: Invalid}
			}
			seen[s] = struct{}{}
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return Error{Code: Invalid}
	}
	_, err = d.Token()
	return err
}

// Lifetime limits are enforced when issuing; receivers check absolute expiry.
func validUntil(now time.Time, expires int64) bool {
	return expires > now.Unix()
}
