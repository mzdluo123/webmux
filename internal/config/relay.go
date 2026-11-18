package config

import (
	"errors"
	"net"
	"net/url"
	"strings"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// RelayConfig selects the public control authority. Data endpoints and their
// pinned trust are assigned by authenticated control, never configured here.
type RelayConfig struct {
	InteractiveConnections int    `toml:"interactive_connections"`
	BulkConnections        int    `toml:"bulk_connections"`
	Enabled                bool   `toml:"enabled"`
	ControlEndpoint        string `toml:"control_endpoint"`
	ControlAudience        string `toml:"control_audience"`
	Transport              string `toml:"transport"`
}

// DefaultRelayConfig selects the official control service without enabling access.
func DefaultRelayConfig() RelayConfig {
	return RelayConfig{
		InteractiveConnections: protocol.DefaultInteractiveConnections,
		BulkConnections:        protocol.DefaultBulkConnections,
		ControlEndpoint:        "https://webmux-relay.rainbit.net",
		ControlAudience:        "webmux-relay-control",
		Transport:              "wss",
	}
}

func (c RelayConfig) Validated() (RelayConfig, error) {
	if c.InteractiveConnections == 0 {
		c.InteractiveConnections = protocol.DefaultInteractiveConnections
	}
	if c.BulkConnections == 0 {
		c.BulkConnections = protocol.DefaultBulkConnections
	}
	if c.InteractiveConnections < 1 || c.InteractiveConnections > protocol.MaxRoleConnections || c.BulkConnections < 1 || c.BulkConnections > protocol.MaxRoleConnections {
		return RelayConfig{}, errors.New("relay connection counts must be between 1 and 16")
	}
	if c.Transport == "" {
		c.Transport = "wss"
	}
	if c.Transport != "wss" {
		return RelayConfig{}, errors.New("relay.transport must be wss")
	}
	if c.ControlEndpoint == "" && c.ControlAudience == "" && !c.Enabled {
		return c, nil
	}
	u, err := url.Parse(c.ControlEndpoint)
	if err != nil || len(c.ControlEndpoint) > protocol.MaxURLBytes || u.Scheme != "https" || u.Host == "" || u.User != nil || c.ControlEndpoint != "https://"+u.Host {
		return RelayConfig{}, errors.New("relay.control_endpoint must be an exact HTTPS origin")
	}
	host := u.Hostname()
	if host == "" || len(host) > 253 || net.ParseIP(host) != nil || strings.ToLower(host) != host || (u.Host != host && u.Host != host+":443") {
		return RelayConfig{}, errors.New("relay.control_endpoint must use a canonical DNS name and port 443")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return RelayConfig{}, errors.New("relay.control_endpoint has an invalid DNS name")
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return RelayConfig{}, errors.New("relay.control_endpoint has an invalid DNS name")
			}
		}
	}
	if len(c.ControlAudience) == 0 || len(c.ControlAudience) > 255 {
		return RelayConfig{}, errors.New("relay.control_audience must contain 1 to 255 printable ASCII characters")
	}
	for _, r := range c.ControlAudience {
		if r < 0x21 || r > 0x7e {
			return RelayConfig{}, errors.New("relay.control_audience must contain 1 to 255 printable ASCII characters")
		}
	}
	return c, nil
}
