package relay

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"net"
	"strconv"
	"time"
)

const (
	// A WSS peer advertises support out of band so old strict JSON decoders
	// never see new messages or fields. The value is echoed only by a peer
	// that can handle selection on this authenticated connection.
	TransportHeader       = "WebMux-Relay-Transport"
	TransportSelectionV1  = "selection.v1"
	TransportCandidateTTL = 45 * time.Second
)

type FRPTransport string

const (
	FRPTransportWSS  FRPTransport = "wss"
	FRPTransportQUIC FRPTransport = "quic"
)

func (p FRPTransport) Valid() bool { return p == FRPTransportWSS || p == FRPTransportQUIC }

// QUICEndpoint is issued by the control authority from the node's applied
// configuration. Address is a UDP address, not an HTTP URL or a host input.
type QUICEndpoint struct {
	Address        string `json:"address"`
	ServerName     string `json:"serverName"`
	SPKI           Digest `json:"spki"`
	CertificatePEM string `json:"certificatePem"`
}

func (e QUICEndpoint) Validate() error {
	host, port, err := net.SplitHostPort(e.Address)
	n, parseErr := strconv.ParseUint(port, 10, 16)
	ip := net.ParseIP(host)
	if err != nil || parseErr != nil || n == 0 || strconv.FormatUint(n, 10) != port || e.Address != net.JoinHostPort(host, port) ||
		(ip == nil && !dnsName(host)) || (ip != nil && (ip.IsUnspecified() || ip.IsMulticast())) ||
		!dnsName(e.ServerName) || net.ParseIP(e.ServerName) != nil || e.SPKI.Validate() != nil {
		return Error{Code: InvalidConfig}
	}
	if _, err := e.leaf(); err != nil {
		return err
	}
	return nil
}

func (e QUICEndpoint) leaf() (*x509.Certificate, error) {
	bad := Error{Code: InvalidConfig}
	if len(e.CertificatePEM) > 8192 || !allocationPEM(e.CertificatePEM, "CERTIFICATE", false) {
		return nil, bad
	}
	block, _ := pem.Decode([]byte(e.CertificatePEM))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil || cert.IsCA || cert.VerifyHostname(e.ServerName) != nil {
		return nil, bad
	}
	spki := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	if e.SPKI != Digest(hex.EncodeToString(spki[:])) {
		return nil, bad
	}
	return cert, nil
}

// VerifyCertificate verifies the authenticated assignment pin and DNS name in
// addition to the caller's normal TLS verification policy.
func (e QUICEndpoint) VerifyCertificate(cert *x509.Certificate, now time.Time) error {
	leaf, err := e.leaf()
	if err != nil || cert == nil || !cert.Equal(leaf) || now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
		return Error{Code: Unauthenticated}
	}
	return nil
}

// QUICConfig provisions an independent server key and pins its public leaf. DisableNewGrants stops new QUIC
// attempts without invalidating a healthy committed route or WSS recovery.
type QUICConfig struct {
	ListenAddress    string       `json:"listenAddress"`
	Endpoint         QUICEndpoint `json:"endpoint"`
	TLSSecret        SecretRef    `json:"tlsSecret"`
	DisableNewGrants bool         `json:"disableNewGrants,omitempty"`
}

func (c QUICConfig) validate() error {
	host, port, err := net.SplitHostPort(c.ListenAddress)
	n, parseErr := strconv.ParseUint(port, 10, 16)
	if err != nil || parseErr != nil || n == 0 || strconv.FormatUint(n, 10) != port || net.ParseIP(host) == nil {
		return Error{Code: InvalidConfig}
	}
	if c.TLSSecret.Purpose != SecretTLS || c.TLSSecret.Validate() != nil {
		return Error{Code: InvalidConfig}
	}
	return c.Endpoint.Validate()
}

// TransportGrant is present on every registration created through selection.
// Deadline fences promotion, not the lifetime of a subsequently committed
// route. These fields never change on renewal and are included in FRPClaims.
type TransportGrant struct {
	Protocol     FRPTransport  `json:"protocol"`
	OperationID  Token         `json:"operationId"`
	RouteVersion Counter       `json:"routeVersion"`
	Deadline     int64         `json:"deadline"`
	Endpoint     *QUICEndpoint `json:"endpoint,omitempty"`
}

func (g *TransportGrant) Equal(other *TransportGrant) bool {
	if g == nil || other == nil {
		return g == other
	}
	return g.Protocol == other.Protocol && g.OperationID == other.OperationID && g.RouteVersion == other.RouteVersion &&
		g.Deadline == other.Deadline && (g.Endpoint == nil && other.Endpoint == nil || g.Endpoint != nil && other.Endpoint != nil && *g.Endpoint == *other.Endpoint)
}

