package api

import (
	"time"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/session"
)

// testSessionOptions mirrors cmd/internal/assemble, which tests outside cmd/
// cannot import.
func testSessionOptions(c config.Config) session.Options {
	return session.Options{
		ScrollbackBytes:      c.ScrollbackBytes,
		DefaultCwd:           c.DefaultCwd,
		Presets:              testPresets(c.Presets),
		IdleAfter:            time.Duration(c.IdleAfterSeconds) * time.Second,
		PromptPatterns:       c.PromptPatterns,
		DisableAgentProtocol: !c.AgentProtocol,
		DisableBellAttention: !c.BellAttention,
	}
}

func testPresets(presets []config.Preset) []session.Preset {
	out := make([]session.Preset, 0, len(presets))
	for _, p := range presets {
		out = append(out, session.Preset{Name: p.Name, Label: p.Label, Cmd: p.Cmd, AllowArgs: p.AllowArgs})
	}
	return out
}
