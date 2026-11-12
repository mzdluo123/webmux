package clicmd

import (
	"context"
	"flag"

	"github.com/rainchan/webmux/internal/mcpcontrol"
)

var mcpCommand = &Command{
	Name: "mcp", Group: GroupGuide,
	Summary:     "Serve WebMux session tools over MCP stdio.",
	Description: "Uses the same local API connection as other session commands. The handoff history MCP remains separate and read-only.",
	Output:      "MCP JSON-RPC on stdout; diagnostics on stderr.",
	Exit:        []ExitItem{{Code: 0, Text: "Input closed."}, {Code: 1, Text: "Transport failed."}, {Code: 2, Text: "Usage error."}},
	Examples:    []Example{{Command: "webmux mcp", Text: "Connect an MCP client to the default instance."}, {Command: "webmux mcp --socket /path/webmux.sock", Text: "Connect to an explicit instance."}},
	SeeAlso:     []string{"webmux guide", "webmux ls"},
	Bind: func(_ *flag.FlagSet) Action {
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 0 {
				return e.Usagef("mcp accepts no positional arguments")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			if e.Stdin == nil {
				return e.Usagef("mcp requires standard input")
			}
			if err := (mcpcontrol.Client{API: client}).Server().Serve(ctx, e.Stdin, e.Stdout); err != nil {
				return e.Fail(err)
			}
			return 0
		}
	},
}
