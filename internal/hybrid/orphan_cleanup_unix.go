//go:build linux || darwin

package hybrid

import (
	"os"
	"syscall"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/procpeek"
	"github.com/rainchan/webmux/internal/runtime"
)

var (
	orphanHangupWait   = 5 * time.Second
	orphanKillWait     = 2 * time.Second
	orphanPollInterval = 50 * time.Millisecond
	orphanSignal       = syscall.Kill
)

// cleanupRecordOrphans terminates leftover processes belonging to sessionID or
// matching root process (pid, identity), waits for their exit, and reports
// whether recorded root identity can be cleared.
func cleanupRecordOrphans(sessionID string, pid int, identity *runtime.Identity) bool {
	self := os.Getpid()
	livePID := 0
	if liveRootProcess(pid, identity, self) {
		livePID = pid
	}
	var marker []byte
	if sessionID != "" {
		marker = []byte(contract.SessionIDEnv + "=" + sessionID)
	}
	table := procpeek.Processes()
	targets := make(map[int]uint64)
	collectOrphanTargets(targets, table, livePID, marker, self)
	expandOrphanDescendants(targets, table, self)
	if len(targets) == 0 && (livePID == 0 || !liveRootProcess(livePID, identity, self)) {
		return true
	}
	if livePID > 0 {
		_ = orphanSignal(-livePID, syscall.SIGHUP)
	}
	signalOrphanTargets(targets, table, syscall.SIGHUP)

	start := time.Now()
	var killed bool
	var killAt time.Time
	ticker := time.NewTicker(orphanPollInterval)
	defer ticker.Stop()
	for {
		<-ticker.C
		table = procpeek.Processes()
		collectOrphanTargets(targets, table, livePID, marker, self)
		expandOrphanDescendants(targets, table, self)
		for child, born := range targets {
			state, exists := table[child]
			if !exists || state.Start != born || state.State == 'Z' {
				delete(targets, child)
			}
		}
		if len(targets) == 0 && (livePID == 0 || !liveRootProcess(livePID, identity, self)) {
			return true
		}
		if time.Since(start) >= orphanHangupWait {
			if livePID > 0 && liveRootProcess(livePID, identity, self) {
				_ = orphanSignal(-livePID, syscall.SIGKILL)
			}
			signalOrphanTargets(targets, table, syscall.SIGKILL)
			if !killed {
				killed = true
				killAt = time.Now()
			} else if time.Since(killAt) >= orphanKillWait {
				return len(targets) == 0 && (livePID == 0 || !liveRootProcess(livePID, identity, self))
			}
		}
	}
}

func liveRootProcess(pid int, identity *runtime.Identity, self int) bool {
	if pid <= 0 || identity == nil || pid == self {
		return false
	}
	current, err := runtime.ProcessIdentity(pid)
	return err == nil && current == *identity
}

func collectOrphanTargets(targets map[int]uint64, table procpeek.Table, livePID int, marker []byte, self int) {
	if livePID > 0 {
		if state, ok := table[livePID]; ok && state.State != 'Z' {
			targets[livePID] = state.Start
		}
	}
	for child, state := range table {
		if child == self || child <= 1 || state.State == 'Z' {
			continue
		}
		if _, known := targets[child]; known {
			continue
		}
		owned := livePID > 0 && (state.SID == livePID || sameOrphanGroup(child, livePID))
		if !owned && marker != nil {
			owned = procpeek.SessionMarked(child, marker)
		}
		if owned {
			targets[child] = state.Start
		}
	}
}

func sameOrphanGroup(pid, pgid int) bool {
	got, err := syscall.Getpgid(pid)
	return err == nil && got == pgid
}

func expandOrphanDescendants(targets map[int]uint64, table procpeek.Table, self int) {
	for changed := true; changed; {
		changed = false
		for child, state := range table {
			if child == self || child <= 1 || state.State == 'Z' {
				continue
			}
			if _, known := targets[child]; known {
				continue
			}
			parentStart, owned := targets[state.PPID]
			if !owned {
				continue
			}
			if parentState, ok := table[state.PPID]; ok && parentState.Start == parentStart {
				targets[child] = state.Start
				changed = true
			}
		}
	}
}

func signalOrphanTargets(targets map[int]uint64, table procpeek.Table, signal syscall.Signal) {
	for pid, born := range targets {
		if state, exists := table[pid]; exists && state.Start == born && state.State != 'Z' {
			_ = orphanSignal(pid, signal)
		}
	}
}
