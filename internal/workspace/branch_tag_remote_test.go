package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchRenameUpstreamTagsAndRemotesRedaction(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	bareOrigin := filepath.Join(t.TempDir(), "origin.git")
	gitTest(t, dir, "init", "--bare", bareOrigin)
	gitTest(t, dir, "remote", "add", "origin", bareOrigin)
	gitTest(t, dir, "push", "-u", "origin", "main")

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	headOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// 1. Branch Rename: preview checked-out `main` -> `trunk`, verify upstream config preserved & collision blocked.
	gitTest(t, dir, "branch", "existing-branch", headOID)
	blockedPrev, err := s.PreviewBranchRename(ctx, repo.ID, BranchRenamePreviewRequest{
		OldName: "main",
		NewName: "existing-branch",
	})
	if err != nil {
		t.Fatalf("PreviewBranchRename collision: %v", err)
	}
	if blockedPrev.ConfirmationID != "" || !hasBlocker(blockedPrev.Blockers, "branch_exists") {
		t.Fatalf("expected branch_exists blocker, got %+v", blockedPrev)
	}

	rnPrev, err := s.PreviewBranchRename(ctx, repo.ID, BranchRenamePreviewRequest{
		OldName: "main",
		NewName: "trunk",
	})
	if err != nil || rnPrev.ConfirmationID == "" {
		t.Fatalf("PreviewBranchRename: %+v %v", rnPrev, err)
	}
	if len(rnPrev.AffectedWorktrees) != 1 || rnPrev.Upstream != "origin/main" {
		t.Fatalf("expected 1 affected worktree and upstream origin/main, got %+v", rnPrev)
	}

	rnRes, err := s.RenameBranch(ctx, repo.ID, RenameBranchRequest{
		ConfirmationID:  rnPrev.ConfirmationID,
		ClientRequestID: "branch-rename-1",
	})
	if err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}
	if rnRes.Branch != "trunk" || rnRes.Upstream != "origin/main" {
		t.Fatalf("unexpected RenameBranch result: %+v", rnRes)
	}
	symRef := strings.TrimSpace(gitTest(t, dir, "symbolic-ref", "--short", "HEAD"))
	if symRef != "trunk" {
		t.Fatalf("worktree HEAD symbolic-ref = %q, want trunk", symRef)
	}

	// 2. Unset and Set Upstream tracking branch!
	unsetRes, err := s.SetBranchUpstream(ctx, repo.ID, BranchUpstreamRequest{
		Branch:          "trunk",
		Upstream:        "",
		ClientRequestID: "upstream-unset-1",
	})
	if err != nil || unsetRes.Upstream != "" {
		t.Fatalf("SetBranchUpstream unset: %+v %v", unsetRes, err)
	}
	setRes, err := s.SetBranchUpstream(ctx, repo.ID, BranchUpstreamRequest{
		Branch:          "trunk",
		Upstream:        "origin/main",
		ClientRequestID: "upstream-set-1",
	})
	if err != nil || setRes.Upstream != "origin/main" {
		t.Fatalf("SetBranchUpstream set: %+v %v", setRes, err)
	}

	// 3. Tags: lightweight + annotated + collision rejection + tag.gpgSign policy + delete recovery command!
	lwTag, err := s.CreateTag(ctx, repo.ID, CreateTagRequest{
		Name:            "v0.1.0",
		TargetOID:       headOID,
		Annotated:       false,
		ClientRequestID: "tag-create-lw-1",
	})
	if err != nil || lwTag.Tag.Kind != "lightweight" || lwTag.Tag.Head != headOID {
		t.Fatalf("CreateTag lightweight: %+v %v", lwTag, err)
	}
	// Duplicate tag name MUST fail with ErrConflict (never force-overwrite `-f`!).
	if _, err := s.CreateTag(ctx, repo.ID, CreateTagRequest{
		Name:      "v0.1.0",
		TargetOID: headOID,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate tag, got %v", err)
	}

	annTag, err := s.CreateTag(ctx, repo.ID, CreateTagRequest{
		Name:            "v0.2.0",
		TargetOID:       headOID,
		Annotated:       true,
		Message:         "Release v0.2.0\n\nDetailed release notes.",
		ClientRequestID: "tag-create-ann-1",
	})
	if err != nil || annTag.Tag.Kind != "annotated" || annTag.Tag.TagOID == "" || !strings.Contains(annTag.Tag.Message, "Release v0.2.0") {
		t.Fatalf("CreateTag annotated: %+v %v", annTag, err)
	}

	// When tag.gpgSign=true, lightweight tag creation is explicitly blocked with clear guidance!
	gitTest(t, dir, "config", "tag.gpgSign", "true")
	if _, err := s.CreateTag(ctx, repo.ID, CreateTagRequest{
		Name:      "v0.3.0-lw",
		TargetOID: headOID,
		Annotated: false,
	}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked for lightweight tag when tag.gpgSign=true, got %v", err)
	}
	gitTest(t, dir, "config", "--unset", "tag.gpgSign")

	delTag, err := s.DeleteTag(ctx, repo.ID, DeleteTagRequest{
		Name:              "v0.1.0",
		ExpectedTargetOID: headOID,
		ClientRequestID:   "tag-delete-1",
	})
	if err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	if delTag.TargetOID != headOID || !strings.Contains(delTag.RecoveryCommand, "git tag v0.1.0 "+headOID) {
		t.Fatalf("unexpected DeleteTag result: %+v", delTag)
	}

	// 4. Remotes: add/update/remove with strict credential redaction (`***`) and `***` placeholder rejection!
	secretFetch := "https://alice:supersecretToken123@example.com/team/repo.git"
	secretPush := "https://bob:pushSecret456@example.com/team/repo-push.git"
	addRem, err := s.AddRemote(ctx, repo.ID, AddRemoteRequest{
		Name:            "mirror",
		FetchURL:        secretFetch,
		PushURL:         secretPush,
		ClientRequestID: "remote-add-1",
	})
	if err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	var mirror Remote
	for _, r := range addRem.Remotes {
		if r.Name == "mirror" {
			mirror = r
		}
	}
	if strings.Contains(mirror.FetchURL, "supersecretToken123") || strings.Contains(mirror.PushURL, "pushSecret456") {
		t.Fatalf("credential leaked in RemoteMutationResult! %+v", mirror)
	}
	if !strings.Contains(mirror.FetchURL, "***") || !strings.Contains(mirror.PushURL, "***") {
		t.Fatalf("expected redacted *** in remote URLs, got %+v", mirror)
	}

	// Reject literal `***` placeholder on update so credentials are never overwritten with stars!
	redactedCopy := mirror.FetchURL
	if _, err := s.UpdateRemote(ctx, repo.ID, "mirror", UpdateRemoteRequest{
		FetchURL: &redactedCopy,
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid when submitting redacted *** URL, got %v", err)
	}

	// Verify operation ledger never stored plaintext token either!
	opRec, err := s.GetGitOperation(repo.ID, addRem.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(opRec.Summary, "supersecretToken123") || strings.Contains(opRec.Preview.Target, "supersecretToken123") {
		t.Fatalf("credential leaked into git-operations.json! %+v", opRec)
	}

	newFetch := "https://example.com/team/public-mirror.git"
	updRem, err := s.UpdateRemote(ctx, repo.ID, "mirror", UpdateRemoteRequest{
		FetchURL:        &newFetch,
		ClearPushURL:    true,
		ClientRequestID: "remote-upd-1",
	})
	if err != nil {
		t.Fatalf("UpdateRemote: %v", err)
	}
	for _, r := range updRem.Remotes {
		if r.Name == "mirror" && (r.FetchURL != newFetch || r.PushURL != newFetch) {
			t.Fatalf("expected mirror fetch & push reset to %q, got %+v", newFetch, r)
		}
	}

	remRes, err := s.RemoveRemote(ctx, repo.ID, "mirror", "remote-rm-1")
	if err != nil {
		t.Fatalf("RemoveRemote: %v", err)
	}
	for _, r := range remRes.Remotes {
		if r.Name == "mirror" {
			t.Fatalf("expected mirror removed, still present in %+v", remRes.Remotes)
		}
	}
}
