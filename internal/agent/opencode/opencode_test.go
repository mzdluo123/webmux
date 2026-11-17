package opencode

import (
	"testing"

	"github.com/rainchan/webmux/internal/agent/agenttest"
	"github.com/rainchan/webmux/internal/agentprofile"
)

func TestContract(t *testing.T) {
	agenttest.Run(t, Agent, agenttest.Fixture{
		Signals:  agentprofile.Signals{ProgramApp: Name, ProgramChildID: "ses_e7fb", RootCwd: "/p"},
		Identity: agentprofile.Identity{Agent: Name, AgentSessionID: "ses_e7fb", Cwd: "/p"},
	})
}
