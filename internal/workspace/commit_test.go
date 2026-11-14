package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func hasBlocker(issues []Issue, code string) bool {
	for _, iss := range issues {
		if iss.Code == code {
			return true
		}
	}
	return false
}

func TestPreviewCommitFullIndexAndBlockers(t *testing.T) {
	// Uses t.Setenv in the missing-identity subcase, so keep top-level serial.
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	// 1. Empty index -> blocked, no ConfirmationID.
	emptyPrev, err := s.PreviewCommit(ctx, repo.ID, wtID)
	if err != nil {
		t.Fatalf("PreviewCommit empty: %v", err)
	}
	if emptyPrev.ConfirmationID != "" || !hasBlocker(emptyPrev.Blockers, "empty_index") {
		t.Fatalf("expected empty_index blocker without confirmation, got %+v", emptyPrev)
	}

	// 2. CLI pre-staged a.go + UI staged b.go + unstaged tracked + untracked d.txt + executable hook.
	putFile(t, filepath.Join(dir, "a.go"), "package a\n")
	gitTest(t, dir, "add", "a.go") // pre-staged in terminal

	putFile(t, filepath.Join(dir, "b.go"), "package b\n")
	bSnap, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "b.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Stage(ctx, StageRequest{RepositoryID: repo.ID, WorktreeID: wtID, Identity: bSnap.Identity}); err != nil {
		t.Fatal(err)
	}

	putFile(t, filepath.Join(dir, "tracked"), "unstaged edit\n")
	putFile(t, filepath.Join(dir, "d.txt"), "untracked\n")
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	prev, err := s.PreviewCommit(ctx, repo.ID, wtID)
	if err != nil {
		t.Fatalf("PreviewCommit full index: %v", err)
	}
	if prev.ConfirmationID == "" || len(prev.Blockers) != 0 {
		t.Fatalf("expected unblocked confirmation, got %+v", prev)
	}
	stagedPaths := make([]string, 0, len(prev.StagedFiles))
	for _, f := range prev.StagedFiles {
		stagedPaths = append(stagedPaths, f.NewPath)
	}
	slices.Sort(stagedPaths)
	if !slices.Equal(stagedPaths, []string{"a.go", "b.go"}) {
		t.Fatalf("expected full index [a.go b.go] in preview, got %v", stagedPaths)
	}
	if prev.UnstagedCount != 1 || prev.UntrackedCount != 1 {
		t.Fatalf("expected 1 unstaged and 1 untracked remaining, got unstaged=%d untracked=%d", prev.UnstagedCount, prev.UntrackedCount)
	}
	if !slices.Contains(prev.ExecutableHooks, "pre-commit") {
		t.Fatalf("expected pre-commit detected in ExecutableHooks, got %v", prev.ExecutableHooks)
	}
	realWriteTree := strings.TrimSpace(gitTest(t, dir, "write-tree"))
	if prev.ExpectedTreeOID != realWriteTree {
		t.Fatalf("ExpectedTreeOID %s != git write-tree %s", prev.ExpectedTreeOID, realWriteTree)
	}

	// 3. Missing user.email -> missing_identity blocker!
	t.Run("MissingIdentityBlocker", func(t *testing.T) {
		t.Setenv("GIT_AUTHOR_EMAIL", "")
		t.Setenv("GIT_COMMITTER_EMAIL", "")
		t.Setenv("EMAIL", "")
		noEmailPrev, pErr := s.PreviewCommit(ctx, repo.ID, wtID)
		if pErr != nil {
			t.Fatalf("PreviewCommit missing email: %v", pErr)
		}
		if noEmailPrev.ConfirmationID != "" || !hasBlocker(noEmailPrev.Blockers, "missing_identity") {
			t.Fatalf("expected missing_identity blocker, got %+v", noEmailPrev)
		}
	})

	// 4. Detached HEAD -> detached_head blocker!
	gitTest(t, dir, "checkout", "--detach", "HEAD")
	detachedPrev, err := s.PreviewCommit(ctx, repo.ID, wtID)
	if err != nil {
		t.Fatalf("PreviewCommit detached: %v", err)
	}
	if detachedPrev.ConfirmationID != "" || detachedPrev.HeadState != "detached" || !hasBlocker(detachedPrev.Blockers, "detached_head") {
		t.Fatalf("expected detached_head blocker, got %+v", detachedPrev)
	}
	gitTest(t, dir, "checkout", "main")

	// 5. In-progress operation (rebase-merge) & unmerged conflict -> blockers!
	rebaseDir := filepath.Join(dir, ".git", "rebase-merge")
	if err := os.MkdirAll(rebaseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	opPrev, err := s.PreviewCommit(ctx, repo.ID, wtID)
	_ = os.RemoveAll(rebaseDir)
	if err != nil {
		t.Fatalf("PreviewCommit operation: %v", err)
	}
	if opPrev.ConfirmationID != "" || !hasBlocker(opPrev.Blockers, "operation_in_progress") {
		t.Fatalf("expected operation_in_progress blocker, got %+v", opPrev)
	}
}

