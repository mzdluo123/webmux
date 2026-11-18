package tray

import (
	"net"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestLANFirewallEntry(t *testing.T) {
	lanOn := Snapshot{
		State:     StateRunning,
		HasStatus: true,
		Reachable: true,
		Status: contract.StatusV1{
			State:           contract.StateRunning,
			Listen:          "0.0.0.0:7681",
			TokenConfigured: true,
			Health:          contract.Health{Status: contract.HealthOK},
			URLs:            contract.URLs{Local: "http://127.0.0.1:7681"},
			Config:          contract.ConfigRef{Exists: true},
		},
	}
	lanOff := lanOn
	lanOff.Status.Listen = "127.0.0.1:7681"

	privatePlan := PlanForward("nat", lanOn, []WinInterface{
		{Name: "Wi-Fi", Up: true, Category: NetworkPrivate, Addrs: []net.IP{net.ParseIP("192.168.1.20")}},
	})
	mixedPlan := PlanForward("nat", lanOn, []WinInterface{
		{Name: "Tailscale", Up: true, Category: NetworkPrivate, Addrs: []net.IP{net.ParseIP("100.64.0.9")}},
		{Name: "Wi-Fi", Up: true, Category: NetworkPublic, Addrs: []net.IP{net.ParseIP("192.168.1.20")}},
	})
	publicOnlyPlan := PlanForward("nat", lanOn, []WinInterface{
		{Name: "Wi-Fi", Up: true, Category: NetworkPublic, Addrs: []net.IP{net.ParseIP("192.168.1.20")}},
	})
	mirroredPlan := PlanForward("mirrored", lanOn, nil)

	validFW := FirewallCheck{Checked: true, RulePresent: true, RuleValid: true, AllowedPrivateDomain: true}

	t.Run("hidden when LAN disabled or forwarding unnecessary or valid on private", func(t *testing.T) {
		if got := LANFirewallEntry(lanOff, privatePlan, FirewallCheck{Checked: true}); got.Show {
			t.Fatalf("shown while LAN off: %+v", got)
		}
		if got := LANFirewallEntry(lanOn, mirroredPlan, FirewallCheck{Checked: true}); got.Show {
			t.Fatalf("shown in mirrored mode: %+v", got)
		}
		if got := LANFirewallEntry(lanOn, privatePlan, validFW); got.Show {
			t.Fatalf("shown when rule valid on private network: %+v", got)
		}
	})

	t.Run("missing rule offers one-click elevation", func(t *testing.T) {
		got := LANFirewallEntry(lanOn, privatePlan, FirewallCheck{Checked: true})
		if !got.Show || got.Action != LANFirewallAllow || got.ActionTitle != "放行 Windows 防火墙…" || !strings.Contains(got.Info, "防火墙未放行") || got.TogglePrompt == "" || got.MenuConfirm != "" {
			t.Fatalf("missing rule: %+v", got)
		}
	})

	t.Run("invalid existing rule offers repair", func(t *testing.T) {
		got := LANFirewallEntry(lanOn, privatePlan, FirewallCheck{Checked: true, RulePresent: true, RuleValid: false})
		if !got.Show || got.Action != LANFirewallAllow || got.ActionTitle != "修复 Windows 防火墙规则…" || !strings.Contains(got.Info, "已失效") {
			t.Fatalf("invalid rule: %+v", got)
		}
	})

	t.Run("local block rule offers clear-and-allow with confirmation", func(t *testing.T) {
		got := LANFirewallEntry(lanOn, privatePlan, FirewallCheck{
			Checked:              true,
			RulePresent:          true,
			RuleValid:            true,
			AllowedPrivateDomain: true,
			LocalBlocks:          []string{"webmux-tray.exe"},
		})
		if !got.Show || got.Action != LANFirewallClearBlocksAndAllow || !strings.Contains(got.MenuConfirm, "webmux-tray.exe") || !strings.Contains(got.TogglePrompt, "webmux-tray.exe") {
			t.Fatalf("local block: %+v", got)
		}
	})

	t.Run("public-only network explains switching to private without pointless UAC", func(t *testing.T) {
		got := LANFirewallEntry(lanOn, publicOnlyPlan, validFW)
		if !got.Show || got.Action != LANFirewallExplain || !got.WarnOnToggle || !strings.Contains(got.Info, "Wi-Fi") || !strings.Contains(got.Guide, "专用网络") {
			t.Fatalf("public-only: %+v", got)
		}
	})

	t.Run("mixed private and public interfaces keep private usable without modal warning", func(t *testing.T) {
		got := LANFirewallEntry(lanOn, mixedPlan, validFW)
		if !got.Show || got.Action != LANFirewallExplain || got.WarnOnToggle || !strings.Contains(got.Info, "Wi-Fi") || !strings.Contains(got.Info, "专用或域网络可正常连接") {
			t.Fatalf("mixed interfaces: %+v", got)
		}
	})

	t.Run("policy block and shielded profile explain policy without UAC", func(t *testing.T) {
		pol := LANFirewallEntry(lanOn, privatePlan, FirewallCheck{Checked: true, AllowedPrivateDomain: true, PolicyBlocked: true})
		if !pol.Show || pol.Action != LANFirewallExplain || !pol.WarnOnToggle || !strings.Contains(pol.Info, "策略") {
			t.Fatalf("policy block: %+v", pol)
		}
		shielded := LANFirewallEntry(lanOn, privatePlan, FirewallCheck{Checked: true, AllowedPrivateDomain: true, PrivateShielded: true})
		if !shielded.Show || shielded.Action != LANFirewallExplain || !shielded.WarnOnToggle || !strings.Contains(shielded.Info, "阻止所有传入连接") {
			t.Fatalf("shielded: %+v", shielded)
		}
	})
}
