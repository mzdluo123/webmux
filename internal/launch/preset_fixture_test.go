package launch

import (
	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agents"
)

// Presets is a test fixture for persistence and restoration policy. Production
// conversion lives in cmd/internal/assemble, which services cannot import.
func Presets(shell string) []Input {
	login := shell
	if login == "" {
		login = "/bin/sh"
	}
	out := []Input{{Name: "Shell", Icon: "terminal", Cmd: []string{login, "-l", "-i"}}}
	startup := agentprofile.DefaultShell(shell)
	for _, p := range agents.LaunchTemplates() {
		out = append(out, Input{Name: p.Name, Icon: p.Icon, Cmd: agentprofile.StartupArgv(startup, p.Argv[0], p.Argv[1:]...), Env: p.Env, Hybrid: p.Hybrid})
	}
	return out
}