func (g TransportGrant) Validate() error {
	if !g.Protocol.Valid() || g.OperationID.Validate() != nil || g.RouteVersion == 0 || g.Deadline <= 0 || g.Deadline > 253402300799 ||
		(g.Endpoint != nil) != (g.Protocol == FRPTransportQUIC) {
		return Error{Code: Invalid}
	}
	if g.Endpoint != nil {
		return g.Endpoint.Validate()
	}
	return nil
}

type TransportAction string

const (
	TransportStateAction TransportAction = "state"
	TransportPrepare     TransportAction = "prepare"
	TransportRenew       TransportAction = "renew"
	TransportConfirm     TransportAction = "confirm"
	TransportCommit      TransportAction = "commit"
	TransportCancel      TransportAction = "cancel"
	TransportQuery       TransportAction = "query"
	TransportRelease     TransportAction = "release"
)

type TransportRequest struct {
	Action            TransportAction    `json:"action"`
	OperationID       Token              `json:"operationId,omitempty"`
	RouteVersion      Counter            `json:"routeVersion,omitempty"`
	Transport         FRPTransport       `json:"transport,omitempty"`
	AllocationRequest *AllocationRequest `json:"allocationRequest,omitempty"`
	AllocationRenew   *AllocationRenew   `json:"allocationRenew,omitempty"`
}

func (r TransportRequest) Validate(private bool) error {
	bad := Error{Code: Invalid}
	if r.Action == TransportStateAction {
		if r.OperationID == "" && r.RouteVersion == 0 && r.Transport == "" && r.AllocationRequest == nil && r.AllocationRenew == nil {
			return nil
		}
		return bad
	}
	if r.RouteVersion == 0 || (r.Action != TransportRelease || r.OperationID != "") && r.OperationID.Validate() != nil {
		return bad
	}
	if r.Action == TransportPrepare {
		if !r.Transport.Valid() || r.AllocationRequest == nil || r.AllocationRenew != nil {
			return bad
		}
		return r.AllocationRequest.validate(private)
	}
	if r.Transport != "" || r.AllocationRequest != nil {
		return bad
	}
	switch r.Action {
	case TransportCancel, TransportQuery:
		if r.AllocationRenew == nil {
			return nil
		}
	case TransportConfirm, TransportRelease:
		if !private {
			return bad
		}
		if r.Action == TransportRelease && r.OperationID != "" && r.AllocationRenew == nil {
			return nil
		}
		fallthrough
	case TransportRenew, TransportCommit:
		if r.AllocationRenew != nil {
			return validateAllocationRef(r.AllocationRenew.AllocationID, r.AllocationRenew.GrantRevision)
		}
	}
	return bad
}

type TransportState string

const (
	TransportIdle      TransportState = "idle"
	TransportPrepared  TransportState = "prepared"
	TransportCommitted TransportState = "committed"
	TransportCancelled TransportState = "cancelled"
	TransportExpired   TransportState = "expired"
	TransportReleased  TransportState = "released"
)

// TransportResult reports the current route and, when an operation ID is
// present, the durable outcome of that operation. A query never issues a grant.
type TransportResult struct {
	OperationID        Token            `json:"operationId,omitempty"`
	RouteVersion       Counter          `json:"routeVersion"`
	Active             FRPTransport     `json:"active,omitempty"`
	ActiveAllocationID Token            `json:"activeAllocationId,omitempty"`
	State              TransportState   `json:"state"`
	Grant              *AllocationGrant `json:"grant,omitempty"`
}

func (r TransportResult) validate(now time.Time, private bool) error {
	bad := Error{Code: Invalid}
	if r.RouteVersion == 0 || (r.Active == "") != (r.ActiveAllocationID == "") || r.Active != "" && (!r.Active.Valid() || r.ActiveAllocationID.Validate() != nil) {
		return bad
	}
	switch r.State {
	case TransportIdle:
		if r.OperationID != "" {
			return bad
		}
	case TransportReleased:
		if r.OperationID != "" && r.OperationID.Validate() != nil {
			return bad
		}
	case TransportPrepared, TransportCommitted, TransportCancelled, TransportExpired:
		if r.OperationID.Validate() != nil {
			return bad
		}
	default:
		return bad
	}
	if r.Grant == nil {
		return nil
	}
	g := r.Grant
	if (r.State != TransportPrepared && r.State != TransportCommitted) || g.Transport == nil || g.Transport.OperationID != r.OperationID {
		return bad
	}
	if err := g.validate(now, private); err != nil {
		return err
	}
	if r.State == TransportPrepared && (g.Transport.RouteVersion != r.RouteVersion || g.ExpiresAt > g.Transport.Deadline || g.Transport.Deadline > now.Add(TransportCandidateTTL+5*time.Second).Unix()) ||
		r.State == TransportCommitted && (g.AllocationID != r.ActiveAllocationID || g.Protocol() != r.Active) {
		return bad
	}
	return nil
}
