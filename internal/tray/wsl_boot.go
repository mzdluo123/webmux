package tray

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/wslclient"
)

// WSLState describes the tray's startup WSL readiness check.
type WSLState int

const (
	// WSLChecking means WSL readiness is being checked or retried during boot.
	WSLChecking WSLState = iota
	// WSLReady means Microsoft Store WSL and at least one distribution are ready.
	WSLReady
	// WSLUnavailable means WSL could not be used; opening the tray menu retries.
	WSLUnavailable
)

// WSLStatus is the immutable view published by WSLBoot.
type WSLStatus struct {
	State   WSLState
	Distros []wslclient.Distro
	Reason  string
}

var defaultWSLBootBackoffs = []time.Duration{
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	15 * time.Second,
	30 * time.Second,
}

// WSLBootOptions configure WSLBoot.
type WSLBootOptions struct {
	// Runner is used by the default Check function when Check is nil.
	Runner Runner
	// Check overrides CheckWSL for tests.
	Check func(context.Context) ([]wslclient.Distro, error)
	// Clock overrides RealClock for tests.
	Clock Clock
	// Backoffs are the wait intervals between boot retries for transient WSL
	// failures. Nil uses the default sequence (2s, 4s, 8s, 15s, 30s).
	Backoffs []time.Duration
	// Publish is called whenever WSLStatus changes.
	Publish func(WSLStatus)
	// OnReady is called once before Publish when WSL becomes ready.
	OnReady func([]wslclient.Distro)
	Logf    func(format string, args ...any)
}

// WSLBoot checks WSL readiness asynchronously on tray startup, retrying
// transient boot-time errors (such as WslService / Hyper-V cold start timeouts)
// with backoff before reporting WSL unavailable, and supporting later retries
// when the user opens the tray menu.
type WSLBoot struct {
	check    func(context.Context) ([]wslclient.Distro, error)
	clock    Clock
	backoffs []time.Duration
	publish  func(WSLStatus)
	onReady  func([]wslclient.Distro)
	logf     func(format string, args ...any)

	// Capacity 1 coalesces repeated menu-open retry signals while one check or
	// backoff wait is active.
	retryCh chan struct{}
	cancel  context.CancelFunc
	done    chan struct{}

	mu     sync.Mutex
	status WSLStatus
}

// NewWSLBoot starts asynchronous WSL readiness detection.
func NewWSLBoot(o WSLBootOptions) *WSLBoot {
	check := o.Check
	if check == nil {
		runner := o.Runner
		check = func(ctx context.Context) ([]wslclient.Distro, error) {
			return CheckWSL(ctx, runner)
		}
	}
	clock := o.Clock
	if clock == nil {
		clock = RealClock
	}
	backoffs := o.Backoffs
	if backoffs == nil {
		backoffs = defaultWSLBootBackoffs
	} else {
		backoffs = slices.Clone(backoffs)
	}
	logf := o.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	b := &WSLBoot{
		check:    check,
		clock:    clock,
		backoffs: backoffs,
		publish:  o.Publish,
		onReady:  o.OnReady,
		logf:     logf,
		retryCh:  make(chan struct{}, 1),
		cancel:   cancel,
		done:     make(chan struct{}),
		status:   WSLStatus{State: WSLChecking, Reason: "正在检查 WSL"},
	}
	go b.loop(ctx)
	return b
}

// Status returns the current WSL readiness snapshot without blocking.
func (b *WSLBoot) Status() WSLStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.status
	st.Distros = slices.Clone(st.Distros)
	return st
}

// Retry wakes a pending boot backoff immediately or triggers a background
// recheck when WSL is currently unavailable. It is a no-op once WSL is ready.
func (b *WSLBoot) Retry() {
	select {
	case b.retryCh <- struct{}{}:
	default:
	}
}

// Close cancels any in-flight WSL check and waits for the goroutine to stop.
func (b *WSLBoot) Close() {
	b.cancel()
	<-b.done
}

func (b *WSLBoot) loop(ctx context.Context) {
	defer close(b.done)
	if b.runBootAttempts(ctx) {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.retryCh:
			if b.runManualCheck(ctx) {
				return
			}
		}
	}
}

func (b *WSLBoot) runBootAttempts(ctx context.Context) bool {
	maxAttempts := 1 + len(b.backoffs)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			b.setStatus(WSLStatus{
				State:  WSLChecking,
				Reason: fmt.Sprintf("正在等待 WSL 就绪（第 %d/%d 次）", attempt, maxAttempts),
			})
		}
		distros, err := b.check(ctx)
		if ctx.Err() != nil {
			return true
		}
		if err == nil {
			if attempt > 1 {
				b.logf("WSL ready on attempt %d/%d (%d distros)", attempt, maxAttempts, len(distros))
			}
			b.finishReady(distros)
			return true
		}
		if TransientWSLError(err) && attempt < maxAttempts {
			delay := b.backoffs[attempt-1]
			b.logf("WSL check attempt %d/%d failed (retrying in %s): %v", attempt, maxAttempts, delay, err)
			seconds := max(1, int((delay+time.Second-1)/time.Second))
			b.setStatus(WSLStatus{
				State:  WSLChecking,
				Reason: fmt.Sprintf("WSL 正在启动，%d 秒后重试", seconds),
			})
			b.drainRetry()
			select {
			case <-ctx.Done():
				return true
			case <-b.clock.After(delay):
			case <-b.retryCh:
				b.logf("WSL check retry requested during boot wait")
			}
			continue
		}
		b.logf("WSL check failed: %v", err)
		b.setStatus(WSLStatus{State: WSLUnavailable, Reason: err.Error()})
		return false
	}
	return false
}

// runManualCheck retries WSL detection after a menu open without switching
// the visible state away from WSLUnavailable, avoiding icon flicker when WSL
// is genuinely absent.
func (b *WSLBoot) runManualCheck(ctx context.Context) bool {
	distros, err := b.check(ctx)
	if ctx.Err() != nil {
		return true
	}
	if err == nil {
		b.logf("WSL became ready on retry (%d distros)", len(distros))
		b.finishReady(distros)
		return true
	}
	reason := err.Error()
	prev := b.Status().Reason
	if reason != prev {
		b.logf("WSL recheck failed: %v", err)
		b.setStatus(WSLStatus{State: WSLUnavailable, Reason: reason})
	}
	return false
}

func (b *WSLBoot) drainRetry() {
	select {
	case <-b.retryCh:
	default:
	}
}

func (b *WSLBoot) setStatus(status WSLStatus) {
	b.mu.Lock()
	changed := b.status.State != status.State || b.status.Reason != status.Reason || !slices.Equal(b.status.Distros, status.Distros)
	b.status = WSLStatus{
		State:   status.State,
		Distros: slices.Clone(status.Distros),
		Reason:  status.Reason,
	}
	b.mu.Unlock()
	if changed && b.publish != nil {
		b.publish(status)
	}
}

func (b *WSLBoot) finishReady(distros []wslclient.Distro) {
	copied := slices.Clone(distros)
	status := WSLStatus{State: WSLReady, Distros: copied}
	b.mu.Lock()
	b.status = status
	b.mu.Unlock()
	if b.onReady != nil {
		b.onReady(slices.Clone(copied))
	}
	if b.publish != nil {
		b.publish(WSLStatus{State: WSLReady, Distros: slices.Clone(copied)})
	}
}
