package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/session"
)

func initCollabRepo(t *testing.T) (string, *Service, string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("requires real Git")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	for _, key := range []string{"GIT_AUTHOR", "GIT_COMMITTER"} {
		t.Setenv(key+"_NAME", "Collab Tester")
		t.Setenv(key+"_EMAIL", "collab@example.invalid")
	}

	root := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("line1\nline2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("beta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "."}, {"commit", "-m", "initial"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}

	stateDir := t.TempDir()
	mgr := session.NewManager(session.Options{})
	t.Cleanup(func() { mgr.Shutdown(context.Background()) })
	svc := New(stateDir, mgr, nil, root)
	t.Cleanup(svc.Close)

	repo, _, err := svc.Register(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, svc, repo.ID, repo.Worktrees[0].ID
}

func TestCommitDraftOptimisticRevisionAndCommitClearing(t *testing.T) {
	root, svc, repoID, wtID := initCollabRepo(t)
	ctx := context.Background()

	initial, err := svc.GetCommitDraft(ctx, repoID, wtID)
	if err != nil {
		t.Fatal(err)
	}
	if initial.Revision != 0 || initial.Subject != "" || initial.Body != "" || initial.UpdatedAt != nil {
		t.Fatalf("unexpected initial draft: %+v", initial)
	}

	saved1, err := svc.PutCommitDraft(ctx, repoID, wtID, PutCommitDraftRequest{
		Subject:      "feat: draft from laptop",
		Body:         "first paragraph",
		BaseRevision: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved1.Revision != 1 || saved1.Subject != "feat: draft from laptop" || saved1.UpdatedAt == nil {
		t.Fatalf("unexpected saved1: %+v", saved1)
	}

	// Stale baseRevision=0 from a phone tab returns DraftConflictError carrying saved1.
	_, err = svc.PutCommitDraft(ctx, repoID, wtID, PutCommitDraftRequest{
		Subject:      "feat: draft from phone",
		Body:         "phone body",
		BaseRevision: 0,
	})
	var conflict *DraftConflictError
	if !errors.As(err, &conflict) || !errors.Is(err, ErrConflict) {
		t.Fatalf("expected DraftConflictError, got %v", err)
	}
	if conflict.Current.Revision != 1 || conflict.Current.Subject != "feat: draft from laptop" {
		t.Fatalf("unexpected conflict current draft: %+v", conflict.Current)
	}

	// Resolving with baseRevision=1 succeeds.
	saved2, err := svc.PutCommitDraft(ctx, repoID, wtID, PutCommitDraftRequest{
		Subject:      "feat: merged draft",
		Body:         "first paragraph\n\nphone body",
		BaseRevision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved2.Revision != 2 {
		t.Fatalf("expected revision 2, got %d", saved2.Revision)
	}

	// Oversized draft rejected with ErrTooLarge without overwriting saved2.
	_, err = svc.PutCommitDraft(ctx, repoID, wtID, PutCommitDraftRequest{
		Subject:      "x",
		Body:         strings.Repeat("a", MaxCommitDraftBytes),
		BaseRevision: 2,
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}

	// Failed commit (expired/invalid confirmationId) MUST preserve draft.
	_, err = svc.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:   repoID,
		WorktreeID:     wtID,
		ConfirmationID: "invalid-confirmation",
		Message:        saved2.Subject,
	})
	if err == nil {
		t.Fatal("expected stale error on invalid confirmation")
	}
	afterFail, err := svc.GetCommitDraft(ctx, repoID, wtID)
	if err != nil {
		t.Fatal(err)
	}
	if afterFail.Subject != "feat: merged draft" || afterFail.Revision != 2 {
		t.Fatalf("draft lost after failed commit: %+v", afterFail)
	}

	// Stage and commit for real -> draft MUST be cleared and revision bumped to 3.
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("line1\nline2\nline3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	staging, err := svc.InspectStaging(ctx, repoID, wtID)
	if err != nil || len(staging.Unstaged) != 1 {
		t.Fatalf("inspect staging: %+v %v", staging, err)
	}
	if err := svc.Stage(ctx, StageRequest{
		RepositoryID: repoID,
		WorktreeID:   wtID,
		Identity:     staging.Unstaged[0].Identity,
	}); err != nil {
		t.Fatal(err)
	}
	preview, err := svc.PreviewCommit(ctx, repoID, wtID)
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:    repoID,
		WorktreeID:      wtID,
		ConfirmationID:  preview.ConfirmationID,
		ClientRequestID: "commit-clear-draft-1",
		Message:         saved2.Subject + "\n\n" + saved2.Body,
	})
	if err != nil || res.Outcome != CommitOutcomeMatched {
		t.Fatalf("execute commit: %+v %v", res, err)
	}
	afterCommit, err := svc.GetCommitDraft(ctx, repoID, wtID)
	if err != nil {
		t.Fatal(err)
	}
	if afterCommit.Subject != "" || afterCommit.Body != "" || afterCommit.Revision != 3 {
		t.Fatalf("expected cleared draft at revision 3, got %+v", afterCommit)
	}
}

func TestReviewsContentInvalidationAndStaleRejection(t *testing.T) {
	root, svc, repoID, wtID := initCollabRepo(t)
	ctx := context.Background()

	// Set task baseline at initial HEAD.
	if _, err := svc.PutTaskBaseline(ctx, repoID, wtID, PutTaskBaselineRequest{}); err != nil {
		t.Fatal(err)
	}

	// Modify a.txt and b.txt in worktree.
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("line1\nupdated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("beta-v2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Inspect uncommitted reviews.
	rev, err := svc.GetReviews(ctx, repoID, wtID, ReviewScopeUncommitted)
	if err != nil {
		t.Fatal(err)
	}
	if rev.TotalFiles != 2 || rev.ReviewedFiles != 0 {
		t.Fatalf("expected 2 unreviewed files, got %+v", rev)
	}

	fileA := rev.Files[0]
	markRes, err := svc.PutReviews(ctx, repoID, wtID, PutReviewsRequest{
		Scope:  ReviewScopeUncommitted,
		Action: ReviewActionMark,
		Item: &ReviewMarkIdentity{
			Scope:          fileA.Scope,
			Area:           fileA.Area,
			Path:           fileA.Path,
			BaseMode:       fileA.BaseMode,
			BaseIdentity:   fileA.BaseIdentity,
			TargetMode:     fileA.TargetMode,
			TargetIdentity: fileA.TargetIdentity,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !markRes.Applied || markRes.ReviewedFiles != 1 {
		t.Fatalf("expected mark applied and 1 reviewed, got %+v", markRes)
	}

	// Also mark a.txt in task scope.
	taskRev, err := svc.GetReviews(ctx, repoID, wtID, ReviewScopeTask)
	if err != nil || taskRev.TotalFiles != 2 {
		t.Fatalf("task reviews: %+v %v", taskRev, err)
	}
	taskFileA := taskRev.Files[0]
	taskMarked, err := svc.PutReviews(ctx, repoID, wtID, PutReviewsRequest{
		Scope:  ReviewScopeTask,
		Action: ReviewActionMark,
		Item: &ReviewMarkIdentity{
			Scope:          taskFileA.Scope,
			Area:           taskFileA.Area,
			Path:           taskFileA.Path,
			BaseMode:       taskFileA.BaseMode,
			BaseIdentity:   taskFileA.BaseIdentity,
			TargetMode:     taskFileA.TargetMode,
			TargetIdentity: taskFileA.TargetIdentity,
		},
	})
	if err != nil || !taskMarked.Applied || taskMarked.ReviewedFiles != 1 {
		t.Fatalf("task mark: %+v %v", taskMarked, err)
	}

	// Agent edits a.txt on disk -> BOTH uncommitted and task review marks for a.txt MUST auto-invalidate!
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("line1\nagent-rewrote-this\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	afterEditUncommitted, err := svc.GetReviews(ctx, repoID, wtID, ReviewScopeUncommitted)
	if err != nil {
		t.Fatal(err)
	}
	if afterEditUncommitted.ReviewedFiles != 0 || afterEditUncommitted.Files[0].Reviewed {
		t.Fatalf("expected uncommitted review invalidated after file edit, got %+v", afterEditUncommitted)
	}
	afterEditTask, err := svc.GetReviews(ctx, repoID, wtID, ReviewScopeTask)
	if err != nil {
		t.Fatal(err)
	}
	if afterEditTask.ReviewedFiles != 0 || afterEditTask.Files[0].Reviewed {
		t.Fatalf("expected task review invalidated after file edit, got %+v", afterEditTask)
	}

	// Offline/stale device tries to PUT mark with OLD TargetIdentity -> MUST be rejected (Applied=false, StaleRejected=true) and leave ReviewedFiles == 0 (A10)!
	staleAttempt, err := svc.PutReviews(ctx, repoID, wtID, PutReviewsRequest{
		Scope:  ReviewScopeUncommitted,
		Action: ReviewActionMark,
		Item: &ReviewMarkIdentity{
			Scope:          fileA.Scope,
			Area:           fileA.Area,
			Path:           fileA.Path,
			BaseMode:       fileA.BaseMode,
			BaseIdentity:   fileA.BaseIdentity,
			TargetMode:     fileA.TargetMode,
			TargetIdentity: fileA.TargetIdentity, // stale hash!
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if staleAttempt.Applied || !staleAttempt.StaleRejected || staleAttempt.ReviewedFiles != 0 {
		t.Fatalf("stale mark should be rejected without marking modified file: %+v", staleAttempt)
	}
}
