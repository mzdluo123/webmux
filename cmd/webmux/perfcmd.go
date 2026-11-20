package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/pathutil"
	"github.com/rainchan/webmux/internal/perf"
)

func perfCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "webmux perf: expected subcommand (report); see webmux help perf")
		return contract.ExitUsage
	}
	switch args[0] {
	case "report":
		return perfReportCommand(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "webmux perf: unknown subcommand %q\n", args[0])
		return contract.ExitUsage
	}
}

func perfReportCommand(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("webmux perf report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPath(), "TOML configuration `PATH`")
	since := fs.Duration("since", 2*time.Hour, "aggregate events within this `DURATION`")
	top := fs.Int("top", 10, "maximum `N` entries per ranking table")
	jsonOut := fs.Bool("json", false, "print the report as JSON")
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "webmux perf report: unexpected argument %q\n", fs.Arg(0))
		return contract.ExitUsage
	}
	if *since <= 0 || *top <= 0 {
		_, _ = fmt.Fprintln(stderr, "webmux perf report: --since and --top must be positive")
		return contract.ExitUsage
	}
	expanded, err := pathutil.Expand(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "webmux perf report: %v\n", err)
		return contract.ExitError
	}
	c, err := config.Load(expanded, false)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "webmux perf report: %v\n", err)
		return contract.ExitError
	}
	if c.StateDir == "" {
		c.StateDir = config.DefaultStateDir()
	} else if dir, expandErr := pathutil.Expand(c.StateDir); expandErr == nil {
		c.StateDir = dir
	}
	report, err := perf.Summarize(filepath.Join(c.StateDir, "perf"), *since, *top)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "webmux perf report: %v\n", err)
		return contract.ExitError
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			_, _ = fmt.Fprintf(stderr, "webmux perf report: %v\n", err)
			return contract.ExitError
		}
		return contract.ExitOK
	}
	_, _ = io.WriteString(stdout, perf.FormatText(report))
	return contract.ExitOK
}
