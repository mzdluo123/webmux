package workspace

import (
	"context"
	"errors"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskBaselineAutoCaptureGCProtectionRestartAndCleanup(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	stateDir := t.TempDir()
	workDir := initRepo(t)
	baseHead := strings.TrimSpace(gitTest(t, workDir, "rev-parse", "HEAD"))

	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remoteDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, remoteDir, "init", "--bare", "-b", "main")
	gitTest(t, workDir, "remote", "add", "origin", remoteDir)
	gitTest(t, workDir, "push", "-u", "origin", "main")

	peerDir := filepath.Join(t.TempDir(), "peer")
	if err := os.MkdirAll(peerDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, peerDir, "clone", "-b", "main", remoteDir, ".")
	putFile(t, filepath.Join(peerDir, "upstream.txt"), "upstream advance\n")
	gitTest(t, peerDir, "add", "upstream.txt")
	gitTest(t, peerDir, "commit", "-m", "feat: upstream advance")
	gitTest(t, peerDir, "push", "origin", "main")

	s, mgr := testService(t, stateDir, workDir)
	repo := registerTest(t, s, workDir)

	wt, err := s.CreateWorktree(ctx, repo.ID, CreateRequest{
		Mode:   "new",
		Branch: "feat/task-auto",
		Base:   "main",
		Path:   filepath.Join(t.TempDir(), "wt-task-auto"),
	})
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Verify 0600 permissions on git-task-baselines.json and .lock.
	for _, name := range []string{"git-task-baselines.json", "git-task-baselines.json.lock"} {
		info, statErr := os.Stat(filepath.Join(stateDir, name))
		if statErr != nil {
			t.Fatalf("stat %s: %v", name, statErr)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("%s mode = %04o, want 0600", name, perm)
		}
	}

	got, err := s.GetTaskBaseline(ctx, repo.ID, wt.ID)
	if err != nil || got == nil {
		t.Fatalf("GetTaskBaseline after CreateWorktree: %+v %v", got, err)
	}
	if got.Source != TaskBaselineWorktreeCreate || got.BaseOID != baseHead || got.Unborn || !got.CleanAtStart {
		t.Fatalf("unexpected auto baseline: %+v (want base=%s)", got, baseHead)
	}

	// Private GC ref exists and is invisible to Branches() and History() refs.
	privRef := baselineRef(wt.ID)
	refOID := strings.TrimSpace(gitTest(t, workDir, "rev-parse", "--verify", privRef))
	if refOID != baseHead {
		t.Fatalf("private ref %s = %q, want %q", privRef, refOID, baseHead)
	}
	branches, err := s.Branches(ctx, repo.ID)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	for _, b := range branches.Branches {
		if strings.Contains(b.Name, "webmux") || strings.Contains(b.Name, "baselines") {
			t.Fatalf("private baseline ref leaked into Branches(): %+v", b)
		}
	}

	// Advance branch, fetch remote, and orphan a manual baseline commit via --amend + git gc --prune=now.
	putFile(t, filepath.Join(wt.Path, "step1.txt"), "step 1\n")
	gitTest(t, wt.Path, "add", "step1.txt")
	gitTest(t, wt.Path, "commit", "-m", "feat: step 1")
	orphanBase := strings.TrimSpace(gitTest(t, wt.Path, "rev-parse", "HEAD"))

	if _, err := s.PutTaskBaseline(ctx, repo.ID, wt.ID, PutTaskBaselineRequest{BaseOID: orphanBase}); err != nil {
		t.Fatalf("PutTaskBaseline(orphanBase): %v", err)
	}
	putFile(t, filepath.Join(wt.Path, "step1.txt"), "step 1 amended\n")
	gitTest(t, wt.Path, "add", "step1.txt")
	gitTest(t, wt.Path, "commit", "--amend", "-m", "feat: step 1 amended")
	putFile(t, filepath.Join(wt.Path, "step2.txt"), "step 2\n")
	gitTest(t, wt.Path, "add", "step2.txt")
	gitTest(t, wt.Path, "commit", "-m", "feat: step 2")

	if _, err := s.FetchRemotes(ctx, repo.ID); err != nil {
		t.Fatalf("FetchRemotes: %v", err)
	}
	gitTest(t, workDir, "reflog", "expire", "--expire=now", "--expire-unreachable=now", "--all")
	gitTest(t, workDir, "gc", "--prune=now")

	// Baseline OID remains fixed AND un-GC'd thanks to refs/webmux/baselines/<wtId>!
	afterGC := strings.TrimSpace(gitTest(t, workDir, "cat-file", "-t", orphanBase))
	if afterGC != "commit" {
		t.Fatalf("expected %s to survive git gc --prune=now, got type %q", orphanBase, afterGC)
	}
	afterFetch, err := s.GetTaskBaseline(ctx, repo.ID, wt.ID)
	if err != nil || afterFetch == nil || afterFetch.BaseOID != orphanBase {
		t.Fatalf("baseline drifted after amend/commits/fetch: %+v %v", afterFetch, err)
	}

	// Start and close/remove a session on wt: baseline MUST remain intact.
	created, err := s.CreateSession(ctx, sessioncreate.Input{Cwd: wt.Path,
		Name: "agent",
		Cmd:  []string{"/bin/sh", "-c", "exit 0"}})
	if err != nil {
		t.Fatalf("CreateSession on wt: %v", err)
	}
	_ = mgr.Close(created.Info().ID)
	_ = mgr.Remove(created.Info().ID)
	stillThere, err := s.GetTaskBaseline(ctx, repo.ID, wt.ID)
	if err != nil || stillThere == nil || stillThere.BaseOID != orphanBase {
		t.Fatalf("session exit/remove must not delete baseline: %+v %v", stillThere, err)
	}

	// Service restart reloads persisted baseline from git-task-baselines.json.
	s.Close()
	s2, _ := testService(t, stateDir, workDir)
	reloaded, err := s2.GetTaskBaseline(ctx, repo.ID, wt.ID)
	if err != nil || reloaded == nil || reloaded.BaseOID != orphanBase {
		t.Fatalf("reloaded baseline mismatch: %+v %v", reloaded, err)
	}
	if err := InspectGitTaskBaselines(stateDir); err != nil || len(s2.baselines) != 1 {
		t.Fatalf("InspectGitTaskBaselines err=%v count=%d, want nil & 1", err, len(s2.baselines))
	}

	// RemoveWorktree cleans both git-task-baselines.json and refs/webmux/baselines/<wtId>.
	remPrev, err := s2.PreviewRemoval(ctx, repo.ID, wt.ID)
	if err != nil {
		t.Fatalf("PreviewRemoval: %v", err)
	}
	if err := s2.RemoveWorktree(ctx, repo.ID, wt.ID, RemoveRequest{
		ConfirmationID: remPrev.ConfirmationID,
		Mode:           "safe",
	}); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	refsLeft := strings.TrimSpace(gitTest(t, workDir, "for-each-ref", "--format=%(refname)", baselineRefPrefix))
	if refsLeft != "" {
		t.Fatalf("expected private baseline refs cleaned on RemoveWorktree, got %q", refsLeft)
	}
	if err := InspectGitTaskBaselines(stateDir); err != nil || len(s2.baselines) != 0 {
		t.Fatalf("expected 0 baselines after RemoveWorktree, err=%v count=%d", err, len(s2.baselines))
	}

	// Explicit PutTaskBaseline + Unregister cleans all repo baselines & refs.
	mainWT := repo.Worktrees[0].ID
	if _, err := s2.PutTaskBaseline(ctx, repo.ID, mainWT, PutTaskBaselineRequest{}); err != nil {
		t.Fatalf("PutTaskBaseline mainWT: %v", err)
	}
	if err := s2.Unregister(repo.ID); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if refsAfterUnreg := strings.TrimSpace(gitTest(t, workDir, "for-each-ref", "--format=%(refname)", baselineRefPrefix)); refsAfterUnreg != "" {
		t.Fatalf("expected private baseline refs cleaned on Unregister, got %q", refsAfterUnreg)
	}
	if err := InspectGitTaskBaselines(stateDir); err != nil || len(s2.baselines) != 0 {
		t.Fatalf("expected 0 baselines after Unregister, err=%v count=%d", err, len(s2.baselines))
	}
}

