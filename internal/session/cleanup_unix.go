//go:build linux || darwin

package session

import (
	"os"
	"syscall"
	"time"

	"github.com/rainchan/webmux/internal/procpeek"
)

// Find every session process by terminal session, process tree, and the
// inherited environment marker, so separate job-control groups are covered too.
// An empty id skips the marker: a shell's sidecars share the marker with its
// PTY run, so stopping a sidecar follows only its own process tree. A run's
// cleanup matches the marker and also ends the shell's sidecars.
func cleanupProcesses(pid int, id string) {
	_ = CleanupOrphan(pid, id)
}

// CleanupOrphan terminates processes belonging to terminal session pid,
// environment marker id, and their descendant trees. It sends SIGHUP, polls up
// to 5 seconds, escalates to SIGKILL, and polls up to 2 seconds to report
// whether all matched processes have exited.
func CleanupOrphan(pid int, id string) bool {
	if pid <= 0 && id == "" {
		return true
	}
	self := os.Getpid()
	table := procpeek.Processes()
	reapAdopted(table)
	targets := make(map[int]uint64)
	var marker []byte
	if id != "" {
		marker = []byte("WEBMUX_SESSION_ID=" + id)
	}
	collectTargets(targets, table, pid, marker, self)
	if len(targets) == 0 {
		return true
	}
	signalTargets(targets, table, syscall.SIGHUP)
	if hasSessionMember(table, pid, self) {
		_ = syscall.Kill(-pid, syscall.SIGHUP)
	}
	start := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		table = procpeek.Processes()
		reapAdopted(table)
		for child, born := range targets {
			state, exists := table[child]
			if !exists || state.Start != born {
				delete(targets, child)
			}
		}
		collectTargets(targets, table, pid, marker, self)
		if len(targets) == 0 {
			return true
		}
		if time.Since(start) >= 5*time.Second {
			signalTargets(targets, table, syscall.SIGKILL)
		}
		if time.Since(start) >= 7*time.Second {
			latest := procpeek.Processes()
			reapAdopted(latest)
			for child, born := range targets {
				if state, exists := latest[child]; exists && state.Start == born && state.State != 'Z' {
					return false
				}
			}
			return true
		}
		<-ticker.C
	}
}

func collectTargets(targets map[int]uint64, table procpeek.Table, pid int, marker []byte, self int) {
	for child, state := range table {
		if child <= 0 || child == self {
			continue
		}
		if _, known := targets[child]; known {
			continue
		}
		owned := pid > 0 && state.SID == pid
		if !owned && marker != nil && state.State != 'Z' {
			owned = procpeek.SessionMarked(child, marker)
		}
		if owned {
			targets[child] = state.Start
		}
	}
	for changed := true; changed; {
		changed = false
		for child, state := range table {
			if child <= 0 || child == self {
				continue
			}
			if _, known := targets[child]; known {
				continue
			}
			parent, exists := table[state.PPID]
			if parentStart, owned := targets[state.PPID]; owned && exists && parent.Start == parentStart {
				targets[child] = state.Start
				changed = true
			}
		}
	}
}

func hasSessionMember(table procpeek.Table, pid, self int) bool {
	if pid <= 0 || pid == self {
		return false
	}
	for child, state := range table {
		if child > 0 && child != self && state.SID == pid && state.State != 'Z' {
			return true
		}
	}
	return false
}

func signalTargets(targets map[int]uint64, table procpeek.Table, signal syscall.Signal) {
	for pid, born := range targets {
		if state, exists := table[pid]; exists && state.Start == born && state.State != 'Z' {
			_ = syscall.Kill(pid, signal)
		}
	}
}

func forceSession(pid int, id string) {
	self := os.Getpid()
	table := procpeek.Processes()
	reapAdopted(table)
	targets := make(map[int]uint64)
	var marker []byte
	if id != "" {
		marker = []byte("WEBMUX_SESSION_ID=" + id)
	}
	collectTargets(targets, table, pid, marker, self)
	signalTargets(targets, table, syscall.SIGKILL)
	if hasSessionMember(table, pid, self) {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}
