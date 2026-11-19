package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
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

func TestKnownWorktreesBaselineAutoPinRetryAndManagedCreate(t *testing.T) {
	isolateGit(t)
	root := initRepo(t)
	stateDir := t.TempDir()

	// Pre-existing linked worktree created BEFORE registration must stay off Home.
	legacyPath := filepath.Join(t.TempDir(), "legacy-wt")
	gitTest(t, root, "worktree", "add", "-b", "feat/legacy", legacyPath)

	var (
		mu       sync.Mutex
		failNext bool
		pinned   = map[string][]string{}
	)
	pin := func(repoID, mainID string, added []string) error {
		mu.Lock()
		defer mu.Unlock()
		if failNext {
			failNext = false
			return errors.New("settings temporarily unavailable")
		}
		list, ok := pinned[repoID]
		if !ok && mainID != "" {
			list = []string{mainID}
		}
		for _, id := range added {
			if !slices.Contains(list, id) {
				list = append(list, id)
			}
		}
		pinned[repoID] = list
		return nil
	}

	s1, _ := testService(t, stateDir, root)
	s1.UseHomeWorktrees(pin)
	reg := registerTest(t, s1, root)

	mu.Lock()
	if len(pinned) != 0 {
		t.Fatalf("initial registration auto-pinned historical worktree: %v", pinned)
	}
	mu.Unlock()

	// Managed CreateWorktree (such as `webmux worktree create --no-pin`) records
	// the new worktree as known without invoking auto-pin.
	managedPath := filepath.Join(t.TempDir(), "managed-wt")
	managed, err := s1.CreateWorktree(context.Background(), reg.ID, CreateRequest{
		Path:   managedPath,
		Mode:   "new",
		Branch: "feat/managed",
		Base:   "main",
	})
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if _, err := s1.Refresh(context.Background(), reg.ID); err != nil {
		t.Fatalf("Refresh after CreateWorktree: %v", err)
	}
	mu.Lock()
	if slices.Contains(pinned[reg.ID], managed.ID) {
		t.Fatalf("managed CreateWorktree should not auto-pin: %v", pinned)
	}
	mu.Unlock()

	// Simulate transient settings failure on external `git worktree add`:
	// baseline must NOT advance past the failed addition, and the next read must retry.
	extPath := filepath.Join(t.TempDir(), "ext-wt")
	gitTest(t, root, "worktree", "add", "-b", "feat/ext", extPath)
	mu.Lock()
	failNext = true
	mu.Unlock()
	if _, err := s1.Refresh(context.Background(), reg.ID); err != nil {
		t.Fatalf("Refresh with failing pin: %v", err)
	}
	mu.Lock()
	if len(pinned[reg.ID]) != 0 {
		t.Fatalf("expected failed pin not to mutate map: %v", pinned)
	}
	mu.Unlock()

	// Next read succeeds and pins `feat/ext`.
	refreshed, err := s1.Refresh(context.Background(), reg.ID)
	if err != nil {
		t.Fatalf("Refresh retry: %v", err)
	}
	var extID string
	for _, wt := range refreshed.Worktrees {
		if wt.Path == canonicalPath(t, extPath) {
			extID = wt.ID
		}
	}
	if extID == "" {
		t.Fatalf("could not find external worktree in %+v", refreshed.Worktrees)
	}
	mu.Lock()
	if !slices.Contains(pinned[reg.ID], extID) {
		t.Fatalf("expected retry to auto-pin %s, got %v", extID, pinned[reg.ID])
	}
	// User explicitly removes `extID` from Home on one device.
	pinned[reg.ID] = slices.DeleteFunc(pinned[reg.ID], func(id string) bool { return id == extID })
	mu.Unlock()

	// Restart service (and create another external worktree while offline!):
	// manual removal of `extID` stays removed, while `offline-wt` auto-pins.
	s1.Close()
	offlinePath := filepath.Join(t.TempDir(), "offline-wt")
	gitTest(t, root, "worktree", "add", "-b", "feat/offline", offlinePath)

	s2, _ := testService(t, stateDir, root)
	s2.UseHomeWorktrees(pin)
	repos, err := s2.List(context.Background())
	if err != nil || len(repos) != 1 {
		t.Fatalf("s2.List: %v %+v", err, repos)
	}
	var offlineID string
	for _, wt := range repos[0].Worktrees {
		if wt.Path == canonicalPath(t, offlinePath) {
			offlineID = wt.ID
		}
	}
	mu.Lock()
	gotPinned := slices.Clone(pinned[reg.ID])
	mu.Unlock()
	if slices.Contains(gotPinned, extID) {
		t.Fatalf("explicitly unpinned worktree %s was resurrected after restart: %v", extID, gotPinned)
	}
	if offlineID == "" || !slices.Contains(gotPinned, offlineID) {
		t.Fatalf("offline external worktree %s was not auto-pinned: %v", offlineID, gotPinned)
	}

	// External removal clears `extID` from knownWorktrees, so recreating at the same path pins again.
	gitTest(t, root, "worktree", "remove", extPath)
	if _, err := s2.Refresh(context.Background(), reg.ID); err != nil {
		t.Fatalf("Refresh after remove: %v", err)
	}
	gitTest(t, root, "worktree", "add", "-b", "feat/ext-recreated", extPath)
	if _, err := s2.Refresh(context.Background(), reg.ID); err != nil {
		t.Fatalf("Refresh after recreate: %v", err)
	}
	mu.Lock()
	if !slices.Contains(pinned[reg.ID], extID) {
		t.Fatalf("recreated worktree at %s was not auto-pinned: %v", extPath, pinned[reg.ID])
	}
	mu.Unlock()
}

