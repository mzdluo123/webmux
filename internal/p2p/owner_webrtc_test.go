//go:build !nop2p

package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/turn/v5"
	"github.com/pion/webrtc/v4"
	"github.com/rainchan/webmux/internal/streamproto"
)

func waitOwnerEvent(t *testing.T, event <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-event:
	case <-time.After(10 * time.Second):
		t.Fatal(description)
	}
}

func ownerClient(t *testing.T, config webrtc.Configuration) *webrtc.PeerConnection {
	t.Helper()
	var settings webrtc.SettingEngine
	settings.SetIncludeLoopbackCandidate(true)
	settings.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	settings.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	pc, err := webrtc.NewAPI(webrtc.WithSettingEngine(settings)).NewPeerConnection(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	return pc
}

func ownerClientChannel(t *testing.T, pc *webrtc.PeerConnection, label string) (*webrtc.DataChannel, <-chan struct{}) {
	t.Helper()
	dc, err := pc.CreateDataChannel(label, nil)
	if err != nil {
		t.Fatal(err)
	}
	opened, closed := make(chan struct{}), make(chan struct{})
	dc.OnOpen(func() { close(opened) })
	dc.OnClose(func() { close(closed) })
	select {
	case <-opened:
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("owner data channel did not open or close")
	}
	return dc, closed
}

func ownerLoopbackTURN(t *testing.T) (ICEServer, webrtc.ICEServer) {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	const realm = "webmux-owner-test"
	passwords := map[string]string{"host": "test-host-password", "browser": "test-browser-password"}
	server, err := turn.NewServer(turn.ServerConfig{
		Realm: realm,
		AuthHandler: func(attributes *turn.RequestAttributes) (string, []byte, bool) {
			password, ok := passwords[attributes.Username]
			if !ok || attributes.Realm != realm {
				return "", nil, false
			}
			return attributes.Username, turn.GenerateAuthKey(attributes.Username, realm, password), true
		},
		PacketConnConfigs: []turn.PacketConnConfig{{
			PacketConn: conn,
			RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
				RelayAddress: net.ParseIP("127.0.0.1"), Address: "127.0.0.1",
			},
		}},
	})
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	urls := []string{"turn:" + conn.LocalAddr().String() + "?transport=udp"}
	return ICEServer{URLs: urls, Username: "host", Credential: passwords["host"]},
		webrtc.ICEServer{URLs: urls, Username: "browser", Credential: passwords["browser"]}
}

func TestOwnerPionDirectAndTURNIndependentOfLocalToggle(t *testing.T) {
	for _, useTURN := range []bool{false, true} {
		name := "direct"
		if useTURN {
			name = "turn"
		}
		t.Run(name, func(t *testing.T) { testOwnerPion(t, useTURN) })
	}
}

