package relay

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func transportTestLeaf(t *testing.T) (QUICEndpoint, *x509.Certificate, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const name = "quic.example.test"
	issue := func(serial int64) (*x509.Certificate, string) {
		t.Helper()
		leaf := &x509.Certificate{
			SerialNumber: big.NewInt(serial), DNSNames: []string{name},
			NotBefore: vectorNow.Add(-time.Minute), NotAfter: vectorNow.Add(time.Minute),
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return cert, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	cert, encoded := issue(1)
	other, _ := issue(2)
	digest := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return QUICEndpoint{Address: "quic.example.test:5443", ServerName: name, SPKI: Digest(hex.EncodeToString(digest[:])), CertificatePEM: encoded}, cert, other
}

func TestQUICEndpointExactCertificate(t *testing.T) {
	endpoint, leaf, sameKey := transportTestLeaf(t)
	if err := endpoint.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.VerifyCertificate(leaf, vectorNow); err != nil {
		t.Fatal(err)
	}
	requireCode(t, endpoint.VerifyCertificate(sameKey, vectorNow), Unauthenticated)
	requireCode(t, endpoint.VerifyCertificate(leaf, vectorNow.Add(time.Minute)), Unauthenticated)
	for name, mutate := range map[string]func(*QUICEndpoint){
		"URL":        func(e *QUICEndpoint) { e.Address = "https://quic.example.test:5443" },
		"wildcard":   func(e *QUICEndpoint) { e.Address = "0.0.0.0:5443" },
		"wrong-name": func(e *QUICEndpoint) { e.ServerName = "elsewhere.example.test" },
		"wrong-pin":  func(e *QUICEndpoint) { e.SPKI = fixtureSPKI },
		"extra-root": func(e *QUICEndpoint) { e.CertificatePEM += e.CertificatePEM },
	} {
		t.Run(name, func(t *testing.T) {
			bad := endpoint
			mutate(&bad)
			requireCode(t, bad.Validate(), InvalidConfig)
		})
	}
}

func TestTransportEnvelopesAndGrantBounds(t *testing.T) {
	endpoint, _, _ := transportTestLeaf(t)
	request := fixtureControl(ControlTransportRequest)
	request.TransportRequest = &TransportRequest{Action: TransportPrepare, OperationID: vectorToken, RouteVersion: 1,
		Transport: FRPTransportQUIC, AllocationRequest: &AllocationRequest{HostServerCSR: fixtureCSR}}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ControlMessage
	if err := Decode(encoded, &decoded, MaxControlBytes); err != nil || decoded.Validate(fixtureScope, vectorNow) != nil {
		t.Fatalf("transport request did not round-trip: %v", err)
	}
	decoded.TransportRequest.AllocationRequest.NodeClientCSR = fixtureCSR
	requireCode(t, decoded.Validate(fixtureScope, vectorNow), Invalid)
	if err := decoded.TransportRequest.Validate(true); err != nil {
		t.Fatal(err)
	}
	for _, action := range []TransportAction{TransportConfirm, TransportRelease} {
		private := TransportRequest{Action: action, OperationID: vectorToken, RouteVersion: 1,
			AllocationRenew: &AllocationRenew{AllocationID: vectorToken, GrantRevision: 1}}
		requireCode(t, private.Validate(false), Invalid)
		if err := private.Validate(true); err != nil {
			t.Fatal(err)
		}
	}
	grant := fixtureAllocationGrant()
	grant.Transport = &TransportGrant{Protocol: FRPTransportQUIC, OperationID: vectorToken, RouteVersion: 1,
		Deadline: vectorNow.Add(TransportCandidateTTL).Unix(), Endpoint: &endpoint}
	grant.ExpiresAt, grant.RenewAfter = grant.Transport.Deadline, vectorNow.Add(20*time.Second).Unix()
	grant.TransportURL, grant.BridgeKey = "", ""
	result := fixtureControl(ControlTransportResult)
	result.TransportResult = &TransportResult{OperationID: vectorToken, RouteVersion: 1, State: TransportPrepared, Grant: &grant}
	if err := result.Validate(fixtureScope, vectorNow); err != nil {
		t.Fatal(err)
	}
	grant.ExpiresAt++
	requireCode(t, result.Validate(fixtureScope, vectorNow), Invalid)
	result.TransportResult.State = TransportCommitted
	result.TransportResult.RouteVersion = 2
	result.TransportResult.Active, result.TransportResult.ActiveAllocationID = FRPTransportQUIC, grant.AllocationID
	if err := result.Validate(fixtureScope, vectorNow); err != nil {
		t.Fatalf("committed registration could not roll its expiry: %v", err)
	}
	grant.BridgeKey = vectorToken
	requireCode(t, result.Validate(fixtureScope, vectorNow), Invalid)
	grant.BridgeKey = ""
	legacy := fixtureControl(ControlFRPGrant)
	legacy.AllocationGrant = &grant
	requireCode(t, legacy.Validate(fixtureScope, vectorNow), Invalid)

	claims := FRPClaims{Transport: grant.Transport}
	encoded, err = json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	var parsed FRPClaims
	if err := Decode(encoded, &parsed, MaxControlBytes); err != nil || !claims.Transport.Equal(parsed.Transport) {
		t.Fatalf("signed transport fields did not round-trip: %v", err)
	}
	parsed.Transport.Endpoint.Address = "elsewhere.example.test:5443"
	if claims.Transport.Equal(parsed.Transport) {
		t.Fatal("assigned UDP address was not part of transport equality")
	}
}
