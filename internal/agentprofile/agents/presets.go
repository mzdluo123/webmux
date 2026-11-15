package agents

import (
	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agy"
	"github.com/rainchan/webmux/internal/agentprofile/claudecode"
	"github.com/rainchan/webmux/internal/agentprofile/codex"
	"github.com/rainchan/webmux/internal/agentprofile/devin"
	"github.com/rainchan/webmux/internal/agentprofile/kimi"
	"github.com/rainchan/webmux/internal/agentprofile/omp"
	"github.com/rainchan/webmux/internal/agentprofile/opencode"
	"github.com/rainchan/webmux/internal/agentprofile/qoder"
	"github.com/rainchan/webmux/internal/agentprofile/qodercn"
)

// LaunchTemplate is an editable CLI starting point, not a capability claim.
// Runtime identities, executable aliases, icons and verified versions belong
// to the adapter's Profile; TUI-only tools need no runtime adapter.
// Shell wrapping and conversion to the launch service's input belong to cmd.
type LaunchTemplate struct {
	Name   string
	Icon   string
	Argv   []string
	Env    map[string]string
	Hybrid bool
}

// LaunchTemplates returns fresh descriptions in the established preset order.
// Permission flags are deliberately part of the editable command, not policy
// enforced by the launch service. The shell preset is supplied by cmd.
func LaunchTemplates() []LaunchTemplate {
	runtime := func(a agentprofile.Agent, args ...string) LaunchTemplate {
		p := a.Profile()
		exe := p.Name
		if len(p.Executables) != 0 {
			exe = p.Executables[0]
		}
		return LaunchTemplate{Name: p.Title, Icon: p.Icon, Argv: append([]string{exe}, args...), Hybrid: true}
	}
	openCode := runtime(opencode.Agent, "--auto")
	// Preserve the saved preset name; the runtime's display title is OpenCode.
	openCode.Name = "Open Code"
	ompCLI := runtime(omp.Agent, "--auto-approve")
	ompCLI.Env = map[string]string{"PI_FORCE_IMAGE_PROTOCOL": "kitty"}
	return []LaunchTemplate{
		runtime(claudecode.Agent, "--dangerously-skip-permissions"),
		runtime(codex.Agent, "--dangerously-bypass-approvals-and-sandbox"),
		runtime(agy.Agent, "--dangerously-skip-permissions"),
		{Name: "Cursor CLI", Icon: "cursor", Argv: []string{"agent"}},
		{Name: "GitHub Copilot CLI", Icon: "githubcopilot", Argv: []string{"copilot"}},
		openCode,
		{Name: "Qwen Code", Icon: "qwen", Argv: []string{"qwen"}},
		runtime(kimi.Agent, "--yolo"),
		{Name: "CodeBuddy Code", Icon: "codebuddy", Argv: []string{"codebuddy"}},
		ompCLI,
		runtime(qoder.Agent, "--dangerously-skip-permissions"),
		runtime(qodercn.Agent, "--dangerously-skip-permissions"),
		runtime(devin.Agent, "--permission-mode", "dangerous"),
	}
}
