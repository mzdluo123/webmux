package relay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"

	"github.com/coder/websocket"
	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// relayDebug never logs error strings: network errors may contain credentials,
// and WebSocket close reasons and protocol fields are peer-controlled.
func relayDebug(stage string, role protocol.Role, err error, kinds ...protocol.ControlType) {
	if os.Getenv("WEBMUX_RELAY_DEBUG") != "1" {
		return
	}
	code := ""
	var remote protocol.Error
	if errors.As(err, &remote) && remote.Code.Valid() {
		code = string(remote.Code)
	}
	var timeout net.Error
	kind := ""
	if len(kinds) != 0 {
		kind = string(kinds[0])
	}
	slog.Info("relay diagnostic", "stage", stage, "role", string(role), "message_type", kind,
		"failed", err != nil, "code", code, "error_type", fmt.Sprintf("%T", err),
		"canceled", errors.Is(err, context.Canceled),
		"timeout", errors.Is(err, context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout(),
		"eof", errors.Is(err, io.EOF), "closed", errors.Is(err, net.ErrClosed),
		"ws_close", int(websocket.CloseStatus(err)))
}
