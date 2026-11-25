package relay

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	FRPPortMin      = 30000
	FRPPortMax      = 39999
	FRPRole         = "frp-data"
	FRPOIDCPath     = "/v1/frp/oidc"
	FRPJWKSPath     = FRPOIDCPath + "/jwks.json"
	FRPGrantOverlap = 10 * time.Second
)

// FRPAudience is the dedicated frp token audience for a validated node ID.
func FRPAudience(nodeID NodeID) string { return "webmux-frp:" + string(nodeID) }

// FRPClaims is the signed payload for stock frp OIDC and the node's own
// allocation admission. The node checks the signature and current grant.
type FRPClaims struct {
	Issuer        string  `json:"iss"`
	Audience      string  `json:"aud"`
	Subject       string  `json:"sub"`
	ID            Token   `json:"jti"`
	IssuedAt      int64   `json:"iat"`
	NotBefore     int64   `json:"nbf"`
	ExpiresAt     int64   `json:"exp"`
	Scope         Scope   `json:"scope"`
	NodeID        NodeID  `json:"nodeId"`
	AllocationID  Token   `json:"allocationId"`
	GrantRevision Counter `json:"grantRevision"`
	Role          string  `json:"role"`
	ProxyName     string  `json:"proxyName"`
	RemotePort    uint16  `json:"remotePort"`
}

// FRPOriginServerName is the internal DNS SAN for a validated allocation ID.
func FRPOriginServerName(id Token) string {
	digest := sha256.Sum256([]byte(id))
	return hex.EncodeToString(digest[:16]) + ".origin.webmux.invalid"
}

// FRPOriginURI binds a validated allocation ID and scope to the origin TLS
// endpoint's role ("host" or "node").
func FRPOriginURI(scope Scope, id Token, role string) *url.URL {
	return &url.URL{
		Scheme: "spiffe",
		Host:   "webmux-relay",
		Path: "/frp/" + role + "/" + string(id) + "/" + string(scope.HostID) +
			"/" + strconv.FormatUint(uint64(scope.BindingGeneration), 10) +
			"/" + strconv.FormatUint(uint64(scope.ConnectorGeneration), 10) +
			"/" + string(scope.ControlEpoch) + "/" + string(scope.NodeEpoch),
	}
}

// AllocationRequest carries the host's CSR on the management stream. The node
// adds its own CSR only when forwarding to control over the private channel.
type AllocationRequest struct {
	HostServerCSR string `json:"hostServerCsr"`
	NodeClientCSR string `json:"nodeClientCsr,omitempty"`
}

