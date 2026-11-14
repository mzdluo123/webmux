package tray

import (
	"fmt"
	"slices"

	"github.com/rainchan/webmux/internal/contract"
)

// InstalledRestartNotice tells the user that an installed server binary takes
// effect after a restart, distinguishing ordinary terminals from hybrid sessions.
const InstalledRestartNotice = "新版本在重启 WebMux 后生效（普通终端会话会结束，混合会话会自动恢复）。"

// SessionCounts summarizes active sessions reported over /ws/sessions.
// Known is false until the live subscription delivers a snapshot (for example
// when only /local/v1/status has reported a total count, or after the
// subscription disconnects).
type SessionCounts struct {
	Total         int
	Known         bool
	Terminal      int
	Hybrid        int
	Working       int
	HybridWorking int
}

// SummarizeSessions classifies the active session map from /ws/sessions.
func SummarizeSessions(items map[string]contract.SessionInfo) SessionCounts {
	counts := SessionCounts{Total: len(items), Known: true}
	for _, item := range items {
		hybrid := slices.Contains(item.Views, contract.ViewChat)
		working := sessionWorking(item)
		if hybrid {
			counts.Hybrid++
		} else {
			counts.Terminal++
		}
		if working {
			counts.Working++
			if hybrid {
				counts.HybridWorking++
			}
		}
	}
	return counts
}

// sessionWorking reports whether a session is actively executing a turn/command,
// switching views, or blocked mid-turn on a permission or question request.
// Idle sessions, exited processes, prompt-waiting shells, and completed turns
// with latched done/error/notify/bell attention are not working.
func sessionWorking(info contract.SessionInfo) bool {
	if info.Status == contract.SessionStatusRunning || info.Transition != nil {
		return true
	}
	if info.Attention != nil {
		switch info.Attention.Kind {
		case contract.AttentionKindPermission, contract.AttentionKindQuestion:
			return true
		}
	}
	return false
}

// EffectiveSessions returns the live session breakdown when present, falling
// back to the total session count with Known=false.
func (s Snapshot) EffectiveSessions() SessionCounts {
	if s.SessionCounts.Known || s.SessionCounts.Total != 0 {
		return s.SessionCounts
	}
	return SessionCounts{Total: s.Sessions}
}

// RestartConfirmPrompt builds the confirmation dialog before restarting WebMux.
// When there are no sessions, or when all sessions are known to be non-working
// hybrid sessions (which automatically resume on restart), no confirmation is
// needed and it returns ("", false).
func RestartConfirmPrompt(prefix string, snap Snapshot) (string, bool) {
	c := snap.EffectiveSessions()
	if c.Total <= 0 || (c.Known && c.Terminal == 0 && c.Working == 0) {
		return "", false
	}
	return prefix + restartImpact(c) + "\n\n是否继续？", true
}

// RestartSessionNote describes how restarting affects existing sessions for
// dialogs that already ask whether to restart now (config init, server upgrade).
func RestartSessionNote(snap Snapshot) string {
	c := snap.EffectiveSessions()
	if c.Total <= 0 {
		return ""
	}
	if c.Known && c.Terminal == 0 && c.Working == 0 {
		return fmt.Sprintf("当前 %d 个混合会话均已空闲，重启后会自动恢复。", c.Hybrid)
	}
	return "重启" + restartImpact(c)
}

// StopConfirmPrompt builds the confirmation dialog before stopping WebMux.
// Stopping pauses service availability, so any active session requires
// confirmation, while accurately stating that hybrid sessions recover on next start.
func StopConfirmPrompt(prefix string, snap Snapshot) (string, bool) {
	c := snap.EffectiveSessions()
	if c.Total <= 0 {
		return "", false
	}
	return prefix + stopImpact(c) + "\n\n是否继续？", true
}

