// Package agents lists the built-in hybrid agent adapters. Adding an agent
// is a new adapter subpackage of agentprofile and one entry here.
package agents

import (
	"github.com/rainchan/webmux/internal/agent/agy"
	"github.com/rainchan/webmux/internal/agent/claudecode"
	"github.com/rainchan/webmux/internal/agent/codex"
	"github.com/rainchan/webmux/internal/agent/devin"
	"github.com/rainchan/webmux/internal/agent/kimi"
	"github.com/rainchan/webmux/internal/agent/omp"
	"github.com/rainchan/webmux/internal/agent/opencode"
	"github.com/rainchan/webmux/internal/agent/qoder"
	"github.com/rainchan/webmux/internal/agent/qodercn"
	"github.com/rainchan/webmux/internal/agentprofile"
)

// defaultRegistry is in Identify order: omp's Warp events take precedence
// over another agent's OSC 7501 record.
var defaultRegistry = agentprofile.NewRegistry(omp.Agent, opencode.Agent, codex.Agent, claudecode.Agent, qoder.Agent, qodercn.Agent, kimi.Agent, agy.Agent, devin.Agent)

// Default returns the immutable registry of built-in agents.
func Default() *agentprofile.Registry { return defaultRegistry }
