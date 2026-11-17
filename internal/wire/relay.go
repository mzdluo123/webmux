package wire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/relaysetting"
)

func RelayAuthSession(session relaysetting.AuthSessionStatus) contract.RelayAuthSession {
	return contract.RelayAuthSession{Authenticated: session.Authenticated, CSRFToken: session.CSRFToken, ExpiresAt: session.ExpiresAt, Relay: session.Relay, Reason: session.Reason}
}
