package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func wrapGitCounter(t *testing.T, s *Service) (count func(string) int, reset func()) {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "git.log")
	realGit := s.gitPath
	wrapper := filepath.Join(t.TempDir(), "git")
	putFile(t, wrapper, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$3\" >> %q\nexec %q \"$@\"\n", logFile, realGit))
	if err := os.Chmod(wrapper, 0o700); err != nil {
		t.Fatal(err)
	}
	s.gitPath = wrapper
	count = func(op string) int {
		data, err := os.ReadFile(logFile)
		if err != nil {
			return 0
		}
		n := 0
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == op {
				n++
			}
		}
		return n
	}
	reset = func() { _ = os.Remove(logFile) }
	return count, reset
}

func TestListWithOptionsSWRFreshStaleAndConditionalNotify(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	s.freshTTL = 120 * time.Millisecond
	s.watchInterval = time.Hour
	r := registerTest(t, s, root)

	sub, err := s.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Unsubscribe(sub)
	drainWatchSignals(sub)

	countOp, resetLog := wrapGitCounter(t, s)

	// 1. Fresh hit right after registration serves from snapshot cache with 0 git calls.
	resetLog()
	list, err := s.ListWithOptions(context.Background(), ListOptions{AllowStale: true})
	if err != nil || len(list) != 1 || list[0].ID != r.ID {
		t.Fatalf("fresh SWR List = %+v, err=%v", list, err)
	}
	if got := countOp("worktree"); got != 0 {
		t.Fatalf("fresh SWR ran %d git worktree calls, want 0", got)
	}
	if got := countOp("status"); got != 0 {
		t.Fatalf("fresh SWR ran %d git status calls, want 0", got)
	}

	// 2. After freshTTL expires with no disk change, SWR returns immediately,
	// revalidates in background, and does NOT emit a spurious watch notification.
	time.Sleep(150 * time.Millisecond)
	resetLog()
	list, err = s.ListWithOptions(context.Background(), ListOptions{AllowStale: true})
	if err != nil || len(list) != 1 {
		t.Fatalf("stale unchanged SWR List = %+v, err=%v", list, err)
	}
	// Wait for background revalidation to finish.
	s.swrWG.Wait()
	if got := countOp("worktree"); got == 0 {
		t.Fatal("stale SWR did not trigger background revalidation")
	}
	select {
	case <-sub.C:
		t.Fatal("spurious watch notification emitted when nothing materially changed")
	default:
	}

	// 3. External `git worktree add` changes repoFingerprint immediately, so
	// even within freshTTL an SWR read triggers background revalidation and
	// notifies subscribers once the new worktree is discovered.
	extPath := filepath.Join(t.TempDir(), "swr-ext-wt")
	gitTest(t, root, "worktree", "add", "-b", "feat/swr-ext", extPath)
	list, err = s.ListWithOptions(context.Background(), ListOptions{AllowStale: true})
	if err != nil || len(list) != 1 {
		t.Fatalf("fingerprint-stale SWR List = %+v, err=%v", list, err)
	}
	waitWatchSignal(t, sub, "SWR background revalidation after worktree add")
	s.swrWG.Wait()

	// Next SWR read hits the newly stored Fresh snapshot with both worktrees and 0 git calls.
	resetLog()
	list, err = s.ListWithOptions(context.Background(), ListOptions{AllowStale: true})
	if err != nil || len(list) != 1 || len(list[0].Worktrees) != 2 {
		t.Fatalf("post-revalidation SWR List = %+v, err=%v", list, err)
	}
	if got := countOp("worktree"); got != 0 {
		t.Fatalf("post-revalidation Fresh read ran %d git worktree calls, want 0", got)
	}
}

func TestWatchFingerprintsMarkOnlyChangedRepositoryDirty(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	rootA := initRepo(t)
	rootB := initRepo(t)
	s, _ := testService(t, t.TempDir(), rootA)
	s.freshTTL = time.Minute
	s.watchInterval = 25 * time.Millisecond
	s.watchDebounce = 15 * time.Millisecond

	repoA := registerTest(t, s, rootA)
	repoB := registerTest(t, s, rootB)

	sub, err := s.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Unsubscribe(sub)
	drainWatchSignals(sub)

	// External worktree addition in repoA must mark only repoA dirty, leaving repoB fresh.
	extA := filepath.Join(t.TempDir(), "ext-a")
	gitTest(t, rootA, "worktree", "add", "-b", "feat/a", extA)
	waitWatchSignal(t, sub, "repoA external worktree add")

	stateA, err := s.repository(repoA.ID)
	if err != nil {
		t.Fatal(err)
	}
	stateB, err := s.repository(repoB.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, freshA, okA := stateA.cachedSnapshot(time.Now(), s.freshTTL, repoFingerprint(stateA.entry))
	_, freshB, okB := stateB.cachedSnapshot(time.Now(), s.freshTTL, repoFingerprint(stateB.entry))
	if !okA || freshA {
		t.Fatalf("repoA should be dirty/stale, got ok=%v fresh=%v", okA, freshA)
	}
	if !okB || !freshB {
		t.Fatalf("repoB should remain fresh, got ok=%v fresh=%v", okB, freshB)
	}
}
