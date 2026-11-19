package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/pion/ice/v4"
	"github.com/pion/turn/v5"
	"github.com/pion/webrtc/v4"
	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/p2p"
	"github.com/rainchan/webmux/internal/p2psettings"
	"github.com/rainchan/webmux/internal/relaysetting"
)

func TestP2PControllerSettingsLifecycle(t *testing.T) {
	dir := t.TempDir()
	defaults := config.P2PConfig{
		Enabled:     true,
		PortMapping: true,
		STUNServers: config.DefaultSTUNServers(),
	}
	store := p2psettings.Open(dir, defaults)
	defer store.Close()
	svc := p2p.New(p2p.Options{
		AppVersion:  "test",
		Enabled:     defaults.Enabled,
		PortMapping: false,
		STUNServers: defaults.STUNServers,
	})
	defer svc.Close()

	ctl := newP2PController(store, svc)
	initial := ctl.Status()
	if initial.Source != "config" || !initial.StorageAvailable {
		t.Fatalf("unexpected initial status: %+v", initial)
	}

	updated, err := ctl.UpdateSettings(contract.P2PSettingsUpdate{
		Enabled:     false,
		PortMapping: false,
		STUNServers: []string{"stun:stun.cloudflare.com:3478"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Enabled || updated.Source != "web" || len(updated.STUNServers) != 1 {
		t.Fatalf("unexpected updated status: %+v", updated)
	}
	if ctl.Enabled() {
		t.Fatal("expected controller Enabled() to be false after disabling P2P")
	}

	reset, err := ctl.ResetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if reset.Source != "config" || !slices.Equal(reset.STUNServers, defaults.STUNServers) {
		t.Fatalf("unexpected reset status: %+v", reset)
	}
}

func TestConfiguredViewCorruptP2POverrideFailsClosed(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")
	file := filepath.Join(root, "p2p-settings.json")
	if err := os.WriteFile(path, []byte(fmt.Sprintf("state_dir = %q\n[p2p]\nenabled = true\nport_mapping = true\n", root)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(`{"v":1,broken`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := configShow([]string{"--config", path, "--json"}, &out, &stderr); code != 0 {
		t.Fatalf("configShow exit %d: %s", code, stderr.String())
	}
	var v contract.Configuration
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v.Fields["p2p.enabled"].Value != false || v.Fields["p2p.port_mapping"].Value != false {
		t.Fatalf("corrupt p2p-settings.json did not fail closed: %+v", v.Fields)
	}
}

type p2pTestRelayControl struct {
	mu       sync.Mutex
	binding  relaysetting.AuthBinding
	grant    protocol.BrowserGrant
	iceTTL   time.Duration
	turnURLs []string
	secrets  map[string]string
}

func (c *p2pTestRelayControl) Scope() (protocol.Scope, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.binding.Scope, true
}

func (c *p2pTestRelayControl) snapshot() (relaysetting.AuthBinding, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.binding, true
}

func (c *p2pTestRelayControl) Exchange(_ context.Context, _ protocol.LoginExchange) (protocol.BrowserGrant, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.grant, nil
}

func (c *p2pTestRelayControl) Resume(_ context.Context, _ protocol.LoginResume) (protocol.BrowserGrant, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.grant, nil
}

func (c *p2pTestRelayControl) ICE(_ context.Context, req protocol.ICERequest) (protocol.ICEConfig, error) {
	c.mu.Lock()
	ttl := c.iceTTL
	urls := slices.Clone(c.turnURLs)
	expiry := time.Now().Add(ttl).Unix()
	accountKey := p2pTestToken(90)
	browserUser, err := protocol.TURNUsername(expiry, accountKey)
	if err != nil {
		c.mu.Unlock()
		return protocol.ICEConfig{}, err
	}
	hostUser, err := protocol.TURNUsername(expiry+1, accountKey)
	if err != nil {
		c.mu.Unlock()
		return protocol.ICEConfig{}, err
	}
	c.secrets[browserUser] = "browser-secret"
	c.secrets[hostUser] = "host-secret"
	c.mu.Unlock()
	return protocol.ICEConfig{
		GrantID:          req.GrantID,
		PeerConnectionID: req.PeerConnectionID,
		NodeID:           "turn-one",
		ConfigRevision:   1,
		Secret:           protocol.SecretRef{Purpose: protocol.SecretTURN, KeyID: "key-one", Version: 1},
		ExpiresAt:        expiry,
		Browser: protocol.ICEEndpoint{
			Role:      "browser",
			ExpiresAt: expiry,
			Servers:   []protocol.ICEServer{{URLs: urls, Username: browserUser, Credential: "browser-secret"}},
		},
		Host: protocol.ICEEndpoint{
			Role:      "host",
			ExpiresAt: expiry + 1,
			Servers:   []protocol.ICEServer{{URLs: urls, Username: hostUser, Credential: "host-secret"}},
		},
	}, nil
}

func startLoopbackTURN(t *testing.T, control *p2pTestRelayControl) []string {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	const realm = "webmux-p2pctl-test"
	server, err := turn.NewServer(turn.ServerConfig{
		Realm: realm,
		AuthHandler: func(attr *turn.RequestAttributes) (string, []byte, bool) {
			control.mu.Lock()
			secret, ok := control.secrets[attr.Username]
			control.mu.Unlock()
			if !ok {
				return "", nil, false
			}
			return attr.Username, turn.GenerateAuthKey(attr.Username, realm, secret), true
		},
		PacketConnConfigs: []turn.PacketConnConfig{{
			PacketConn: conn,
			RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
				RelayAddress: net.ParseIP("127.0.0.1"),
				Address:      "127.0.0.1",
			},
		}},
	})
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return []string{"turn:localhost:" + port + "?transport=udp"}
}

func p2pTestToken(b byte) protocol.Token {
	return protocol.Token(base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat(string([]byte{b}), 32))))
}

func newBrowserOfferPeer(t *testing.T) (*webrtc.PeerConnection, *webrtc.DataChannel, <-chan struct{}, chan string, string) {
	t.Helper()
	var se webrtc.SettingEngine
	se.SetIncludeLoopbackCandidate(true)
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	pc, err := webrtc.NewAPI(webrtc.WithSettingEngine(se)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	dc, err := pc.CreateDataChannel("webmux:control", nil)
	if err != nil {
		t.Fatal(err)
	}
	opened := make(chan struct{})
	var openOnce sync.Once
	dc.OnOpen(func() { openOnce.Do(func() { close(opened) }) })
	messages := make(chan string, 4)
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		messages <- string(msg.Data)
	})
	gathered := webrtc.GatheringCompletePromise(pc)
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	select {
	case <-gathered:
	case <-time.After(10 * time.Second):
		t.Fatal("browser offer gathering timed out")
	}
	return pc, dc, opened, messages, pc.LocalDescription().SDP
}

func TestP2PControllerRelayOwnerPeerChurnAndTURNRefresh(t *testing.T) {
	dir := t.TempDir()
	defaults := config.P2PConfig{Enabled: false, PortMapping: false}
	store := p2psettings.Open(dir, defaults)
	defer store.Close()
	svc := p2p.New(p2p.Options{AppVersion: "test", Enabled: false, PortMapping: false})
	defer svc.Close()

	host := protocol.HostID("0123456789abcdef0123456789abcdef")
	origin := "https://" + host.Label() + ".example.com"
	scope := protocol.Scope{HostID: host, BindingGeneration: 1, ConnectorGeneration: 1, ControlEpoch: p2pTestToken(1), NodeEpoch: p2pTestToken(2)}
	control := &p2pTestRelayControl{
		binding: relaysetting.AuthBinding{
			Binding:       protocol.Binding{HostID: host, OwnerID: "123", BindingGeneration: 1, PublicURL: origin},
			Scope:         scope,
			ControlOrigin: "https://control.example.com",
		},
		grant: protocol.BrowserGrant{
			ID:                  p2pTestToken(10),
			ConsoleSessionID:    p2pTestToken(11),
			OwnerID:             "123",
			HostID:              host,
			BindingGeneration:   1,
			ConnectorGeneration: 1,
			ControlEpoch:        scope.ControlEpoch,
			NodeEpoch:           scope.NodeEpoch,
			Origin:              origin,
			ExpiresAt:           time.Now().Add(time.Hour).Unix(),
		},
		iceTTL:  30 * time.Second,
		secrets: make(map[string]string),
	}
	control.turnURLs = startLoopbackTURN(t, control)
	auth, err := relaysetting.NewAuth(relaysetting.AuthConfig{}, relaysetting.AuthDependencies{
		Client:   control,
		Snapshot: control.snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = auth.Close() }()

	start, err := auth.Start(context.Background(), origin, "")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(start.URL)
	if err != nil {
		t.Fatal(err)
	}
	finish, err := auth.Finish(context.Background(), origin, start.Cookie.Value, parsed.Query().Get("state"), string(p2pTestToken(20)))
	if err != nil {
		t.Fatal(err)
	}
	ownerCtx := api.RelayOwnerContext(context.Background(), finish.Principal)

	ctl := newP2PController(store, svc)
	ctl.relayAuth = auth

	// 1. Repeatedly create and close > 8 (MaxPeersPerSession) owner PeerConnections,
	// including failed offer negotiations, without hitting 429 rate_limited.
	_, _, _, _, sampleSDP := newBrowserOfferPeer(t)
	for i := byte(1); i <= 4; i++ {
		connID := p2pTestToken(30 + i)
		if _, err := auth.ICE(context.Background(), finish.Principal, connID); err != nil {
			t.Fatalf("failed-offer iteration %d ICE: %v", i, err)
		}
		if _, err := ctl.Signal(ownerCtx, contract.P2PSignalRequest{
			Action:           "offer",
			PeerConnectionID: string(connID),
			SDP:              sampleSDP,
			Candidates:       []contract.P2PICECandidate{{Candidate: "invalid-ice-candidate-syntax"}},
		}); err == nil {
			t.Fatalf("failed-offer iteration %d unexpectedly succeeded", i)
		}
	}
	for i := byte(1); i <= 10; i++ {
		connID := p2pTestToken(50 + i)
		if _, err := auth.ICE(context.Background(), finish.Principal, connID); err != nil {
			t.Fatalf("iteration %d ICE failed (quota leak?): %v", i, err)
		}
		pc, _, _, _, offerSDP := newBrowserOfferPeer(t)
		resp, err := ctl.Signal(ownerCtx, contract.P2PSignalRequest{
			Action:           "offer",
			PeerConnectionID: string(connID),
			SDP:              offerSDP,
		})
		if err != nil {
			t.Fatalf("iteration %d offer Signal failed: %v", i, err)
		}
		if _, err := ctl.Signal(ownerCtx, contract.P2PSignalRequest{
			Action:           "close",
			PeerID:           resp.PeerID,
			PeerConnectionID: string(connID),
		}); err != nil {
			t.Fatalf("iteration %d close Signal failed: %v", i, err)
		}
		_ = pc.Close()
	}

	// 2. TURN credential refresh extends peer.expires; PeerConnection survives
	// past the initial expiry timestamp instead of disconnecting at Host.ExpiresAt.
	control.mu.Lock()
	control.iceTTL = 2 * time.Second
	control.mu.Unlock()
	liveConnID := p2pTestToken(80)
	initialICE, err := auth.ICE(context.Background(), finish.Principal, liveConnID)
	if err != nil {
		t.Fatal(err)
	}
	initialDeadline := time.Unix(initialICE.ExpiresAt+1, 0)

	clientPC, dc, opened, messages, liveOfferSDP := newBrowserOfferPeer(t)
	liveResp, err := ctl.Signal(ownerCtx, contract.P2PSignalRequest{
		Action:           "offer",
		PeerConnectionID: string(liveConnID),
		SDP:              liveOfferSDP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := clientPC.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: liveResp.SDP}); err != nil {
		t.Fatal(err)
	}
	for _, c := range liveResp.Candidates {
		if err := clientPC.AddICECandidate(webrtc.ICECandidateInit{Candidate: c.Candidate, SDPMid: c.SDPMid, SDPMLineIndex: c.SDPMLineIndex}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-opened:
	case <-time.After(10 * time.Second):
		t.Fatal("owner data channel did not open")
	}

	// Refresh TURN credentials before initial expiry to extend peer.expires.
	control.mu.Lock()
	control.iceTTL = 30 * time.Second
	control.mu.Unlock()
	if _, err := auth.ICE(context.Background(), finish.Principal, liveConnID); err != nil {
		t.Fatalf("TURN refresh ICE failed: %v", err)
	}

	if wait := time.Until(initialDeadline) + 400*time.Millisecond; wait > 0 {
		time.Sleep(wait)
	}
	if status := svc.Status(); status.ActivePeers != 1 || status.Peers[0].ID != liveResp.PeerID {
		t.Fatalf("owner peer disconnected at initial TURN expiry despite refresh: %+v", status)
	}
	if err := dc.SendText(`{"type":"ping","seq":42}`); err != nil {
		t.Fatalf("send ping after initial TURN expiry: %v", err)
	}
	select {
	case raw := <-messages:
		if !strings.Contains(raw, `"pong"`) || !strings.Contains(raw, `42`) {
			t.Fatalf("unexpected reply after initial TURN expiry: %s", raw)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ping timed out after initial TURN expiry")
	}

	if _, err := ctl.Signal(ownerCtx, contract.P2PSignalRequest{
		Action:           "close",
		PeerID:           liveResp.PeerID,
		PeerConnectionID: string(liveConnID),
	}); err != nil {
		t.Fatalf("close live peer: %v", err)
	}
}
