package pluginapi

import "github.com/rainchan/webmux/internal/transport"

func authorityHostname(authority string) string { return transport.AuthorityHostname(authority) }
func normalizeAuthority(authority string) (string, bool) {
	return transport.NormalizeAuthority(authority)
}
