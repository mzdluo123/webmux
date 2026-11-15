package relay

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"time"
)

type SecretPurpose string

const (
	SecretTLS  SecretPurpose = "service-tls"
	SecretTURN SecretPurpose = "turn-auth"
)

type SecretRef struct {
	Purpose SecretPurpose `json:"purpose"`
	KeyID   string        `json:"keyId"`
	Version Counter       `json:"version"`
}

func (s SecretRef) Validate() error {
	if s.Purpose != SecretTLS && s.Purpose != SecretTURN || !identifier(s.KeyID, 64) || s.Version == 0 {
		return Error{Code: InvalidConfig}
	}
	return nil
}

// NodeIdentity is installed by an administrator BEFORE the first connection.
// Revision increases for every explicit pin/enable rotation, independently of
// config. Trust may hold current and next pins; nodes cannot modify this record.
type NodeIdentity struct {
	NodeID   NodeID       `json:"nodeId"`
	Revision Counter      `json:"revision"`
	Enabled  bool         `json:"enabled"`
	Trust    ServiceTrust `json:"trust"`
}

func (i NodeIdentity) Validate() error {
	if err := i.NodeID.Validate(); err != nil {
		return err
	}
	if i.Revision == 0 {
		return Error{Code: InvalidConfig}
	}
	return i.Trust.Validate()
}

// VerifyNodeCertificate requires prior normal client certificate chain/time and
// clientAuth EKU verification by mTLS. URI SAN must exactly identify this node;
// a valid chain or matching node-supplied nodeId alone is insufficient.
func VerifyNodeCertificate(cert *x509.Certificate, identity NodeIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	if !identity.Enabled {
		return Error{Code: Forbidden}
	}
	if err := VerifyCertificatePin(cert, identity.Trust); err != nil {
		return err
	}
	want := "spiffe://webmux-relay/node/" + string(identity.NodeID)
	if len(cert.URIs) != 1 || cert.URIs[0].String() != want {
		return Error{Code: Forbidden}
	}
	return nil
}

// NodeBootstrap is non-secret configuration. IdentityPrivateKeyFile names the
// deployment-injected identity; neither its contents nor runtime service secrets
// belong in bootstrap configuration or persisted diagnostics.
type NodeBootstrap struct {
	ControlEndpoint         string       `json:"controlEndpoint"`
	ControlAudience         string       `json:"controlAudience"`
	NodeID                  NodeID       `json:"nodeId"`
	IdentityPrivateKeyFile  string       `json:"identityPrivateKeyFile"`
	IdentityCertificateFile string       `json:"identityCertificateFile"`
	InitialTrust            ServiceTrust `json:"initialTrust"`
}

func (b NodeBootstrap) Validate() error {
	if _, err := secureURL(b.ControlEndpoint, "https", true); err != nil {
		return err
	}
	if !audience(b.ControlAudience) || b.IdentityPrivateKeyFile == "" || b.IdentityCertificateFile == "" {
		return Error{Code: InvalidConfig}
	}
	if err := b.NodeID.Validate(); err != nil {
		return err
	}
	return b.InitialTrust.Validate()
}

type Limits struct {
	MaxStreamsPerHost        uint32 `json:"maxStreamsPerHost"`
	MaxStreamsPerAccount     uint32 `json:"maxStreamsPerAccount"`
	ConnectionBufferBytes    uint32 `json:"connectionBufferBytes"`
	StreamBufferBytes        uint32 `json:"streamBufferBytes"`
	AccountBytesPerSecond    uint64 `json:"accountBytesPerSecond,string"`
	AccountTURNAllocations   uint32 `json:"accountTurnAllocations"`
	TURNSessionMaxBPS        uint64 `json:"turnSessionMaxBps,string"`
	NodeTURNAllocations      uint32 `json:"nodeTurnAllocations"`
	TURNNodeCapacityBPS      uint64 `json:"turnNodeCapacityBps,string"`
	TURNMaxAllocationSeconds uint32 `json:"turnMaxAllocationSeconds"`
	ICETTLSeconds            uint32 `json:"iceTtlSeconds"`
}

func (l Limits) Validate(c Capabilities) error {
	if c.Has(CapabilityWSS) && (l.MaxStreamsPerHost == 0 || l.MaxStreamsPerAccount < l.MaxStreamsPerHost || l.ConnectionBufferBytes == 0 || l.StreamBufferBytes == 0 || l.StreamBufferBytes > l.ConnectionBufferBytes || l.AccountBytesPerSecond == 0) {
		return Error{Code: InvalidConfig}
	}
	if c.Has(CapabilityTURN) && (l.AccountTURNAllocations == 0 || l.NodeTURNAllocations < l.AccountTURNAllocations || l.TURNSessionMaxBPS == 0 || l.TURNNodeCapacityBPS < l.TURNSessionMaxBPS || l.TURNMaxAllocationSeconds == 0 || l.ICETTLSeconds < 2 || l.ICETTLSeconds > uint32(MaxICETTL/time.Second)) {
		return Error{Code: InvalidConfig}
	}
	return nil
}

