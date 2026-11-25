package relay

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

var vectorNow = time.Unix(1800000000, 0)

const vectorToken Token = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const vectorHost HostID = "00112233445566778899aabbccddeeff"

func vectorChallenge() ConnectorChallenge {
	return ConnectorChallenge{Version: V1, Audience: "relay-test", HostID: vectorHost,
		BindingGeneration: 7, ConnectorGeneration: 42, Role: RoleControl,
		Nonce: vectorToken, ExpiresAt: vectorNow.Unix() + 30, ControlEpoch: vectorToken, NodeEpoch: vectorToken}
}

func vectorKey(t *testing.T) (ed25519.PrivateKey, PublicKey) {
	t.Helper()
	// RFC 8032 section 7.1 test 1 seed/public key. Signature is over OUR
	// canonical transcript, not the RFC's empty-message signature.
	seed, err := hex.DecodeString("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60")
	if err != nil {
		t.Fatal(err)
	}
	key := ed25519.NewKeyFromSeed(seed)
	if hex.EncodeToString(key.Public().(ed25519.PublicKey)) != "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a" {
		t.Fatal("public key vector mismatch")
	}
	return key, PublicKey{Algorithm: HostAlgorithm, Key: base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey))}
}

func requireCode(t *testing.T, err error, code Code) {
	t.Helper()
	var e Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("got %v, want code %s", err, code)
	}
}

func TestCanonicalConnectorVector(t *testing.T) {
	c := vectorChallenge()
	b, err := ConnectorTranscript(c)
	if err != nil {
		t.Fatal(err)
	}
	want := "WebMux-Relay-v1\nrelay-test\n00112233445566778899aabbccddeeff\n7\n42\ncontrol\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if string(b) != want {
		t.Fatalf("wrong transcript: %q", b)
	}
	key, pub := vectorKey(t)
	p, err := SignConnector(key, c, vectorNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyConnector(pub, c, p, vectorNow); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*ConnectorChallenge)
		want   Code
	}{
		{"audience", func(c *ConnectorChallenge) { c.Audience = "other-relay" }, InvalidSignature},
		{"host", func(c *ConnectorChallenge) { c.HostID = "00112233445566778899aabbccddeefe" }, InvalidSignature},
		{"binding", func(c *ConnectorChallenge) { c.BindingGeneration++ }, InvalidSignature},
		{"connector", func(c *ConnectorChallenge) { c.ConnectorGeneration++ }, InvalidSignature},
		{"role", func(c *ConnectorChallenge) { c.Role = "bulk" }, Invalid},
		{"nonce", func(c *ConnectorChallenge) {
			c.Nonce = Token(base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
		}, Unauthenticated},
		{"version", func(c *ConnectorChallenge) { c.Version = 2 }, Incompatible},
		{"expired", func(c *ConnectorChallenge) { c.ExpiresAt = vectorNow.Unix() }, Expired},
		{"newline", func(c *ConnectorChallenge) { c.Audience = "relay\ninjected" }, Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := c
			tt.mutate(&changed)
			requireCode(t, VerifyConnector(pub, changed, p, vectorNow), tt.want)
		})
	}
	wrong := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, 32))
	bad, err := SignConnector(wrong, c, vectorNow)
	if err != nil {
		t.Fatal(err)
	}
	requireCode(t, VerifyConnector(pub, c, bad, vectorNow), InvalidSignature)
	p.Signature += "="
	requireCode(t, VerifyConnector(pub, c, p, vectorNow), InvalidSignature)
}

func TestHostPurposeDomainSeparation(t *testing.T) {
	key, pub := vectorKey(t)
	c := HostChallenge{Version: V1, Audience: "control-test", HostID: vectorHost, BindingGeneration: 7, Purpose: PurposeAssignment, Nonce: vectorToken, ExpiresAt: vectorNow.Unix() + 30, ControlEpoch: vectorToken}
	b, err := HostTranscript(c)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "WebMux-Relay-v1/assignment\ncontrol-test\n00112233445566778899aabbccddeeff\n7\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
		t.Fatalf("wrong host transcript %q", b)
	}
	p, err := SignHost(key, c, vectorNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyHost(pub, c, p, vectorNow); err != nil {
		t.Fatal(err)
	}
	c.Purpose = PurposeDelete
	requireCode(t, VerifyHost(pub, c, p, vectorNow), InvalidSignature)
}

