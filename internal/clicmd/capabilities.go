package clicmd

import (
	"context"
	"strings"

	"github.com/rainchan/webmux/internal/localclient"
)

// RequiredCapabilities describes unconditional command dependencies. Help is a
// static command catalog, not a claim that the connected instance enables them.
func RequiredCapabilities(name string) []string {
	root, _, _ := strings.Cut(name, " ")
	switch root {
	case "launch-config":
		if name == "launch-config options" {
			return []string{"launch.configs", "chat.acp", "workspace.manage"}
		}
		return []string{"launch.configs"}
	case "repo", "worktree":
		return []string{"workspace.manage"}
	case "chat", "view", "resume":
		return []string{"chat.acp"}
	case "forward":
		return []string{"forward.ports"}
	case "p2p":
		if name == "p2p status" || name == "p2p peers" || name == "p2p probe" {
			// Server-backed diagnostics require selection, not direct connectivity.
			// The shared status reader checks selection; an explicit STUN probe
			// does not contact the server at all.
			return nil
		}
		return []string{"p2p.direct"}
	case "dir":
		return []string{"files.upload"}
	case "mcp":
		return []string{"mcp.control"}
	case "tunnel":
		return []string{"cloudflare.manage"}
	case "tailscale":
		return []string{"tailscale.manage"}
	default:
		return nil
	}
}

func requireCapabilities(ctx context.Context, client *localclient.Client, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	snapshot, err := client.Capabilities(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := localclient.RequireSnapshotCapability(snapshot, id); err != nil {
			return err
		}
	}
	return nil
}
