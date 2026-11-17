package claudecode

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/rainchan/webmux/internal/agentprofile/trustconfig"
)

// Trust describes folder trust without requiring runtime/ACP support.
var Trust = trustconfig.Policy{
	Name:  "claude",
	Match: func(base string) bool { return base == "claude" },
	Plan:  planTrust,
}

func planTrust(c trustconfig.Context, dir string) (trustconfig.Plan, error) {
	path, err := c.ConfigPath("CLAUDE_CONFIG_DIR", "", ".claude.json")
	return trustconfig.Plan{Path: path, Edit: func(data []byte) ([]byte, bool, error) { return claudeTrusted(data, dir) }}, err
}

// claudeTrusted returns data with the trust record added. changed is false
// when the record is already there.
func claudeTrusted(data []byte, dir string) (updated []byte, changed bool, err error) {
	root, err := trustconfig.ObjectOf(data)
	if err != nil {
		return nil, false, err
	}
	projects, err := trustconfig.ObjectOf(root["projects"])
	if err != nil {
		return nil, false, fmt.Errorf("projects: %w", err)
	}
	entry, err := trustconfig.ObjectOf(projects[dir])
	if err != nil {
		return nil, false, fmt.Errorf("projects[%q]: %w", dir, err)
	}
	if bytes.Equal(entry["hasTrustDialogAccepted"], []byte("true")) {
		return nil, false, nil
	}
	entry["hasTrustDialogAccepted"] = json.RawMessage("true")
	if projects[dir], err = trustconfig.EncodeJSON(entry); err != nil {
		return nil, false, err
	}
	if root["projects"], err = trustconfig.EncodeJSON(projects); err != nil {
		return nil, false, err
	}
	updated, err = trustconfig.IndentJSON(root)
	return updated, err == nil, err
}
