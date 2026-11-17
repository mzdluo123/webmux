package hybrid

import (
	"context"
	"github.com/rainchan/webmux/internal/acp"
)

func (a *actor) sessionMCPServers(ctx context.Context) ([]acp.MCPServer, string) {
	var out []acp.MCPServer
	if a.svc.sessionMCPServers != nil {
		out = append(out, a.svc.sessionMCPServers(ctx, a.id, a.cmd.Profile)...)
	}
	if a.svc.mcpServers != nil {
		out = append(out, a.svc.mcpServers(ctx)...)
	}
	if a.svc.historyReader != nil {
		out = append(out, a.svc.historyReader(ctx, a.id)...)
	}
	return out, ""
}
