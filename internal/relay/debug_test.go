package relay

import (
	"bytes"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"testing"

	"github.com/coder/websocket"
	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

func TestRelayDebugRedactsPeerAndNetworkErrors(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	t.Setenv("WEBMUX_RELAY_DEBUG", "1")
	const secret = "secret-must-not-be-logged"
	for _, err := range []error{
		&url.Error{Op: "Get", URL: "https://user:" + secret + "@example.test/", Err: errors.New(secret)},
		websocket.CloseError{Code: websocket.StatusPolicyViolation, Reason: secret},
		protocol.Error{Code: protocol.Code(secret)},
	} {
		relayDebug("wss-dial", protocol.RoleControl, err)
	}
	if strings.Contains(logs.String(), secret) {
		t.Fatal("diagnostics exposed peer-controlled secret")
	}
	if !strings.Contains(logs.String(), `"ws_close":1008`) {
		t.Fatal("diagnostics lost WebSocket close status")
	}
	logs.Reset()
	t.Setenv("WEBMUX_RELAY_DEBUG", "0")
	relayDebug("wss-dial", protocol.RoleControl, errors.New(secret))
	if logs.Len() != 0 {
		t.Fatal("disabled diagnostics emitted logs")
	}
}
