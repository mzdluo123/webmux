package relaysetting

import (
	"context"
	"strings"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

func authTestICE(t *testing.T, auth *Auth, control *authTestControl) {
	t.Helper()
	browserExpiry := auth.deps.Now().Add(30 * time.Second).Unix()
	hostExpiry := browserExpiry + 1
	browserUsername, err := protocol.TURNUsername(browserExpiry, authTestToken(50))
	if err != nil {
		t.Fatal(err)
	}
	hostUsername, err := protocol.TURNUsername(hostExpiry, authTestToken(50))
	if err != nil {
		t.Fatal(err)
	}
	control.mu.Lock()
	control.ice = protocol.ICEConfig{NodeID: "turn-one", ConfigRevision: 4, Secret: protocol.SecretRef{Purpose: protocol.SecretTURN, KeyID: "key-one", Version: 1}, ExpiresAt: browserExpiry, Browser: protocol.ICEEndpoint{Role: "browser", ExpiresAt: browserExpiry, Servers: []protocol.ICEServer{{URLs: []string{"turn:turn.example.com:3478?transport=udp"}, Username: browserUsername, Credential: "browser-password"}}}, Host: protocol.ICEEndpoint{Role: "host", ExpiresAt: hostExpiry, Servers: []protocol.ICEServer{{URLs: []string{"turn:turn.example.com:3478?transport=udp"}, Username: hostUsername, Credential: "host-password"}}}}
	control.mu.Unlock()
}

func TestAuthICEAndPeerBoundToCurrentSession(t *testing.T) {
	auth, control, now := newAuthTest(t, AuthConfig{})
	first := finishAuthLogin(t, auth, control, 10)
	second := finishAuthLogin(t, auth, control, 11)
	authTestICE(t, auth, control)
	peerID := authTestToken(60)
	browser, err := auth.ICE(context.Background(), first.Principal, peerID)
	if err != nil {
		t.Fatal(err)
	}
	if browser.Role != "browser" || browser.Servers[0].Credential != "browser-password" {
		t.Fatal("ICE exposed the wrong endpoint")
	}
	fingerprint := "sha-256 " + strings.TrimSuffix(strings.Repeat("AB:", 32), ":")
	_, err = auth.AuthorizePeer(second.Principal, peerID, fingerprint)
	requireAuthCode(t, err, protocol.Unauthenticated)
	peer, err := auth.AuthorizePeer(first.Principal, peerID, fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if peer.Host.Role != "host" || peer.Host.Servers[0].Credential != "host-password" || peer.Session != first.Principal.Session {
		t.Fatal("host endpoint not retained for its owner and peer")
	}
	// Returned endpoint mutations cannot corrupt Auth's retained credentials.
	peer.Host.Servers[0].URLs[0] = "turn:untrusted.example.com:3478"
	current, err := auth.LookupPeer(first.Principal, peerID, fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if current.Host.Servers[0].URLs[0] == peer.Host.Servers[0].URLs[0] {
		t.Fatal("caller changed retained ICE endpoint")
	}
	_, err = auth.AuthorizePeer(first.Principal, peerID, fingerprint)
	requireAuthCode(t, err, protocol.Replay)
	_, err = auth.LookupPeer(first.Principal, peerID, strings.Replace(fingerprint, "AB", "AC", 1))
	requireAuthCode(t, err, protocol.Unauthenticated)
	now.Add(30)
	_, err = auth.LookupPeer(first.Principal, peerID, fingerprint)
	requireAuthCode(t, err, protocol.Unauthenticated)
	if err := auth.Logout(context.Background(), first.Principal); err != nil {
		t.Fatal(err)
	}
	select {
	case <-peer.Context.Done():
	default:
		t.Fatal("logout left peer context live")
	}
}

func TestAuthICERefreshFailureCancelsAuthorizedPeer(t *testing.T) {
	auth, control, _ := newAuthTest(t, AuthConfig{})
	finish := finishAuthLogin(t, auth, control, 10)
	authTestICE(t, auth, control)
	peerID := authTestToken(60)
	if _, err := auth.ICE(context.Background(), finish.Principal, peerID); err != nil {
		t.Fatal(err)
	}
	fingerprint := "sha-256 " + strings.TrimSuffix(strings.Repeat("CD:", 32), ":")
	peer, err := auth.AuthorizePeer(finish.Principal, peerID, fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	control.mu.Lock()
	control.ice.Host.Servers[0].Credential = control.ice.Browser.Servers[0].Credential
	control.mu.Unlock()
	_, err = auth.ICE(context.Background(), finish.Principal, peerID)
	requireAuthCode(t, err, protocol.Invalid)
	select {
	case <-peer.Context.Done():
	default:
		t.Fatal("failed refresh left peer access alive")
	}
}

func TestAuthPeerRejectsMalformedFingerprints(t *testing.T) {
	auth, control, _ := newAuthTest(t, AuthConfig{})
	finish := finishAuthLogin(t, auth, control, 10)
	authTestICE(t, auth, control)
	peerID := authTestToken(60)
	if _, err := auth.ICE(context.Background(), finish.Principal, peerID); err != nil {
		t.Fatal(err)
	}
	for _, fingerprint := range []string{"", "sha-1 AB:CD", "sha-256 AB", "sha-256 " + strings.TrimSuffix(strings.Repeat("ZZ:", 32), ":")} {
		_, err := auth.AuthorizePeer(finish.Principal, peerID, fingerprint)
		requireAuthCode(t, err, protocol.Invalid)
	}
	fingerprint := "sha-256 " + strings.TrimSuffix(strings.Repeat("EF:", 32), ":")
	if _, err := auth.AuthorizePeer(finish.Principal, peerID, fingerprint); err != nil {
		t.Fatal("failed validation consumed valid peer authorization:", err)
	}
}