func (r *AllocationRequest) UnmarshalJSON(b []byte) error {
	var wire struct {
		HostServerCSR string          `json:"hostServerCsr"`
		NodeClientCSR json.RawMessage `json:"nodeClientCsr"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return Error{Code: Invalid}
	}
	var nodeCSR string
	if wire.NodeClientCSR != nil && (json.Unmarshal(wire.NodeClientCSR, &nodeCSR) != nil || nodeCSR == "") {
		return Error{Code: Invalid}
	}
	*r = AllocationRequest{HostServerCSR: wire.HostServerCSR, NodeClientCSR: nodeCSR}
	return nil
}

func (r AllocationRequest) validate(private bool) error {
	if len(r.HostServerCSR)+len(r.NodeClientCSR) > MaxControlBytes || private != (r.NodeClientCSR != "") ||
		!allocationPEM(r.HostServerCSR, "CERTIFICATE REQUEST", false) ||
		private && !allocationPEM(r.NodeClientCSR, "CERTIFICATE REQUEST", false) {
		return Error{Code: Invalid}
	}
	return nil
}

type AllocationRenew struct {
	AllocationID  Token   `json:"allocationId"`
	GrantRevision Counter `json:"grantRevision"`
}

// AllocationOrigin is the public certificate material needed by the host. Leaf
// chains begin with the leaf; TrustCertPEM contains the dedicated origin trust
// anchors. SPKI digests are lowercase hex SHA-256 of DER SubjectPublicKeyInfo.
type AllocationOrigin struct {
	HostServerCertPEM string `json:"hostServerCertPem"`
	TrustCertPEM      string `json:"trustCertPem"`
	NodeClientSPKI    Digest `json:"nodeClientSpki"`
}

// AllocationNodeOrigin is private-channel-only public certificate material.
// Neither form of the grant contains either endpoint's private key.
type AllocationNodeOrigin struct {
	NodeClientCertPEM string `json:"nodeClientCertPem"`
	HostServerSPKI    Digest `json:"hostServerSpki"`
}

type AllocationGrant struct {
	AllocationID  Token                 `json:"allocationId"`
	GrantRevision Counter               `json:"grantRevision"`
	ProxyName     string                `json:"proxyName"`
	RemotePort    uint16                `json:"remotePort"`
	TransportURL  string                `json:"transportUrl"`
	BridgeKey     Token                 `json:"bridgeKey"`
	JWT           string                `json:"jwt"`
	IssuedAt      int64                 `json:"issuedAt"`
	NotBefore     int64                 `json:"notBefore"`
	ExpiresAt     int64                 `json:"expiresAt"`
	RenewAfter    int64                 `json:"renewAfter"`
	Origin        AllocationOrigin      `json:"origin"`
	NodeOrigin    *AllocationNodeOrigin `json:"nodeOrigin,omitempty"` // Strip before sending to host.
}

func (g AllocationGrant) validate(now time.Time, private bool) error {
	if err := validateAllocationRef(g.AllocationID, g.GrantRevision); err != nil {
		return err
	}
	if err := g.BridgeKey.Validate(); err != nil {
		return err
	}
	if !identifier(g.ProxyName, 128) || g.RemotePort == 0 || private != (g.NodeOrigin != nil) {
		return Error{Code: Invalid}
	}
	u, err := secureURL(g.TransportURL, "wss", false)
	if err != nil || !strings.HasPrefix(g.TransportURL, "wss://") || strings.Contains(g.TransportURL, "#") ||
		u.Host != u.Hostname() && u.Port() != "443" || u.Path != "/v1/frp/"+string(g.AllocationID) {
		return Error{Code: Invalid}
	}
	if g.IssuedAt <= 0 || g.NotBefore <= 0 || g.RenewAfter <= g.IssuedAt || g.RenewAfter <= g.NotBefore || g.ExpiresAt <= g.RenewAfter {
		return Error{Code: Invalid}
	}
	if !validUntil(now, g.ExpiresAt) {
		return Error{Code: Expired}
	}
	// Tolerate small issuer clock skew on activation only, never on expiration.
	latest := now.Add(5 * time.Second).Unix()
	if g.IssuedAt > latest || g.NotBefore > latest {
		return Error{Code: Invalid}
	}
	size := len(g.JWT) + len(g.Origin.HostServerCertPEM) + len(g.Origin.TrustCertPEM)
	if private {
		size += len(g.NodeOrigin.NodeClientCertPEM)
	}
	if size > MaxControlBytes || !allocationJWT(g.JWT) || g.Origin.NodeClientSPKI.Validate() != nil ||
		!allocationPEM(g.Origin.HostServerCertPEM, "CERTIFICATE", true) ||
		!allocationPEM(g.Origin.TrustCertPEM, "CERTIFICATE", true) {
		return Error{Code: Invalid}
	}
	if private && (g.NodeOrigin.HostServerSPKI.Validate() != nil || !allocationPEM(g.NodeOrigin.NodeClientCertPEM, "CERTIFICATE", true)) {
		return Error{Code: Invalid}
	}
	return nil
}

type AllocationStatusReason string

const (
	AllocationStatusUnavailable AllocationStatusReason = "unavailable"
	AllocationStatusProbeFailed AllocationStatusReason = "probe_failed"
)

type AllocationStatus struct {
	AllocationID  Token                  `json:"allocationId"`
	GrantRevision Counter                `json:"grantRevision"`
	DataReady     bool                   `json:"dataReady"`
	Reason        AllocationStatusReason `json:"reason,omitempty"`
	CheckedAt     int64                  `json:"checkedAt,omitempty"` // Unix milliseconds at probe completion; zero if no measurement.
}

func (s *AllocationStatus) UnmarshalJSON(b []byte) error {
	type status AllocationStatus
	var wire struct {
		status
		DataReady *bool `json:"dataReady"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil || wire.DataReady == nil {
		return Error{Code: Invalid}
	}
	*s = AllocationStatus(wire.status)
	s.DataReady = *wire.DataReady
	return nil
}

func (s AllocationStatus) validate(expiresAt int64) error {
	if err := validateAllocationRef(s.AllocationID, s.GrantRevision); err != nil {
		return err
	}
	if s.CheckedAt < 0 || s.CheckedAt/1000 >= expiresAt || s.CheckedAt > 253402300799999 {
		return Error{Code: Invalid}
	}
	switch s.Reason {
	case "":
		return nil
	case AllocationStatusUnavailable, AllocationStatusProbeFailed:
		if !s.DataReady {
			return nil
		}
	}
	return Error{Code: Invalid}
}

func validateAllocationRef(id Token, revision Counter) error {
	if revision == 0 {
		return Error{Code: Invalid}
	}
	return id.Validate()
}

// allocationJWT checks only the compact serialization shape. The admission
// receiver, not the wire codec, verifies signature, claims and current grant.
func allocationJWT(s string) bool {
	header, rest, ok := strings.Cut(s, ".")
	if !ok {
		return false
	}
	claims, signature, ok := strings.Cut(rest, ".")
	return ok && identifier(header, MaxControlBytes) && identifier(claims, MaxControlBytes) && identifier(signature, MaxControlBytes)
}

// allocationPEM checks block framing and prevents extra text, headers, or other
// PEM types from entering a CSR or chain. Receivers verify DER, chains and keys
// against their respective issuance or TLS trust policy.
func allocationPEM(s, label string, chain bool) bool {
	begin := []byte("-----BEGIN " + label + "-----")
	end := []byte("\n-----END " + label + "-----")
	rest := []byte(strings.TrimSpace(s))
	for len(rest) > 0 {
		if !bytes.HasPrefix(rest, begin) {
			return false
		}
		i := bytes.Index(rest, end)
		if i < len(begin) || bytes.Contains(rest[len(begin):i], []byte("-----BEGIN ")) {
			return false
		}
		cut := i + len(end)
		block, extra := pem.Decode(rest[:cut])
		if block == nil || block.Type != label || len(block.Bytes) == 0 || len(block.Headers) != 0 || len(extra) != 0 {
			return false
		}
		if cut == len(rest) {
			return true
		}
		if !chain || rest[cut] != '\n' && !(rest[cut] == '\r' && cut+1 < len(rest) && rest[cut+1] == '\n') {
			return false
		}
		rest = bytes.TrimSpace(rest[cut:])
	}
	return false
}