func TestWorktreePinsConvergeOnRefreshAndUnregister(t *testing.T) {
	stateDir := t.TempDir()
	root := initRepo(t)
	s, _ := testService(t, stateDir, root)
	defer s.Close()

	var mu sync.Mutex
	pins := map[string][]string{}
	s.UseWorktreePins(
		func(repoID string, liveIDs []string) error {
			mu.Lock()
			defer mu.Unlock()
			live := map[string]bool{}
			for _, id := range liveIDs {
				live[id] = true
			}
			pins[repoID] = slices.DeleteFunc(slices.Clone(pins[repoID]), func(id string) bool { return !live[id] })
			if len(pins[repoID]) == 0 {
				delete(pins, repoID)
			}
			return nil
		},
		func(repoID string) error {
			mu.Lock()
			defer mu.Unlock()
			delete(pins, repoID)
			return nil
		},
	)

	reg := registerTest(t, s, root)
	wt1Path := filepath.Join(t.TempDir(), "wt-pin-1")
	wt2Path := filepath.Join(t.TempDir(), "wt-pin-2")
	gitTest(t, root, "worktree", "add", "-b", "feat/pin-1", wt1Path)
	gitTest(t, root, "worktree", "add", "-b", "feat/pin-2", wt2Path)

	refreshed, err := s.Refresh(context.Background(), reg.ID)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	var wt1ID, wt2ID string
	for _, wt := range refreshed.Worktrees {
		switch wt.Path {
		case canonicalPath(t, wt1Path):
			wt1ID = wt.ID
		case canonicalPath(t, wt2Path):
			wt2ID = wt.ID
		}
	}
	if wt1ID == "" || wt2ID == "" {
		t.Fatalf("missing worktree IDs: %+v", refreshed.Worktrees)
	}

	mu.Lock()
	pins[reg.ID] = []string{wt2ID, wt1ID}
	pins["other-repo"] = []string{"keep"}
	mu.Unlock()

	// Removing wt1 externally converges reg.ID's pins on refresh without touching sibling repos.
	gitTest(t, root, "worktree", "remove", wt1Path)
	if _, err := s.Refresh(context.Background(), reg.ID); err != nil {
		t.Fatalf("Refresh after remove: %v", err)
	}
	mu.Lock()
	if !slices.Equal(pins[reg.ID], []string{wt2ID}) || !slices.Equal(pins["other-repo"], []string{"keep"}) {
		t.Fatalf("unexpected pins after remove: %+v", pins)
	}
	mu.Unlock()

	// Unregistering reg.ID drops its entry immediately.
	if err := s.Unregister(reg.ID); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	mu.Lock()
	_, hasReg := pins[reg.ID]
	otherKept := slices.Equal(pins["other-repo"], []string{"keep"})
	mu.Unlock()
	if hasReg || !otherKept {
		t.Fatalf("unexpected pins after unregister: %+v", pins)
	}
}

func canonicalPath(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", p, err)
	}
	return resolved
}
