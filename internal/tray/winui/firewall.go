//go:build windows

package winui

import (
	"context"
	"time"

	"golang.org/x/sys/windows"

	"github.com/rainchan/webmux/internal/tray"
)

func (a *App) inspectFirewall() tray.FirewallCheck {
	if a.o.InspectFirewall == nil || a.o.Executable == "" {
		return tray.FirewallCheck{}
	}
	return a.o.InspectFirewall(a.o.Executable)
}

func (a *App) currentNetMode(timeout time.Duration) string {
	a.mu.Lock()
	mode := a.netMode
	noWSL := a.noWSL != ""
	settings := a.settings
	a.mu.Unlock()
	if mode != "" || noWSL {
		return mode
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	mode = tray.NetworkingMode(ctx, a.o.Runner, tray.TargetOf(settings))
	if mode != "" {
		a.mu.Lock()
		if a.settings.Distro == settings.Distro && a.netMode == "" {
			a.netMode = mode
		}
		a.mu.Unlock()
	}
	return mode
}

// checkFirewallBeforeLAN inspects the Windows Firewall before switching WSL
// listen to 0.0.0.0:<port>, so a missing or broken rule can be added before
// forwarding binds any non-loopback address. Declining or skipping keeps the
// user's LAN toggle in effect and surfaces a retry entry in the menu.
func (a *App) checkFirewallBeforeLAN(snap tray.Snapshot) {
	a.firewallMu.Lock()
	defer a.firewallMu.Unlock()

	if a.o.InspectFirewall == nil || a.o.ApplyFirewall == nil || a.o.Executable == "" {
		return
	}
	netMode := a.currentNetMode(10 * time.Second)
	plan := tray.PlanForward(netMode, snap, a.windowsInterfaces(true))
	if !plan.Needed {
		return
	}
	item := tray.AssessLANFirewall(plan, a.inspectFirewall())
	if !item.Show {
		return
	}

	switch item.Action {
	case tray.LANFirewallAllow, tray.LANFirewallClearBlocksAndAllow:
		if messageBox(item.TogglePrompt, windows.MB_YESNO|windows.MB_ICONQUESTION) != idYes {
			a.log.Printf("LAN firewall configuration skipped by user")
			return
		}
		clearBlocks := item.Action == tray.LANFirewallClearBlocksAndAllow
		res := a.o.ApplyFirewall(a.o.Executable, true, clearBlocks)
		switch res {
		case tray.FirewallResultDone:
			a.log.Printf("firewall rule configured for %s (clearBlocks=%v)", a.o.Executable, clearBlocks)
		case tray.FirewallResultDeclined:
			a.log.Printf("firewall administrator prompt declined")
			a.notifier.Show("WebMux", "已跳过防火墙管理员确认；其他设备可能无法连接，稍后可在托盘菜单放行")
		default:
			a.log.Printf("firewall configuration failed (code %d)", res)
			messageBox(
				"未能配置 Windows 防火墙规则，已继续开启局域网访问。\n\n其他设备可能暂时无法连接，稍后可在托盘菜单点击“"+item.ActionTitle+"”重试，或在管理员终端执行：\n\""+a.o.Executable+"\" firewall allow",
				windows.MB_OK|windows.MB_ICONWARNING,
			)
		}
	case tray.LANFirewallExplain:
		if item.WarnOnToggle && item.Guide != "" {
			messageBox(item.Guide, windows.MB_OK|windows.MB_ICONINFORMATION)
		}
	}
}

func (a *App) handleLANFirewallAction() {
	a.firewallMu.Lock()
	defer a.firewallMu.Unlock()

	snap := a.snapshot()
	netMode := a.currentNetMode(10 * time.Second)
	plan := tray.PlanForward(netMode, snap, a.windowsInterfaces(true))
	item := tray.LANFirewallEntry(snap, plan, a.inspectFirewall())
	if !item.Show {
		a.kickUI()
		return
	}

	switch item.Action {
	case tray.LANFirewallExplain:
		if item.Guide != "" {
			messageBox(item.Guide, windows.MB_OK|windows.MB_ICONINFORMATION)
		}
		a.kickUI()
	case tray.LANFirewallAllow, tray.LANFirewallClearBlocksAndAllow:
		if item.MenuConfirm != "" && !confirm(item.MenuConfirm) {
			a.kickUI()
			return
		}
		if a.o.ApplyFirewall == nil || a.o.Executable == "" {
			a.kickUI()
			return
		}
		clearBlocks := item.Action == tray.LANFirewallClearBlocksAndAllow
		res := a.o.ApplyFirewall(a.o.Executable, true, clearBlocks)
		a.kickUI()
		switch res {
		case tray.FirewallResultDone:
			a.log.Printf("firewall rule repaired from menu for %s (clearBlocks=%v)", a.o.Executable, clearBlocks)
			afterSnap := a.snapshot()
			afterPlan := tray.PlanForward(netMode, afterSnap, a.windowsInterfaces(true))
			afterItem := tray.LANFirewallEntry(afterSnap, afterPlan, a.inspectFirewall())
			if afterItem.Show && afterItem.WarnOnToggle && afterItem.Guide != "" {
				messageBox("已配置专用网络与域网络的防火墙放行规则。\n\n"+afterItem.Guide, windows.MB_OK|windows.MB_ICONINFORMATION)
			} else {
				a.notifier.Show("WebMux", "已放行 Windows 防火墙（专用网络与域网络）")
			}
		case tray.FirewallResultDeclined:
			a.log.Printf("firewall repair administrator prompt declined")
		default:
			a.log.Printf("firewall repair failed (code %d)", res)
			messageBox(
				"未能配置 Windows 防火墙规则。\n\n请稍后重试，或在管理员终端执行：\n\""+a.o.Executable+"\" firewall allow",
				windows.MB_OK|windows.MB_ICONWARNING,
			)
		}
	}
}
