package relay

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const (
	fixtureScopeJSON = `{"hostId":"` + string(vectorHost) + `","bindingGeneration":"1","connectorGeneration":"2","controlEpoch":"` + string(vectorToken) + `","nodeEpoch":"` + string(vectorToken) + `"}`
	fixtureCSR       = "-----BEGIN CERTIFICATE REQUEST-----\nYQ==\n-----END CERTIFICATE REQUEST-----\n"
	fixtureCert      = "-----BEGIN CERTIFICATE-----\nYQ==\n-----END CERTIFICATE-----\n"
	fixtureSPKI      = Digest("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	fixtureFRPURL    = "wss://connector.example.test/v1/frp/" + string(vectorToken)
)

var fixtureScope = Scope{HostID: vectorHost, BindingGeneration: 1, ConnectorGeneration: 2, ControlEpoch: vectorToken, NodeEpoch: vectorToken}

func fixtureControl(kind ControlType) ControlMessage {
	return ControlMessage{Version: V1, Type: kind, RequestID: vectorToken, Scope: fixtureScope, ExpiresAt: vectorNow.Add(ControlMessageTTL).Unix()}
}

func fixtureControlJSON(kind ControlType) string {
	return `{"version":1,"type":"` + string(kind) + `","requestId":"` + string(vectorToken) + `","scope":` + fixtureScopeJSON + `,"expiresAt":1800000060`
}

func fixtureAllocationGrant() AllocationGrant {
	return AllocationGrant{
		AllocationID: vectorToken, GrantRevision: 1, ProxyName: "frp-proxy", RemotePort: 20123,
		TransportURL: fixtureFRPURL, BridgeKey: vectorToken, JWT: "aGVhZGVy.Y2xhaW1z.c2ln",
		IssuedAt: vectorNow.Unix(), NotBefore: vectorNow.Unix(), RenewAfter: vectorNow.Unix() + 60, ExpiresAt: vectorNow.Unix() + 120,
		Origin: AllocationOrigin{HostServerCertPEM: fixtureCert, TrustCertPEM: fixtureCert, NodeClientSPKI: fixtureSPKI},
	}
}

func TestFRPOriginIdentityVector(t *testing.T) {
	if got := FRPOriginServerName(vectorToken); got != "0f007385b6f9d4b7eeb2748605afe1a9.origin.webmux.invalid" {
		t.Fatalf("origin DNS SAN = %q", got)
	}
	host := FRPOriginURI(fixtureScope, vectorToken, "host").String()
	want := "spiffe://webmux-relay/frp/host/" + string(vectorToken) + "/" + string(vectorHost) + "/1/2/" + string(vectorToken) + "/" + string(vectorToken)
	if host != want {
		t.Fatalf("origin URI SAN = %q", host)
	}
	if FRPOriginURI(fixtureScope, vectorToken, "node").String() == host {
		t.Fatal("host and node share origin URI SAN")
	}
	newScope := fixtureScope
	newScope.ConnectorGeneration++
	if FRPOriginURI(newScope, vectorToken, "host").String() == host {
		t.Fatal("origin URI SAN did not bind to connector generation")
	}
}

func TestFRPClaimsWireFixture(t *testing.T) {
	claims := FRPClaims{
		Issuer: "https://control.example.test" + FRPOIDCPath, Audience: FRPAudience("node-a"), Subject: "allocation-a",
		ID: vectorToken, IssuedAt: 1800000000, NotBefore: 1800000000, ExpiresAt: 1800000120,
		Scope: fixtureScope, NodeID: "node-a", AllocationID: vectorToken, GrantRevision: 9007199254740993,
		Role: FRPRole, ProxyName: "frp-proxy", RemotePort: FRPPortMin,
	}
	encoded, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"iss":"https://control.example.test/v1/frp/oidc","aud":"webmux-frp:node-a","sub":"allocation-a","jti":"` + string(vectorToken) +
		`","iat":1800000000,"nbf":1800000000,"exp":1800000120,"scope":` + fixtureScopeJSON + `,"nodeId":"node-a","allocationId":"` +
		string(vectorToken) + `","grantRevision":"9007199254740993","role":"frp-data","proxyName":"frp-proxy","remotePort":30000}`
	if string(encoded) != want {
		t.Fatalf("unexpected FRP claims: %s", encoded)
	}
	var decoded FRPClaims
	if err := Decode(encoded, &decoded, MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	if decoded != claims {
		t.Fatal("FRP claims changed during wire round trip")
	}
}

func frameFixture(t *testing.T, src, dst any, want string, max int) {
	t.Helper()
	var frame bytes.Buffer
	if err := WriteFrame(&frame, src, max); err != nil {
		t.Fatal(err)
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(want)))
	if !bytes.Equal(frame.Bytes()[:4], prefix[:]) || string(frame.Bytes()[4:]) != want {
		t.Fatalf("unexpected frame: %q", frame.Bytes())
	}
	frame.WriteByte('!')
	if err := ReadFrame(&frame, dst, max); err != nil {
		t.Fatal(err)
	}
	if frame.String() != "!" {
		t.Fatal("consumed bytes from following frame")
	}
}

func TestManagementWireFixtures(t *testing.T) {
	header := StreamHeader{Version: V1, Kind: StreamControl, RequestID: vectorToken, Scope: fixtureScope, Role: RoleControl}
	var decodedHeader StreamHeader
	frameFixture(t, header, &decodedHeader, `{"version":1,"kind":"control","requestId":"`+string(vectorToken)+`","scope":`+fixtureScopeJSON+`,"role":"control"}`, MaxStreamHeaderBytes)
	if err := decodedHeader.Validate(fixtureScope, RoleControl); err != nil {
		t.Fatal(err)
	}
	decodedHeader.Version++
	requireCode(t, decodedHeader.Validate(fixtureScope, RoleControl), Incompatible)
	decodedHeader = header
	decodedHeader.Scope.ConnectorGeneration++
	requireCode(t, decodedHeader.Validate(fixtureScope, RoleControl), StaleGeneration)
	requireCode(t, ReadFrame(bytes.NewReader([]byte{0x7f, 0xff, 0xff, 0xff}), &decodedHeader, MaxStreamHeaderBytes), Invalid)
	if err := ReadFrame(bytes.NewReader([]byte{0, 0, 0, 10, '{'}), &decodedHeader, MaxStreamHeaderBytes); err == nil {
		t.Fatal("accepted truncated management frame")
	}

	result := StreamResult{Version: V1, RequestID: header.RequestID}
	var decodedResult StreamResult
	frameFixture(t, result, &decodedResult, `{"version":1,"requestId":"`+string(vectorToken)+`"}`, MaxStreamHeaderBytes)
	if decodedResult != result {
		t.Fatal("management stream result lost request correlation")
	}

	resume := fixtureControl(ControlResume)
	resume.Resume = &LoginResume{GrantID: vectorToken}
	var decoded ControlMessage
	frameFixture(t, resume, &decoded, fixtureControlJSON(ControlResume)+`,"resume":{"grantId":"`+string(vectorToken)+`"}}`, MaxControlBytes)
	if err := decoded.Validate(fixtureScope, vectorNow); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []ControlType{ControlPing, ControlPong} {
		var heartbeat ControlMessage
		frameFixture(t, fixtureControl(kind), &heartbeat, fixtureControlJSON(kind)+`}`, MaxControlBytes)
		if err := heartbeat.Validate(fixtureScope, vectorNow); err != nil {
			t.Fatal(err)
		}
	}

	revokeAck := fixtureControl(ControlRevokeAck)
	revokeAck.RevocationAck = &RevocationAck{ID: vectorToken, HostID: vectorHost, BindingGeneration: 1, ConnectorGeneration: 2, AccessClosed: true}
	var ack ControlMessage
	frameFixture(t, revokeAck, &ack, fixtureControlJSON(ControlRevokeAck)+`,"revocationAck":{"id":"`+string(vectorToken)+`","hostId":"`+string(vectorHost)+`","bindingGeneration":"1","connectorGeneration":"2","accessClosed":true}}`, MaxControlBytes)
	if err := ack.Validate(fixtureScope, vectorNow); err != nil {
		t.Fatal(err)
	}
}

func TestFRPControlWireFixtures(t *testing.T) {
	allocate := fixtureControl(ControlFRPAllocate)
	allocate.AllocationRequest = &AllocationRequest{HostServerCSR: fixtureCSR}
	renew := fixtureControl(ControlFRPRenew)
	renew.AllocationRenew = &AllocationRenew{AllocationID: vectorToken, GrantRevision: 9007199254740993}
	grant := fixtureControl(ControlFRPGrant)
	allocation := fixtureAllocationGrant()
	grant.AllocationGrant = &allocation
	status := fixtureControl(ControlFRPStatus)
	status.AllocationStatus = &AllocationStatus{AllocationID: vectorToken, GrantRevision: 1, Reason: AllocationStatusProbeFailed}

	for _, tc := range []struct {
		message ControlMessage
		payload string
	}{
		{allocate, `"allocationRequest":{"hostServerCsr":"-----BEGIN CERTIFICATE REQUEST-----\nYQ==\n-----END CERTIFICATE REQUEST-----\n"}`},
		{renew, `"allocationRenew":{"allocationId":"` + string(vectorToken) + `","grantRevision":"9007199254740993"}`},
		{grant, `"allocationGrant":{"allocationId":"` + string(vectorToken) + `","grantRevision":"1","proxyName":"frp-proxy","remotePort":20123,"transportUrl":"` + fixtureFRPURL + `","bridgeKey":"` + string(vectorToken) + `","jwt":"aGVhZGVy.Y2xhaW1z.c2ln","issuedAt":1800000000,"notBefore":1800000000,"expiresAt":1800000120,"renewAfter":1800000060,"origin":{"hostServerCertPem":"-----BEGIN CERTIFICATE-----\nYQ==\n-----END CERTIFICATE-----\n","trustCertPem":"-----BEGIN CERTIFICATE-----\nYQ==\n-----END CERTIFICATE-----\n","nodeClientSpki":"` + string(fixtureSPKI) + `"}}`},
		{status, `"allocationStatus":{"allocationId":"` + string(vectorToken) + `","grantRevision":"1","dataReady":false,"reason":"probe_failed"}`},
	} {
		t.Run(string(tc.message.Type), func(t *testing.T) {
			var got ControlMessage
			frameFixture(t, tc.message, &got, fixtureControlJSON(tc.message.Type)+","+tc.payload+"}", MaxControlBytes)
			if err := got.Validate(fixtureScope, vectorNow); err != nil {
				t.Fatal(err)
			}
			got.Version++
			requireCode(t, got.Validate(fixtureScope, vectorNow), Incompatible)
			got.Version = V1
			other := fixtureScope
			other.ConnectorGeneration++
			requireCode(t, got.Validate(other, vectorNow), StaleGeneration)
			requireCode(t, got.Validate(fixtureScope, time.Unix(got.ExpiresAt, 0)), Expired)
			got.Type = ControlPing
			requireCode(t, got.Validate(fixtureScope, vectorNow), Invalid)
			got.Type = tc.message.Type
			got.Resume = &LoginResume{GrantID: vectorToken}
			requireCode(t, got.Validate(fixtureScope, vectorNow), Invalid)
		})
	}
}

func TestFRPControlRejectsInvalidWire(t *testing.T) {
	for _, tc := range []struct {
		kind    ControlType
		payload string
	}{
		{ControlFRPAllocate, `,"allocationRequest":{"hostServerCsr":"x","nodeClientCsr":""}`},
		{ControlFRPAllocate, `,"allocationRequest":{"hostServerCsr":"x","nodeClientCsr":null}`},
		{ControlFRPAllocate, `,"allocationRequest":{"hostServerCsr":"x","hostServerCsr":"y"}`},
		{ControlFRPAllocate, `,"allocationRequest":{"hostServerCsr":"x","privateKey":"y"}`},
		{ControlFRPRenew, `,"allocationRenew":{"allocationId":"x","grantRevision":1}`},
		{ControlFRPRenew, `,"allocationRenew":{"allocationId":"x","grantRevision":"1","remotePort":20123}`},
		{ControlFRPStatus, `,"allocationStatus":{"allocationId":"x","grantRevision":"1"}`},
		{ControlFRPStatus, `,"allocationStatus":{"allocationId":"x","grantRevision":"1","dataReady":null}`},
	} {
		var m ControlMessage
		requireCode(t, Decode([]byte(fixtureControlJSON(tc.kind)+tc.payload+"}"), &m, MaxControlBytes), Invalid)
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], MaxControlBytes+1)
	var m ControlMessage
	requireCode(t, ReadFrame(bytes.NewReader(prefix[:]), &m, MaxControlBytes), Invalid)
	allocate := fixtureControl(ControlFRPAllocate)
	allocate.AllocationRequest = &AllocationRequest{HostServerCSR: strings.Repeat("x", MaxControlBytes+1)}
	requireCode(t, WriteFrame(&bytes.Buffer{}, allocate, MaxControlBytes), Invalid)
	allocate.AllocationRequest.HostServerCSR = fixtureCSR
	allocate.Type = ControlFRPGrant
	requireCode(t, allocate.Validate(fixtureScope, vectorNow), Invalid)
	renew := fixtureControl(ControlFRPRenew)
	renew.AllocationRenew = &AllocationRenew{AllocationID: vectorToken}
	requireCode(t, renew.Validate(fixtureScope, vectorNow), Invalid)
	status := fixtureControl(ControlFRPStatus)
	status.AllocationStatus = &AllocationStatus{AllocationID: vectorToken, GrantRevision: 1, Reason: "upstream credential"}
	requireCode(t, status.Validate(fixtureScope, vectorNow), Invalid)
	status.AllocationStatus.DataReady = true
	status.AllocationStatus.Reason = AllocationStatusProbeFailed
	requireCode(t, status.Validate(fixtureScope, vectorNow), Invalid)
}

