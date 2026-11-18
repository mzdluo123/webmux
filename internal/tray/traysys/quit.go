package traysys

import (
	"errors"
	"flag"
	"io"
	"time"
)

// Exit codes of `webmux-tray quit`.
const (
	QuitDone    = 0 // the running tray has exited
	QuitRunning = 1 // a tray still runs
	QuitUsage   = 2 // invalid arguments
	QuitNone    = 3 // no tray was running
)

// QuitOptions configures `webmux-tray quit`.
type QuitOptions struct {
	KillPath      string
	StopKeepalive bool
}

// ParseQuit parses the arguments after `quit`: optional `--kill-path PATH`
// and `--stop-keepalive`.
func ParseQuit(args []string) (opts QuitOptions, err error) {
	fs := flag.NewFlagSet("quit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.KillPath, "kill-path", "", "")
	fs.BoolVar(&opts.StopKeepalive, "stop-keepalive", false, "")
	if err := fs.Parse(args); err != nil {
		return QuitOptions{}, err
	}
	if fs.NArg() > 0 {
		return QuitOptions{}, errors.New("quit takes no positional arguments")
	}
	return opts, nil
}

// Quitter asks the running tray of this Windows user to exit as with
// "仅退出托盘", then waits for it. Running, Signal and Kill are the Windows
// primitives; the durations default to the documented 10 s and 2 s.
type Quitter struct {
	// Running reports whether a tray of this user still runs.
	Running func() bool
	// Signal sets the tray's quit event; false means the running tray is
	// an older one without the event.
	Signal func() bool
	// Kill terminates this user's trays started from path.
	Kill func(path string) error
	// StopKeepalive ends the recorded WSL keepalive process and deletes its
	// record file once no tray runs.
	StopKeepalive func() error
	Logf          func(format string, args ...any)

	Wait, Grace, Poll time.Duration
}

// Quit returns QuitNone, QuitDone or QuitRunning. With opts.KillPath set,
// trays started from it are terminated once Wait passes; trays elsewhere are
// left alone. With opts.StopKeepalive set, the recorded WSL keepalive is
// stopped once no tray runs (QuitDone or QuitNone).
func (q Quitter) Quit(opts QuitOptions) int {
	code := q.quitTray(opts.KillPath)
	if opts.StopKeepalive && q.StopKeepalive != nil && (code == QuitDone || code == QuitNone) {
		if err := q.StopKeepalive(); err != nil {
			q.logf("stop keepalive: %v", err)
		}
	}
	return code
}

func (q Quitter) quitTray(killPath string) int {
	if !q.Running() {
		return QuitNone
	}
	if !q.Signal() {
		q.logf("the running tray does not support quit requests")
	}
	if q.until(q.durationOr(q.Wait, 10*time.Second)) {
		return QuitDone
	}
	if killPath == "" {
		return QuitRunning
	}
	if err := q.Kill(killPath); err != nil {
		q.logf("terminate %s: %v", killPath, err)
	}
	if q.until(q.durationOr(q.Grace, 2*time.Second)) {
		return QuitDone
	}
	return QuitRunning
}

// until polls Running for at most d and reports whether the tray is gone.
func (q Quitter) until(d time.Duration) bool {
	poll := q.durationOr(q.Poll, 100*time.Millisecond)
	deadline := time.Now().Add(d)
	for {
		if !q.Running() {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(poll)
	}
}

func (q Quitter) durationOr(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

func (q Quitter) logf(format string, args ...any) {
	if q.Logf != nil {
		q.Logf(format, args...)
	}
}
