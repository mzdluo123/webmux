package assemble

import (
	"strings"

	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agents"
	"github.com/rainchan/webmux/internal/launch"
)

// DefaultShellCmd returns the login-shell argv for shell, falling back to
// /bin/sh when shell is empty.
func DefaultShellCmd(shell string) []string {
	login := strings.TrimSpace(shell)
	if login == "" {
		login = "/bin/sh"
	}
	return []string{login, "-l", "-i"}
}

// LaunchPresets converts built-in CLI descriptions into launch inputs. Only
// launch-enabled assembly calls this; core-only startup needs no agent tools.
// An empty shell keeps the historical /bin/sh login and bash startup wrapper.
func LaunchPresets(shell string) []launch.Input {
	templates := agents.LaunchTemplates()
	out := make([]launch.Input, 0, len(templates)+1)
	out = append(out, launch.Input{Name: "Shell", Icon: "terminal", Cmd: DefaultShellCmd(shell)})
	startup := agentprofile.DefaultShell(shell)
	for _, p := range templates {
		out = append(out, launch.Input{
			Name:   p.Name,
			Icon:   p.Icon,
			Cmd:    agentprofile.StartupArgv(startup, p.Argv[0], p.Argv[1:]...),
			Env:    p.Env,
			Hybrid: p.Hybrid,
		})
	}
	return out
}
