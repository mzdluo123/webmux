// Package qodercn is the hybrid adapter for QoderCN CLI (Chinese edition).
// It is the Qoder CLI under another name, so it reuses the qoder adapter.
package qodercn

import (
	"github.com/rainchan/webmux/internal/agent/qoder"
	ap "github.com/rainchan/webmux/internal/agentprofile"
)

// Name identifies QoderCN on the wire and in hybrid session records.
const Name = "qodercn"

// Agent is the QoderCN adapter.
var Agent ap.Agent = qoder.New(qoder.Variant{
	Name:        Name,
	Title:       "QoderCN",
	Executables: []string{"qodercn", "qoderclicn", "qoder-cn"},
	ProgramApps: []string{"qoder-cli-cn", "qoderclicn"},
})
