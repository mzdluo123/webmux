package clicmd

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"text/tabwriter"

	"github.com/rainchan/webmux/internal/contract"
)

var agentsCommand = &Command{
	Name: "agents", Group: GroupLaunch,
	Summary: "Inspect locally installed CLI agent tools, ACP connectors, versions and available updates.",
	Description: "By default only installed agent tools (and their ACP connectors) are shown from the server's local startup-shell scan, " +
		"without contacting external package registries. Pass --all to include supported tools that are not installed, " +
		"--refresh to bypass the local scan cache, and --check to query public package registries for upstream updates.",
	Output: "A table: TOOL, KIND, VERSION, LATEST, STATUS, PATH (with indented '+ <connector>' rows for external ACP connectors). " +
		"With --json, the AgentToolsV1 object.",
	Exit: exitsCommon(
		ExitItem{Code: 1, Text: "Also: the launch feature is disabled or the server predates this command."},
	),
	Examples: []Example{
		{Command: "webmux agents", Text: "List locally installed agent tools and their ACP connectors."},
		{Command: "webmux agents --all", Text: "Include supported agent tools that are not installed."},
		{Command: "webmux agents --check --json", Text: "Query upstream package registries for updates and print JSON."},
	},
	SeeAlso: []string{"webmux launch-config ls", "webmux launch-config presets"},
	Bind: func(fs *flag.FlagSet) Action {
		check := fs.Bool("check", false, "query upstream package registries for available updates")
		refresh := fs.Bool("refresh", false, "rescan local executables instead of using cached results")
		all := fs.Bool("all", false, "include supported agent tools that are not installed")
		asJSON := fs.Bool("json", false, "print the AgentToolsV1 object as JSON")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 0 {
				return e.Usagef("agents takes no arguments")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()

			var report contract.AgentToolsReport
			var err error
			if *check {
				err = client.Do(ctx, http.MethodPost, "/api/agent-tools/check", contract.AgentToolsCheckRequest{
					RefreshLocal: *refresh,
				}, &report)
			} else {
				path := "/api/agent-tools"
				if *refresh {
					path += "?refresh=1"
				}
				err = client.Do(ctx, http.MethodGet, path, nil, &report)
			}
			if err != nil {
				return e.failNew(ctx, client, err)
			}

			tools := filterAgentTools(report.Tools, *all)
			if *asJSON {
				writeJSON(e.Stdout, contract.AgentToolsV1{
					V:               1,
					CheckedAt:       report.CheckedAt,
					RemoteCheckedAt: report.RemoteCheckedAt,
					Shell:           report.Shell,
					Tools:           tools,
				})
				return contract.ExitOK
			}

			printAgentToolsTable(e, tools)
			return contract.ExitOK
		}
	},
}

func filterAgentTools(list []contract.AgentToolStatus, includeAll bool) []contract.AgentToolStatus {
	if includeAll {
		if list == nil {
			return []contract.AgentToolStatus{}
		}
		return list
	}
	out := make([]contract.AgentToolStatus, 0, len(list))
	for _, item := range list {
		if item.Installed || (item.Connector != nil && item.Connector.Installed) {
			out = append(out, item)
		}
	}
	return out
}

func printAgentToolsTable(e Env, tools []contract.AgentToolStatus) {
	w := tabwriter.NewWriter(e.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TOOL\tKIND\tVERSION\tLATEST\tSTATUS\tPATH")
	for _, t := range tools {
		status := formatAgentUpdateStatus(t.UpdateStatus, t.BelowVerified, t.VerifiedVersion, t.CheckError)
		_, _ = fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			t.Title,
			t.Kind,
			valueOr(t.InstalledVersion, "-"),
			valueOr(t.LatestVersion, "-"),
			status,
			valueOr(t.Path, "-"),
		)
		if t.Connector != nil {
			c := t.Connector
			cStatus := formatAgentUpdateStatus(c.UpdateStatus, c.BelowVerified, c.VerifiedVersion, c.CheckError)
			_, _ = fmt.Fprintf(
				w,
				"  + %s\tconnector\t%s\t%s\t%s\t%s\n",
				c.Executable,
				valueOr(c.InstalledVersion, "-"),
				valueOr(c.LatestVersion, "-"),
				cStatus,
				valueOr(c.Path, "-"),
			)
		}
	}
	_ = w.Flush()
}

func formatAgentUpdateStatus(status string, belowVerified bool, verifiedVersion, checkError string) string {
	out := valueOr(status, "-")
	if belowVerified && verifiedVersion != "" {
		out += " (<" + verifiedVersion + ")"
	}
	if status == "check_failed" && checkError != "" {
		out += " (" + checkError + ")"
	}
	return out
}
