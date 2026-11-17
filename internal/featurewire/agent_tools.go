package featurewire

import (
	"github.com/rainchan/webmux/internal/agentcheck"
	"github.com/rainchan/webmux/internal/contract"
)

// AgentToolsReport converts a service-layer agent tool report into its wire DTO.
func AgentToolsReport(r agentcheck.Report) contract.AgentToolsReport {
	return contract.AgentToolsReport{
		CheckedAt:       r.CheckedAt,
		RemoteCheckedAt: r.RemoteCheckedAt,
		Shell:           r.Shell,
		Tools:           each(r.Tools, AgentToolStatus),
	}
}

// AgentToolStatus converts a single tool status into its wire DTO.
func AgentToolStatus(t agentcheck.ToolStatus) contract.AgentToolStatus {
	var conn *contract.AgentConnectorStatus
	if t.Connector != nil {
		c := AgentConnectorStatus(*t.Connector)
		conn = &c
	}
	return contract.AgentToolStatus{
		ID:               t.ID,
		Title:            t.Title,
		Icon:             t.Icon,
		Kind:             string(t.Kind),
		Installed:        t.Installed,
		Executable:       t.Executable,
		Executables:      required(t.Executables),
		Path:             t.Path,
		InstalledVersion: t.InstalledVersion,
		LatestVersion:    t.LatestVersion,
		VerifiedVersion:  t.VerifiedVersion,
		BelowVerified:    t.BelowVerified,
		UpdateStatus:     string(t.UpdateStatus),
		CheckError:       t.CheckError,
		InstallCommand:   t.InstallCommand,
		UpgradeCommand:   t.UpgradeCommand,
		DocsURL:          t.DocsURL,
		Connector:        conn,
	}
}

// AgentConnectorStatus converts an ACP connector status into its wire DTO.
func AgentConnectorStatus(c agentcheck.ConnectorStatus) contract.AgentConnectorStatus {
	return contract.AgentConnectorStatus{
		Executable:       c.Executable,
		Title:            c.Title,
		Installed:        c.Installed,
		Path:             c.Path,
		InstalledVersion: c.InstalledVersion,
		LatestVersion:    c.LatestVersion,
		VerifiedVersion:  c.VerifiedVersion,
		BelowVerified:    c.BelowVerified,
		UpdateStatus:     string(c.UpdateStatus),
		CheckError:       c.CheckError,
		InstallCommand:   c.InstallCommand,
		UpgradeCommand:   c.UpgradeCommand,
		DocsURL:          c.DocsURL,
	}
}
