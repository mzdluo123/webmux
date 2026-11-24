package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/rainchan/webmux/internal/auth"
	"github.com/rainchan/webmux/internal/client"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/pathutil"
	"github.com/rainchan/webmux/internal/wire"
)

const authUsage = "usage: webmux auth status|setup|pair|reset [--config PATH] [--json]"

func authCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, authUsage)
		return contract.ExitUsage
	}
	switch args[0] {
	case "--help", "-h", "help":
		_, _ = fmt.Fprintln(stdout, authUsage)
		return contract.ExitOK
	case "status":
		return authStatusCmd(args[1:], stdout, stderr)
	case "setup":
		return authSetupCmd(args[1:], stdout, stderr)
	case "pair":
		return authPairCmd(args[1:], stdout, stderr)
	case "reset":
		return authResetCmd(args[1:], stdout, stderr)
	}
	_, _ = fmt.Fprintf(stderr, "webmux auth: unknown subcommand %q\n%s\n", args[0], authUsage)
	return contract.ExitUsage
}

func authFlags(name string, args []string, stderr io.Writer, define func(*flag.FlagSet)) (string, bool, bool) {
	flags, path := clientFlags("auth "+name, stderr)
	if define != nil {
		define(flags)
	}
	if err := flags.Parse(args); err != nil {
		return "", false, false
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "webmux auth %s: unexpected argument %q\n", name, flags.Arg(0))
		return "", false, false
	}
	explicit := false
	flags.Visit(func(f *flag.Flag) { explicit = explicit || f.Name == "config" })
	expanded, err := pathutil.Expand(*path)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "webmux auth:", err)
		return "", false, false
	}
	return expanded, explicit, true
}

func authStatusCmd(args []string, stdout, stderr io.Writer) int {
	var asJSON *bool
	path, explicit, ok := authFlags("status", args, stderr, func(f *flag.FlagSet) {
		asJSON = f.Bool("json", false, "print JSON")
	})
	if !ok {
		return contract.ExitUsage
	}
	target := client.Locate(path, explicit)
	if target.ConfigErr != nil {
		_, _ = fmt.Fprintln(stderr, "webmux auth status:", target.ConfigErr)
		return contract.ExitError
	}
	var summary contract.AuthSummary
	if target.Record != nil {
		conn := target.LocalClient()
		ctx, cancel := context.WithTimeout(context.Background(), client.RequestTimeout)
		status, err := conn.Status(ctx)
		cancel()
		conn.Close()
		if err == nil && status.Auth != nil {
			summary = *status.Auth
		} else {
			insp, _ := auth.Inspect(target.Config.StateDir)
			summary = wire.AuthSummary(insp)
		}
	} else {
		insp, _ := auth.Inspect(target.Config.StateDir)
		summary = wire.AuthSummary(insp)
	}
	if *asJSON {
		writeJSON(stdout, summary)
		return contract.ExitOK
	}
	_, _ = fmt.Fprintf(stdout, "Auth state:      %s\n", summary.State)
	_, _ = fmt.Fprintf(stdout, "Passkeys:        %d\n", summary.PasskeyCount)
	if len(summary.RPIDs) > 0 {
		_, _ = fmt.Fprintf(stdout, "Domains (RP ID): %s\n", strings.Join(summary.RPIDs, ", "))
	}
	_, _ = fmt.Fprintf(stdout, "Active sessions: %d\n", summary.SessionCount)
	_, _ = fmt.Fprintf(stdout, "Recovery codes:  %d remaining\n", summary.RecoveryCodesRemaining)
	_, _ = fmt.Fprintf(stdout, "API tokens:      %d\n", summary.APITokenCount)
	if summary.Error != "" {
		_, _ = fmt.Fprintf(stdout, "Error:           %s\n", summary.Error)
	}
	return contract.ExitOK
}

func authSetupCmd(args []string, stdout, stderr io.Writer) int {
	var asJSON, refresh *bool
	path, explicit, ok := authFlags("setup", args, stderr, func(f *flag.FlagSet) {
		asJSON = f.Bool("json", false, "print JSON")
		refresh = f.Bool("refresh", false, "rotate the current setup code")
	})
	if !ok {
		return contract.ExitUsage
	}
	target := client.Locate(path, explicit)
	if target.ConfigErr != nil {
		_, _ = fmt.Fprintln(stderr, "webmux auth setup:", target.ConfigErr)
		return contract.ExitError
	}
	if target.Record == nil {
		_, _ = fmt.Fprintln(stderr, "webmux auth setup: server is not running; start WebMux first")
		return contract.ExitNotRunning
	}
	conn := target.LocalClient()
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), client.RequestTimeout)
	defer cancel()
	resp, err := conn.AuthSetup(ctx, *refresh)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "webmux auth setup:", err)
		return contract.ExitError
	}
	if *asJSON {
		writeJSON(stdout, resp)
		return contract.ExitOK
	}
	_, _ = fmt.Fprintf(stdout, "Setup URL:   %s\n", resp.SetupURL)
	_, _ = fmt.Fprintf(stdout, "Setup code:  %s\n", resp.DisplayCode)
	_, _ = fmt.Fprintf(stdout, "Expires at:  %s\n", resp.ExpiresAt)
	return contract.ExitOK
}

