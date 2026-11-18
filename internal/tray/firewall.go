package tray

import (
	"slices"
	"strings"
)

// FirewallApplyResult mirrors the exit codes of `webmux-tray firewall`.
type FirewallApplyResult int

const (
	FirewallResultDone     FirewallApplyResult = 0
	FirewallResultFailed   FirewallApplyResult = 1
	FirewallResultUsage    FirewallApplyResult = 2
	FirewallResultDeclined FirewallApplyResult = 3
)

// FirewallCheck summarizes the read-only Windows Defender Firewall state for
// the running tray executable.
type FirewallCheck struct {
	Checked              bool
	Err                  string
	RulePresent          bool
	RuleValid            bool
	AllowedPrivateDomain bool
	LocalBlocks          []string
	UnnamedLocalBlock    bool
	PolicyBlocked        bool
	DomainShielded       bool // DoNotAllowExceptions=1 on active Domain profile
	PrivateShielded      bool // DoNotAllowExceptions=1 on active Private profile
}

// LANFirewallAction identifies what clicking the LAN firewall menu entry does.
type LANFirewallAction int

const (
	LANFirewallNone LANFirewallAction = iota
	// LANFirewallAllow adds or repairs the tray's Private/Domain inbound TCP allow rule.
	LANFirewallAllow
	// LANFirewallClearBlocksAndAllow deletes conflicting local inbound Block
	// rules for this tray and adds/repairs its Private/Domain allow rule.
	LANFirewallClearBlocksAndAllow
	// LANFirewallExplain shows an informational dialog without triggering UAC.
	LANFirewallExplain
)

// LANFirewallItem describes the firewall status line, menu action, and dialog
// copy associated with "允许局域网访问".
type LANFirewallItem struct {
	Show         bool
	Info         string
	ActionTitle  string
	Action       LANFirewallAction
	TogglePrompt string
	MenuConfirm  string
	Guide        string
	WarnOnToggle bool
}

// LANFirewallEntry derives the runtime menu entries shown below "允许局域网访问"
// while LAN access is enabled and Windows-side forwarding is needed.
func LANFirewallEntry(snap Snapshot, plan ForwardPlan, fw FirewallCheck) LANFirewallItem {
	if !snap.LANEnabled() {
		return LANFirewallItem{}
	}
	return AssessLANFirewall(plan, fw)
}

