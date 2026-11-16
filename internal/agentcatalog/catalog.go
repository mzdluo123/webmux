// Package agentcatalog builds the ordered list of CLI agent entries inspected
// by local tool and update checks: hybrid agents from an injected Registry plus
// built-in terminal-only preset agents.
package agentcatalog

import (
	"slices"

	"github.com/rainchan/webmux/internal/agentprofile"
)

// Kind distinguishes hybrid-capable agents from terminal-only preset agents.
type Kind string

const (
	KindHybrid   Kind = "hybrid"
	KindTerminal Kind = "terminal"
)

// ConnectorEntry describes an external ACP connector executable associated with
// a hybrid agent entry.
type ConnectorEntry struct {
	Executable      string
	Title           string
	VerifiedVersion string
	Distribution    agentprofile.Distribution
}

// Entry is the static metadata of one CLI agent tool inspected on the host.
type Entry struct {
	ID              string
	Title           string
	Icon            string
	Kind            Kind
	Executables     []string
	VerifiedVersion string
	Distribution    agentprofile.Distribution
	Connector       *ConnectorEntry
}

// FromRegistry projects the registered hybrid agents in registry order. A nil
// or empty registry yields an empty slice without falling back to built-ins.
func FromRegistry(r *agentprofile.Registry) []Entry {
	agents := r.Agents()
	out := make([]Entry, 0, len(agents))
	for _, a := range agents {
		p := a.Profile()
		exes := slices.Clone(p.Executables)
		if len(exes) == 0 {
			exes = []string{p.Name}
		}
		var conn *ConnectorEntry
		if p.Connector != nil {
			c := p.Connector
			conn = &ConnectorEntry{
				Executable:      c.Executable,
				Title:           c.Title,
				VerifiedVersion: c.VerifiedVersion,
				Distribution: agentprofile.Distribution{
					InstallCommand: c.InstallCommand,
					UpgradeCommand: c.UpgradeCommand,
					DocsURL:        c.DocsURL,
					Upstream:       c.Upstream,
				},
			}
		}
		out = append(out, Entry{
			ID:              p.Name,
			Title:           p.Title,
			Icon:            p.Icon,
			Kind:            KindHybrid,
			Executables:     exes,
			VerifiedVersion: p.VerifiedVersion,
			Distribution:    p.Distribution,
			Connector:       conn,
		})
	}
	return out
}

// TerminalPresets returns the built-in terminal-only CLI agent entries that
// ship as launch presets alongside the hybrid registry.
func TerminalPresets() []Entry {
	return []Entry{
		{
			ID:          "cursor",
			Title:       "Cursor CLI",
			Icon:        "cursor",
			Kind:        KindTerminal,
			Executables: []string{"agent", "cursor-agent"},
			Distribution: agentprofile.Distribution{
				DocsURL: "https://cursor.com/docs/cli/overview",
			},
		},
		{
			ID:          "copilot",
			Title:       "GitHub Copilot CLI",
			Icon:        "githubcopilot",
			Kind:        KindTerminal,
			Executables: []string{"copilot"},
			Distribution: agentprofile.Distribution{
				InstallCommand: "npm install -g @github/copilot",
				UpgradeCommand: "npm install -g @github/copilot@latest",
				DocsURL:        "https://docs.github.com/en/copilot/how-tos/copilot-cli/use-copilot-cli/overview",
				Upstream:       agentprofile.NPMUpstream("@github/copilot"),
			},
		},
		{
			ID:          "qwen",
			Title:       "Qwen Code",
			Icon:        "qwen",
			Kind:        KindTerminal,
			Executables: []string{"qwen"},
			Distribution: agentprofile.Distribution{
				InstallCommand: "npm install -g @qwen-code/qwen-code",
				UpgradeCommand: "npm install -g @qwen-code/qwen-code@latest",
				DocsURL:        "https://github.com/QwenLM/qwen-code",
				Upstream:       agentprofile.NPMUpstream("@qwen-code/qwen-code"),
			},
		},
		{
			ID:          "codebuddy",
			Title:       "CodeBuddy Code",
			Icon:        "codebuddy",
			Kind:        KindTerminal,
			Executables: []string{"codebuddy"},
			Distribution: agentprofile.Distribution{
				InstallCommand: "npm install -g @tencent-ai/codebuddy-code",
				UpgradeCommand: "npm install -g @tencent-ai/codebuddy-code@latest",
				DocsURL:        "https://www.codebuddy.ai/docs/cli/quickstart",
				Upstream:       agentprofile.NPMUpstream("@tencent-ai/codebuddy-code"),
			},
		},
	}
}

// Build combines the injected hybrid agent registry with the terminal-only
// preset entries, skipping any duplicate IDs.
func Build(r *agentprofile.Registry) []Entry {
	hybrid := FromRegistry(r)
	seen := make(map[string]bool, len(hybrid))
	for _, e := range hybrid {
		seen[e.ID] = true
	}
	out := make([]Entry, 0, len(hybrid)+4)
	out = append(out, hybrid...)
	for _, e := range TerminalPresets() {
		if !seen[e.ID] {
			out = append(out, e)
		}
	}
	return out
}
