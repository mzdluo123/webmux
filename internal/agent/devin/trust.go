package devin

import (
	"path/filepath"

	"github.com/rainchan/webmux/internal/agentprofile/trustconfig"
)

// Trust describes folder trust without requiring runtime/ACP support.
var Trust = trustconfig.Policy{
	Name:  "devin",
	Match: func(base string) bool { return base == "devin" },
	Plan:  planTrust,
}

func planTrust(c trustconfig.Context, dir string) (trustconfig.Plan, error) {
	base := c.Env("XDG_DATA_HOME")
	if base == "" {
		home, err := c.Home()
		if err != nil {
			return trustconfig.Plan{}, err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return trustconfig.Plan{Path: filepath.Join(base, "devin", "cli", "trusted_workspaces.json"), Create: true, CreateParents: true, Edit: func(data []byte) ([]byte, bool, error) { return trusted(data, "trusted_paths", dir) }}, nil
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
