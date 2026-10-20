package tunnel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// DefaultReadyTimeout bounds how long Start waits for the first edge
// connection before giving up on announcing the public URL.
const DefaultReadyTimeout = 60 * time.Second

// Quick Tunnel prints its hostname inside a boxed banner; matching the box
// avoids unrelated URLs such as api.trycloudflare.com in error messages.
var quickURL = regexp.MustCompile(`\|\s+(https://[a-z0-9-]+\.trycloudflare\.com)\s+\|`)

const registeredLine = "Registered tunnel connection"

// Options configures one cloudflared process.
type Options struct {
	Binary   string // cloudflared executable
	Local    string // origin URL, e.g. http://127.0.0.1:7681
	Token    string // Named Tunnel credential; empty selects a Quick Tunnel
	Hostname string // Named Tunnel public hostname
	Logger   *slog.Logger

	ReadyTimeout time.Duration // zero selects DefaultReadyTimeout
}

// Tunnel is a running cloudflared process.
type Tunnel struct {
	ready  chan string
	done   chan struct{}
	err    error
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Start launches cloudflared. Ready delivers the public https origin once the
// first edge connection registers; Done closes when the process exits.
func Start(ctx context.Context, o Options) (*Tunnel, error) {
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}
	readyTimeout := o.ReadyTimeout
	if readyTimeout == 0 {
		readyTimeout = DefaultReadyTimeout
	}
	// An empty --config keeps ~/.cloudflared/config.yml ingress rules from
	// overriding --url: a Quick Tunnel otherwise serves that file's catch-all.
	args := []string{"tunnel", "--no-autoupdate", "--config", os.DevNull}
	env := withoutTunnelToken(os.Environ())
	public := ""
	if o.Token != "" {
		args = append(args, "run")
		// The credential stays out of argv, which other local users can read.
		env = append(env, "TUNNEL_TOKEN="+o.Token)
		public = "https://" + o.Hostname
	} else {
		args = append(args, "--url", o.Local)
	}
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, o.Binary, args...)
	cmd.Env = env
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start cloudflared: %w", err)
	}
	t := &Tunnel{ready: make(chan string, 1), done: make(chan struct{}), cancel: cancel}
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.scan(ctx, stderr, public, readyTimeout, logger)
		err := cmd.Wait()
		switch {
		case ctx.Err() != nil:
			t.err = nil
		case err != nil:
			t.err = fmt.Errorf("cloudflared exited: %w", err)
		default:
			t.err = errors.New("cloudflared exited")
		}
		close(t.done)
	}()
	return t, nil
}

func withoutTunnelToken(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, "TUNNEL_TOKEN=") {
			out = append(out, kv)
		}
	}
	return out
}

// scan forwards cloudflared logs and announces readiness at most once,
// before the deadline. It returns when stderr closes.
func (t *Tunnel) scan(ctx context.Context, stderr io.Reader, public string, timeout time.Duration, logger *slog.Logger) {
	const waiting, announced, expired = 0, 1, 2
	var state atomic.Int32
	deadline := time.AfterFunc(timeout, func() {
		if state.CompareAndSwap(waiting, expired) && ctx.Err() == nil {
			logger.Error("cloudflared did not connect in time; public URL unavailable", "timeout", timeout)
		}
	})
	defer deadline.Stop()
	registered := false
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		logger.Debug("cloudflared", "line", line)
		if state.Load() != waiting {
			continue
		}
		if public == "" {
			if m := quickURL.FindStringSubmatch(line); m != nil {
				public = m[1]
			}
		}
		if strings.Contains(line, registeredLine) {
			registered = true
		}
		if public != "" && registered && state.CompareAndSwap(waiting, announced) {
			t.ready <- public
		}
	}
	// Drain so a blocked writer cannot stall Wait after a scanner error.
	_, _ = io.Copy(io.Discard, stderr)
}

// Ready delivers the public origin, e.g. https://abc.trycloudflare.com.
func (t *Tunnel) Ready() <-chan string { return t.ready }

// Done closes when cloudflared exits.
func (t *Tunnel) Done() <-chan struct{} { return t.done }

// Err reports why cloudflared exited; nil after Close. Valid once Done closes.
func (t *Tunnel) Err() error {
	<-t.done
	return t.err
}

// Close stops cloudflared and waits for the process and log reader to finish.
func (t *Tunnel) Close() {
	t.cancel()
	t.wg.Wait()
}
