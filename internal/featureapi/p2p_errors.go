package featureapi

import (
	"errors"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/p2p"
	"github.com/rainchan/webmux/internal/p2psettings"
)

func p2pSettingsStatus(err error) int {
	switch {
	case errors.Is(err, p2psettings.ErrInvalid):
		return 400
	case errors.Is(err, p2psettings.ErrUnavailable):
		return 503
	default:
		return 500
	}
}

func p2pSignalStatus(err error) int {
	var remote protocol.Error
	if errors.As(err, &remote) {
		return protocol.HTTPStatus(remote.Code)
	}
	switch {
	case errors.Is(err, p2p.ErrInvalid):
		return 400
	case errors.Is(err, p2p.ErrPeerMissing):
		return 404
	case errors.Is(err, p2p.ErrDisabled), errors.Is(err, p2p.ErrUnsupported):
		return 409
	case errors.Is(err, p2p.ErrRateLimited), errors.Is(err, p2p.ErrTooMany):
		return 429
	case cancelled(err):
		return 504
	default:
		return 500
	}
}
