package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rainchan/webmux/cmd/internal/assemble"
	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/clicmd"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/runtime"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/transport"
	"github.com/rainchan/webmux/internal/update"
)

var version = "dev"

// serverExecutable is the running executable as os.Executable reports it,
// or "" with a warning when it cannot be determined. On Linux it reads
// /proc/self/exe, which already names the real file and drops the
// " (deleted)" suffix an in-place upgrade leaves.
var serverExecutable = sync.OnceValue(func() string {
	path, err := os.Executable()
	if err != nil {
		slog.Warn("executable path unavailable; sessions get no WEBMUX_BIN", "error", err)
		return ""
	}
	return path
})

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

func runServe(args []string) int {
	if err := run(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return contract.ExitOK
		}
		slog.Error("webmux failed", "error", err)
		return contract.ExitError
	}
	return contract.ExitOK
}

func run(args []string) error {
	c, err := loadConfig(args)
	if err != nil {
		return err
	}
	defer c.closeSettings()
	if c.LogFile != "" {
		if err := redirectOutput(c.LogFile); err != nil {
			return fmt.Errorf("log file: %w", err)
		}
	}
	applyLaunchEnv()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serve(ctx, c, os.Stdout)
}

// serve runs until ctx ends. c must come from loadConfig, which generates or
// vets the token. Access URLs go to out, never to slog, so tokens stay out of
// structured logs.
func serve(ctx context.Context, c serverRun, out io.Writer) error {
	if err := c.Plugins.Validate(); err != nil {
		return err
	}
	if c.Plugins.Has("mcp") && c.MCP.Enabled && c.Token == "" {
		return errMCPOwnerToken
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	listener, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			slog.Warn("HTTP listener close failed", "error", closeErr)
		}
	}()
	sessionOptions := assemble.SessionOptions(c.Config)
	sessionOptions.TerminalVersion = version
	sessionOptions.Executable = serverExecutable()
	m := session.NewManager(sessionOptions)
	preferences := settings.Open(c.StateDir)
	plugins := &pluginServices{}
	var server *http.Server
	var rt *localRuntime
	var resumed <-chan struct{}
	defer func() {
		// Stop all writers and their processes before releasing any backing
		// store. This path also releases a partially constructed plugin graph.
		cancel()
		if resumed != nil {
			<-resumed
		}
		plugins.stop()
		shutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		m.Shutdown(shutdown)
		if server != nil {
			if shutdownErr := server.Shutdown(shutdown); shutdownErr != nil {
				_ = server.Close()
				slog.Warn("forced HTTP shutdown", "error", shutdownErr)
			}
		}
		if rt != nil {
			rt.close(shutdown)
		}
		plugins.release()
		preferences.Close()
	}()
	public := &transport.PublicOrigins{}
	guide := &agentguide.Instance{
		Guide:   agentguide.New(clicmd.Directory(commandRegistry())),
		Version: version, LocalURL: localOrigin(listener.Addr()), Bin: serverExecutable(),
	}
	var localAPI atomic.Pointer[localRuntime]
	var bridge transport.Bridge
	if err := plugins.open(ctx, c, m, preferences, public, listener.Addr(), guide, localAPI.Load, &bridge, out); err != nil {
		return err
	}
	deps := plugins.bind(c.Config, m, preferences, public, guide, localOrigin(listener.Addr()))
	deps.DefaultShell = agentprofile.DefaultShell(os.Getenv("SHELL"))
	deps.InstalledVersion = update.NewInstalledVersion(version).Pending
	deps.LocalURL = localOrigin(listener.Addr())
	deps.Guide = guide
	deps.Launcher = runtime.Launcher(os.Getenv)
	if c.Plugins.Has("mcp") && c.MCP.Enabled {
		handler, closeMCP, err := networkMCP(ctx, c.Config, listener.Addr(), deps.Capabilities)
		if err != nil {
			return fmt.Errorf("configure network MCP: %w", err)
		}
		deps.Contributions = append(deps.Contributions, networkMCPContribution(handler))
		defer closeMCP()
	}
	bridge = api.NewBridge(deps)
	providers := plugins.accessProviders()
	rt = startRuntime(ctx, c, listener.Addr(), deps, cancel, plugins, providers)
	localAPI.Store(rt)
	server = &http.Server{
		Handler:           bridge.HTTPHandler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	ended := make(chan error, 1)
	go func() { ended <- server.Serve(listener) }()
	rt.serve()
	slog.Info("webmux listening", "address", listener.Addr(), "version", version)
	if plugins.chat != nil {
		done := make(chan struct{})
		resumed = done
		go func() {
			defer close(done)
			resumeRestarted(ctx, plugins.recovery, plugins.chat.Restarted())
		}()
	}
	printConfigNotice(out, c)
	local := localOrigin(listener.Addr())
	_, _ = fmt.Fprintln(out, "WebMux local:  "+local)
	if plugins.tunnel != nil && plugins.tunnel.Settings().Enabled {
		plugins.tunnel.startLocated(plugins.tunnelPath)
	}
	if plugins.tailscale != nil {
		plugins.tailscale.Start(ctx, local)
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-ended:
		if err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

// printConfigNotice names the configuration in use and points at the local
// API for the current token without putting credentials in captured output.
func printConfigNotice(out io.Writer, c serverRun) {
	if c.Token != "" {
		_, _ = fmt.Fprintln(out, "Token: reveal it with: webmux status --json --reveal-token")
	}
	if c.ConfigPath == "" {
		return
	}
	if c.ConfigCreated {
		_, _ = fmt.Fprintln(out, "Created config:  "+c.ConfigPath)
	} else {
		_, _ = fmt.Fprintln(out, "Using config:    "+c.ConfigPath)
	}
}
