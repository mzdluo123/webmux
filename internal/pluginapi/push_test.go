package pluginapi

import (
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/transport"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPushOrigin(t *testing.T) {
	named := config.Config{Tunnel: config.TunnelConfig{Token: "cred", Hostname: "term.example.com"}}
	quick := &transport.PublicOrigins{}
	if err := quick.Set("cloudflare", "https://abc.trycloudflare.com"); err != nil {
		t.Fatal(err)
	}
	tailscale := &transport.PublicOrigins{}
	if err := tailscale.Set("tailscale", "https://node.tailnet.ts.net:8443"); err != nil {
		t.Fatal(err)
	}
	quickWithNamed := &transport.PublicOrigins{}
	if err := quickWithNamed.Set("cloudflare", "https://abc.trycloudflare.com"); err != nil {
		t.Fatal(err)
	}
	allowedHTTPS := config.Config{AllowedOrigins: []string{"https://proxy.example:9443"}}
	allowedHTTPSPorts := config.Config{AllowedOrigins: []string{"https://proxy.example:9443", "https://proxy.example:8443"}}
	for _, row := range []struct {
		name         string
		server       *Server
		host, origin string
		want, reason string
	}{
		{"named origin", &Server{Deps: Deps{Config: named}}, "term.example.com", "https://term.example.com", "https://term.example.com", ""},
		{"named without origin header", &Server{Deps: Deps{Config: named}}, "term.example.com", "", "https://term.example.com", ""},
		{"named http origin", &Server{Deps: Deps{Config: named}}, "term.example.com", "http://term.example.com", "", pushUnsupportedOrigin},
		{"named origin with default port", &Server{Deps: Deps{Config: named}}, "term.example.com:443", "https://term.example.com:443", "https://term.example.com", ""},
		{"quick tunnel", &Server{Deps: Deps{Public: quick}}, "abc.trycloudflare.com", "https://abc.trycloudflare.com", "", pushQuickTunnel},
		{"quick tunnel without origin header", &Server{Deps: Deps{Public: quick}}, "abc.trycloudflare.com", "", "", pushQuickTunnel},
		{"quick tunnel while named configured", &Server{Deps: Deps{Config: named, Public: quickWithNamed}}, "abc.trycloudflare.com", "https://abc.trycloudflare.com", "", pushQuickTunnel},
		{"ipv4 loopback", &Server{}, "127.0.0.1:7681", "http://127.0.0.1:7681", "http://127.0.0.1:7681", ""},
		{"ipv6 loopback", &Server{}, "[::1]:7681", "http://[::1]:7681", "http://[::1]:7681", ""},
		{"localhost without origin header", &Server{}, "localhost:7681", "", "http://localhost:7681", ""},
		{"vpn address", &Server{Deps: Deps{Config: config.Config{AllowedOrigins: []string{"http://100.64.0.2:7681"}}}}, "100.64.0.2:7681", "http://100.64.0.2:7681", "", pushUnsupportedOrigin},
		{"other https proxy", &Server{Deps: Deps{Config: named}}, "box.tail1234.ts.net", "https://box.tail1234.ts.net", "https://box.tail1234.ts.net", ""},
		{"uppercase https host", &Server{}, "Box.Tail1234.TS.NET", "https://Box.Tail1234.TS.NET", "https://box.tail1234.ts.net", ""},
		{"ipv6 https origin", &Server{}, "[fd7a:115c:a1e0::1]", "https://[fd7a:115c:a1e0::1]", "https://[fd7a:115c:a1e0::1]", ""},
		{"tailscale origin keeps port", &Server{Deps: Deps{Public: tailscale}}, "node.tailnet.ts.net:8443", "https://node.tailnet.ts.net:8443", "https://node.tailnet.ts.net:8443", ""},
		{"tailscale without origin header", &Server{Deps: Deps{Public: tailscale}}, "node.tailnet.ts.net:8443", "", "https://node.tailnet.ts.net:8443", ""},
		{"allowed https origin", &Server{Deps: Deps{Config: allowedHTTPS}}, "proxy.example:9443", "https://proxy.example:9443", "https://proxy.example:9443", ""},
		{"allowed https origin without header", &Server{Deps: Deps{Config: allowedHTTPS}}, "proxy.example:9443", "", "https://proxy.example:9443", ""},
		{"allowed https origin with stripped host port", &Server{Deps: Deps{Config: allowedHTTPS}}, "proxy.example", "", "https://proxy.example:9443", ""},
		{"allowed https origin prefers matching port", &Server{Deps: Deps{Config: allowedHTTPSPorts}}, "proxy.example:8443", "", "https://proxy.example:8443", ""},
		{"https origin with path", &Server{}, "box.example", "https://box.example/path", "", pushUnsupportedOrigin},
		{"https origin with credentials", &Server{}, "box.example", "https://user:pass@box.example", "", pushUnsupportedOrigin},
		{"local socket", &Server{Deps: Deps{Config: named}}, "webmux", "", "", pushUnsupportedOrigin},
	} {
		r := httptest.NewRequest(http.MethodGet, "http://"+row.host+"/api/push", nil)
		if row.origin != "" {
			r.Header.Set("Origin", row.origin)
		}
		if row.name == "local socket" {
			r = r.WithContext(transport.LocalContext(r.Context()))
		}
		origin, reason := row.server.pushOrigin(r)
		if origin != row.want || reason != row.reason {
			t.Errorf("%s: (%q, %q), want (%q, %q)", row.name, origin, reason, row.want, row.reason)
		}
	}
}
