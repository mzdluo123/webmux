package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitWatchSignal(t *testing.T, sub *Subscription, reason string) {
	t.Helper()
	select {
	case <-sub.C:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for workspace watch signal (%s)", reason)
	}
}

func drainWatchSignals(sub *Subscription) {
	for {
		select {
		case <-sub.C:
		default:
			return
		}
	}
}

func TestWatchDetectsExternalWorktreeAddRemoveAndDirectoryDeletion(t *testing.T) {
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	s.watchInterval = 25 * time.Millisecond
	s.watchDebounce = 15 * time.Millisecond

	registerTest(t, s, root)

	sub1, err := s.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	sub2, err := s.Subscribe()
	if err != nil {
		t.Fatalf("second Subscribe: %v", err)
	}
	defer s.Unsubscribe(sub2)

	// Allow the baseline tick to settle without spurious notifications.
	time.Sleep(60 * time.Millisecond)
	select {
	case <-sub1.C:
		t.Fatal("unexpected signal without worktree changes")
	default:
	}

	// 1. External `git worktree add` triggers notification on all subscribers.
	extPath := filepath.Join(t.TempDir(), "external-wt")
	gitTest(t, root, "worktree", "add", "-b", "feat/external", extPath)
	waitWatchSignal(t, sub1, "external git worktree add (sub1)")
	waitWatchSignal(t, sub2, "external git worktree add (sub2)")
	drainWatchSignals(sub1)
	drainWatchSignals(sub2)

	// 2. External branch switch inside the linked worktree triggers notification.
	gitTest(t, extPath, "checkout", "-b", "feat/external-2")
	waitWatchSignal(t, sub1, "external branch checkout")
	drainWatchSignals(sub1)
	drainWatchSignals(sub2)

	// 3. External `rm -rf` of the linked worktree directory (without prune) triggers notification.
	if err := os.RemoveAll(extPath); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	waitWatchSignal(t, sub1, "external worktree directory deletion")
	drainWatchSignals(sub1)
	drainWatchSignals(sub2)

	// 4. External `git worktree prune` triggers notification.
	gitTest(t, root, "worktree", "prune")
	waitWatchSignal(t, sub1, "external git worktree prune")

	// Unsubscribing the first subscriber leaves the second active; unsubscribing
	// the last stops the watcher loop cleanly.
	s.Unsubscribe(sub1)
	s.Unsubscribe(sub2)
	s.watchMu.Lock()
	running := s.watchStop != nil
	s.watchMu.Unlock()
	if running {
		t.Fatal("expected watcher to stop after last Unsubscribe")
	}
}
