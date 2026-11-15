package assemble

import (
	"crypto/tls"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/relay"
)

func RelayOptions(c config.RelayConfig, loopbackAddress string, loopbackTLS *tls.Config) relay.Config {
	return relay.Config{ControlEndpoint: c.ControlEndpoint, ControlAudience: c.ControlAudience, InitialTrust: c.InitialTrust, LoopbackAddress: loopbackAddress, LoopbackTLSConfig: loopbackTLS}
}
