package assemble

import (
	"crypto/tls"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/relay"
)

func RelayOptions(c config.RelayConfig, loopbackAddress string, loopbackTLS *tls.Config) relay.Config {
	return relay.Config{InteractiveConnections: c.InteractiveConnections, BulkConnections: c.BulkConnections, ControlEndpoint: c.ControlEndpoint, ControlAudience: c.ControlAudience, LoopbackAddress: loopbackAddress, LoopbackTLSConfig: loopbackTLS}
}
