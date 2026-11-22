package tray

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/wslclient"
)

type funcRunner struct {
	run func(ctx context.Context, argv, env []string) (Result, error)
}

func (f funcRunner) Run(ctx context.Context, argv, env []string) (Result, error) {
	return f.run(ctx, argv, env)
}

func (funcRunner) Start([]string, []string) (Process, error) {
	return nil, errors.New("unused")
}

func waitWSLStatus(t *testing.T, ch <-chan WSLStatus, match func(WSLStatus) bool) WSLStatus {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case st := <-ch:
			if match(st) {
				return st
			}
		case <-deadline:
			t.Fatal("timed out waiting for WSLStatus")
		}
	}
}

func TestCheckWSLClassification(t *testing.T) {
	verboseOK := []byte("  NAME      STATE           VERSION\n* Debian    Running         2\n")
	quietOK := []byte("Debian\n")

	tests := []struct {
		name          string
		give          func(ctx context.Context, argv []string) (Result, error)
		cancelParent  bool
		wantErr       bool
		wantNoWSL     bool
		wantTransient bool
		wantDistros   int
	}{
		{
			name: "version timeout is transient",
			give: func(_ context.Context, _ []string) (Result, error) {
				return Result{}, context.DeadlineExceeded
			},
			wantErr:       true,
			wantNoWSL:     true,
			wantTransient: true,
		},
		{
			name: "list timeout is transient",
			give: func(_ context.Context, argv []string) (Result, error) {
				if len(argv) > 1 && argv[1] == "--version" {
					return Result{Code: 0}, nil
				}
				return Result{}, context.DeadlineExceeded
			},
			wantErr:       true,
			wantNoWSL:     true,
			wantTransient: true,
		},
		{
			name: "list exit failure after version ok is transient",
			give: func(_ context.Context, argv []string) (Result, error) {
				if len(argv) > 1 && argv[1] == "--version" {
					return Result{Code: 0}, nil
				}
				return Result{Code: 1, Stdout: []byte("WslService unavailable")}, nil
			},
			wantErr:       true,
			wantNoWSL:     true,
			wantTransient: true,
		},
		{
			name: "missing wsl.exe is definitive",
			give: func(_ context.Context, _ []string) (Result, error) {
				return Result{}, exec.ErrNotFound
			},
			wantErr:       true,
			wantNoWSL:     true,
			wantTransient: false,
		},
		{
			name: "inbox wsl --version failure is definitive",
			give: func(_ context.Context, _ []string) (Result, error) {
				return Result{Code: 1}, nil
			},
			wantErr:       true,
			wantNoWSL:     true,
			wantTransient: false,
		},
		{
			name: "zero distros is definitive",
			give: func(_ context.Context, argv []string) (Result, error) {
				if len(argv) > 1 && argv[1] == "--version" {
					return Result{Code: 0}, nil
				}
				if len(argv) > 2 && argv[2] == "--quiet" {
					return Result{Code: 0, Stdout: nil}, nil
				}
				return Result{Code: 0, Stdout: []byte("  NAME      STATE           VERSION\n")}, nil
			},
			wantErr:       true,
			wantNoWSL:     true,
			wantTransient: false,
		},
		{
			name: "cancelled parent context propagates directly",
			give: func(ctx context.Context, _ []string) (Result, error) {
				return Result{}, ctx.Err()
			},
			cancelParent:  true,
			wantErr:       true,
			wantNoWSL:     false,
			wantTransient: false,
		},
		{
			name: "ready distribution",
			give: func(_ context.Context, argv []string) (Result, error) {
				if len(argv) > 2 && argv[2] == "--verbose" {
					return Result{Stdout: verboseOK}, nil
				}
				if len(argv) > 2 && argv[2] == "--quiet" {
					return Result{Stdout: quietOK}, nil
				}
				return Result{Code: 0}, nil
			},
			wantDistros: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			if tt.cancelParent {
				cancel()
			} else {
				defer cancel()
			}
			runner := funcRunner{run: func(ctx context.Context, argv, _ []string) (Result, error) {
				return tt.give(ctx, argv)
			}}
			distros, err := CheckWSL(ctx, runner)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CheckWSL err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(distros) != tt.wantDistros {
					t.Fatalf("distros = %+v, want %d", distros, tt.wantDistros)
				}
				return
			}
			if errors.Is(err, ErrNoWSL) != tt.wantNoWSL {
				t.Fatalf("errors.Is(%v, ErrNoWSL) = %v, want %v", err, errors.Is(err, ErrNoWSL), tt.wantNoWSL)
			}
			if TransientWSLError(err) != tt.wantTransient {
				t.Fatalf("TransientWSLError(%v) = %v, want %v", err, TransientWSLError(err), tt.wantTransient)
			}
		})
	}
}

