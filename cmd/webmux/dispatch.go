package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rainchan/webmux/internal/contract"
)

// serverHelpFooter ends the top-level help. The Windows console client
// looks for "-log-file" in `webmux -h` to tell a webmux with the local API
// from an older one, so it must stay.
const serverHelpFooter = "Without a command, webmux runs serve. Server options: --config PATH, --listen HOST:PORT, " +
	"--token TOKEN, --tunnel, --log-file PATH. Services: Linux uses systemd --user and macOS a launchd LaunchAgent; " +
	"do not use sudo for them. Only \"update apply\" and \"update channel <stable|dev>\" run as root."

// dispatch routes by the first argument. No argument or a flag first keeps
// the original server invocation used by existing service templates.
func dispatch(args []string, stdout, stderr io.Writer) int {
	registry := commandRegistry()
	e := sessionEnv(stdout, stderr)
	e.Commands = registry
	e.HelpFooter = serverHelpFooter
	if len(args) > 0 {
		switch args[0] {
		case "help":
			return e.HelpCommand(registry, version, args[1:])
		case "--help", "-help", "-h":
			return e.HelpCommand(registry, version, nil)
		case "--version":
			_, _ = fmt.Fprintln(stdout, "webmux", version)
			return contract.ExitOK
		}
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return runServe(args)
	}
	// The operations commands parse their own flags; help comes from the
	// command table for all of them.
	if c, rest := registry.Find(args); c != nil && isOps(c.Name) {
		if c.Bind == nil && len(rest) == 0 {
			return e.Run(context.Background(), c, nil)
		}
		if helpRequested(rest) && (c.Bind != nil || len(rest) == 1) {
			e.PrintHelp(c)
			return contract.ExitOK
		}
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "start":
		return startCommand(args[1:], stdout, stderr)
	case "version":
		return versionCommand(args[1:], stdout, stderr)
	case "status":
		return statusCommand(args[1:], stdout, stderr)
	case "stop":
		return stopCommand(args[1:], stdout, stderr)
	case "mcp-serve":
		return mcpCommand(args[1:], os.Stdin, stdout, stderr)
	case "bridge":
		return bridgeCommand(args[1:], os.Stdin, stdout, stderr)
	case "config":
		return configCommand(args[1:], stdout, stderr)
	case "service":
		return serviceCommand(args[1:], stdout, stderr)
	case "update":
		return updateCommand(args[1:], stdout, stderr)
	}
	if code, ok := sessionCommand(args, stdout, stderr); ok {
		return code
	}
	_, _ = fmt.Fprintf(stderr, "webmux: unknown command %q\nRun 'webmux help' for the list of commands.\n", args[0])
	return contract.ExitUsage
}

// isOps reports whether command name belongs to the operations commands,
// which keep their own parsers.
func isOps(name string) bool {
	first, _, _ := strings.Cut(name, " ")
	switch first {
	case "serve", "start", "version", "status", "stop", "bridge", "mcp-serve", "config", "service", "update", "help":
		return true
	}
	return false
}

// helpRequested reports whether args ask for help before any "--".
func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "-h" || arg == "--help" || arg == "-help" {
			return true
		}
	}
	return false
}
