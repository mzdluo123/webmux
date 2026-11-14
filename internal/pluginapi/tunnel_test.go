package pluginapi

import (
	"net/http/httptest"
	"testing"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/transport"
)

// These reads must use the live settings view, not the startup TOML values.
// Embed the mutation boundary because this test exercises only Settings.
type tunnelSettingsView struct {
	TunnelSettingsControl
	view contract.TunnelSettings
}

func (c tunnelSettingsView) Settings() contract.TunnelSettings { return c.view }

func TestTunnelDynamicConfigAndPushOrigin(t *testing.T) {
	control := tunnelSettingsView{view: contract.TunnelSettings{Mode: "named", Hostname: "new.example.com"}}
	public := &transport.PublicOrigins{}
	if err := public.Set("cloudflare", "https://new.example.com"); err != nil {
		t.Fatal(err)
	}
	s := &Server{Deps: Deps{
		Config:         config.Config{Tunnel: config.TunnelConfig{Token: "old-private", Hostname: "old.example.com"}},
		Public:         public,
		TunnelSettings: control,
	}}
	view := contract.ServerConfig{}
	s.EnrichConfig(&view)
	if got := view.Tunnel; !got.Named || got.Hostname != "new.example.com" {
		t.Fatalf("stale config %+v", got)
	}
	// The live Named hostname anchors subscriptions. The stale TOML
	// hostname is no longer authorized, even though push normalization
	// accepts a syntactically valid HTTPS origin.
	for _, row := range []struct {
		host       string
		authorized bool
		want       string
	}{
		{"new.example.com", true, "https://new.example.com"},
		{"old.example.com", false, "https://old.example.com"},
	} {
		// Tunnel and reverse-proxy traffic reaches the server over plain HTTP.
		r := httptest.NewRequest("GET", "http://"+row.host+"/api/push", nil)
		r.Header.Set("Origin", "https://"+row.host)
		if got := transport.OriginAllowed(s.Config, s.Public, r); got != row.authorized {
			t.Fatalf("%s authorized = %v, want %v", row.host, got, row.authorized)
		}
		origin, reason := s.pushOrigin(r)
		if origin != row.want || reason != "" {
			t.Fatalf("%s push origin (%q, %q)", row.host, origin, reason)
		}
	}
}