func TestWSLBootRetriesTransientAndRecovers(t *testing.T) {
	clock := newFakeClock()
	statusCh := make(chan WSLStatus, 16)
	var readyMu sync.Mutex
	var readyCalls int
	var readyDistros []wslclient.Distro
	var sawUnavailable atomic.Bool
	var attempts atomic.Int32

	boot := NewWSLBoot(WSLBootOptions{
		Clock:    clock,
		Backoffs: []time.Duration{2 * time.Second, 4 * time.Second},
		Check: func(_ context.Context) ([]wslclient.Distro, error) {
			switch attempts.Add(1) {
			case 1:
				return nil, fmt.Errorf("%w：%w", ErrNoWSL, context.DeadlineExceeded)
			case 2:
				return nil, fmt.Errorf("%w：%w", ErrNoWSL, errWSLList)
			default:
				return []wslclient.Distro{{Name: "Debian", Default: true, Version: 2}}, nil
			}
		},
		Publish: func(st WSLStatus) {
			if st.State == WSLUnavailable {
				sawUnavailable.Store(true)
			}
			statusCh <- st
		},
		OnReady: func(d []wslclient.Distro) {
			readyMu.Lock()
			readyCalls++
			readyDistros = d
			readyMu.Unlock()
		},
	})
	defer boot.Close()

	// Wait for first retry backoff state.
	st := waitWSLStatus(t, statusCh, func(st WSLStatus) bool {
		return st.State == WSLChecking && st.Reason != "正在检查 WSL"
	})
	if st.State != WSLChecking {
		t.Fatalf("state = %v, want WSLChecking", st.State)
	}

	clock.Advance(2 * time.Second)
	waitWSLStatus(t, statusCh, func(st WSLStatus) bool {
		return attempts.Load() >= 2 && st.State == WSLChecking
	})

	clock.Advance(4 * time.Second)
	final := waitWSLStatus(t, statusCh, func(st WSLStatus) bool {
		return st.State == WSLReady
	})

	if sawUnavailable.Load() {
		t.Fatal("entered WSLUnavailable while transient boot retries remained")
	}
	if len(final.Distros) != 1 || final.Distros[0].Name != "Debian" {
		t.Fatalf("final distros = %+v", final.Distros)
	}
	readyMu.Lock()
	defer readyMu.Unlock()
	if readyCalls != 1 || len(readyDistros) != 1 || readyDistros[0].Name != "Debian" {
		t.Fatalf("OnReady called %d times with %+v", readyCalls, readyDistros)
	}
}

