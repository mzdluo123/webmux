package assemble

import (
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/relay"
)

func RelayOptions(c config.RelayConfig) relay.Config {
	return relay.Config{ControlEndpoint: c.ControlEndpoint, ControlAudience: c.ControlAudience}
}
