package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/frphelper"
	"github.com/xtaci/smux"
)

func (c *Client) dial(ctx context.Context, key ed25519.PrivateKey, a protocol.Assignment) (_ *physical, err error) {
	authCtx, cancel := context.WithTimeout(ctx, protocol.AuthTimeout)
	defer cancel()
	httpClient := c.httpClient(&a.Trust)
	defer httpClient.CloseIdleConnections()
	var headers http.Header
	if frphelper.Available() == nil {
		headers = http.Header{protocol.TransportHeader: []string{protocol.TransportSelectionV1}}
	}
	ws, response, err := websocket.Dial(authCtx, a.ConnectorURL, &websocket.DialOptions{
		HTTPClient: httpClient, Subprotocols: []string{protocol.ConnectorSubprotocol}, CompressionMode: websocket.CompressionDisabled,
		HTTPHeader: headers,
	})
	if err != nil {
		if response != nil && response.StatusCode >= 300 && response.StatusCode < 400 {
			return nil, protocol.Error{Code: protocol.Forbidden}
		}
		if response != nil {
			if retryAfter := relayRetryAfter(response.Header.Get("Retry-After"), time.Now()); retryAfter > 0 {
				if response.StatusCode == http.StatusTooManyRequests {
					return nil, protocol.Error{Code: protocol.RateLimited, RetryAfterSeconds: retryAfter}
				}
				return nil, protocol.Error{Code: protocol.Unavailable, RetryAfterSeconds: retryAfter}
			}
		}
		return nil, transportError(authCtx, err)
	}
	defer func() {
		if err != nil {
			_ = ws.CloseNow()
		}
	}()
	if ws.Subprotocol() != protocol.ConnectorSubprotocol {
		return nil, protocol.Error{Code: protocol.Incompatible}
	}
	ws.SetReadLimit(protocol.MaxAuthBytes)
	hello := protocol.ConnectorHello{Version: protocol.V1, HostID: a.Binding.HostID, BindingGeneration: a.Binding.BindingGeneration, Role: protocol.RoleControl}
	if err := writeAuth(authCtx, ws, hello); err != nil {
		return nil, err
	}
	var challenge protocol.ConnectorChallenge
	if err := readAuth(authCtx, ws, &challenge); err != nil {
		return nil, err
	}
	if err := checkChallenge(challenge, a, time.Now()); err != nil {
		return nil, err
	}
	proof, err := protocol.SignConnector(key, challenge, time.Now())
	if err != nil {
		return nil, err
	}
	if err := writeAuth(authCtx, ws, proof); err != nil {
		return nil, err
	}
	var ready protocol.ConnectorReady
	if err := readAuth(authCtx, ws, &ready); err != nil {
		return nil, err
	}
	scope := protocol.Scope{HostID: challenge.HostID, BindingGeneration: challenge.BindingGeneration, ConnectorGeneration: challenge.ConnectorGeneration, ControlEpoch: challenge.ControlEpoch, NodeEpoch: challenge.NodeEpoch}
	if ready.Version != protocol.V1 || ready.HostID != scope.HostID || ready.BindingGeneration != scope.BindingGeneration || ready.ConnectorGeneration != scope.ConnectorGeneration || ready.ControlEpoch != scope.ControlEpoch || ready.NodeEpoch != scope.NodeEpoch || ready.Role != protocol.RoleControl {
		return nil, protocol.Error{Code: protocol.StaleGeneration}
	}
	conn := websocket.NetConn(ctx, ws, websocket.MessageBinary)
	// NetConn disables this limit, so restore it AFTER creating the adapter.
	ws.SetReadLimit(maxBinaryMessage)
	mux, err := smux.Client(conn, smuxConfig())
	if err != nil {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	offer := response.Header.Values(protocol.TransportHeader)
	selection := headers != nil && len(offer) == 1 && offer[0] == protocol.TransportSelectionV1
	return &physical{ws: ws, mux: mux, scope: scope, selection: selection}, nil
}

func checkChallenge(challenge protocol.ConnectorChallenge, a protocol.Assignment, now time.Time) error {
	if err := challenge.Validate(now); err != nil {
		return err
	}
	if challenge.Audience != a.NodeAudience || challenge.HostID != a.Binding.HostID || challenge.BindingGeneration != a.Binding.BindingGeneration || challenge.Role != protocol.RoleControl || challenge.ControlEpoch != a.ControlEpoch {
		return protocol.Error{Code: protocol.WrongAudience}
	}
	return nil
}

func smuxConfig() *smux.Config {
	cfg := smux.DefaultConfig()
	cfg.Version = 2
	cfg.MaxReceiveBuffer = max(1, connectionBuffer)
	cfg.MaxStreamBuffer = min(streamBuffer, cfg.MaxReceiveBuffer)
	cfg.KeepAliveInterval = protocol.KeepAliveInterval
	cfg.KeepAliveTimeout = protocol.ControlLease
	return cfg
}

func writeAuth(ctx context.Context, ws *websocket.Conn, value any) error {
	data, err := json.Marshal(value)
	if err != nil || len(data) > protocol.MaxAuthBytes {
		return protocol.Error{Code: protocol.Invalid}
	}
	if err := ws.Write(ctx, websocket.MessageText, data); err != nil {
		return transportError(ctx, err)
	}
	return nil
}

func readAuth(ctx context.Context, ws *websocket.Conn, value any) error {
	kind, data, err := ws.Read(ctx)
	if err != nil {
		return transportError(ctx, err)
	}
	if kind != websocket.MessageText {
		return protocol.Error{Code: protocol.Invalid}
	}
	if err := protocol.Decode(data, value, protocol.MaxAuthBytes); err == nil {
		return nil
	}
	var failure protocol.Error
	if protocol.Decode(data, &failure, protocol.MaxAuthBytes) == nil && failure.Code.Valid() {
		return protocol.Error{Code: failure.Code, RetryAfterSeconds: failure.RetryAfterSeconds}
	}
	return protocol.Error{Code: protocol.Invalid}
}

func newToken() (protocol.Token, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", protocol.Error{Code: protocol.Unavailable}
	}
	return protocol.Token(base64.RawURLEncoding.EncodeToString(b[:])), nil
}
