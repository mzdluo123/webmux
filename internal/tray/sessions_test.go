package tray

import (
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestSummarizeSessions(t *testing.T) {
	items := map[string]contract.SessionInfo{
		"idle-hybrid": {
			ID:     "idle-hybrid",
			Status: contract.SessionStatusIdle,
			Views:  []contract.SessionView{contract.ViewTerminal, contract.ViewChat},
		},
		"done-hybrid": {
			ID:        "done-hybrid",
			Status:    contract.SessionStatusWaiting,
			Views:     []contract.SessionView{contract.ViewTerminal, contract.ViewChat},
			Attention: &contract.SessionAttention{Kind: contract.AttentionKindDone},
		},
		"prompt-hybrid": {
			ID:     "prompt-hybrid",
			Status: contract.SessionStatusWaiting,
			Views:  []contract.SessionView{contract.ViewTerminal, contract.ViewChat},
		},
		"running-hybrid": {
			ID:     "running-hybrid",
			Status: contract.SessionStatusRunning,
			Views:  []contract.SessionView{contract.ViewTerminal, contract.ViewChat},
		},
		"approval-hybrid": {
			ID:        "approval-hybrid",
			Status:    contract.SessionStatusWaiting,
			Views:     []contract.SessionView{contract.ViewTerminal, contract.ViewChat},
			Attention: &contract.SessionAttention{Kind: contract.AttentionKindPermission},
		},
		"switching-hybrid": {
			ID:         "switching-hybrid",
			Status:     contract.SessionStatusIdle,
			Views:      []contract.SessionView{contract.ViewTerminal, contract.ViewChat},
			Transition: &contract.SessionTransition{To: contract.ViewChat, Phase: contract.PhaseStarting},
		},
		"idle-shell": {
			ID:     "idle-shell",
			Status: contract.SessionStatusWaiting,
			Views:  []contract.SessionView{contract.ViewTerminal},
		},
		"running-shell": {
			ID:     "running-shell",
			Status: contract.SessionStatusRunning,
			Views:  []contract.SessionView{contract.ViewTerminal},
		},
	}

	got := SummarizeSessions(items)
	want := SessionCounts{
		Total:         8,
		Known:         true,
		Terminal:      2,
		Hybrid:        6,
		Working:       4,
		HybridWorking: 3,
	}
	if got != want {
		t.Fatalf("SummarizeSessions = %+v, want %+v", got, want)
	}
}

func TestRestartConfirmPrompt(t *testing.T) {
	tests := []struct {
		name   string
		snap   Snapshot
		needed bool
		want   string
	}{
		{
			name:   "no sessions",
			snap:   Snapshot{},
			needed: false,
		},
		{
			name: "only idle or finished hybrid sessions skip prompt",
			snap: Snapshot{
				Sessions:      3,
				SessionCounts: SessionCounts{Total: 3, Known: true, Hybrid: 3},
			},
			needed: false,
		},
		{
			name: "single working hybrid session",
			snap: Snapshot{
				Sessions:      1,
				SessionCounts: SessionCounts{Total: 1, Known: true, Hybrid: 1, Working: 1, HybridWorking: 1},
			},
			needed: true,
			want:   "重启 WebMux 将中断 1 个正在工作的混合会话；重启后该会话会自动恢复（排队消息不保留）。\n\n是否继续？",
		},
		{
			name: "multiple hybrid sessions with one working",
			snap: Snapshot{
				Sessions:      3,
				SessionCounts: SessionCounts{Total: 3, Known: true, Hybrid: 3, Working: 1, HybridWorking: 1},
			},
			needed: true,
			want:   "重启 WebMux 将中断 1 个正在工作的混合会话；3 个混合会话会在重启后自动恢复（排队消息不保留）。\n\n是否继续？",
		},
		{
			name: "idle ordinary terminal session still prompts",
			snap: Snapshot{
				Sessions:      2,
				SessionCounts: SessionCounts{Total: 2, Known: true, Terminal: 2},
			},
			needed: true,
			want:   "重启 WebMux 将结束 2 个普通终端会话，普通终端会话不会保存。\n\n是否继续？",
		},
		{
			name: "working ordinary terminal session",
			snap: Snapshot{
				Sessions:      2,
				SessionCounts: SessionCounts{Total: 2, Known: true, Terminal: 2, Working: 1},
			},
			needed: true,
			want:   "重启 WebMux 将结束 2 个普通终端会话（其中 1 个正在工作），普通终端会话不会保存。\n\n是否继续？",
		},
		{
			name: "ordinary terminal plus idle hybrid sessions",
			snap: Snapshot{
				Sessions:      3,
				SessionCounts: SessionCounts{Total: 3, Known: true, Terminal: 1, Hybrid: 2},
			},
			needed: true,
			want:   "重启 WebMux 将结束 1 个普通终端会话（不会保存）；2 个混合会话会在重启后自动恢复。\n\n是否继续？",
		},
		{
			name: "ordinary terminal working plus idle hybrid sessions",
			snap: Snapshot{
				Sessions:      3,
				SessionCounts: SessionCounts{Total: 3, Known: true, Terminal: 1, Hybrid: 2, Working: 1},
			},
			needed: true,
			want:   "重启 WebMux 将结束 1 个普通终端会话（其中 1 个正在工作，不会保存）；2 个混合会话会在重启后自动恢复。\n\n是否继续？",
		},
		{
			name: "ordinary terminal plus working hybrid sessions",
			snap: Snapshot{
				Sessions:      3,
				SessionCounts: SessionCounts{Total: 3, Known: true, Terminal: 1, Hybrid: 2, Working: 1, HybridWorking: 1},
			},
			needed: true,
			want:   "重启 WebMux 将结束 1 个普通终端会话（不会保存），并中断 1 个正在工作的混合会话；2 个混合会话会在重启后自动恢复（排队消息不保留）。\n\n是否继续？",
		},
		{
			name: "unknown breakdown falls back conservatively",
			snap: Snapshot{
				Sessions: 2,
			},
			needed: true,
			want:   "重启 WebMux 将结束普通终端会话，并中断正在运行的任务；混合会话会在重启后自动恢复（当前共 2 个会话）。\n\n是否继续？",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, needed := RestartConfirmPrompt("重启 WebMux ", tc.snap)
			if needed != tc.needed || got != tc.want {
				t.Fatalf("RestartConfirmPrompt = (%q, %v), want (%q, %v)", got, needed, tc.want, tc.needed)
			}
			if strings.Contains(got, "不可恢复") {
				t.Fatalf("prompt still claims sessions are unrecoverable: %q", got)
			}
		})
	}
}

func TestStopConfirmPromptAndNotes(t *testing.T) {
	if got, needed := StopConfirmPrompt("停止 WebMux ", Snapshot{}); needed || got != "" {
		t.Fatalf("empty stop prompt = (%q, %v)", got, needed)
	}

	idleHybrid := Snapshot{
		Sessions:      2,
		SessionCounts: SessionCounts{Total: 2, Known: true, Hybrid: 2},
	}
	got, needed := StopConfirmPrompt("停止 WebMux ", idleHybrid)
	want := "停止 WebMux 将停止当前的 2 个混合会话；下次启动时会自动恢复。\n\n是否继续？"
	if !needed || got != want {
		t.Fatalf("idle hybrid stop prompt = (%q, %v), want (%q, true)", got, needed, want)
	}
	if note := RestartSessionNote(idleHybrid); note != "当前 2 个混合会话均已空闲，重启后会自动恢复。" {
		t.Fatalf("idle hybrid note = %q", note)
	}

	quit := QuitPrompt(2, true)
	if !strings.Contains(quit, "结束普通终端会话，混合会话下次启动时恢复") || !strings.Contains(quit, "托盘转发会随托盘退出而停止") {
		t.Fatalf("QuitPrompt = %q", quit)
	}
}
