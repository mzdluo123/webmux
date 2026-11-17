package codeview

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/workspace"
)

func TestGitTaskChangesAndDiffSingleNetDiffNoDoubleCount(t *testing.T) {
	t.Parallel()
	cv, ws, _, base := setupGitContextFixture(t)
	ctx := context.Background()

	repoDir := filepath.Join(base, "repo")
	if err := os.MkdirAll(repoDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repoDir, "init", "-q", "-b", "main")
	write(t, repoDir, "both.go", "package app\nconst step = 0\n")
	write(t, repoDir, "revert.txt", "original\n")
	write(t, repoDir, "recreate.txt", "before delete\n")
	write(t, repoDir, "same_recreate.txt", "identical content\n")
	gitTest(t, repoDir, "add", ".")
	gitTest(t, repoDir, "commit", "-q", "-m", "baseline commit")

	repo, _, err := ws.Register(ctx, repoDir)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	wt, err := ws.CreateWorktree(ctx, repo.ID, workspace.CreateRequest{
		Mode:   "new",
		Branch: "feat/net-diff",
		Base:   "main",
		Path:   filepath.Join(base, "wt-net-diff"),
	})
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Scenario A (A09): Agent performs 2 commits and leaves working tree completely clean!
	write(t, wt.Path, "both.go", "package app\nconst step = 1\n")
	write(t, wt.Path, "agent_added.txt", "added by agent commit 1\n")
	write(t, wt.Path, "ephemeral.txt", "added then removed\n")
	write(t, wt.Path, "revert.txt", "changed in commit 1\n")
	gitTest(t, wt.Path, "rm", "-q", "recreate.txt", "same_recreate.txt")
	gitTest(t, wt.Path, "add", "-A")
	gitTest(t, wt.Path, "commit", "-q", "-m", "feat: agent commit 1")

	write(t, wt.Path, "agent_second.txt", "added by agent commit 2\n")
	gitTest(t, wt.Path, "add", "agent_second.txt")
	gitTest(t, wt.Path, "commit", "-q", "-m", "feat: agent commit 2")

	cleanTask, err := cv.GitTaskChanges(ctx, repo.ID, wt.ID, ChangeQuery{})
	if err != nil {
		t.Fatalf("GitTaskChanges clean after 2 commits: %v", err)
	}
	if cleanTask.Status != workspace.TaskScopeReady || cleanTask.CommitCount != 2 || cleanTask.UncommittedCount != 0 {
		t.Fatalf("expected commitCount=2 uncommittedCount=0 on clean tree, got %+v", cleanTask)
	}
	if cleanTask.Changes.Total != 7 {
		t.Fatalf("expected 7 net changed files after 2 commits, got %d (%+v)", cleanTask.Changes.Total, cleanTask.Changes.Entries)
	}

	// Verify GitTaskDiff + GitSnapshotFile + GitSnapshotRaw work on clean committed task file.
	cleanDiff, err := cv.GitTaskDiff(ctx, repo.ID, wt.ID, "both.go", "")
	if err != nil {
		t.Fatalf("GitTaskDiff(both.go) clean: %v", err)
	}
	if cleanDiff.Old.Text != "package app\nconst step = 0\n" || cleanDiff.New.Text != "package app\nconst step = 1\n" {
		t.Fatalf("unexpected cleanDiff texts: old=%q new=%q", cleanDiff.Old.Text, cleanDiff.New.Text)
	}
	fullAfter, err := cv.GitSnapshotFile(ctx, repo.ID, wt.ID, "", cleanDiff.Snapshot, "after")
	if err != nil || fullAfter.Document.Text != "package app\nconst step = 1\n" {
		t.Fatalf("GitSnapshotFile(after): %+v %v", fullAfter, err)
	}
	transfer, err := cv.GitSnapshotRaw(ctx, repo.ID, wt.ID, "", cleanDiff.Snapshot, "before")
	if err != nil {
		t.Fatalf("GitSnapshotRaw(before) open: %v", err)
	}
	var rawBuf bytes.Buffer
	wErr := transfer.WriteTo(ctx, &rawBuf)
	transfer.Close()
	if wErr != nil || rawBuf.String() != "package app\nconst step = 0\n" {
		t.Fatalf("GitSnapshotRaw(before) stream: %q %v", rawBuf.String(), wErr)
	}

	// Scenario B (G01 anti-double-counting & net cancellation):
	// 1. Modify both.go AGAIN in working tree (unstaged) -> MUST appear ONCE in task-changes with step 0 -> step 2!
	// 2. Remove ephemeral.txt in working tree -> added in commit 1 then deleted on disk => net 0 change (omitted)!
	// 3. Restore revert.txt to "original\n" on disk => net 0 change (omitted)!
	// 4. Recreate recreate.txt as untracked with new content => collapsed from D+untracked into single M!
	// 5. Recreate same_recreate.txt as untracked with identical baseline content => net 0 change (omitted)!
	// 6. Create brand_new_untracked.txt => single A!
	write(t, wt.Path, "both.go", "package app\nconst step = 2\n")
	if err := os.Remove(filepath.Join(wt.Path, "ephemeral.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, wt.Path, "revert.txt", "original\n")
	write(t, wt.Path, "recreate.txt", "recreated on disk\n")
	write(t, wt.Path, "same_recreate.txt", "identical content\n")
	write(t, wt.Path, "brand_new_untracked.txt", "untracked leaf\n")

	netChanges, err := cv.GitTaskChanges(ctx, repo.ID, wt.ID, ChangeQuery{})
	if err != nil {
		t.Fatalf("GitTaskChanges mixed committed+uncommitted: %v", err)
	}
	if netChanges.CommitCount != 2 || netChanges.UncommittedCount != 6 {
		t.Fatalf("unexpected counts: commits=%d uncommitted=%d", netChanges.CommitCount, netChanges.UncommittedCount)
	}

	byPath := map[string]Change{}
	for _, entry := range netChanges.Changes.Entries {
		if _, dup := byPath[entry.NewPath]; dup {
			t.Fatalf("duplicate path %q in net task-changes: %+v", entry.NewPath, netChanges.Changes.Entries)
		}
		byPath[entry.NewPath] = entry
	}
	for _, omitted := range []string{"ephemeral.txt", "revert.txt", "same_recreate.txt"} {
		if _, exists := byPath[omitted]; exists {
			t.Fatalf("expected zero-net-change file %q to be omitted, got %+v", omitted, byPath[omitted])
		}
	}
	if byPath["both.go"].Status != "M" || byPath["recreate.txt"].Status != "M" || byPath["brand_new_untracked.txt"].Status != "A" {
		t.Fatalf("unexpected net statuses: %+v", byPath)
	}
	if len(byPath) != 5 {
		t.Fatalf("expected exactly 5 net entries (agent_added, agent_second, both.go, brand_new_untracked, recreate), got %d: %+v", len(byPath), byPath)
	}

	bothDiff, err := cv.GitTaskDiff(ctx, repo.ID, wt.ID, "both.go", "")
	if err != nil {
		t.Fatalf("GitTaskDiff(both.go) net: %v", err)
	}
	if bothDiff.Old.Text != "package app\nconst step = 0\n" || bothDiff.New.Text != "package app\nconst step = 2\n" {
		t.Fatalf("both.go net diff did not span baseline->worktree: old=%q new=%q", bothDiff.Old.Text, bothDiff.New.Text)
	}

	recreateDiff, err := cv.GitTaskDiff(ctx, repo.ID, wt.ID, "recreate.txt", "")
	if err != nil {
		t.Fatalf("GitTaskDiff(recreate.txt) net: %v", err)
	}
	if recreateDiff.Old.Text != "before delete\n" || recreateDiff.New.Text != "recreated on disk\n" {
		t.Fatalf("recreate.txt net diff mismatch: old=%q new=%q", recreateDiff.Old.Text, recreateDiff.New.Text)
	}

	// Scenario C: No baseline -> task-changes returns status=no_task_baseline, task-diff returns ErrNoTaskBaseline.
	mainWT := repo.Worktrees[0].ID
	noBaseChanges, err := cv.GitTaskChanges(ctx, repo.ID, mainWT, ChangeQuery{})
	if err != nil || noBaseChanges.Status != workspace.TaskScopeNoTaskBaseline || noBaseChanges.Baseline != nil {
		t.Fatalf("expected no_task_baseline on mainWT, got %+v %v", noBaseChanges, err)
	}
	if _, err := cv.GitTaskDiff(ctx, repo.ID, mainWT, "both.go", ""); !errors.Is(err, ErrNoTaskBaseline) || !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrNoTaskBaseline + ErrConflict on GitTaskDiff without baseline, got %v", err)
	}

	// Scenario D: Unborn baseline -> net diff from empty tree to live working tree.
	unbornDir := filepath.Join(base, "unborn-repo")
	if err := os.MkdirAll(unbornDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, unbornDir, "init", "-q", "-b", "main")
	unbornRepo, _, err := ws.Register(ctx, unbornDir)
	if err != nil {
		t.Fatalf("Register unbornRepo: %v", err)
	}
	unbornWT := unbornRepo.Worktrees[0].ID
	if _, err := ws.PutTaskBaseline(ctx, unbornRepo.ID, unbornWT, workspace.PutTaskBaselineRequest{}); err != nil {
		t.Fatalf("PutTaskBaseline unborn: %v", err)
	}
	write(t, unbornDir, "init.txt", "line 1\n")
	gitTest(t, unbornDir, "add", "init.txt")
	gitTest(t, unbornDir, "commit", "-q", "-m", "feat: initial commit")
	write(t, unbornDir, "init.txt", "line 1\nline 2\n")

	unbornChanges, err := cv.GitTaskChanges(ctx, unbornRepo.ID, unbornWT, ChangeQuery{})
	if err != nil || unbornChanges.CommitCount != 1 || unbornChanges.Changes.Total != 1 {
		t.Fatalf("unexpected unborn task-changes: %+v %v", unbornChanges, err)
	}
	unbornDiff, err := cv.GitTaskDiff(ctx, unbornRepo.ID, unbornWT, "init.txt", "")
	if err != nil {
		t.Fatalf("GitTaskDiff unborn: %v", err)
	}
	if unbornDiff.Old.Source != "empty" || unbornDiff.Old.Text != "" || !strings.Contains(unbornDiff.New.Text, "line 2") {
		t.Fatalf("unexpected unborn diff: old=%+v new=%+v", unbornDiff.Old, unbornDiff.New)
	}
}