func TestChallengeClockSkew(t *testing.T) {
	key, pub := vectorKey(t)
	receiverNow := vectorNow.Add(-3 * time.Second)
	connector := vectorChallenge()
	proof, err := SignConnector(key, connector, receiverNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyConnector(pub, connector, proof, vectorNow); err != nil {
		t.Fatal(err)
	}
	host := HostChallenge{Version: V1, Audience: "control-test", HostID: vectorHost, BindingGeneration: 7, Purpose: PurposeAssignment, Nonce: vectorToken, ExpiresAt: vectorNow.Add(NonceTTL).Unix(), ControlEpoch: vectorToken}
	hostProof, err := SignHost(key, host, receiverNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyHost(pub, host, hostProof, vectorNow); err != nil {
		t.Fatal(err)
	}
	expires := time.Unix(host.ExpiresAt, 0)
	requireCode(t, VerifyHost(pub, host, hostProof, expires), Expired)
	requireCode(t, VerifyConnector(pub, connector, proof, expires), Expired)
}

func TestStrictJSONAndCounter(t *testing.T) {
	tests := []string{
		`{"version":1,"version":2,"handle":"` + string(vectorToken) + `"}`,
		`{"version":1,"handle":"` + string(vectorToken) + `","token":"sensitive"}`,
		`{"version":1,"handle":"` + string(vectorToken) + `"} {}`,
	}
	for _, input := range tests {
		var p EnrollmentPoll
		requireCode(t, Decode([]byte(input), &p, MaxAuthBytes), Invalid)
	}
	var c Counter
	for _, input := range []string{`1`, `"01"`, `"+1"`, `"-1"`, `"18446744073709551616"`} {
		if json.Unmarshal([]byte(input), &c) == nil {
			t.Fatalf("accepted counter %s", input)
		}
	}
	if err := json.Unmarshal([]byte(`"18446744073709551615"`), &c); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(c)
	if err != nil || string(b) != `"18446744073709551615"` {
		t.Fatalf("counter lost precision %s: %v", b, err)
	}
	var p EnrollmentPoll
	requireCode(t, Decode(bytes.Repeat([]byte{' '}, MaxAuthBytes+1), &p, MaxAuthBytes), Invalid)
	for _, token := range []Token{vectorToken + "=", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB", "short"} {
		requireCode(t, token.Validate(), Invalid)
	}
}

func testConfig() ConfigPayload {
	return ConfigPayload{NodeID: "test-node", Revision: 1, Config: NodeConfig{
		Capabilities: Capabilities{CapabilityWSS},
		WSS:          &WSSConfig{ListenAddress: "127.0.0.1:443", ConnectorURL: "wss://connector.example.test/v1/connector", PublicHostSuffix: "hosts.example.test", Audience: "node-test", Trust: ServiceTrust{Algorithm: ServiceAlgorithm, Pins: []string{string(vectorToken)}}, TLSSecret: SecretRef{Purpose: SecretTLS, KeyID: "tls", Version: 1}},
		Limits:       Limits{MaxStreamsPerHost: 8, MaxStreamsPerAccount: 8, ConnectionBufferBytes: 4194304, StreamBufferBytes: 1048576, AccountBytesPerSecond: 1048576},
	}}
}

func TestNodeConfigReservesPrivateFRPPorts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*NodeConfig)
		fails  bool
	}{
		{"WSS range minimum", func(c *NodeConfig) { c.WSS.ListenAddress = "127.0.0.1:30000" }, true},
		{"WSS range maximum", func(c *NodeConfig) { c.WSS.ListenAddress = "[::1]:39999" }, true},
		{"WSS outside range", func(c *NodeConfig) { c.WSS.ListenAddress = "127.0.0.1:40000" }, false},
		{"TURN TCP", func(c *NodeConfig) { c.TURN.TCPPort = FRPPortMin }, true},
		{"TURN TLS", func(c *NodeConfig) { c.TURN.TLSPort = FRPPortMax }, true},
		{"TURN UDP", func(c *NodeConfig) { c.TURN.UDPPort = FRPPortMin }, false},
		{"TURN range contains FRP range", func(c *NodeConfig) { c.TURN.RelayMinPort, c.TURN.RelayMaxPort = FRPPortMin-1, FRPPortMax+1 }, true},
		{"TURN range shares lower boundary", func(c *NodeConfig) { c.TURN.RelayMinPort, c.TURN.RelayMaxPort = FRPPortMin-5, FRPPortMin }, true},
		{"TURN range shares upper boundary", func(c *NodeConfig) { c.TURN.RelayMinPort, c.TURN.RelayMaxPort = FRPPortMax, FRPPortMax+5 }, true},
		{"TURN range outside", func(c *NodeConfig) { c.TURN.RelayMinPort, c.TURN.RelayMaxPort = FRPPortMax+1, FRPPortMax+5 }, false},
		{"TURN-only unchanged", func(c *NodeConfig) {
			c.Capabilities, c.WSS = Capabilities{CapabilityTURN}, nil
			c.TURN.TCPPort, c.TURN.TLSPort = FRPPortMin, FRPPortMin+1
			c.TURN.RelayMinPort, c.TURN.RelayMaxPort = FRPPortMin+2, FRPPortMax
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testConfig().Config
			c.Capabilities = Capabilities{CapabilityTURN, CapabilityWSS}
			c.TURN = &TURNConfig{
				ListenIP: "127.0.0.1", AdvertisedPublicIP: "203.0.113.1",
				UDPPort: 3478, TCPPort: 3478, TLSPort: 5349, RelayMinPort: 49152, RelayMaxPort: 49200,
				URLs: []string{"turn:turn.example.test:3478?transport=udp"}, Realm: "turn.example.test",
				AuthSecret: SecretRef{Purpose: SecretTURN, KeyID: "turn", Version: 1},
				TLSSecret:  &SecretRef{Purpose: SecretTLS, KeyID: "tls", Version: 1},
			}
			c.Limits.AccountTURNAllocations, c.Limits.NodeTURNAllocations = 2, 32
			c.Limits.TURNSessionMaxBPS, c.Limits.TURNNodeCapacityBPS = 1000, 32000
			c.Limits.TURNMaxAllocationSeconds, c.Limits.ICETTLSeconds = 60, 30
			tc.change(&c)
			err := c.Validate()
			if tc.fails {
				requireCode(t, err, InvalidConfig)
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConfigReplayAndActivation(t *testing.T) {
	p := testConfig()
	d, err := ConfigDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	s := ConfigSnapshot{Payload: p, Digest: d}
	if err := s.Validate(p.NodeID, 1, d); err != nil {
		t.Fatal(err)
	}
	requireCode(t, s.Validate("other-node", 1, d), Forbidden)
	requireCode(t, s.Validate(p.NodeID, 2, d), Conflict)
	changed := testConfig()
	changed.Config.Limits.MaxStreamsPerHost = 7
	changedDigest, err := ConfigDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	requireCode(t, (ConfigSnapshot{Payload: changed, Digest: changedDigest}).Validate(p.NodeID, 1, d), Conflict)
	ack := ConfigAck{NodeID: p.NodeID, DesiredRevision: 1, DesiredDigest: d, AppliedRevision: 1, AppliedDigest: d, State: "applied", Components: []ComponentStatus{{Component: ComponentRelay, AppliedRevision: 1, Ready: true, ActiveSecrets: []SecretRef{p.Config.WSS.TLSSecret}}}}
	if err := ack.Validate(s); err != nil {
		t.Fatal(err)
	}
	ack.Components[0].ActiveSecrets[0].Version++
	requireCode(t, ack.Validate(s), InvalidConfig)
	approval := RestartApproval{NodeID: p.NodeID, Revision: 1, Digest: d, Components: []Component{ComponentRelay}}
	if err := approval.Validate(s); err != nil {
		t.Fatal(err)
	}
	// A both-to-WSS change must approve stopping the removed coturn component.
	// The runtime checks the exact component set against the old actual config.
	approval.Components = []Component{ComponentCoturn}
	if err := approval.Validate(s); err != nil {
		t.Fatalf("cannot approve removing coturn: %v", err)
	}
	approval.Revision++
	requireCode(t, approval.Validate(s), Conflict)
	p.Config.Capabilities = Capabilities{CapabilityTURN}
	requireCode(t, p.Config.Validate(), InvalidConfig)
}

func TestOriginContinuityAndTURNOnlyRejection(t *testing.T) {
	b := Binding{HostID: vectorHost, OwnerID: "123", BindingGeneration: 1, PublicURL: "https://" + vectorHost.Label() + ".hosts.example.test"}
	a := Assignment{Version: V1, Revision: 1, ControlEpoch: vectorToken, ControlAudience: "control-test", Source: "authenticated-control", ExpiresAt: vectorNow.Unix() + 30, Binding: b, NodeID: "test-node", Capabilities: Capabilities{CapabilityWSS}, ConfigRevision: 1, ConnectorURL: "wss://connector.example.test/v1/connector", DataOrigin: "https://connector.example.test", NodeAudience: "node-test", Trust: ServiceTrust{Algorithm: ServiceAlgorithm, Pins: []string{string(vectorToken)}}}
	if err := a.ValidateForBinding(b, vectorNow); err != nil {
		t.Fatal(err)
	}
	a.Binding.PublicURL = "https://" + vectorHost.Label() + ".changed.example.test"
	requireCode(t, a.ValidateForBinding(b, vectorNow), Conflict)
	a.Binding = b
	a.Capabilities = Capabilities{CapabilityTURN}
	requireCode(t, a.Validate(vectorNow), Invalid)
}

func TestNodeCertificateMapping(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(spki)
	u, err := url.Parse("spiffe://webmux-relay/node/test-node")
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{PublicKey: &key.PublicKey, RawSubjectPublicKeyInfo: spki, URIs: []*url.URL{u}}
	identity := NodeIdentity{NodeID: "test-node", Revision: 1, Enabled: true, Trust: ServiceTrust{Algorithm: ServiceAlgorithm, Pins: []string{base64.RawURLEncoding.EncodeToString(h[:])}}}
	if err := VerifyNodeCertificate(cert, identity); err != nil {
		t.Fatal(err)
	}
	identity.NodeID = "other-node"
	requireCode(t, VerifyNodeCertificate(cert, identity), Forbidden)
	identity.NodeID = "test-node"
	identity.Trust.Pins = []string{string(vectorToken)}
	requireCode(t, VerifyNodeCertificate(cert, identity), Unauthenticated)
	cert.PublicKey = ed25519.NewKeyFromSeed(make([]byte, 32)).Public()
	requireCode(t, VerifyNodeCertificate(cert, identity), Unauthenticated)
}

func TestControlUnionAndBrowserAuthority(t *testing.T) {
	scope := Scope{HostID: vectorHost, BindingGeneration: 1, ConnectorGeneration: 2, ControlEpoch: vectorToken, NodeEpoch: vectorToken}
	m := ControlMessage{Version: V1, Type: ControlICERequest, RequestID: vectorToken, Scope: scope, ExpiresAt: vectorNow.Unix() + 10, ICERequest: &ICERequest{GrantID: vectorToken, PeerConnectionID: vectorToken}}
	if err := m.Validate(scope, vectorNow); err != nil {
		t.Fatal(err)
	}
	m.Grant = &BrowserGrant{}
	requireCode(t, m.Validate(scope, vectorNow), Invalid)
	m.Grant = nil
	m.ICERequest.GrantID = ""
	requireCode(t, m.Validate(scope, vectorNow), Invalid)
	m.ICERequest.GrantID = vectorToken
	old := scope
	old.ConnectorGeneration++
	requireCode(t, m.Validate(old, vectorNow), StaleGeneration)
	errText := (Error{Code: Code("sensitive-signature-value"), RequestID: string(vectorToken)}).Error()
	if strings.Contains(errText, "sensitive-signature-value") || strings.Contains(errText, string(vectorToken)) || errText != "relay: internal_error" {
		t.Fatal("error leaked untrusted material")
	}
}

func TestControlResumeUnionAndScope(t *testing.T) {
	scope := Scope{HostID: vectorHost, BindingGeneration: 1, ConnectorGeneration: 2, ControlEpoch: vectorToken, NodeEpoch: vectorToken}
	m := ControlMessage{Version: V1, Type: ControlResume, RequestID: vectorToken, Scope: scope, ExpiresAt: vectorNow.Add(time.Minute).Unix(), Resume: &LoginResume{GrantID: vectorToken}}
	if err := m.Validate(scope, vectorNow); err != nil {
		t.Fatal(err)
	}
	var decoded ControlMessage
	var wire bytes.Buffer
	if err := WriteFrame(&wire, m, MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	if err := ReadFrame(&wire, &decoded, MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != ControlResume || decoded.Resume == nil || *decoded.Resume != *m.Resume {
		t.Fatal("resume wire round trip lost payload")
	}
	m.Resume.GrantID = ""
	requireCode(t, m.Validate(scope, vectorNow), Invalid)
	m.Resume.GrantID = vectorToken
	m.Exchange = &LoginExchange{}
	requireCode(t, m.Validate(scope, vectorNow), Invalid)
	m.Exchange = nil
	m.Type = ControlICERequest
	requireCode(t, m.Validate(scope, vectorNow), Invalid)
	m.Type = ControlResume
	other := scope
	other.ConnectorGeneration++
	requireCode(t, m.Validate(other, vectorNow), StaleGeneration)
	requireCode(t, m.Validate(scope, vectorNow.Add(time.Minute)), Expired)
}

func TestControlMessageExpiry(t *testing.T) {
	scope := Scope{HostID: vectorHost, BindingGeneration: 1, ConnectorGeneration: 2, ControlEpoch: vectorToken, NodeEpoch: vectorToken}
	for _, kind := range []ControlType{ControlPing, ControlPong} {
		m := ControlMessage{Version: V1, Type: kind, RequestID: vectorToken, Scope: scope, ExpiresAt: vectorNow.Add(time.Minute).Unix()}
		for _, delay := range []time.Duration{0, 15 * time.Second, 59 * time.Second} {
			if err := m.Validate(scope, vectorNow.Add(delay)); err != nil {
				t.Fatalf("%s delayed by %s: %v", kind, delay, err)
			}
		}
		requireCode(t, m.Validate(scope, vectorNow.Add(time.Minute)), Expired)
		for _, lead := range []time.Duration{1370 * time.Millisecond, 5 * time.Second, time.Hour} {
			m.ExpiresAt = vectorNow.Add(ControlMessageTTL + lead).Unix()
			if err := m.Validate(scope, vectorNow); err != nil {
				t.Fatalf("%s sender clock ahead by %s: %v", kind, lead, err)
			}
		}
		m.ExpiresAt = vectorNow.Unix()
		requireCode(t, m.Validate(scope, vectorNow), Expired)
	}
}

func TestControlConnectorHello(t *testing.T) {
	for _, role := range []Role{"interactive", "bulk"} {
		hello := ConnectorHello{Version: V1, HostID: vectorHost, BindingGeneration: 1, ConnectorGeneration: 2, Role: role}
		requireCode(t, hello.Validate(), Invalid)
	}
	hello := ConnectorHello{Version: V1, HostID: vectorHost, BindingGeneration: 1, Role: RoleControl}
	if err := hello.Validate(); err != nil {
		t.Fatal(err)
	}
	hello.ConnectorGeneration = 2
	requireCode(t, hello.Validate(), Invalid)
}

func TestIndependentSignatureVector(t *testing.T) {
	key, _ := vectorKey(t)
	p, err := SignConnector(key, vectorChallenge(), vectorNow)
	if err != nil {
		t.Fatal(err)
	}
	// Generated independently with node:crypto (Ed25519 PKCS#8 seed).
	const want = "qiP-QeQU3CDl9mUhnH2Y-y2ORvH5o3qqbFKhUBf0LI13dAMYAP015YZtD2IwcbRf7HxRIB0oS3LD61ovQhZeAw"
	if p.Signature != want {
		t.Fatalf("signature vector mismatch: %s", p.Signature)
	}
}

func TestStockCoturnAccountGrouping(t *testing.T) {
	one, err := TURNUsername(vectorNow.Unix()+59, vectorToken)
	if err != nil {
		t.Fatal(err)
	}
	two, err := TURNUsername(vectorNow.Unix()+60, vectorToken)
	if err != nil {
		t.Fatal(err)
	}
	expiry, account, err := ParseTURNUsername(one)
	if err != nil || expiry != vectorNow.Unix()+59 || account != vectorToken {
		t.Fatalf("username lost account grouping: %v", err)
	}
	_, otherAccount, err := ParseTURNUsername(two)
	if err != nil || account != otherAccount || one == two {
		t.Fatal("roles must differ only in reserved expiry")
	}
	for _, suffix := range []string{":browser", ":host", ":random"} {
		_, _, err := ParseTURNUsername(one + suffix)
		requireCode(t, err, Invalid)
	}
	config := ICEConfig{NodeID: "turn-node", ConfigRevision: 1, Secret: SecretRef{Purpose: SecretTURN, KeyID: "turn", Version: 1}, GrantID: vectorToken, PeerConnectionID: vectorToken, ExpiresAt: vectorNow.Unix() + 59,
		Browser: ICEEndpoint{Role: "browser", ExpiresAt: vectorNow.Unix() + 59, Servers: []ICEServer{{URLs: []string{"turn:turn.example.test:3478?transport=udp"}, Username: one, Credential: "browser-password"}}},
		Host:    ICEEndpoint{Role: "host", ExpiresAt: vectorNow.Unix() + 60, Servers: []ICEServer{{URLs: []string{"turn:turn.example.test:3478?transport=udp"}, Username: two, Credential: "host-password"}}},
	}
	if err := config.Validate(vectorNow); err != nil {
		t.Fatal(err)
	}
	config.Host.Servers[0].Username = one
	requireCode(t, config.Validate(vectorNow), Invalid)
}

func TestNodeHelloUsesAuthenticatedIdentity(t *testing.T) {
	message := NodeMessage{
		Version: V1, Type: NodeHelloType, RequestID: vectorToken,
		NodeID: "node-a", Audience: "webmux-control", NodeEpoch: vectorToken,
		Sequence: 1, ExpiresAt: vectorNow.Add(RequestTimeout).Unix(),
		Hello: &NodeHello{},
	}
	data, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	var decoded NodeMessage
	if err := Decode(data, &decoded, MaxSnapshotBytes); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate("node-a", "webmux-control", "", vectorToken, vectorNow); err != nil {
		t.Fatalf("first handshake requires undeployed identity metadata: %v", err)
	}
	requireCode(t, decoded.Validate("node-b", "webmux-control", "", vectorToken, vectorNow), Forbidden)
	decoded.Hello = nil
	requireCode(t, decoded.Validate("node-a", "webmux-control", "", vectorToken, vectorNow), Invalid)
}

func TestManagementRejectsRetiredWireRoles(t *testing.T) {
	scope := Scope{HostID: vectorHost, BindingGeneration: 1, ConnectorGeneration: 2, ControlEpoch: vectorToken, NodeEpoch: vectorToken}
	header := StreamHeader{Version: V1, Kind: "http", RequestID: vectorToken, Scope: scope, Role: RoleControl}
	requireCode(t, header.Validate(scope, RoleControl), Invalid)
	for _, role := range []Role{"interactive", "bulk"} {
		header.Kind, header.Role = StreamControl, role
		requireCode(t, header.Validate(scope, role), Invalid)
		requireCode(t, header.Validate(scope, RoleControl), Invalid)
	}
	legacy := ControlMessage{Version: V1, Type: "transport.grow", RequestID: vectorToken, Scope: scope, ExpiresAt: vectorNow.Add(ControlMessageTTL).Unix()}
	requireCode(t, legacy.Validate(scope, vectorNow), Invalid)
	if ConnectorSubprotocol != "webmux-relay.v2" {
		t.Fatal("management WSS subprotocol changed")
	}
}
