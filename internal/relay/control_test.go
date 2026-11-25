package relay

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

func TestControlExchangeICEAndOwnerIsolation(t *testing.T) {
	id := newTestIdentity(t)
	key := testKey(t)
	a := testAssignment(t, id)
	handler, controls := nodeFixture(t, key, a)
	remote := tlsTestServer(t, id, handler)
	c := newTestClient(t, id, remote.Listener.Addr().String())
	owned := cloneAssignment(a)
	c.assignment = &owned
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	callbacks := testCallbacks()
	ready := make(chan struct{})
	callbacks.Ready = func(protocol.Scope) { close(ready) }
	go func() { done <- c.Connect(ctx, key, a, callbacks) }()
	control := awaitRole(t, controls)
	stream, result := openNodeStream(t, control, protocol.StreamControl, protocol.RoleControl, control.scope)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("framed stream did not publish control readiness")
	}
	// A delayed FRP probe expires independently of the management RPC stream.
	expired := protocol.ControlMessage{Version: protocol.V1, Type: protocol.ControlFRPStatus, RequestID: testToken(t), Scope: control.scope,
		ExpiresAt: time.Now().Add(-time.Second).Unix(), AllocationStatus: &protocol.AllocationStatus{AllocationID: testToken(t), GrantRevision: 1, DataReady: true, CheckedAt: time.Now().Add(-2 * time.Second).UnixMilli()}}
	if err := protocol.WriteFrame(stream, expired, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	exchange := protocol.LoginExchange{Code: testToken(t), Verifier: strings.Repeat("a", 43), RedirectURI: a.Binding.PublicURL + protocol.HostLoginCallback}
	type grantResult struct {
		grant protocol.BrowserGrant
		err   error
	}
	granted := make(chan grantResult, 1)
	go func() { grant, err := c.Exchange(ctx, exchange); granted <- grantResult{grant, err} }()
	request := readManagement(t, stream, control.scope)
	if request.Type != protocol.ControlExchange || request.Exchange == nil || *request.Exchange != exchange {
		t.Fatal("exchange payload not preserved")
	}
	grant := protocol.BrowserGrant{ID: testToken(t), ConsoleSessionID: testToken(t), OwnerID: a.Binding.OwnerID, HostID: control.scope.HostID, BindingGeneration: control.scope.BindingGeneration, ConnectorGeneration: control.scope.ConnectorGeneration, ControlEpoch: control.scope.ControlEpoch, NodeEpoch: control.scope.NodeEpoch, Origin: a.Binding.PublicURL, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	response := protocol.ControlMessage{Version: protocol.V1, Type: protocol.ControlGrant, RequestID: request.RequestID, Scope: control.scope, ExpiresAt: time.Now().Add(9 * time.Second).Unix(), Grant: &grant}
	if err := protocol.WriteFrame(stream, response, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-granted:
		if got.err != nil || got.grant != grant {
			t.Fatalf("grant = %#v, %v", got.grant, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("grant RPC did not finish")
	}

	go func() {
		grant, err := c.Resume(ctx, protocol.LoginResume{GrantID: grant.ID})
		granted <- grantResult{grant, err}
	}()
	request = readManagement(t, stream, control.scope)
	if request.Type != protocol.ControlResume || request.Resume == nil || request.Resume.GrantID != grant.ID {
		t.Fatal("resume grant identity not preserved")
	}
	response.RequestID = request.RequestID
	if err := protocol.WriteFrame(stream, response, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-granted:
		if got.err != nil || got.grant != grant {
			t.Fatalf("resumed grant = %#v, %v", got.grant, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("resume RPC did not finish")
	}

	iceRequest := protocol.ICERequest{GrantID: grant.ID, PeerConnectionID: testToken(t)}
	type iceResult struct {
		config protocol.ICEConfig
		err    error
	}
	configured := make(chan iceResult, 1)
	go func() { config, err := c.ICE(ctx, iceRequest); configured <- iceResult{config, err} }()
	request = readManagement(t, stream, control.scope)
	if request.Type != protocol.ControlICERequest || request.ICERequest == nil || *request.ICERequest != iceRequest {
		t.Fatal("ICE owner request not preserved")
	}
	accountKey := testToken(t)
	browserExpiry := time.Now().Add(90 * time.Second).Unix()
	hostExpiry := browserExpiry + 1
	browserName, err := protocol.TURNUsername(browserExpiry, accountKey)
	if err != nil {
		t.Fatal(err)
	}
	hostName, err := protocol.TURNUsername(hostExpiry, accountKey)
	if err != nil {
		t.Fatal(err)
	}
	ice := protocol.ICEConfig{NodeID: "separate-turn-node", ConfigRevision: 3, Secret: protocol.SecretRef{Purpose: protocol.SecretTURN, KeyID: "turn-key", Version: 2}, GrantID: grant.ID, PeerConnectionID: iceRequest.PeerConnectionID, ExpiresAt: browserExpiry, Browser: protocol.ICEEndpoint{Role: "browser", ExpiresAt: browserExpiry, Servers: []protocol.ICEServer{{URLs: []string{"turn:turn.example:3478?transport=udp"}, Username: browserName, Credential: "browser-secret"}}}, Host: protocol.ICEEndpoint{Role: "host", ExpiresAt: hostExpiry, Servers: []protocol.ICEServer{{URLs: []string{"turn:turn.example:3478?transport=udp"}, Username: hostName, Credential: "host-secret"}}}}
	response = protocol.ControlMessage{Version: protocol.V1, Type: protocol.ControlICEConfig, RequestID: request.RequestID, Scope: control.scope, ExpiresAt: time.Now().Add(9 * time.Second).Unix(), ICEConfig: &ice}
	if err := protocol.WriteFrame(stream, response, protocol.MaxControlBytes); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-configured:
		if got.err != nil || got.config.NodeID != ice.NodeID || got.config.Host.Servers[0].Credential != "host-secret" || got.config.GrantID != grant.ID {
			t.Fatalf("ICE RPC = %#v, %v", got.config, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("ICE RPC did not finish")
	}

	// A structurally valid grant for a different owner is NOT usable, even
	// though all connector scope fields and its public host are correct.
	go func() { grant, err := c.Exchange(ctx, exchange); granted <- grantResult{grant, err} }()
	request = readManagement(t, stream, control.scope)
	grant.OwnerID = "456"
	response = protocol.ControlMessage{Version: protocol.V1, Type: protocol.ControlGrant, RequestID: request.RequestID, Scope: control.scope, ExpiresAt: time.Now().Add(9 * time.Second).Unix(), Grant: &grant}
	// The host can reject the grant and close the connection before smux
	// returns the write result. Only tolerate that closure: the Forbidden
	// result from Connect below proves the response reached owner validation.
	if err := protocol.WriteFrame(stream, response, protocol.MaxControlBytes); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	select {
	case got := <-granted:
		if got.err == nil {
			t.Fatal("wrong owner grant accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("invalid grant did not stop RPC")
	}
	select {
	case err := <-done:
		if protocolCode(err) != protocol.Forbidden {
			t.Fatalf("wrong owner grant closed generation with %v, want %s", err, protocol.Forbidden)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("invalid grant did not close generation")
	}
	if _, ok := c.Scope(); ok {
		t.Fatal("invalid owner left active scope")
	}
}

func TestFRPObservationUsesActualProbeTime(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	grant := protocol.AllocationGrant{AllocationID: testToken(t), GrantRevision: 2, ExpiresAt: now.Add(time.Minute).Unix()}
	st := timedFRPStatus{AllocationStatus: protocol.AllocationStatus{
		AllocationID: grant.AllocationID, GrantRevision: grant.GrantRevision,
		DataReady: true, CheckedAt: now.Add(-time.Second).UnixMilli(),
	}, until: now.Add(time.Minute)}
	got, ok := st.observation(now, &grant)
	checkedAt := time.UnixMilli(st.CheckedAt)
	if !ok || !got.Ready || !got.CheckedAt.Equal(checkedAt) || !got.ValidUntil.Equal(checkedAt.Add(frpObservationTTL)) {
		t.Fatalf("fresh observation: %+v, matched=%t", got, ok)
	}
	st.CheckedAt = now.Add(-frpObservationTTL - time.Second).UnixMilli()
	got, ok = st.observation(now, &grant)
	if !ok || got.CheckedAt.UnixMilli() != st.CheckedAt || now.Before(got.ValidUntil) {
		t.Fatalf("delayed receipt made an old probe fresh: %+v, matched=%t", got, ok)
	}
	st.CheckedAt = 0
	got, ok = st.observation(now, &grant)
	if !ok || got != (DataObservation{}) {
		t.Fatalf("unsampled notification invented a result: %+v, matched=%t", got, ok)
	}
	st.GrantRevision--
	if _, ok := st.observation(now, &grant); ok {
		t.Fatal("older grant supplied the current observation")
	}
	st.GrantRevision++
	st.until = now
	if _, ok := st.observation(now, &grant); ok {
		t.Fatal("expired envelope supplied an observation")
	}
}
