// Package copilot owns Copilot CLI configuration capabilities.
package copilot

import (
	"github.com/rainchan/webmux/internal/agentprofile/trustconfig"
)

// Trust describes folder trust without requiring runtime/ACP support.
var Trust = trustconfig.Policy{
	Name:  "copilot",
	Match: func(base string) bool { return base == "copilot" },
	Plan:  planTrust,
}

func planTrust(c trustconfig.Context, dir string) (trustconfig.Plan, error) {
	path, err := c.ConfigPath("COPILOT_HOME", ".copilot", "config.json")
	return trustconfig.Plan{Path: path, Edit: func(data []byte) ([]byte, bool, error) { return copilotTrusted(data, dir) }}, err
}

// copilotFoldersKey is the array Copilot CLI reads, per GitHub's "Configuring
// GitHub Copilot CLI". Older snake_case spelling is only kept when a file
// already uses it and has no current key.
const (
	copilotFoldersKey       = "trustedFolders"
	copilotLegacyFoldersKey = "trusted_folders"
)

func copilotTrusted(data []byte, dir string) (updated []byte, changed bool, err error) {
	root, err := trustconfig.ObjectOf(data)
	if err != nil {
		return nil, false, err
	}
	key := copilotFoldersKey
	if _, current := root[key]; !current {
		if _, legacy := root[copilotLegacyFoldersKey]; legacy {
			key = copilotLegacyFoldersKey
		}
	}
	changed, err = trustconfig.AppendPath(root, key, dir)
	if err != nil || !changed {
		return nil, false, err
	}
	updated, err = trustconfig.IndentJSON(root)
	return updated, err == nil, err
}
