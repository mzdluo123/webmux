package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitAmendPushedProtectionAuthorOverrideAndSignoff(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	// 1. Root commit Amend (len(HEAD.parents) == 0) + Empty-Amend blocker!
	rootDir := filepath.Join(t.TempDir(), "root-amend")
	if err := os.MkdirAll(rootDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, rootDir, "init", "-b", "main")
	putFile(t, filepath.Join(rootDir, "only.txt"), "v1\n")
	gitTest(t, rootDir, "add", "only.txt")
	gitTest(t, rootDir, "commit", "-m", "feat: root v1")

	s, _ := testService(t, t.TempDir(), rootDir)
	rootRepo := registerTest(t, s, rootDir)
	rootWTID := rootRepo.Worktrees[0].ID

	// Removing the only file from index makes tree == emptyTree -> blocked by empty_amend!
	gitTest(t, rootDir, "rm", "--cached", "only.txt")
	emptyAmendPrev, err := s.PreviewCommitWithOptions(ctx, rootRepo.ID, rootWTID, CommitPreviewOptions{Amend: true})
	if err != nil {
		t.Fatalf("PreviewCommitWithOptions empty root amend: %v", err)
	}
	if emptyAmendPrev.ConfirmationID != "" || !hasBlocker(emptyAmendPrev.Blockers, "empty_amend") {
		t.Fatalf("expected empty_amend blocker on root commit, got %+v", emptyAmendPrev)
	}

	// Re-add updated file and amend root commit with AuthorOverride + Signoff!
	putFile(t, filepath.Join(rootDir, "only.txt"), "v2 root amended\n")
	gitTest(t, rootDir, "add", "only.txt")
	rootAmendPrev, err := s.PreviewCommitWithOptions(ctx, rootRepo.ID, rootWTID, CommitPreviewOptions{
		Amend:          true,
		AuthorOverride: "Grace Hopper <grace@example.com>",
		Signoff:        true,
	})
	if err != nil || rootAmendPrev.ConfirmationID == "" {
		t.Fatalf("PreviewCommitWithOptions root amend: %+v %v", rootAmendPrev, err)
	}
	if !rootAmendPrev.Amend || rootAmendPrev.AmendPushed || !rootAmendPrev.Identity.AuthorOverride {
		t.Fatalf("unexpected amend preview flags: %+v", rootAmendPrev)
	}
	if rootAmendPrev.Identity.AuthorName != "Grace Hopper" || rootAmendPrev.Identity.AuthorEmail != "grace@example.com" {
		t.Fatalf("unexpected override author in preview: %+v", rootAmendPrev.Identity)
	}
	if !strings.HasPrefix(rootAmendPrev.Identity.SignoffTrailer, "Signed-off-by: ") {
		t.Fatalf("expected SignoffTrailer populated, got %q", rootAmendPrev.Identity.SignoffTrailer)
	}

	rootAmendRes, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:    rootRepo.ID,
		WorktreeID:      rootWTID,
		ConfirmationID:  rootAmendPrev.ConfirmationID,
		ClientRequestID: "amend-root-1",
		Message:         "feat: root v2 amended",
	})
	if err != nil {
		t.Fatalf("ExecuteCommit root amend: %v", err)
	}
	if rootAmendRes.Outcome != CommitOutcomeMatched || !rootAmendRes.Amended || len(rootAmendRes.Parents) != 0 {
		t.Fatalf("expected matched root amend with 0 parents, got %+v", rootAmendRes)
	}
	if rootAmendRes.AuthorName != "Grace Hopper" || rootAmendRes.AuthorEmail != "grace@example.com" {
		t.Fatalf("expected Grace Hopper author on amended commit, got %+v", rootAmendRes)
	}
	fullBody := gitTest(t, rootDir, "log", "-1", "--format=%B", rootAmendRes.CommitOID)
	if !strings.Contains(fullBody, "Signed-off-by: ") {
		t.Fatalf("expected Signed-off-by trailer in commit body, got %q", fullBody)
	}

	// 2. Invalid AuthorOverride format rejected with ErrInvalid.
	for _, badAuthor := range []string{"Grace Hopper", "<grace@example.com>", "Grace <not-an-email>", "-flag <a@b.com>"} {
		if _, err := s.PreviewCommitWithOptions(ctx, rootRepo.ID, rootWTID, CommitPreviewOptions{
			AuthorOverride: badAuthor,
		}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid for bad author %q, got %v", badAuthor, err)
		}
	}

	// 3. Pushed Commit Protection: push to bare remote, then attempt Amend!
	bareRemote := filepath.Join(t.TempDir(), "origin.git")
	gitTest(t, rootDir, "init", "--bare", bareRemote)
	gitTest(t, rootDir, "remote", "add", "origin", bareRemote)
	gitTest(t, rootDir, "push", "-u", "origin", "main")

	// Message-only amend on pushed HEAD (no staged changes needed in Amend mode!).
	pushedPrev, err := s.PreviewCommitWithOptions(ctx, rootRepo.ID, rootWTID, CommitPreviewOptions{
		Amend: true,
	})
	if err != nil || pushedPrev.ConfirmationID == "" {
		t.Fatalf("PreviewCommitWithOptions pushed amend: %+v %v", pushedPrev, err)
	}
	if !pushedPrev.AmendPushed || pushedPrev.HeadPush.State != PushPushed {
		t.Fatalf("expected AmendPushed=true and HeadPush=pushed, got %+v", pushedPrev)
	}
	var sawPushedWarning bool
	for _, w := range pushedPrev.Warnings {
		if w.Code == "amend_pushed" && strings.Contains(w.Message, "该提交已推送至远端") {
			sawPushedWarning = true
		}
	}
	if !sawPushedWarning {
		t.Fatalf("expected explicit pushed-commit warning, got %+v", pushedPrev.Warnings)
	}

	// ExecuteCommit WITHOUT AcknowledgePushedAmend MUST fail with ErrBlocked!
	if _, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:           rootRepo.ID,
		WorktreeID:             rootWTID,
		ConfirmationID:         pushedPrev.ConfirmationID,
		Message:                "feat: attempt unacknowledged pushed amend",
		AcknowledgePushedAmend: false,
	}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked when amending pushed commit without acknowledgement, got %v", err)
	}

	// Re-preview and ExecuteCommit WITH AcknowledgePushedAmend: true succeeds!
	pushedPrev2, err := s.PreviewCommitWithOptions(ctx, rootRepo.ID, rootWTID, CommitPreviewOptions{
		Amend: true,
	})
	if err != nil || pushedPrev2.ConfirmationID == "" {
		t.Fatal(err)
	}
	pushedRes, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:           rootRepo.ID,
		WorktreeID:             rootWTID,
		ConfirmationID:         pushedPrev2.ConfirmationID,
		ClientRequestID:        "amend-pushed-ack-1",
		Message:                "feat: acknowledged pushed amend",
		AcknowledgePushedAmend: true,
	})
	if err != nil || pushedRes.Outcome != CommitOutcomeMatched || !pushedRes.Amended {
		t.Fatalf("expected matched acknowledged pushed amend, got %+v %v", pushedRes, err)
	}
}
