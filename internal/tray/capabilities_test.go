package tray

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/builtin"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

type capabilityDialer struct{ address string }

func (d capabilityDialer) DialContext(ctx context.Context) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "tcp", d.address)
}

func TestTrayUsesDiscoveredCapabilitiesIndependently(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		cloudflare, tailscale, push, invalid bool
		pushReason                           string
		providersOff                         bool
	}{
		{name: "core only"},
		{name: "cloudflare only", cloudflare: true},
		{name: "tailscale only", tailscale: true},
		{name: "notifications only", push: true},
		{name: "all providers", cloudflare: true, tailscale: true, push: true},
		{name: "push configuration off", push: true, pushReason: contract.CapabilityReasonConfigurationOff},
		{name: "push storage unavailable", push: true, pushReason: contract.CapabilityReasonStorageUnavailable},
		{name: "access providers configured off", cloudflare: true, tailscale: true, providersOff: true},
		{name: "invalid discovery", cloudflare: true, tailscale: true, push: true, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enabled := map[string]bool{"cloudflare": tc.cloudflare, "tailscale": tc.tailscale, "notifications": tc.push}
			snapshot := contract.Capabilities{Version: 1, Plugins: []contract.PluginStatus{}, Capabilities: []contract.Capability{{ID: "terminal.pty", Available: true}}}
			for plugin, capability := range map[string]string{"cloudflare": "cloudflare.manage", "tailscale": "tailscale.manage", "notifications": "notifications.push"} {
				value := contract.Capability{ID: capability, Available: enabled[plugin]}
				if !value.Available {
					value.Reason = contract.CapabilityReasonPluginDisabled
				} else if plugin == "notifications" && tc.pushReason != "" {
					value.Available = false
					value.Reason = tc.pushReason
				}
				snapshot.Capabilities = append(snapshot.Capabilities, value)
			}
			for _, p := range builtin.Catalog() {
				status := contract.PluginStatus{ID: p.ID, Status: contract.PluginStatusDisabled, Reason: contract.CapabilityReasonPluginDisabled}
				if enabled[p.ID] {
					status.Enabled = true
					status.Status = contract.PluginStatusReady
					status.Reason = ""
					if p.ID == "notifications" && tc.pushReason != "" && tc.pushReason != contract.CapabilityReasonConfigurationOff {
						status.Status = contract.PluginStatusUnavailable
						status.Reason = tc.pushReason
					}
				}
				snapshot.Plugins = append(snapshot.Plugins, status)
			}
			if tc.invalid {
				snapshot.Version = 99
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != contract.PathCapabilities {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(snapshot)
			}))
			t.Cleanup(server.Close)
			service := &WSLService{Dial: func(Target) localclient.Dialer {
				return capabilityDialer{address: strings.TrimPrefix(server.URL, "http://")}
			}}
			t.Cleanup(service.Close)
			remote := []contract.RemoteURL{{URL: "http://192.168.1.2:8080", Kind: "lan"}, {URL: "http://100.64.0.1:8080", Kind: "tailscale"}}
			links := []contract.AccessLink{{Provider: "cloudflare", URL: "https://tunnel.example"}, {Provider: "tailscale", URL: "https://device.example"}}
			tunnel := &contract.TunnelStatus{State: contract.TunnelReady, PublicURL: links[0].URL, SettingsAvailable: true}
			tailscale := &contract.TailscaleSummary{}
			if tc.providersOff {
				tunnel.State, tunnel.PublicURL = contract.TunnelOff, ""
				tailscale.Mode, tailscale.State = "off", contract.TailscaleOff
			}
			push := &contract.PushStatus{Enabled: tc.pushReason != contract.CapabilityReasonConfigurationOff, Available: tc.pushReason == ""}
			if tc.pushReason == contract.CapabilityReasonStorageUnavailable {
				push.Error = "subscription storage unavailable"
			}
			status := service.capabilityStatus(context.Background(), contract.StatusV1{
				State:     contract.StateRunning,
				Tunnel:    tunnel,
				Tailscale: tailscale,
				Push:      push,
				URLs:      contract.URLs{Local: "http://localhost", Remote: remote, Access: links},
			})
			wantCloudflare, wantTailscale, wantPush := tc.cloudflare && !tc.invalid, tc.tailscale && !tc.invalid, tc.push && !tc.invalid
			if (status.Tunnel != nil) != wantCloudflare || (status.Tailscale != nil) != wantTailscale || (status.Push != nil) != wantPush {
				t.Fatalf("provider controls do not match capabilities: %+v", status)
			}
			wantLinks := []contract.AccessLink{}
			if wantCloudflare {
				wantLinks = append(wantLinks, links[0])
			}
			if wantTailscale {
				wantLinks = append(wantLinks, links[1])
			}
			if !reflect.DeepEqual(status.URLs.Access, wantLinks) {
				t.Fatalf("provider links=%+v, want %+v", status.URLs.Access, wantLinks)
			}
			if status.URLs.Local != "http://localhost" || !reflect.DeepEqual(status.URLs.Remote, remote) {
				t.Fatalf("capability filtering changed network addresses: %+v", status.URLs)
			}
			snap := Snapshot{State: StateRunning, Mode: ModeTray, HasStatus: true, Status: status}
			if got := TunnelEntry(snap); got.Show != wantCloudflare {
				t.Fatalf("cloudflare visibility=%t, want %t", got.Show, wantCloudflare)
			}
			if got := TailscaleEntry(snap); got.Show != wantTailscale {
				t.Fatalf("tailscale visibility=%t, want %t", got.Show, wantTailscale)
			}
			if got := PushEntry(snap); got.Show != wantPush {
				t.Fatalf("push visibility=%t, want %t", got.Show, wantPush)
			} else if wantPush {
				if got.Guide == "" {
					t.Fatal("selected push provider lost recovery instructions")
				}
				if tc.pushReason == contract.CapabilityReasonConfigurationOff && !strings.Contains(got.Info, "已在配置中停用") {
					t.Fatalf("missing configuration diagnostic: %+v", got)
				}
				if tc.pushReason == contract.CapabilityReasonStorageUnavailable && !strings.Contains(got.Info, push.Error) {
					t.Fatalf("missing storage diagnostic: %+v", got)
				}
			}
			if tc.providersOff {
				tunnelItem, tailscaleItem := TunnelEntry(snap), TailscaleEntry(snap)
				if tunnelItem.Checked || tailscaleItem.Checked || tunnelItem.SettingsURL == "" || tailscaleItem.SettingsURL == "" {
					t.Fatalf("configured-off access settings unavailable: tunnel=%+v tailscale=%+v", tunnelItem, tailscaleItem)
				}
			}
			if status.State != contract.StateRunning {
				t.Fatal("capability failure changed core health")
			}
		})
	}
}