func TestExecuteCommitMatchedLockBusyStaleHooksAndConcurrentCLI(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	// 1. Unborn HEAD initial commit -> matched!
	unbornDir := filepath.Join(t.TempDir(), "unborn-commit")
	if err := os.MkdirAll(unbornDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, unbornDir, "init", "-b", "main")
	putFile(t, filepath.Join(unbornDir, "init.txt"), "hello unborn\n")
	gitTest(t, unbornDir, "add", "init.txt")

	s, _ := testService(t, t.TempDir(), unbornDir)
	unbornRepo := registerTest(t, s, unbornDir)
	unbornWTID := unbornRepo.Worktrees[0].ID

	uPrev, err := s.PreviewCommit(ctx, unbornRepo.ID, unbornWTID)
	if err != nil || uPrev.ConfirmationID == "" || uPrev.HeadState != "unborn" {
		t.Fatalf("PreviewCommit unborn: %+v %v", uPrev, err)
	}
	uRes, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:   unbornRepo.ID,
		WorktreeID:     unbornWTID,
		ConfirmationID: uPrev.ConfirmationID,
		Message:        "feat: initial commit from workbench\n",
	})
	if err != nil || uRes.Outcome != CommitOutcomeMatched || len(uRes.Parents) != 0 || uRes.TreeOID != uPrev.ExpectedTreeOID {
		t.Fatalf("ExecuteCommit unborn mismatch: %+v %v", uRes, err)
	}

	// 2. CLI tamper after PreviewCommit invalidates confirmation without creating commit!
	dir := initRepo(t)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	putFile(t, filepath.Join(dir, "planned.txt"), "planned\n")
	gitTest(t, dir, "add", "planned.txt")
	stalePrev, err := s.PreviewCommit(ctx, repo.ID, wtID)
	if err != nil || stalePrev.ConfirmationID == "" {
		t.Fatalf("PreviewCommit before CLI tamper: %+v %v", stalePrev, err)
	}
	// External CLI stages extra.txt before user clicks confirm!
	putFile(t, filepath.Join(dir, "extra.txt"), "sneaky\n")
	gitTest(t, dir, "add", "extra.txt")

	if _, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: stalePrev.ConfirmationID,
		Message:        "should not commit extra.txt",
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict when index changed after preview, got %v", err)
	}

	// 3. External .git/index.lock reports busy (ErrConflict) and NEVER deletes the lock file!
	lockPrev, err := s.PreviewCommit(ctx, repo.ID, wtID)
	if err != nil || lockPrev.ConfirmationID == "" {
		t.Fatalf("PreviewCommit before lock test: %+v %v", lockPrev, err)
	}
	lockFile := filepath.Join(dir, ".git", "index.lock")
	putFile(t, lockFile, "external pid\n")
	_, lockErr := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: lockPrev.ConfirmationID,
		Message:        "blocked by lock",
	})
	if !errors.Is(lockErr, ErrConflict) {
		t.Fatalf("expected ErrConflict on external index.lock, got %v", lockErr)
	}
	if _, statErr := os.Stat(lockFile); statErr != nil {
		t.Fatalf("external .git/index.lock was deleted or missing! err=%v", statErr)
	}
	if err := os.Remove(lockFile); err != nil {
		t.Fatal(err)
	}

	// 4. A05: pre-commit hook reformats & re-stages file -> Outcome == needs_review!
	preCommitPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	preCommitScript := "#!/bin/sh\nprintf 'formatted by pre-commit\\n' > planned.txt\ngit add planned.txt\n"
	if err := os.WriteFile(preCommitPath, []byte(preCommitScript), 0o755); err != nil {
		t.Fatal(err)
	}
	a05Prev, err := s.PreviewCommit(ctx, repo.ID, wtID)
	if err != nil || a05Prev.ConfirmationID == "" {
		t.Fatalf("PreviewCommit A05: %+v %v", a05Prev, err)
	}
	a05Res, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: a05Prev.ConfirmationID,
		Message:        "feat: trigger pre-commit restage",
	})
	if err != nil {
		t.Fatalf("ExecuteCommit A05: %v", err)
	}
	if a05Res.Outcome != CommitOutcomeNeedsReview || a05Res.CommitOID == "" || a05Res.TreeOID == a05Prev.ExpectedTreeOID {
		t.Fatalf("expected needs_review when pre-commit changed tree, got %+v", a05Res)
	}
	_ = os.Remove(preCommitPath)

	// 5. post-commit hook amends commit -> Outcome == needs_review!
	postCommitPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	postCommitScript := "#!/bin/sh\nif [ -z \"$WEBMUX_AMENDED\" ]; then\n  WEBMUX_AMENDED=1 git commit --amend --no-verify -m 'amended by post-commit'\nfi\n"
	if err := os.WriteFile(postCommitPath, []byte(postCommitScript), 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(dir, "post.txt"), "post hook test\n")
	gitTest(t, dir, "add", "post.txt")
	postPrev, err := s.PreviewCommit(ctx, repo.ID, wtID)
	if err != nil || postPrev.ConfirmationID == "" {
		t.Fatalf("PreviewCommit post-commit: %+v %v", postPrev, err)
	}
	postRes, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: postPrev.ConfirmationID,
		Message:        "feat: before post-commit amend",
	})
	_ = os.Remove(postCommitPath)
	if err != nil || postRes.Outcome != CommitOutcomeNeedsReview {
		t.Fatalf("expected needs_review on post-commit amend, got %+v %v", postRes, err)
	}

	// 6. A06: External CLI immediately commits C2 right after our git commit creates C1!
	// ExecuteCommit MUST attribute and return C1 (matched), NEVER external C2!
	putFile(t, filepath.Join(dir, "c1.txt"), "our commit C1\n")
	gitTest(t, dir, "add", "c1.txt")
	a06Prev, err := s.PreviewCommit(ctx, repo.ID, wtID)
	if err != nil || a06Prev.ConfirmationID == "" {
		t.Fatalf("PreviewCommit A06: %+v %v", a06Prev, err)
	}

	var externalC2OID string
	raceCtx := context.WithValue(ctx, afterCommitHookKey{}, func(worktreePath string) {
		putFile(t, filepath.Join(worktreePath, "c2.txt"), "concurrent CLI C2\n")
		gitTest(t, worktreePath, "add", "c2.txt")
		gitTestEnv(t, worktreePath, []string{"GIT_REFLOG_ACTION=external-cli"}, "commit", "-m", "external CLI C2")
		externalC2OID = strings.TrimSpace(gitTest(t, worktreePath, "rev-parse", "HEAD"))
	})

	a06Res, err := s.ExecuteCommit(raceCtx, ExecuteCommitRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: a06Prev.ConfirmationID,
		Message:        "feat: our commit C1",
	})
	if err != nil {
		t.Fatalf("ExecuteCommit A06: %v", err)
	}
	if externalC2OID == "" || a06Res.CommitOID == externalC2OID {
		t.Fatalf("ExecuteCommit misattributed external C2 (%s) as its own commit: %+v", externalC2OID, a06Res)
	}
	if a06Res.Outcome != CommitOutcomeMatched || a06Res.Subject != "feat: our commit C1" {
		t.Fatalf("expected matched C1 outcome despite subsequent external C2, got %+v", a06Res)
	}
	// Confirm C1 is indeed C2's parent on disk!
	c2Parent := strings.TrimSpace(gitTest(t, dir, "rev-parse", externalC2OID+"^"))
	if c2Parent != a06Res.CommitOID {
		t.Fatalf("expected C2 parent (%s) == C1 (%s)", c2Parent, a06Res.CommitOID)
	}
}