func authPairCmd(args []string, stdout, stderr io.Writer) int {
	var asJSON, showQR *bool
	var targetURL *string
	path, explicit, ok := authFlags("pair", args, stderr, func(f *flag.FlagSet) {
		asJSON = f.Bool("json", false, "print JSON")
		targetURL = f.String("url", "", "target HTTPS origin for the pairing link")
		showQR = f.Bool("qr", false, "print a QR code for the pairing link")
	})
	if !ok {
		return contract.ExitUsage
	}
	target := client.Locate(path, explicit)
	if target.ConfigErr != nil {
		_, _ = fmt.Fprintln(stderr, "webmux auth pair:", target.ConfigErr)
		return contract.ExitError
	}
	if target.Record == nil {
		_, _ = fmt.Fprintln(stderr, "webmux auth pair: server is not running; start WebMux first")
		return contract.ExitNotRunning
	}
	conn := target.LocalClient()
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), client.RequestTimeout)
	defer cancel()
	chosenURL := strings.TrimSpace(*targetURL)
	if chosenURL == "" {
		if status, err := conn.Status(ctx); err == nil {
			for _, link := range status.URLs.Access {
				if link.URL != "" {
					chosenURL = link.URL
					break
				}
			}
		}
	}
	resp, err := conn.AuthPair(ctx, chosenURL)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "webmux auth pair:", err)
		return contract.ExitError
	}
	if *asJSON {
		writeJSON(stdout, resp)
		return contract.ExitOK
	}
	_, _ = fmt.Fprintf(stdout, "Pairing code: %s\n", resp.DisplayCode)
	if resp.PairURL != "" {
		_, _ = fmt.Fprintf(stdout, "Pair URL:     %s\n", resp.PairURL)
	}
	_, _ = fmt.Fprintf(stdout, "Expires at:   %s\n", resp.ExpiresAt)
	if *showQR && resp.PairURL != "" {
		if qrText, qrErr := renderQR(resp.PairURL); qrErr == nil {
			_, _ = fmt.Fprint(stdout, qrText)
		}
	}
	return contract.ExitOK
}

func authResetCmd(args []string, stdout, stderr io.Writer) int {
	var asJSON, force *bool
	path, explicit, ok := authFlags("reset", args, stderr, func(f *flag.FlagSet) {
		asJSON = f.Bool("json", false, "print JSON")
		force = f.Bool("force", false, "confirm resetting all passkeys and active sessions")
	})
	if !ok {
		return contract.ExitUsage
	}
	if !*force {
		_, _ = fmt.Fprintln(stderr, "webmux auth reset: pass --force to revoke all passkeys, sessions, and recovery codes")
		return contract.ExitUsage
	}
	target := client.Locate(path, explicit)
	if target.ConfigErr != nil {
		_, _ = fmt.Fprintln(stderr, "webmux auth reset:", target.ConfigErr)
		return contract.ExitError
	}
	if target.Record != nil {
		conn := target.LocalClient()
		defer conn.Close()
		ctx, cancel := context.WithTimeout(context.Background(), client.RequestTimeout)
		defer cancel()
		resp, err := conn.AuthReset(ctx)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "webmux auth reset:", err)
			return contract.ExitError
		}
		if *asJSON {
			writeJSON(stdout, resp)
			return contract.ExitOK
		}
		_, _ = fmt.Fprintln(stdout, "Authentication reset. Complete onboarding in your browser:")
		_, _ = fmt.Fprintf(stdout, "Setup URL:   %s\n", resp.SetupURL)
		_, _ = fmt.Fprintf(stdout, "Setup code:  %s\n", resp.DisplayCode)
		return contract.ExitOK
	}
	if err := auth.ResetOffline(target.Config.StateDir); err != nil {
		_, _ = fmt.Fprintln(stderr, "webmux auth reset:", err)
		return contract.ExitError
	}
	if *asJSON {
		writeJSON(stdout, contract.Ack{OK: true})
		return contract.ExitOK
	}
	_, _ = fmt.Fprintln(stdout, "Authentication reset (offline). Start WebMux to run onboarding.")
	return contract.ExitOK
}