// QuitPrompt builds the three-way tray exit question for a managed running WebMux.
func QuitPrompt(sessions int, forwarding bool) string {
	text := fmt.Sprintf("WebMux 正在运行（%d 个会话）。\n\n是：停止 WebMux 并退出（结束普通终端会话，混合会话下次启动时恢复）\n否：仅退出托盘（WebMux 继续运行）\n取消：不退出", sessions)
	if forwarding {
		text += "\n\n托盘转发会随托盘退出而停止，其他设备将无法再经转发地址访问。"
	}
	return text
}

func restartImpact(c SessionCounts) string {
	if !c.Known {
		return fmt.Sprintf("将结束普通终端会话，并中断正在运行的任务；混合会话会在重启后自动恢复（当前共 %d 个会话）。", c.Total)
	}
	if c.Hybrid == 0 {
		if c.Working > 0 {
			return fmt.Sprintf("将结束 %d 个普通终端会话（其中 %d 个正在工作），普通终端会话不会保存。", c.Terminal, c.Working)
		}
		return fmt.Sprintf("将结束 %d 个普通终端会话，普通终端会话不会保存。", c.Terminal)
	}
	if c.Terminal == 0 {
		if c.Hybrid == 1 {
			return "将中断 1 个正在工作的混合会话；重启后该会话会自动恢复（排队消息不保留）。"
		}
		return fmt.Sprintf("将中断 %d 个正在工作的混合会话；%d 个混合会话会在重启后自动恢复（排队消息不保留）。", c.HybridWorking, c.Hybrid)
	}
	if c.HybridWorking > 0 {
		return fmt.Sprintf("将结束 %d 个普通终端会话（不会保存），并中断 %d 个正在工作的混合会话；%d 个混合会话会在重启后自动恢复（排队消息不保留）。", c.Terminal, c.HybridWorking, c.Hybrid)
	}
	if c.Working > 0 {
		return fmt.Sprintf("将结束 %d 个普通终端会话（其中 %d 个正在工作，不会保存）；%d 个混合会话会在重启后自动恢复。", c.Terminal, c.Working, c.Hybrid)
	}
	return fmt.Sprintf("将结束 %d 个普通终端会话（不会保存）；%d 个混合会话会在重启后自动恢复。", c.Terminal, c.Hybrid)
}

func stopImpact(c SessionCounts) string {
	if !c.Known {
		return fmt.Sprintf("将结束普通终端会话；混合会话会在下次启动时恢复（当前共 %d 个会话）。", c.Total)
	}
	if c.Hybrid == 0 {
		if c.Working > 0 {
			return fmt.Sprintf("将结束 %d 个普通终端会话（其中 %d 个正在工作），普通终端会话不会保存。", c.Terminal, c.Working)
		}
		return fmt.Sprintf("将结束 %d 个普通终端会话，普通终端会话不会保存。", c.Terminal)
	}
	if c.Terminal == 0 {
		if c.HybridWorking > 0 {
			if c.Hybrid == 1 {
				return "将中断 1 个正在工作的混合会话；下次启动时会自动恢复（排队消息不保留）。"
			}
			return fmt.Sprintf("将中断 %d 个正在工作的混合会话；%d 个混合会话会在下次启动时恢复（排队消息不保留）。", c.HybridWorking, c.Hybrid)
		}
		return fmt.Sprintf("将停止当前的 %d 个混合会话；下次启动时会自动恢复。", c.Hybrid)
	}
	if c.HybridWorking > 0 {
		return fmt.Sprintf("将结束 %d 个普通终端会话（不会保存），并中断 %d 个正在工作的混合会话；%d 个混合会话会在下次启动时恢复（排队消息不保留）。", c.Terminal, c.HybridWorking, c.Hybrid)
	}
	if c.Working > 0 {
		return fmt.Sprintf("将结束 %d 个普通终端会话（其中 %d 个正在工作，不会保存）；%d 个混合会话会在下次启动时恢复。", c.Terminal, c.Working, c.Hybrid)
	}
	return fmt.Sprintf("将结束 %d 个普通终端会话（不会保存）；%d 个混合会话会在下次启动时恢复。", c.Terminal, c.Hybrid)
}
