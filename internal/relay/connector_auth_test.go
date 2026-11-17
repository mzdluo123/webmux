package relay

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

func TestAuxiliaryAuthorizationRejectionClosesLogicalConnection(t *testing.T) {
	id := newTestIdentity(t)
	key := testKey(t)
	a := testAssignment(t, id)
	next, controls, _ := nodeFixture(t, id, key, a)
	var attempts atomic.Int32
	remote := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{protocol.ConnectorSubprotocol}})
		if err != nil {
			return
		}
		defer ws.CloseNow()
		var hello protocol.ConnectorHello
		if readAuth(r.Context(), ws, &hello) == nil {
			_ = writeAuth(r.Context(), ws, protocol.Error{Code: protocol.Forbidden})
		}
	}), false)
	c := newTestClient(t, id, remote.Listener.Addr().String(), "127.0.0.1:12345")
	c.assignment = &a
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	callbacks := testCallbacks()
	var disconnected atomic.Bool
	callbacks.Disconnected = func() { disconnected.Store(true) }
	done := make(chan error, 1)
	go func() { done <- c.Connect(ctx, key, a, callbacks) }()
	primary := awaitRole(t, controls)
	_, result := openNodeStream(t, primary, protocol.StreamControl, primary.scope)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	select {
	case err := <-done:
		var failure protocol.Error
		if !errors.As(err, &failure) || failure.Code != protocol.Forbidden || !disconnected.Load() {
			t.Fatalf("authorization rejection: err=%v disconnected=%v", err, disconnected.Load())
		}
		if _, ok := c.Scope(); ok {
			t.Fatal("rejected connector retained authorization scope")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("authorization rejection did not close logical connection")
	}
}

func TestAuxiliaryOperationalErrorsRemainRetryable(t *testing.T) {
	for _, code := range []protocol.Code{protocol.Unavailable, protocol.RateLimited, protocol.StaleGeneration, protocol.Expired, protocol.UpstreamFailed, protocol.Conflict, protocol.Invalid} {
		if permanentError(protocol.Error{Code: code}) {
			t.Fatalf("operational error %s would tear down healthy roles", code)
		}
	}
}