type WSSConfig struct {
	ListenAddress    string       `json:"listenAddress"`
	ConnectorURL     string       `json:"connectorUrl"`
	PublicHostSuffix string       `json:"publicHostSuffix"`
	Audience         string       `json:"audience"`
	Trust            ServiceTrust `json:"trust"`
	TLSSecret        SecretRef    `json:"tlsSecret"`
}

type TURNConfig struct {
	ListenIP           string     `json:"listenIp"`
	UDPPort            uint16     `json:"udpPort"`
	TCPPort            uint16     `json:"tcpPort"`
	TLSPort            uint16     `json:"tlsPort"`
	RelayMinPort       uint16     `json:"relayMinPort"`
	RelayMaxPort       uint16     `json:"relayMaxPort"`
	AdvertisedPublicIP string     `json:"advertisedPublicIp"`
	MappedLocalIP      string     `json:"mappedLocalIp,omitempty"`
	URLs               []string   `json:"urls"`
	Realm              string     `json:"realm"`
	AuthSecret         SecretRef  `json:"authSecret"`
	TLSSecret          *SecretRef `json:"tlsSecret,omitempty"`
}

type NodeConfig struct {
	Capabilities Capabilities `json:"capabilities"`
	WSS          *WSSConfig   `json:"wss,omitempty"`
	TURN         *TURNConfig  `json:"turn,omitempty"`
	Limits       Limits       `json:"limits"`
}

func (c NodeConfig) Validate() error {
	if err := c.Capabilities.Validate(); err != nil {
		return err
	}
	if c.Capabilities.Has(CapabilityWSS) != (c.WSS != nil) || c.Capabilities.Has(CapabilityTURN) != (c.TURN != nil) {
		return Error{Code: InvalidConfig}
	}
	if w := c.WSS; w != nil {
		host, port, err := net.SplitHostPort(w.ListenAddress)
		n, parseErr := strconv.ParseUint(port, 10, 16)
		if err != nil || parseErr != nil || n == 0 || net.ParseIP(host) == nil {
			return Error{Code: InvalidConfig}
		}
		u, err := secureURL(w.ConnectorURL, "wss", false)
		if err != nil || u.Path != PathConnector || !dnsName(w.PublicHostSuffix) || len(w.PublicHostSuffix) > 218 || !audience(w.Audience) || w.TLSSecret.Purpose != SecretTLS {
			return Error{Code: InvalidConfig}
		}
		if err := w.Trust.Validate(); err != nil {
			return err
		}
		if err := w.TLSSecret.Validate(); err != nil {
			return err
		}
	}
	if t := c.TURN; t != nil {
		if net.ParseIP(t.ListenIP) == nil || net.ParseIP(t.AdvertisedPublicIP) == nil || t.MappedLocalIP != "" && net.ParseIP(t.MappedLocalIP) == nil || t.UDPPort == 0 && t.TCPPort == 0 && t.TLSPort == 0 || t.RelayMinPort == 0 || t.RelayMaxPort < t.RelayMinPort || !dnsName(t.Realm) || t.AuthSecret.Purpose != SecretTURN {
			return Error{Code: InvalidConfig}
		}
		if err := validateTURNURLs(t.URLs); err != nil {
			return err
		}
		if err := t.AuthSecret.Validate(); err != nil {
			return err
		}
		if (t.TLSPort != 0) != (t.TLSSecret != nil) {
			return Error{Code: InvalidConfig}
		}
		if t.TLSSecret != nil {
			if t.TLSSecret.Purpose != SecretTLS {
				return Error{Code: InvalidConfig}
			}
			if err := t.TLSSecret.Validate(); err != nil {
				return err
			}
		}
		for _, raw := range t.URLs {
			u, err := url.Parse(raw)
			if err != nil {
				return Error{Code: InvalidConfig}
			}
			if u.Scheme == "turns" && t.TLSPort == 0 || u.Scheme == "turn" && u.RawQuery == "transport=udp" && t.UDPPort == 0 || u.Scheme == "turn" && u.RawQuery == "transport=tcp" && t.TCPPort == 0 {
				return Error{Code: InvalidConfig}
			}
		}
	}
	return c.Limits.Validate(c.Capabilities)
}

// ConfigPayload's declared field order and encoding/json encoding are the v1
// canonical digest layout. No maps, raw JSON, floats or insignificant fields.
type ConfigPayload struct {
	NodeID   NodeID     `json:"nodeId"`
	Revision Counter    `json:"revision"`
	Config   NodeConfig `json:"config"`
}

type ConfigSnapshot struct {
	Payload ConfigPayload `json:"payload"`
	Digest  Digest        `json:"digest"`
}