func TestFRPGrantWireValidation(t *testing.T) {
	for _, target := range []string{
		"ws://connector.example.test/v1/frp/" + string(vectorToken),
		"wss://user@connector.example.test/v1/frp/" + string(vectorToken),
		"wss://connector.example.test:444/v1/frp/" + string(vectorToken),
		"wss://connector.example.test:/v1/frp/" + string(vectorToken),
		"wss://127.0.0.1/v1/frp/" + string(vectorToken),
		"wss://connector.example.test/v1/%66rp/" + string(vectorToken),
		"wss://connector.example.test/v1/frp/unrelated",
		fixtureFRPURL + "/", fixtureFRPURL + "?", fixtureFRPURL + "?secret=1", fixtureFRPURL + "#", fixtureFRPURL + "#fragment",
	} {
		g := fixtureAllocationGrant()
		g.TransportURL = target
		requireCode(t, g.validate(vectorNow, false), Invalid)
	}
	g := fixtureAllocationGrant()
	g.TransportURL = strings.Replace(g.TransportURL, ".test/", ".test:443/", 1)
	g.NotBefore = vectorNow.Add(5 * time.Second).Unix()
	g.Origin.HostServerCertPEM += fixtureCert
	if err := g.validate(vectorNow, false); err != nil {
		t.Fatal(err)
	}
	g.NotBefore++
	requireCode(t, g.validate(vectorNow, false), Invalid)
	g = fixtureAllocationGrant()
	requireCode(t, g.validate(time.Unix(g.ExpiresAt, 0), false), Expired)
	g.Origin.TrustCertPEM += "-----BEGIN PRIVATE KEY-----\nYQ==\n-----END PRIVATE KEY-----\n"
	requireCode(t, g.validate(vectorNow, false), Invalid)
	for _, mutate := range []func(*AllocationGrant){
		func(g *AllocationGrant) { g.ProxyName = "unexpected/path" },
		func(g *AllocationGrant) { g.RemotePort = 0 },
		func(g *AllocationGrant) { g.BridgeKey = "" },
		func(g *AllocationGrant) { g.JWT = "browser token" },
		func(g *AllocationGrant) { g.RenewAfter = g.ExpiresAt },
		func(g *AllocationGrant) { g.Origin.NodeClientSPKI = "" },
	} {
		g := fixtureAllocationGrant()
		mutate(&g)
		requireCode(t, g.validate(vectorNow, false), Invalid)
	}
}