// AssessLANFirewall evaluates firewall rules, block overrides and candidate
// interface network categories for Windows-side forwarding.
func AssessLANFirewall(plan ForwardPlan, fw FirewallCheck) LANFirewallItem {
	if !plan.Needed || !fw.Checked {
		return LANFirewallItem{}
	}
	publicIfaces, nonPublicCount, allTrustedShielded := summarizeCandidateCategories(plan.Candidates, fw)
	publicNote := formatPublicNetworkNote(publicIfaces, nonPublicCount)

	if fw.Err != "" {
		return LANFirewallItem{
			Show:        true,
			Info:        "无法读取防火墙状态，其他设备可能无法连接",
			ActionTitle: "放行 Windows 防火墙…",
			Action:      LANFirewallAllow,
			TogglePrompt: "未能读取当前 Windows 防火墙状态（" + fw.Err + "）。\n\n" +
				"选择“是”将弹出一次管理员确认，为当前托盘配置专用网络与域网络的入站 TCP 允许规则；选择“否”仍开启局域网访问，稍后可在托盘菜单中放行。" + publicNote,
		}
	}

	if fw.PolicyBlocked {
		return LANFirewallItem{
			Show:         true,
			Info:         "系统策略阻止了托盘入站连接，其他设备无法连接",
			ActionTitle:  "防火墙与网络说明…",
			Action:       LANFirewallExplain,
			WarnOnToggle: true,
			Guide: "检测到系统组策略或设备管理（MDM）下发了针对当前托盘（webmux-tray.exe）的 Windows 防火墙入站阻止规则。\n\n" +
				"策略阻止规则优先于本机允许规则，本机命令无法直接移除。请联系系统管理员或在“高级安全 Windows Defender 防火墙（wf.msc）”中检查入站策略规则。",
		}
	}

	if allTrustedShielded {
		return LANFirewallItem{
			Show:         true,
			Info:         "防火墙开启了“阻止所有传入连接”，其他设备无法连接",
			ActionTitle:  "防火墙与网络说明…",
			Action:       LANFirewallExplain,
			WarnOnToggle: true,
			Guide: "当前网络的 Windows 防火墙开启了“阻止所有传入连接，包括位于允许应用列表中的应用”，入站允许规则在该设置开启期间不会生效。\n\n" +
				"如需允许其他设备经托盘转发访问，请在“Windows 安全中心 → 防火墙和网络保护”中打开当前网络配置文件，取消勾选“阻止所有传入连接”。",
		}
	}

	if fw.UnnamedLocalBlock {
		return LANFirewallItem{
			Show:         true,
			Info:         "防火墙存在阻止规则，其他设备无法连接",
			ActionTitle:  "防火墙与网络说明…",
			Action:       LANFirewallExplain,
			WarnOnToggle: true,
			Guide: "检测到本机 Windows 防火墙中存在针对当前托盘（webmux-tray.exe）的无名称入站阻止规则；Windows 防火墙中阻止规则优先于允许规则。\n\n" +
				"请打开“高级安全 Windows Defender 防火墙（wf.msc） → 入站规则”，按程序路径筛选并删除针对 webmux-tray.exe 的阻止规则。",
		}
	}

	if len(fw.LocalBlocks) > 0 {
		names := strings.Join(fw.LocalBlocks, "、")
		return LANFirewallItem{
			Show:        true,
			Info:        "防火墙存在阻止规则，其他设备无法连接",
			ActionTitle: "修复 Windows 防火墙规则…",
			Action:      LANFirewallClearBlocksAndAllow,
			MenuConfirm: "检测到针对当前托盘的 Windows 防火墙入站阻止规则（" + names + "）。Windows 防火墙中阻止规则优先于允许规则，会导致其他设备无法经托盘转发连接。\n\n" +
				"继续操作将弹出一次管理员确认，清理上述本机入站阻止规则，并为当前托盘配置专用网络与域网络的入站 TCP 允许规则。" + publicNote + "\n\n确定继续？",
			TogglePrompt: "检测到针对当前托盘的 Windows 防火墙入站阻止规则（" + names + "），它会覆盖允许规则并导致其他设备无法连接。\n\n" +
				"选择“是”将弹出一次管理员确认，清理该本机入站阻止规则并配置专用网络与域网络的入站 TCP 允许规则；选择“否”仍开启局域网访问，稍后可在托盘菜单中修复。" + publicNote,
		}
	}

	if !fw.AllowedPrivateDomain {
		if fw.RulePresent && !fw.RuleValid {
			return LANFirewallItem{
				Show:        true,
				Info:        "防火墙规则已失效，其他设备可能无法连接",
				ActionTitle: "修复 Windows 防火墙规则…",
				Action:      LANFirewallAllow,
				TogglePrompt: "当前托盘的 Windows 防火墙放行规则已被禁用或属性变更，其他设备可能无法经托盘转发访问 WebMux。\n\n" +
					"选择“是”将弹出一次管理员确认，重建专用网络与域网络的入站 TCP 允许规则；选择“否”仍开启局域网访问，稍后可在托盘菜单中修复。" + publicNote,
			}
		}
		return LANFirewallItem{
			Show:        true,
			Info:        "防火墙未放行，其他设备可能无法连接",
			ActionTitle: "放行 Windows 防火墙…",
			Action:      LANFirewallAllow,
			TogglePrompt: "开启局域网访问后，托盘会在 Windows 网卡地址上转发请求，需要添加专用网络与域网络的 Windows 防火墙入站 TCP 允许规则。\n\n" +
				"选择“是”将弹出一次管理员确认完成放行；选择“否”仍开启局域网访问，稍后可在托盘菜单中放行。" + publicNote,
		}
	}

	if len(publicIfaces) > 0 {
		joined := strings.Join(publicIfaces, "、")
		if nonPublicCount == 0 {
			return LANFirewallItem{
				Show:         true,
				Info:         "当前网络（" + joined + "）为公用网络，防火墙未放行",
				ActionTitle:  "防火墙与网络说明…",
				Action:       LANFirewallExplain,
				WarnOnToggle: true,
				Guide: "当前候选转发网卡（" + joined + "）在 Windows 中被归类为“公用网络”。\n\n" +
					"基于安全边界，WebMux 防火墙规则仅放行专用网络与域网络，公用网络默认保持拦截。\n\n" +
					"若当前连接的是家庭、办公室或 Tailscale 等可信网络，请在“Windows 设置 → 网络和 Internet”中打开对应网络连接，将“网络配置文件类型”切换为“专用网络”。",
			}
		}
		return LANFirewallItem{
			Show:        true,
			Info:        "公用网络（" + joined + "）未放行，专用或域网络可正常连接",
			ActionTitle: "防火墙与网络说明…",
			Action:      LANFirewallExplain,
			Guide: "Windows 防火墙放行规则已就绪（仅限专用网络与域网络）：\n\n" +
				"• 专用网络与域网络网卡可正常通过防火墙供其他设备连接；\n" +
				"• 公用网络网卡（" + joined + "）出于安全边界保持拦截。\n\n" +
				"若“" + joined + "”属于家庭或办公等可信网络，可在“Windows 设置 → 网络和 Internet”中将其网络配置文件类型切换为“专用网络”。",
		}
	}

	return LANFirewallItem{}
}

func summarizeCandidateCategories(candidates []Candidate, fw FirewallCheck) (publicIfaces []string, nonPublicCount int, allTrustedShielded bool) {
	if len(candidates) == 0 {
		return nil, 0, fw.DomainShielded && fw.PrivateShielded
	}
	trustedCount, shieldedCount := 0, 0
	for _, c := range candidates {
		switch c.Category {
		case NetworkPublic:
			name := c.Interface
			if name == "" {
				name = c.Address
			}
			if !slices.Contains(publicIfaces, name) {
				publicIfaces = append(publicIfaces, name)
			}
		case NetworkPrivate:
			nonPublicCount++
			trustedCount++
			if fw.PrivateShielded {
				shieldedCount++
			}
		case NetworkDomain:
			nonPublicCount++
			trustedCount++
			if fw.DomainShielded {
				shieldedCount++
			}
		default:
			nonPublicCount++
		}
	}
	allTrustedShielded = trustedCount > 0 && trustedCount == nonPublicCount && shieldedCount == trustedCount
	return publicIfaces, nonPublicCount, allTrustedShielded
}

func formatPublicNetworkNote(publicIfaces []string, nonPublicCount int) string {
	if len(publicIfaces) == 0 {
		return ""
	}
	joined := strings.Join(publicIfaces, "、")
	if nonPublicCount == 0 {
		return "\n\n注意：当前网卡（" + joined + "）在 Windows 中被识别为公用网络。基于安全边界，放行规则仅适用于专用网络与域网络；若该网络属于可信网络，还需在“Windows 设置 → 网络和 Internet”中将其切换为“专用网络”。"
	}
	return "\n\n说明：专用/域网络网卡将在放行后可用；公用网络网卡（" + joined + "）出于安全边界保持拦截。"
}