func ConfigDigest(p ConfigPayload) (Digest, error) {
	if err := p.NodeID.Validate(); err != nil {
		return "", err
	}
	if p.Revision == 0 {
		return "", Error{Code: InvalidConfig}
	}
	if err := p.Config.Validate(); err != nil {
		return "", err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", Error{Code: InvalidConfig}
	}
	if len(b) > MaxSnapshotBytes {
		return "", Error{Code: InvalidConfig}
	}
	d := sha256.Sum256(b)
	return Digest(hex.EncodeToString(d[:])), nil
}

// Validate rejects rollback and same-version substitution. The caller persists
// highestSeen/digest only after validation, independently of applied revision.
func (s ConfigSnapshot) Validate(node NodeID, highestSeen Counter, seenDigest Digest) error {
	if s.Payload.NodeID != node {
		return Error{Code: Forbidden}
	}
	d, err := ConfigDigest(s.Payload)
	if err != nil {
		return err
	}
	if s.Digest != d {
		return Error{Code: InvalidConfig}
	}
	if s.Payload.Revision < highestSeen || s.Payload.Revision == highestSeen && s.Digest != seenDigest {
		return Error{Code: Conflict}
	}
	return nil
}

type Component string

const (
	ComponentRelay  Component = "relay"
	ComponentCoturn Component = "coturn"
)

type RestartApproval struct {
	NodeID     NodeID      `json:"nodeId"`
	Revision   Counter     `json:"revision"`
	Digest     Digest      `json:"digest"`
	Components []Component `json:"components"`
}

type ComponentStatus struct {
	Component       Component   `json:"component"`
	AppliedRevision Counter     `json:"appliedRevision"`
	Ready           bool        `json:"ready"`
	ActiveSecrets   []SecretRef `json:"activeSecrets"`
	Error           Code        `json:"error,omitempty"`
}

type ConfigAck struct {
	NodeID          NodeID            `json:"nodeId"`
	DesiredRevision Counter           `json:"desiredRevision"`
	DesiredDigest   Digest            `json:"desiredDigest"`
	AppliedRevision Counter           `json:"appliedRevision"`
	AppliedDigest   Digest            `json:"appliedDigest,omitempty"`
	State           string            `json:"state"`
	Components      []ComponentStatus `json:"components"`
	Error           Code              `json:"error,omitempty"`
}

// SecretRequest/SecretDelivery use a separate node-mTLS HTTP endpoint, never the
// public connector, configuration snapshot or host control stream.
type SecretRequest struct {
	Version        Version   `json:"version"`
	NodeID         NodeID    `json:"nodeId"`
	ControlEpoch   Token     `json:"controlEpoch"`
	NodeEpoch      Token     `json:"nodeEpoch"`
	ConfigRevision Counter   `json:"configRevision"`
	Ref            SecretRef `json:"ref"`
}

type SecretDelivery struct {
	Version        Version   `json:"version"`
	NodeID         NodeID    `json:"nodeId"`
	ControlEpoch   Token     `json:"controlEpoch"`
	NodeEpoch      Token     `json:"nodeEpoch"`
	ConfigRevision Counter   `json:"configRevision"`
	Ref            SecretRef `json:"ref"`
	// TURN uses Value (canonical base64url bytes); TLS uses certificate/key PEM.
	Value          string `json:"value,omitempty"`
	CertificatePEM string `json:"certificatePem,omitempty"`
	PrivateKeyPEM  string `json:"privateKeyPem,omitempty"`
}

// ValidateForBinding enforces persistent full-origin continuity on rediscovery.
// Endpoint/IP/trust changes are allowed; a different public origin is not a v1
// migration. Rebinding a key must preserve the existing Binding.PublicURL too.
func (a Assignment) ValidateForBinding(b Binding, now time.Time) error {
	if err := a.Validate(now); err != nil {
		return err
	}
	if a.Binding != b {
		return Error{Code: Conflict}
	}
	return nil
}

func PublicOrigin(host HostID, suffix string) (string, error) {
	if err := host.Validate(); err != nil {
		return "", err
	}
	if !dnsName(suffix) || len(suffix) > 218 {
		return "", Error{Code: InvalidConfig}
	}
	return "https://" + host.Label() + "." + suffix, nil
}

// SecretAssociatedData binds encrypted storage to node, purpose, key and exact
// secret version. This is an encoding helper, not an encryption implementation.
func SecretAssociatedData(node NodeID, ref SecretRef) ([]byte, error) {
	if err := node.Validate(); err != nil {
		return nil, err
	}
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	return []byte("WebMux-Relay-v1/secret\n" + string(node) + "\n" + string(ref.Purpose) + "\n" + ref.KeyID + "\n" + strconv.FormatUint(uint64(ref.Version), 10)), nil
}
