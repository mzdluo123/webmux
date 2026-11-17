package assemble

import (
	"reflect"
	"slices"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile/agents"
	"github.com/rainchan/webmux/internal/launch"
)

// Keep the complete previous preset contract, including permission flags and
// TUI-only tools, independent of the adapter-derived descriptions.
func TestLaunchPresetsCompatibility(t *testing.T) {
	want := []launch.Input{
		{Name: "Shell", Icon: "terminal", Cmd: []string{"/usr/bin/fish", "-l", "-i"}},
		{Name: "Claude Code", Icon: "claudecode", Cmd: []string{"claude", "--dangerously-skip-permissions"}, Hybrid: true},
		{Name: "Codex CLI", Icon: "codex", Cmd: []string{"codex", "--dangerously-bypass-approvals-and-sandbox"}, Hybrid: true},
		{Name: "Antigravity CLI", Icon: "antigravity", Cmd: []string{"agy", "--dangerously-skip-permissions"}, Hybrid: true},
		{Name: "Cursor CLI", Icon: "cursor", Cmd: []string{"agent"}},
		{Name: "GitHub Copilot CLI", Icon: "githubcopilot", Cmd: []string{"copilot"}},
		{Name: "Open Code", Icon: "opencode", Cmd: []string{"opencode", "--auto"}, Hybrid: true},
		{Name: "Qwen Code", Icon: "qwen", Cmd: []string{"qwen"}},
		{Name: "Kimi Code CLI", Icon: "kimi", Cmd: []string{"kimi", "--yolo"}, Hybrid: true},
		{Name: "CodeBuddy Code", Icon: "codebuddy", Cmd: []string{"codebuddy"}},
		{Name: "omp", Icon: "pi", Cmd: []string{"omp", "--auto-approve"}, Env: map[string]string{"PI_FORCE_IMAGE_PROTOCOL": "kitty"}, Hybrid: true},
		{Name: "Qoder", Icon: "qoder", Cmd: []string{"qoder", "--dangerously-skip-permissions"}, Hybrid: true},
		{Name: "QoderCN", Icon: "qoder", Cmd: []string{"qodercn", "--dangerously-skip-permissions"}, Hybrid: true},
		{Name: "Devin CLI", Icon: "devin", Cmd: []string{"devin", "--permission-mode", "dangerous"}, Hybrid: true},
	}
	if got := LaunchPresets("/usr/bin/fish"); !reflect.DeepEqual(got, want) {
		t.Fatalf("launch presets = %+v, want %+v", got, want)
	}
	// Callers may edit inputs without changing later seed/restore descriptions.
	first := LaunchPresets("/usr/bin/fish")
	first[1].Cmd[0] = "custom"
	first[10].Env["PI_FORCE_IMAGE_PROTOCOL"] = "custom"
	if got := LaunchPresets("/usr/bin/fish"); !reflect.DeepEqual(got, want) {
		t.Fatalf("launch presets were mutated: %+v", got)
	}
}

func TestAgentIconsComeFromLaunchCatalog(t *testing.T) {
	registry := agents.Default()
	service := launch.Open(t.TempDir(), registry)
	t.Cleanup(service.Close)
	for _, a := range registry.Agents() {
		p := a.Profile()
		if p.Title == "" || p.Icon == "" || p.Icon == "terminal" {
			t.Errorf("agent %q lacks a title or brand icon: %+v", p.Name, p)
			continue
		}
		if _, err := service.Create(launch.Input{Name: p.Title, Icon: p.Icon, Cmd: []string{p.Name}}); err != nil {
			t.Errorf("agent %q icon %q is not a launch icon: %v", p.Name, p.Icon, err)
		}
	}
	for _, preset := range LaunchPresets("/bin/bash") {
		if !preset.Hybrid {
			continue
		}
		support := registry.Check(preset.Cmd)
		a, ok := registry.Lookup(support.Agent)
		if !support.Supported || !ok {
			t.Errorf("hybrid preset %q has no runtime capability: %+v", preset.Name, support)
			continue
		}
		if got := a.Profile().Icon; got != preset.Icon {
			t.Errorf("agent %q icon %q, preset %q has %q", support.Agent, got, preset.Name, preset.Icon)
		}
	}
}

func TestPresetsFollowDefaultShell(t *testing.T) {
	tests := []struct {
		name      string
		shell     string
		wantLogin []string
		wantOMP   []string
	}{
		{
			name:      "bash",
			shell:     "/bin/bash",
			wantLogin: []string{"/bin/bash", "-l", "-i"},
			wantOMP:   []string{"/usr/bin/env", "bash", "-ic", `exec -- "$@"`, "webmux", "omp", "--auto-approve"},
		},
		{
			name:      "zsh",
			shell:     "/bin/zsh",
			wantLogin: []string{"/bin/zsh", "-l", "-i"},
			wantOMP:   []string{"zsh", "-ic", `exec -- "$@"`, "webmux", "omp", "--auto-approve"},
		},
		{
			name:      "unset falls back to bash",
			shell:     "",
			wantLogin: []string{"/bin/sh", "-l", "-i"},
			wantOMP:   []string{"/usr/bin/env", "bash", "-ic", `exec -- "$@"`, "webmux", "omp", "--auto-approve"},
		},
		{
			name:      "other shell runs directly",
			shell:     "/usr/bin/fish",
			wantLogin: []string{"/usr/bin/fish", "-l", "-i"},
			wantOMP:   []string{"omp", "--auto-approve"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			presets := LaunchPresets(tt.shell)
			if len(presets) != 14 || presets[0].Name != "Shell" || !slices.Equal(presets[0].Cmd, tt.wantLogin) {
				t.Fatalf("login shell preset = %+v", presets[:min(1, len(presets))])
			}
			for _, preset := range presets {
				if preset.Name != "omp" {
					continue
				}
				if !slices.Equal(preset.Cmd, tt.wantOMP) {
					t.Fatalf("omp argv = %q, want %q", preset.Cmd, tt.wantOMP)
				}
				return
			}
			t.Fatal("omp preset missing")
		})
	}
}
