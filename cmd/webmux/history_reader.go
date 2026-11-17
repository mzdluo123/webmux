package main

import (
	"context"
	"github.com/rainchan/webmux/internal/acp"
)

func historyReader(stateDir, executable string) func(context.Context, string) []acp.MCPServer {
	return func(ctx context.Context, id string) []acp.MCPServer {
		socket, ok := mcpSocket(stateDir, executable)
		if !ok {
			return nil
		}
		return []acp.MCPServer{{Name: "webmux-history", Command: executable, Args: []string{"mcp-serve", "--socket", socket, "--session", id}, Env: []acp.MCPEnv{}}}
	}
}
