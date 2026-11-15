package agy

import (
	"path/filepath"

	"github.com/rainchan/webmux/internal/agentprofile/trustconfig"
)

// Trust describes folder trust without requiring runtime/ACP support.
var Trust = trustconfig.Policy{
	Name:  "agy",
	Match: func(base string) bool { return base == "agy" },
	Plan:  planTrust,
}

func planTrust(c trustconfig.Context, dir string) (trustconfig.Plan, error) {
	home, err := c.Home()
	return trustconfig.Plan{Path: filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"), Edit: func(data []byte) ([]byte, bool, error) { return trusted(data, "trustedWorkspaces", dir) }}, err
}

func trusted(data []byte, key, dir string) ([]byte, bool, error) {
	root, err := trustconfig.ObjectOf(data)
	if err != nil {
		return nil, false, err
	}
	changed, err := trustconfig.AppendPath(root, key, dir)
	if err != nil || !changed {
		return nil, false, err
	}
	updated, err := trustconfig.IndentJSON(root)
	return updated, err == nil, err
}