func testOwnerPion(t *testing.T, useTURN bool) {
	t.Helper()
	var hostServers []ICEServer
	clientConfig := webrtc.Configuration{}
	if useTURN {
		host, browser := ownerLoopbackTURN(t)
		hostServers = []ICEServer{host}
		clientConfig = webrtc.Configuration{ICEServers: []webrtc.ICEServer{browser}, ICETransportPolicy: webrtc.ICETransportPolicyRelay}
	}
	type contextMarker struct{}
	lifetime, revoke := context.WithCancel(context.WithValue(context.Background(), contextMarker{}, true))
	defer revoke()
	streamStarted := make(chan string, 4)
	rpcStarted := make(chan streamproto.RPCRequest, 4)
	childCanceled := make(chan struct{}, 2)
	childrenMayExit := make(chan struct{})
	svc := New(Options{
		Enabled: false, PortMapping: false, STUNServers: []string{"stun:127.0.0.1:1"},
		StreamHandler: func(ctx context.Context, path string, conn streamproto.StreamConn) {
			if ctx.Value(contextMarker{}) != true {
				t.Error("owner stream lost its trusted context")
			}
			streamStarted <- path
			<-ctx.Done()
			childCanceled <- struct{}{}
			<-childrenMayExit
		},
		RPCHandler: func(ctx context.Context, request streamproto.RPCRequest) streamproto.RPCResponse {
			if ctx.Value(contextMarker{}) != true {
				t.Error("owner RPC lost its trusted context")
			}
			rpcStarted <- request
			<-ctx.Done()
			childCanceled <- struct{}{}
			<-childrenMayExit
			return streamproto.RPCResponse{Status: 200}
		},
	})
	defer func() {
		revoke()
		select {
		case <-childrenMayExit:
		default:
			close(childrenMayExit)
		}
		svc.Close()
	}()
	client := ownerClient(t, clientConfig)
	control, err := client.CreateDataChannel(controlChannelLabel, nil)
	if err != nil {
		t.Fatal(err)
	}
	opened := make(chan struct{})
	control.OnOpen(func() { close(opened) })
	replies := make(chan controlEnvelope, 4)
	control.OnMessage(func(message webrtc.DataChannelMessage) {
		var reply controlEnvelope
		if json.Unmarshal(message.Data, &reply) == nil {
			replies <- reply
		}
	})
	gathered := webrtc.GatheringCompletePromise(client)
	offer, err := client.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	waitOwnerEvent(t, gathered, "owner browser ICE gathering timed out")
	rawOffer := client.LocalDescription().SDP
	fingerprint, err := OfferFingerprint(rawOffer)
	if err != nil {
		t.Fatal(err)
	}
	var accessDone <-chan struct{}
	var closeAccess func()
	released := make(chan struct{})
	owner := &OwnerAuthorization{
		OwnerID: "opaque-owner-session", Context: lifetime, PeerConnectionID: "bound-connection",
		Fingerprint: fingerprint, ICEServers: hostServers, ExpiresAt: time.Now().Add(time.Minute),
		RegisterAccess: func(closePeer func(), done <-chan struct{}) (func(), error) {
			closeAccess, accessDone = closePeer, done
			return func() { close(released) }, nil
		},
	}
	if _, err := svc.Signal(context.Background(), SignalInput{Action: "offer", SDP: rawOffer}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("local disabled offer: %v", err)
	}
	badFingerprint := *owner
	badFingerprint.Fingerprint = "sha-256 " + strings.Repeat("00:", 31) + "00"
	if _, err := svc.Signal(context.Background(), SignalInput{Action: "offer", SDP: rawOffer, Owner: &badFingerprint}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbound DTLS fingerprint: %v", err)
	}
	out, err := svc.Signal(context.Background(), SignalInput{Action: "offer", SDP: rawOffer, Owner: owner})
	if err != nil {
		t.Fatal(err)
	}
	if accessDone == nil || closeAccess == nil || len(out.STUNServers) != 0 {
		t.Fatal("owner peer lacks registered lifetime or inherited manual STUN")
	}
	svc.mu.Lock()
	entry := svc.peers[out.PeerID]
	svc.mu.Unlock()
	configuration := entry.pc.GetConfiguration()
	if len(configuration.ICEServers) != len(hostServers) {
		t.Fatalf("owner ICE isolation failed: %d servers", len(configuration.ICEServers))
	}
	if useTURN {
		if configuration.ICEServers[0].Username != "host" || configuration.ICEServers[0].Credential != "test-host-password" {
			t.Fatal("host did not receive its own TURN credentials")
		}
		hostRelay := strings.Contains(out.SDP, " typ relay")
		for _, candidate := range out.Candidates {
			hostRelay = hostRelay || strings.Contains(candidate.Candidate, " typ relay")
		}
		if !hostRelay || !strings.Contains(rawOffer, " typ relay") {
			t.Fatal("both actual Pion endpoints must gather authenticated TURN candidates")
		}
	}
	if err := client.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: out.SDP}); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range out.Candidates {
		if err := client.AddICECandidate(webrtc.ICECandidateInit{Candidate: candidate.Candidate, SDPMid: candidate.SDPMid, SDPMLineIndex: candidate.SDPMLineIndex}); err != nil {
			t.Fatal(err)
		}
	}
	waitOwnerEvent(t, opened, "owner Pion connection did not open")
	if _, err := svc.Signal(context.Background(), SignalInput{Action: "offer", SDP: rawOffer, Owner: owner}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("owner peer connection ID reused: %v", err)
	}
	otherContext, cancelOther := context.WithCancel(context.Background())
	defer cancelOther()
	wrongOwner, wrongConnection, wrongDTLS, wrongContext := *owner, *owner, *owner, *owner
	wrongOwner.OwnerID = "other-owner"
	wrongConnection.PeerConnectionID = "other-connection"
	wrongDTLS.Fingerprint = badFingerprint.Fingerprint
	wrongContext.Context = otherContext
	for _, authorization := range []*OwnerAuthorization{nil, &wrongOwner, &wrongConnection, &wrongDTLS, &wrongContext} {
		for _, action := range []string{"candidate", "close"} {
			if _, err := svc.Signal(context.Background(), SignalInput{Action: action, PeerID: out.PeerID, Owner: authorization}); !errors.Is(err, ErrPeerMissing) {
				t.Fatalf("cross-scope %s accepted: %v", action, err)
			}
		}
	}
	if _, err := svc.Signal(context.Background(), SignalInput{Action: "candidate", PeerID: out.PeerID, Owner: owner}); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.OwnerPeerFingerprint(owner.OwnerID, owner.PeerConnectionID, out.PeerID); err != nil || got != fingerprint {
		t.Fatalf("trusted owner binding lookup failed: %v", err)
	}
	// Local peers remain governed only by their independent toggle.
	svc.Reconfigure(Config{Enabled: true})
	local, err := svc.Signal(context.Background(), SignalInput{Action: "offer", SDP: rawOffer})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"candidate", "close"} {
		if _, err := svc.Signal(context.Background(), SignalInput{Action: action, PeerID: local.PeerID, Owner: owner}); !errors.Is(err, ErrPeerMissing) {
			t.Fatalf("owner modified local peer: %v", err)
		}
	}
	svc.Reconfigure(Config{Enabled: false})
	if status := svc.Status(); status.ActivePeers != 1 || status.Peers[0].ID != out.PeerID || svc.Enabled() {
		t.Fatal("local toggle closed or enabled the independent owner connection")
	}
	if err := control.SendText(`{"type":"peer_meta","candidateType":"spoofed","ipFamily":"ipv6"}`); err != nil {
		t.Fatal(err)
	}
	if err := control.SendText(`{"type":"ping","seq":7}`); err != nil {
		t.Fatal(err)
	}
	select {
	case reply := <-replies:
		if reply.Type != "pong" || reply.Seq != 7 {
			t.Fatal("owner application ping failed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("owner application ping timed out")
	}
	diagnostic := svc.Status().Peers[0]
	wantPath := "webrtc-direct"
	if useTURN {
		wantPath = "webrtc-turn"
	}
	if diagnostic.Path != wantPath || diagnostic.Transport != "udp" || diagnostic.OwnerID != owner.OwnerID || diagnostic.CandidateType == "spoofed" || diagnostic.IPFamily != "ipv4" {
		t.Fatalf("diagnostic did not use actual selected pair: %+v", diagnostic)
	}
	_, forbiddenStreamClosed := ownerClientChannel(t, client, "stream:/ws/settings")
	waitOwnerEvent(t, forbiddenStreamClosed, "owner settings stream was not rejected")
	forbiddenRPC, forbiddenRPCClosed := ownerClientChannel(t, client, "rpc:forbidden")
	header, _ := json.Marshal(rpcRequestWire{Method: "POST", Path: "/api/uploads", BodyLength: 0})
	if err := forbiddenRPC.SendText(string(header)); err != nil {
		t.Fatal(err)
	}
	waitOwnerEvent(t, forbiddenRPCClosed, "owner upload RPC was not rejected")
	for _, bodyLength := range []int{1, maxOwnerRPCBodyBytes + 1} {
		invalidRPC, closed := ownerClientChannel(t, client, "rpc:invalid-body")
		header, _ := json.Marshal(rpcRequestWire{Method: "GET", Path: "/api/sessions", BodyLength: bodyLength})
		if err := invalidRPC.SendText(string(header)); err != nil {
			t.Fatal(err)
		}
		if bodyLength == 1 {
			if err := invalidRPC.Send([]byte{}); err != nil {
				t.Fatal(err)
			}
		}
		waitOwnerEvent(t, closed, "owner RPC body mismatch or limit was not rejected")
	}
	select {
	case path := <-streamStarted:
		t.Fatalf("forbidden stream dispatched: %s", path)
	case request := <-rpcStarted:
		t.Fatalf("forbidden RPC dispatched: %s", request.Path)
	default:
	}
	const terminalPath = "/ws/s/terminal?client=browser&generation=g1&offset=123"
	_, _ = ownerClientChannel(t, client, "stream:"+terminalPath)
	select {
	case path := <-streamStarted:
		if path != terminalPath {
			t.Fatalf("terminal resume position was changed: %s", path)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("owner terminal was not dispatched")
	}
	rpc, _ := ownerClientChannel(t, client, "rpc:list")
	header, _ = json.Marshal(rpcRequestWire{Method: "GET", Path: "/api/sessions", Headers: map[string]string{"Authorization": "Bearer browser-supplied"}, BodyLength: 0})
	if err := rpc.SendText(string(header)); err != nil {
		t.Fatal(err)
	}
	if err := rpc.Send([]byte{}); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-rpcStarted:
		if request.Path != "/api/sessions" || len(request.Headers) != 0 {
			t.Fatal("owner RPC did not preserve its strict identity boundary")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("owner list RPC was not dispatched")
	}
	if useTURN {
		closeAccess()
	} else {
		revoke()
	}
	for range 2 {
		waitOwnerEvent(t, childCanceled, "revocation did not cancel a live owner child")
	}
	select {
	case <-accessDone:
		t.Fatal("owner access reported closed before its children joined")
	default:
	}
	close(childrenMayExit)
	waitOwnerEvent(t, accessDone, "actual owner peer did not close")
	waitOwnerEvent(t, released, "actual owner peer registration was not released")
	if entry.pc.ConnectionState() != webrtc.PeerConnectionStateClosed || svc.Status().ActivePeers != 0 {
		t.Fatal("revocation left actual Pion access alive")
	}
}

func TestOfferFingerprintCanonicalAndConflictingIdentities(t *testing.T) {
	const prefix = "v=0\r\no=- 1 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"
	const media = "m=application 9 UDP/DTLS/SCTP webrtc-datachannel\r\n"
	lower := "sha-256 " + strings.Repeat("ab:", 31) + "ab"
	upper := "sha-256 " + strings.Repeat("AB:", 31) + "AB"
	for _, raw := range []string{
		prefix + "a=fingerprint:" + lower + "\r\n" + media,
		prefix + media + "a=fingerprint:" + lower + "\r\n",
	} {
		if got, err := OfferFingerprint(raw); err != nil || got != upper {
			t.Fatalf("canonical fingerprint failed: %q %v", got, err)
		}
	}
	other := "sha-256 " + strings.Repeat("CD:", 31) + "CD"
	for _, raw := range []string{
		prefix + media,
		prefix + media + "a=fingerprint:sha-1 00:00\r\n",
		prefix + media + "a=fingerprint:" + strings.Replace(lower, "ab", "zz", 1) + "\r\n",
		prefix + "a=fingerprint:" + lower + "\r\n" + media + "a=fingerprint:" + other + "\r\n",
		prefix + media + "a=fingerprint:" + lower + "\r\n" + media,
	} {
		if _, err := OfferFingerprint(raw); err == nil {
			t.Fatal("unbound or conflicting SDP identity accepted")
		}
	}
}

func TestOwnerPionExpiryAndRejectedAccessCloseActualPeer(t *testing.T) {
	for _, reject := range []bool{false, true} {
		name := "expires"
		if reject {
			name = "registration-rejected"
		}
		t.Run(name, func(t *testing.T) {
			svc := New(Options{Enabled: false})
			defer svc.Close()
			client := ownerClient(t, webrtc.Configuration{})
			if _, err := client.CreateDataChannel(controlChannelLabel, nil); err != nil {
				t.Fatal(err)
			}
			offer, err := client.CreateOffer(nil)
			if err != nil {
				t.Fatal(err)
			}
			fingerprint, err := OfferFingerprint(offer.SDP)
			if err != nil {
				t.Fatal(err)
			}
			lifetime, cancel := context.WithCancel(context.Background())
			defer cancel()
			var done <-chan struct{}
			released := make(chan struct{})
			denied := errors.New("access registration denied")
			owner := &OwnerAuthorization{
				OwnerID: "session", Context: lifetime, PeerConnectionID: "connection",
				Fingerprint: fingerprint, ExpiresAt: time.Now().Add(time.Second),
				RegisterAccess: func(_ func(), joined <-chan struct{}) (func(), error) {
					done = joined
					if reject {
						return nil, denied
					}
					return func() { close(released) }, nil
				},
			}
			_, err = svc.Signal(context.Background(), SignalInput{Action: "offer", SDP: offer.SDP, Owner: owner})
			if reject && !errors.Is(err, denied) {
				t.Fatalf("registration rejection was lost: %v", err)
			}
			if !reject && err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("unexpected expiry negotiation failure: %v", err)
			}
			if done == nil {
				t.Fatal("actual peer was not registered")
			}
			waitOwnerEvent(t, done, "expiry or rejected access left an actual peer alive")
			if !reject {
				waitOwnerEvent(t, released, "expired peer retained its access registration")
			}
			if svc.Status().ActivePeers != 0 {
				t.Fatal("expired or rejected peer retained application access")
			}
		})
	}
}
