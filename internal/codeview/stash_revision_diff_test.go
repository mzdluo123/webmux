package codeview

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitStashDiffAndGitRevisionDiff(t *testing.T) {
	t.Parallel()
	cv, ws, _, base := setupGitContextFixture(t)
	ctx := context.Background()

	repoDir := filepath.Join(base, "stash-rev-repo")
	if err := os.MkdirAll(repoDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repoDir, "init", "-q", "-b", "main")
	write(t, repoDir, "app.go", "package main\nfunc run() { println(\"v1\") }\n")
	gitTest(t, repoDir, "add", ".")
	gitTest(t, repoDir, "commit", "-q", "-m", "feat: v1")
	commitV1 := strings.TrimSpace(gitTest(t, repoDir, "rev-parse", "HEAD"))

	// Create stash with tracked modification + 3rd-parent untracked file!
	write(t, repoDir, "app.go", "package main\nfunc run() { println(\"stashed-v2\") }\n")
	write(t, repoDir, "draft.txt", "untracked stash note\n")
	gitTest(t, repoDir, "stash", "push", "--include-untracked", "-m", "WIP: stash diff test")
	stashOID := strings.TrimSpace(gitTest(t, repoDir, "rev-parse", "refs/stash"))

	// Modify working tree file so GitRevisionDiff compares commitV1 -> live worktree!
	write(t, repoDir, "app.go", "package main\nfunc run() { println(\"worktree-live\") }\n")

	repo, _, err := ws.Register(ctx, repoDir)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	wtID := repo.Worktrees[0].ID

	repoCtx, err := cv.GitContext(ctx, repo.ID, "")
	if err != nil {
		t.Fatalf("GitContext repo: %v", err)
	}
	wtCtx, err := cv.GitContext(ctx, repo.ID, wtID)
	if err != nil {
		t.Fatalf("GitContext worktree: %v", err)
	}

	// 1. GitStashDiff for tracked file works even in repository-only context!
	trackedDiff, err := cv.GitStashDiff(ctx, repo.ID, "", repoCtx.ID, stashOID, "app.go", false)
	if err != nil {
		t.Fatalf("GitStashDiff tracked: %v", err)
	}
	if !strings.Contains(trackedDiff.Old.Text, "v1") || !strings.Contains(trackedDiff.New.Text, "stashed-v2") {
		t.Fatalf("unexpected tracked stash diff: old=%q new=%q", trackedDiff.Old.Text, trackedDiff.New.Text)
	}
	snapFile, err := cv.GitSnapshotFile(ctx, repo.ID, "", repoCtx.ID, trackedDiff.Snapshot, "after")
	if err != nil || !strings.Contains(snapFile.Document.Text, "stashed-v2") {
		t.Fatalf("GitSnapshotFile stash tracked after: %+v %v", snapFile, err)
	}

	// 2. GitStashDiff for 3rd-parent untracked file (`empty -> S^3`)!
	untrackedDiff, err := cv.GitStashDiff(ctx, repo.ID, "", repoCtx.ID, stashOID, "draft.txt", true)
	if err != nil {
		t.Fatalf("GitStashDiff untracked: %v", err)
	}
	if untrackedDiff.Old.Source != "empty" || !strings.Contains(untrackedDiff.New.Text, "untracked stash note") {
		t.Fatalf("unexpected untracked stash diff: %+v", untrackedDiff)
	}

	// Non-stash commit OID must be rejected with ErrNotFound!
	if _, err := cv.GitStashDiff(ctx, repo.ID, "", repoCtx.ID, commitV1, "app.go", false); errorKind(err) != ErrNotFound {
		t.Fatalf("expected ErrNotFound for non-stash commit OID, got %v", err)
	}

	// 3. GitRevisionDiff requires a worktree and compares commitV1 -> live worktree!
	if _, err := cv.GitRevisionDiff(ctx, repo.ID, "", repoCtx.ID, commitV1, "app.go"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly without worktreeID, got %v", err)
	}
	revDiff, err := cv.GitRevisionDiff(ctx, repo.ID, wtID, wtCtx.ID, commitV1, "app.go")
	if err != nil {
		t.Fatalf("GitRevisionDiff: %v", err)
	}
	if !strings.Contains(revDiff.Old.Text, "v1") || !strings.Contains(revDiff.New.Text, "worktree-live") {
		t.Fatalf("unexpected revision diff: old=%q new=%q", revDiff.Old.Text, revDiff.New.Text)
	}
}
