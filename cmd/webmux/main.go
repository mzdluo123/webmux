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
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rainchan/webmux/cmd/internal/assemble"
	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agents"
	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/auth"
	"github.com/rainchan/webmux/internal/clicmd"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/perf"
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
	if err := c.Features.Validate(); err != nil {
		return err
	}
	if c.Features.Has("mcp") && c.MCP.Enabled && c.Token == "" {
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
	registry := agents.Default()
	sessionOptions := assemble.SessionOptions(c.Config, registry)
	sessionOptions.TerminalVersion = version
	sessionOptions.Executable = serverExecutable()
	m := session.NewManager(sessionOptions)
	var perfRec *perf.Recorder
	if c.Perf.Enabled {
		rec, perfErr := perf.New(perf.Options{
			Dir:            filepath.Join(c.StateDir, "perf"),
			SlowMS:         c.Perf.SlowMS,
			SampleSeconds:  c.Perf.SampleSeconds,
			MaxBytesMB:     c.Perf.MaxBytesMB,
			RetentionDays:  c.Perf.RetentionDays,
			FlightRecorder: c.Perf.FlightRecorder,
			SpikeMS:        c.Perf.SpikeMS,
			Sessions: func() []perf.SessionRef {
				items := m.List()
				refs := make([]perf.SessionRef, 0, len(items))
				for _, item := range items {
					if item.Status == session.StatusExited {
						continue
					}
					pid := 0
					if s, getErr := m.Get(item.ID); getErr == nil {
						if procs := s.RunProcesses(); len(procs) > 0 {
							pid = procs[0].PID
						}
					}
					refs = append(refs, perf.SessionRef{
						ID:     item.ID,
						Name:   item.Title,
						PID:    pid,
						Status: string(item.Status),
					})
				}
				return refs
			},
		})
		if perfErr != nil {
			slog.Warn("performance recorder disabled", "error", perfErr)
		} else {
			perfRec = rec
			perf.SetDefault(perfRec)
			defer func() {
				perf.SetDefault(nil)
				perfRec.Close()
			}()
		}
	}
	preferences := settings.Open(c.StateDir)
	hostname, _ := os.Hostname()
	rpName := "WebMux"
	if hostname != "" {
		rpName = "WebMux (" + hostname + ")"
	}
	userName := "webmux"
	if currentUser, uErr := user.Current(); uErr == nil && currentUser.Username != "" {
		userName = currentUser.Username
		if hostname != "" {
			userName += "@" + hostname
		}
	}
	authSvc := auth.Open(c.StateDir, auth.Options{
		Disabled:      c.InsecureNoAuth,
		LegacyToken:   c.Token,
		RPDisplayName: rpName,
		UserName:      userName,
	})
	features := &featureServices{agents: registry, authReady: authSvc.Ready}
	var server *http.Server
	var rt *localRuntime
	defer func() {
		// Stop all writers and their processes before releasing any backing
		// store. This path also releases a partially constructed feature graph.
		cancel()
		features.stop()
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
		features.release()
		authSvc.Close()
		preferences.Close()
	}()
	public := &transport.PublicOrigins{}
	guide := &agentguide.Instance{
		Guide:   agentguide.New(clicmd.Directory(commandRegistry())),
		Version: version, LocalURL: localOrigin(listener.Addr()), Bin: serverExecutable(),
	}
	var localAPI atomic.Pointer[localRuntime]
	var bridge transport.Bridge
	if err := features.open(ctx, c, m, preferences, public, listener.Addr(), guide, localAPI.Load, &bridge, out); err != nil {
		return err
	}
	relayListener, relayTLS, err := relayLoopback(ctx)
	if err != nil {
		return fmt.Errorf("relay loopback: %w", err)
	}
	defer func() { _ = relayListener.Close() }()
	relayControl, err := newRelayController(ctx, c.Config, public, relayListener.Addr().String(), relayTLS)
	if err != nil {
		return fmt.Errorf("relay settings: %w", err)
	}
	defer relayControl.Close()
	relayControl.p2p = features.p2p
	if features.p2pControl != nil {
		features.p2pControl.relayAuth = relayControl.auth
	}
	deps := features.bind(c.Config, m, preferences, public, guide, localOrigin(listener.Addr()))
	deps.Auth = authSvc
	deps.DefaultShell = agentprofile.DefaultShell(os.Getenv("SHELL"))
	deps.InstalledVersion = update.NewInstalledVersion(version).Pending
	deps.LocalURL = browserOrigin(listener.Addr())
	deps.Guide = guide
	deps.Launcher = runtime.Launcher(os.Getenv)
	deps.Relay = &api.Relay{Control: relayControl, Auth: relayControl.auth}
	deps.Perf = perfRec
	if c.Features.Has("mcp") && c.MCP.Enabled {
		handler, closeMCP, err := networkMCP(ctx, c.Config, listener.Addr(), deps.Capabilities)
		if err != nil {
			return fmt.Errorf("configure network MCP: %w", err)
		}
		deps.Contributions = append(deps.Contributions, networkMCPContribution(handler))
		defer closeMCP()
	}
	bridge = api.NewBridge(deps)
	providers := features.accessProviders()
	providers = append(providers, relayControl)
	rt = startRuntime(ctx, c, listener.Addr(), deps, cancel, features, providers)
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
	relayServer := relayHTTPServer(ctx, bridge.HTTPHandler)
	relayEnded := make(chan struct{})
	go func() {
		defer close(relayEnded)
		if err := relayServer.Serve(relayListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("relay loopback stopped")
		}
	}()
	defer func() {
		_ = relayServer.Close()
		<-relayEnded
	}()
	relayControl.Start()
	slog.Info("webmux listening", "address", listener.Addr(), "version", version)
	printConfigNotice(out, c)
	local := localOrigin(listener.Addr())
	browser := browserOrigin(listener.Addr())
	_, _ = fmt.Fprintln(out, "WebMux local:  "+browser)
	if c.InsecureNoAuth {
		slog.Warn("authentication disabled via --insecure-no-auth (loopback only)")
	} else if authSvc.State() == auth.StateNeedsOnboarding {
		if grant, setupErr := authSvc.SetupInfo(); setupErr == nil {
			_, _ = fmt.Fprintln(out, "WebMux setup:  "+browser+"/onboarding#setup="+url.QueryEscape(grant.Code))
			_, _ = fmt.Fprintln(out, "Setup code:    "+grant.DisplayCode)
		}
	} else if authSvc.State() == auth.StateStorageError {
		slog.Error("auth storage unavailable (fail-closed); run 'webmux auth status' or 'webmux auth reset'", "error", authSvc.StorageError())
	}
	if features.tunnel != nil && features.tunnel.Settings().Enabled {
		features.tunnel.startLocated(features.tunnelPath)
	}
	if features.tailscale != nil {
		features.tailscale.Start(ctx, local)
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