func TestTaskBaselineUnbornDirtyManualSelectAndHistoryTaskScope(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	// 1. Unborn repository: session start on unborn main worktree and orphan worktree creation capture unborn=true, baseOid="".
	emptyDir := filepath.Join(t.TempDir(), "empty-repo")
	if err := os.MkdirAll(emptyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, emptyDir, "init", "-b", "main")
	s, _ := testService(t, t.TempDir(), emptyDir)
	emptyRepo := registerTest(t, s, emptyDir)
	emptyMainWT := emptyRepo.Worktrees[0].ID

	// Before baseline: scope=task returns no_task_baseline.
	noBasePage, err := s.History(ctx, HistoryQuery{
		RepositoryID: emptyRepo.ID,
		WorktreeID:   emptyMainWT,
		Scope:        HistoryScopeTask,
	})
	if err != nil || noBasePage.Status != HistoryStatusNoTaskBaseline {
		t.Fatalf("expected no_task_baseline before session/manual set, got %+v %v", noBasePage, err)
	}

	// Starting a session on unborn worktree captures unborn=true without fake all-zero OID.
	if _, err := s.CreateSession(ctx, sessioncreate.Input{Cwd: emptyDir,
		Name: "unborn-shell",
		Cmd:  []string{"/bin/sh", "-c", "exit 0"}}); err != nil {
		t.Fatalf("CreateSession on unborn worktree: %v", err)
	}
	unbornBase, err := s.GetTaskBaseline(ctx, emptyRepo.ID, emptyMainWT)
	if err != nil || unbornBase == nil || !unbornBase.Unborn || unbornBase.BaseOID != "" {
		t.Fatalf("expected unborn=true with empty baseOid, got %+v %v", unbornBase, err)
	}

	orphanWT, err := s.CreateWorktree(ctx, emptyRepo.ID, CreateRequest{
		Mode:   "orphan",
		Branch: "feat/unborn-orphan",
		Path:   filepath.Join(t.TempDir(), "wt-unborn-orphan"),
	})
	if err != nil {
		t.Fatalf("CreateWorktree orphan on unborn repo: %v", err)
	}
	orphanBase, err := s.GetTaskBaseline(ctx, emptyRepo.ID, orphanWT.ID)
	if err != nil || orphanBase == nil || !orphanBase.Unborn || orphanBase.BaseOID != "" {
		t.Fatalf("expected orphan worktree unborn baseline, got %+v %v", orphanBase, err)
	}

	// Once commits land on unborn worktree, scope=task lists root..HEAD!
	putFile(t, filepath.Join(emptyDir, "first.txt"), "1\n")
	gitTest(t, emptyDir, "add", "first.txt")
	gitTest(t, emptyDir, "commit", "-m", "feat: c1")
	putFile(t, filepath.Join(emptyDir, "second.txt"), "2\n")
	gitTest(t, emptyDir, "add", "second.txt")
	gitTest(t, emptyDir, "commit", "-m", "feat: c2")

	taskFromUnborn, err := s.History(ctx, HistoryQuery{
		RepositoryID: emptyRepo.ID,
		WorktreeID:   emptyMainWT,
		Scope:        HistoryScopeTask,
	})
	if err != nil || taskFromUnborn.Status != HistoryStatusReady || len(taskFromUnborn.Commits) != 2 {
		t.Fatalf("expected 2 commits for unborn baseline root..HEAD, got %+v %v", taskFromUnborn, err)
	}

	// 2. Dirty existing worktree manual baseline selection + validation of invalid/fake OIDs.
	workDir := initRepo(t)
	c0 := strings.TrimSpace(gitTest(t, workDir, "rev-parse", "HEAD"))
	putFile(t, filepath.Join(workDir, "committed.txt"), "v1\n")
	gitTest(t, workDir, "add", "committed.txt")
	gitTest(t, workDir, "commit", "-m", "feat: committed v1")
	c1 := strings.TrimSpace(gitTest(t, workDir, "rev-parse", "HEAD"))

	repo := registerTest(t, s, workDir)
	wtID := repo.Worktrees[0].ID

	// Reject fake all-zero OID, short SHA, and missing commit OID.
	for _, bad := range []string{
		strings.Repeat("0", 40),
		c0[:8],
		"--flag",
	} {
		if _, err := s.PutTaskBaseline(ctx, repo.ID, wtID, PutTaskBaselineRequest{BaseOID: bad}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("PutTaskBaseline(%q) = %v, want ErrInvalid", bad, err)
		}
	}
	if _, err := s.PutTaskBaseline(ctx, repo.ID, wtID, PutTaskBaselineRequest{BaseOID: strings.Repeat("deadbeef", 5)}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("PutTaskBaseline(missing) = %v, want ErrNotFound", err)
	}

	// Make worktree dirty (1 staged, 1 unstaged, 1 untracked) and set manual baseline to c0.
	putFile(t, filepath.Join(workDir, "staged.txt"), "staged\n")
	gitTest(t, workDir, "add", "staged.txt")
	putFile(t, filepath.Join(workDir, "committed.txt"), "v2 unstaged\n")
	putFile(t, filepath.Join(workDir, "untracked.txt"), "untracked\n")

	dirtyBase, err := s.PutTaskBaseline(ctx, repo.ID, wtID, PutTaskBaselineRequest{BaseOID: c0})
	if err != nil {
		t.Fatalf("PutTaskBaseline(c0) on dirty wt: %v", err)
	}
	if dirtyBase.Source != TaskBaselineManualSelect || dirtyBase.CleanAtStart {
		t.Fatalf("expected manual_select with cleanAtStart=false, got %+v", dirtyBase)
	}
	if dirtyBase.PreExistingChanges.Staged != 1 || dirtyBase.PreExistingChanges.Unstaged != 1 || dirtyBase.PreExistingChanges.Untracked != 1 {
		t.Fatalf("unexpected preExistingChanges: %+v", dirtyBase.PreExistingChanges)
	}

	summary, err := s.ResolveTaskScope(ctx, repo.ID, wtID)
	if err != nil {
		t.Fatalf("ResolveTaskScope: %v", err)
	}
	if summary.Status != TaskScopeReady || summary.CommitCount != 1 || summary.UncommittedCount != 3 || summary.HeadOID != c1 {
		t.Fatalf("unexpected ResolveTaskScope summary: %+v", summary)
	}

	taskHist, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        HistoryScopeTask,
	})
	if err != nil || taskHist.Status != HistoryStatusReady || len(taskHist.Commits) != 1 || taskHist.Commits[0].OID != c1 {
		t.Fatalf("expected single commit c1 in c0..HEAD, got %+v %v", taskHist, err)
	}

	if err := s.DeleteTaskBaseline(ctx, repo.ID, wtID); err != nil {
		t.Fatalf("DeleteTaskBaseline: %v", err)
	}
	cleared, err := s.GetTaskBaseline(ctx, repo.ID, wtID)
	if err != nil || cleared != nil {
		t.Fatalf("expected nil baseline after DeleteTaskBaseline, got %+v %v", cleared, err)
	}
}
