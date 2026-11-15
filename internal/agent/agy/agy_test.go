package agy

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agent/agenttest"
	ap "github.com/rainchan/webmux/internal/agentprofile"
)

func TestContract(t *testing.T) {
	const sid = "89c9a933-60f6-46ad-977b-a8daf22f8e4d"
	agenttest.Run(t, Agent, agenttest.Fixture{
		Signals:  ap.Signals{ProgramApp: "antigravity", ProgramChildID: sid, RootCwd: "/p"},
		Identity: ap.Identity{Agent: Name, AgentSessionID: sid, Cwd: "/p"},
	})
	if got := ap.ReapplyOnLoad(Agent); !slices.Equal(got, []string{"model", "thought_level"}) {
		t.Fatalf("reapply options = %q", got)
	}
	if !ap.ResumesSessions(Agent) {
		t.Fatal("agy must support session/resume")
	}
}

func TestTUISettingsHookExtractsIDAndConversationTitle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell hook runs on Linux and macOS")
	}
	var settings struct {
		Title struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		} `json:"title"`
	}
	if err := json.Unmarshal([]byte(TUISettingsJSON), &settings); err != nil {
		t.Fatalf("TUISettingsJSON invalid JSON: %v", err)
	}
	if settings.Title.Type != "command" || settings.Title.Command == "" {
		t.Fatalf("unexpected title settings: %+v", settings)
	}

	const sid = "2fdddea4-143a-4098-9ca4-d1ac317b1e57"
	strPtr := func(s string) *string { return &s }
	type payload struct {
		AgentState        string  `json:"agent_state"`
		ConversationID    string  `json:"conversation_id"`
		ConversationTitle *string `json:"conversation_title,omitempty"`
		Cwd               string  `json:"cwd"`
		Product           string  `json:"product"`
	}

	cases := []struct {
		name      string
		raw       string
		wantID    string
		wantTitle string
	}{
		{
			name:   "before first message",
			raw:    `{"agent_state":"idle","conversation_id":"","conversation_title":null,"product":"antigravity"}`,
			wantID: "",
		},
		{
			name:   "null title on initial working state",
			raw:    `{"agent_state":"working","conversation_id":"` + sid + `","conversation_title":null,"product":"antigravity"}`,
			wantID: sid,
		},
		{
			name:   "omitted title",
			raw:    mustMarshal(t, payload{AgentState: "working", ConversationID: sid, Product: "antigravity"}),
			wantID: sid,
		},
		{
			name:   "whitespace-only title",
			raw:    mustMarshal(t, payload{AgentState: "idle", ConversationID: sid, ConversationTitle: strPtr("  \n\t "), Product: "antigravity"}),
			wantID: sid,
		},
		{
			name:      "cjk and emoji title",
			raw:       mustMarshal(t, payload{AgentState: "idle", ConversationID: sid, ConversationTitle: strPtr("🏛️ Agy无标题任务替代方案分析"), Product: "antigravity"}),
			wantID:    sid,
			wantTitle: "🏛️ Agy无标题任务替代方案分析",
		},
		{
			name:      "json escapes and leading dash",
			raw:       mustMarshal(t, payload{AgentState: "idle", ConversationID: sid, ConversationTitle: strPtr(`-n Fix "ACP" & TUI <switch> -> \n path/to`), Product: "antigravity"}),
			wantID:    sid,
			wantTitle: `-n Fix "ACP" & TUI <switch> -> \n path/to`,
		},
		{
			name:      "spaced json formatting",
			raw:       `{ "conversation_id": "` + sid + `", "conversation_title": "Spaced JSON Title" }`,
			wantID:    sid,
			wantTitle: "Spaced JSON Title",
		},
	}

	dir := t.TempDir()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ttyFile := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "_")+".tty")
			if err := os.WriteFile(ttyFile, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			cmdText := strings.ReplaceAll(settings.Title.Command, "/dev/tty", ttyFile)
			cmd := exec.Command("/bin/sh", "-c", cmdText)
			cmd.Stdin = strings.NewReader(tc.raw)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("hook command failed: %v (stderr: %s)", err, stderr.String())
			}
			ttyBytes, err := os.ReadFile(ttyFile)
			if err != nil {
				t.Fatal(err)
			}
			wantTTY := ""
			if tc.wantID != "" {
				wantTTY = "\x1b]7501;state=working:id=" + tc.wantID + "\x07"
			}
			wantTTY += "\x1b]2;" + tc.wantTitle + "\x07"
			if string(ttyBytes) != wantTTY {
				t.Fatalf("tty = %q, want %q", string(ttyBytes), wantTTY)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