func TestFRPPrivateNodeAllocation(t *testing.T) {
	m := NodeMessage{
		Version: V1, Type: NodeFRPAllocate, RequestID: vectorToken, Scope: &fixtureScope,
		NodeID: "node-a", Audience: "webmux-control", ControlEpoch: vectorToken, NodeEpoch: vectorToken, Sequence: 1, ExpiresAt: vectorNow.Add(RequestTimeout).Unix(),
		AllocationRequest: &AllocationRequest{HostServerCSR: fixtureCSR, NodeClientCSR: fixtureCSR},
	}
	check := func(m NodeMessage) error {
		return m.Validate("node-a", "webmux-control", vectorToken, vectorToken, vectorNow)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var got NodeMessage
	if err := Decode(b, &got, MaxSnapshotBytes); err != nil {
		t.Fatal(err)
	}
	if err := check(got); err != nil {
		t.Fatal(err)
	}
	if *got.Scope != fixtureScope || got.RequestID != vectorToken || *got.AllocationRequest != *m.AllocationRequest {
		t.Fatal("private allocation changed scope, correlation, or CSR")
	}
	stale := *got.Scope
	stale.NodeEpoch = Token("AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	got.Scope = &stale
	requireCode(t, check(got), StaleGeneration)
	got.Scope = nil
	requireCode(t, check(got), Invalid)
	got.Scope = &fixtureScope
	got.AllocationRequest.NodeClientCSR = ""
	requireCode(t, check(got), Invalid)
	got.AllocationRequest.NodeClientCSR = fixtureCSR + fixtureCSR
	requireCode(t, check(got), Invalid)

	host := fixtureControl(ControlFRPAllocate)
	host.AllocationRequest = m.AllocationRequest
	requireCode(t, host.Validate(fixtureScope, vectorNow), Invalid)
	host.AllocationRequest = &AllocationRequest{HostServerCSR: fixtureCSR}
	m.Type, m.Scope, m.AllocationRequest, m.Forward = NodeForwardType, nil, nil, &host
	requireCode(t, check(m), Invalid)
	m.Type, m.Scope, m.Forward = NodeFRPRenew, &fixtureScope, nil
	m.AllocationRenew = &AllocationRenew{AllocationID: vectorToken, GrantRevision: 1}
	if err := check(m); err != nil {
		t.Fatal(err)
	}
	m.Type = NodeFRPConfirm
	b, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got = NodeMessage{}
	if err := Decode(b, &got, MaxSnapshotBytes); err != nil {
		t.Fatal(err)
	}
	if got.Type != NodeFRPConfirm || got.RequestID != m.RequestID || *got.Scope != *m.Scope || *got.AllocationRenew != *m.AllocationRenew {
		t.Fatal("private confirmation lost allocation, revision, scope, or correlation")
	}
	if err := check(got); err != nil {
		t.Fatal(err)
	}
	got.AllocationRenew.GrantRevision = 0
	requireCode(t, check(got), Invalid)
	got.AllocationRenew.GrantRevision = 1
	got.Scope = &stale
	requireCode(t, check(got), StaleGeneration)
	publicConfirm := fixtureControl(ControlType(NodeFRPConfirm))
	publicConfirm.AllocationRenew = m.AllocationRenew
	requireCode(t, publicConfirm.Validate(fixtureScope, vectorNow), Invalid)

	grant := fixtureAllocationGrant()
	m.Type, m.AllocationRenew, m.AllocationGrant = NodeFRPGrant, nil, &grant
	requireCode(t, check(m), Invalid)
	grant.NodeOrigin = &AllocationNodeOrigin{NodeClientCertPEM: fixtureCert, HostServerSPKI: fixtureSPKI}
	b, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got = NodeMessage{}
	if err := Decode(b, &got, MaxSnapshotBytes); err != nil {
		t.Fatal(err)
	}
	if err := check(got); err != nil {
		t.Fatal(err)
	}
	if *got.AllocationGrant.NodeOrigin != *grant.NodeOrigin {
		t.Fatal("private grant lost node origin material")
	}
	host.Type, host.AllocationRequest, host.AllocationGrant = ControlFRPGrant, nil, &grant
	requireCode(t, host.Validate(fixtureScope, vectorNow), Invalid)
	grant.NodeOrigin = nil
	if err := host.Validate(fixtureScope, vectorNow); err != nil {
		t.Fatal(err)
	}
}

func TestFRPSyncAckWireFixture(t *testing.T) {
	for _, drained := range []bool{false, true} {
		ack := SyncAck{Revision: 7, AccessReconciled: true, FRPReconciled: drained}
		data, err := json.Marshal(ack)
		if err != nil {
			t.Fatal(err)
		}
		want := `{"revision":"7","accessReconciled":true}`
		if drained {
			want = `{"revision":"7","accessReconciled":true,"frpReconciled":true}`
		}
		if string(data) != want {
			t.Fatalf("unexpected sync acknowledgement: %s", data)
		}
		var decoded SyncAck
		if err := Decode(data, &decoded, MaxSnapshotBytes); err != nil {
			t.Fatal(err)
		}
		if decoded != ack {
			t.Fatal("sync acknowledgement changed during wire round trip")
		}
	}
}