func TestWSLBootDefinitiveErrorAndManualRetry(t *testing.T) {
	statusCh := make(chan WSLStatus, 16)
	checkedCh := make(chan int, 8)
	var readyCalls atomic.Int32
	var stateMu sync.Mutex
	allowReady := false
	var attempt int

	boot := NewWSLBoot(WSLBootOptions{
		Clock:    newFakeClock(),
		Backoffs: []time.Duration{2 * time.Second, 4 * time.Second},
		Check: func(_ context.Context) ([]wslclient.Distro, error) {
			stateMu.Lock()
			attempt++
			n := attempt
			ok := allowReady
			stateMu.Unlock()
			checkedCh <- n
			if ok {
				return []wslclient.Distro{{Name: "Ubuntu", Default: true, Version: 2}}, nil
			}
			return nil, fmt.Errorf("%w：%w", ErrNoWSL, exec.ErrNotFound)
		},
		Publish: func(st WSLStatus) { statusCh <- st },
		OnReady: func([]wslclient.Distro) { readyCalls.Add(1) },
	})
	defer boot.Close()

	// Definitive failure goes straight to WSLUnavailable after attempt 1.
	st := waitWSLStatus(t, statusCh, func(st WSLStatus) bool {
		return st.State == WSLUnavailable
	})
	if st.Reason == "" {
		t.Fatal("expected unavailable reason")
	}
	if got := <-checkedCh; got != 1 {
		t.Fatalf("first check count = %d, want 1", got)
	}

	// Manual retry while still unavailable stays WSLUnavailable with no flicker.
	boot.Retry()
	select {
	case n := <-checkedCh:
		if n != 2 {
			t.Fatalf("second check count = %d, want 2", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for manual retry check")
	}
	if boot.Status().State != WSLUnavailable {
		t.Fatalf("state after failed manual retry = %v, want WSLUnavailable", boot.Status().State)
	}

	// Manual retry once WSL is installed recovers to WSLReady.
	stateMu.Lock()
	allowReady = true
	stateMu.Unlock()
	boot.Retry()
	final := waitWSLStatus(t, statusCh, func(st WSLStatus) bool {
		return st.State == WSLReady
	})
	if len(final.Distros) != 1 || final.Distros[0].Name != "Ubuntu" {
		t.Fatalf("final distros = %+v", final.Distros)
	}
	if readyCalls.Load() != 1 {
		t.Fatalf("OnReady calls = %d, want 1", readyCalls.Load())
	}
}

func TestWSLBootExhaustedRetriesThenManualRetry(t *testing.T) {
	clock := newFakeClock()
	statusCh := make(chan WSLStatus, 16)
	var attempts atomic.Int32
	var allowReady atomic.Bool

	boot := NewWSLBoot(WSLBootOptions{
		Clock:    clock,
		Backoffs: []time.Duration{2 * time.Second},
		Check: func(_ context.Context) ([]wslclient.Distro, error) {
			attempts.Add(1)
			if allowReady.Load() {
				return []wslclient.Distro{{Name: "Debian", Default: true, Version: 2}}, nil
			}
			return nil, fmt.Errorf("%w：%w", ErrNoWSL, context.DeadlineExceeded)
		},
		Publish: func(st WSLStatus) { statusCh <- st },
	})
	defer boot.Close()

	waitWSLStatus(t, statusCh, func(st WSLStatus) bool {
		return st.State == WSLChecking && attempts.Load() == 1
	})
	clock.Advance(2 * time.Second)
	waitWSLStatus(t, statusCh, func(st WSLStatus) bool {
		return st.State == WSLUnavailable
	})
	if attempts.Load() != 2 {
		t.Fatalf("attempts before unavailable = %d, want 2", attempts.Load())
	}

	allowReady.Store(true)
	boot.Retry()
	waitWSLStatus(t, statusCh, func(st WSLStatus) bool {
		return st.State == WSLReady
	})
}

func TestWSLBootRetryWakesBackoff(t *testing.T) {
	clock := newFakeClock()
	statusCh := make(chan WSLStatus, 16)
	var attempts atomic.Int32

	boot := NewWSLBoot(WSLBootOptions{
		Clock:    clock,
		Backoffs: []time.Duration{10 * time.Minute},
		Check: func(_ context.Context) ([]wslclient.Distro, error) {
			if attempts.Add(1) == 1 {
				return nil, fmt.Errorf("%w：%w", ErrNoWSL, context.DeadlineExceeded)
			}
			return []wslclient.Distro{{Name: "Debian", Default: true, Version: 2}}, nil
		},
		Publish: func(st WSLStatus) { statusCh <- st },
	})
	defer boot.Close()

	waitWSLStatus(t, statusCh, func(st WSLStatus) bool {
		return st.State == WSLChecking && attempts.Load() == 1
	})
	// Retry wakes the 10-minute backoff immediately without advancing clock.
	boot.Retry()
	waitWSLStatus(t, statusCh, func(st WSLStatus) bool {
		return st.State == WSLReady
	})
}

func TestWSLBootCloseCancelsInFlightCheck(t *testing.T) {
	started := make(chan struct{})
	var readyCalls atomic.Int32

	boot := NewWSLBoot(WSLBootOptions{
		Check: func(ctx context.Context) ([]wslclient.Distro, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		},
		OnReady: func([]wslclient.Distro) { readyCalls.Add(1) },
	})

	<-started
	done := make(chan struct{})
	go func() {
		boot.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not cancel and exit promptly")
	}
	if readyCalls.Load() != 0 {
		t.Fatalf("OnReady called %d times on cancelled boot", readyCalls.Load())
	}
}
