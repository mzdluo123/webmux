package contract

import "time"

// AgentConnectorStatus describes the local installation and upstream update
// state of an external ACP connector executable associated with a hybrid agent.
type AgentConnectorStatus struct {
	Executable       string `json:"executable"`
	Title            string `json:"title"`
	Installed        bool   `json:"installed"`
	Path             string `json:"path,omitempty"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	LatestVersion    string `json:"latestVersion,omitempty"`
	VerifiedVersion  string `json:"verifiedVersion,omitempty"`
	BelowVerified    bool   `json:"belowVerified,omitempty"`
	UpdateStatus     string `json:"updateStatus"`
	CheckError       string `json:"checkError,omitempty"`
	InstallCommand   string `json:"installCommand,omitempty"`
	UpgradeCommand   string `json:"upgradeCommand,omitempty"`
	DocsURL          string `json:"docsUrl,omitempty"`
}

// AgentToolStatus describes the local installation and upstream update state of
// one built-in CLI agent entry.
type AgentToolStatus struct {
	ID               string                `json:"id"`
	Title            string                `json:"title"`
	Icon             string                `json:"icon,omitempty"`
	Kind             string                `json:"kind"`
	Installed        bool                  `json:"installed"`
	Executable       string                `json:"executable"`
	Executables      []string              `json:"executables"`
	Path             string                `json:"path,omitempty"`
	InstalledVersion string                `json:"installedVersion,omitempty"`
	LatestVersion    string                `json:"latestVersion,omitempty"`
	VerifiedVersion  string                `json:"verifiedVersion,omitempty"`
	BelowVerified    bool                  `json:"belowVerified,omitempty"`
	UpdateStatus     string                `json:"updateStatus"`
	CheckError       string                `json:"checkError,omitempty"`
	InstallCommand   string                `json:"installCommand,omitempty"`
	UpgradeCommand   string                `json:"upgradeCommand,omitempty"`
	DocsURL          string                `json:"docsUrl,omitempty"`
	Connector        *AgentConnectorStatus `json:"connector,omitempty"`
}

// AgentToolsReport is the response body of GET /api/agent-tools and
// POST /api/agent-tools/check.
type AgentToolsReport struct {
	CheckedAt       time.Time         `json:"checkedAt"`
	RemoteCheckedAt *time.Time        `json:"remoteCheckedAt,omitempty"`
	Shell           string            `json:"shell"`
	Tools           []AgentToolStatus `json:"tools"`
}

// AgentToolsCheckRequest is the optional request body of
// POST /api/agent-tools/check.
type AgentToolsCheckRequest struct {
	RefreshLocal bool `json:"refreshLocal,omitempty"`
}

// AgentToolsV1 is printed by `webmux agents --json`.
type AgentToolsV1 struct {
	V               int               `json:"v"`
	CheckedAt       time.Time         `json:"checkedAt"`
	RemoteCheckedAt *time.Time        `json:"remoteCheckedAt,omitempty"`
	Shell           string            `json:"shell"`
	Tools           []AgentToolStatus `json:"tools"`
}
