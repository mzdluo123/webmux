//go:build !linux && !darwin

package session

import (
	"syscall"
	"time"
)

func enableReaper() error { return nil }

func cleanupProcesses(pid int, _ string) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGHUP)
	start := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if syscall.Kill(-pid, 0) == syscall.ESRCH {
			return
		}
		if time.Since(start) >= 5*time.Second {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}
		if time.Since(start) >= 7*time.Second {
			return
		}
		<-ticker.C
	}
}

// CleanupOrphan requires procpeek session inspection to rule out PID reuse
// after the leader has been reaped.
func CleanupOrphan(_ int, _ string) bool { return true }

func forceSession(pid int, _ string) {
	if pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}
