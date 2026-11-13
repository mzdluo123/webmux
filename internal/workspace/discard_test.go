package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscardFilesHunksLinesFreshnessAndRecoverySnapshot(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	aPath := filepath.Join(dir, "a.txt")
	bPath := filepath.Join(dir, "sub", "b.txt")
	delPath := filepath.Join(dir, "remove-me.txt")
	putFile(t, aPath, "h1\nold-1\nold-2\nold-3\nold-4\nold-5\nmid1\nmid2\nmid3\nmid4\nbot-old\n")
	putFile(t, bPath, "base-b\n")
	putFile(t, delPath, "restore-content\n")
	gitTest(t, dir, "add", "a.txt", "sub/b.txt", "remove-me.txt")
	gitTest(t, dir, "commit", "-m", "initial files for discard test")

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	// Stage a change on sub/b.txt FIRST, then make an unstaged edit on sub/b.txt too:
	// Discard Changes MUST restore worktree to the STAGED index version, never touching Staged!
	putFile(t, bPath, "staged-b\n")
	gitTest(t, dir, "add", "sub/b.txt")
	putFile(t, bPath, "unstaged-b-dirty\n")
	if err := os.Remove(delPath); err != nil {
		t.Fatal(err)
	}
	untrackedPath := filepath.Join(dir, "sub", "scratch.txt")
	putFile(t, untrackedPath, "precious untracked work\n")

	// 1. Freshness Guard: CLI Agent modifies sub/b.txt AFTER PreviewDiscard -> MUST fail with ErrStale and preserve agent's edit!
	stalePrev, err := s.PreviewDiscard(ctx, DiscardPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        DiscardScopeFiles,
		Paths:        []string{"sub/b.txt", "sub/scratch.txt"},
	})
	if err != nil || stalePrev.ConfirmationID == "" {
		t.Fatalf("PreviewDiscard: %+v %v", stalePrev, err)
	}
	putFile(t, bPath, "cli-agent-concurrent-write\n")
	if _, err := s.ExecuteDiscard(ctx, ExecuteDiscardRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: stalePrev.ConfirmationID,
	}); !errors.Is(err, ErrStale) {
		t.Fatalf("expected ErrStale after post-preview edit, got %v", err)
	}
	if got, _ := os.ReadFile(bPath); string(got) != "cli-agent-concurrent-write\n" {
		t.Fatalf("post-preview agent write was clobbered! got %q", string(got))
	}
	if _, err := os.Stat(untrackedPath); err != nil {
		t.Fatalf("untracked file deleted despite stale batch check: %v", err)
	}

	// 2. Multi-file Discard (modified tracked + deleted tracked + untracked) + Recovery OID verification!
	batchPrev, err := s.PreviewDiscard(ctx, DiscardPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        DiscardScopeFiles,
		Paths:        []string{"sub/b.txt", "remove-me.txt", "sub/scratch.txt"},
	})
	if err != nil || batchPrev.ConfirmationID == "" {
		t.Fatalf("PreviewDiscard batch: %+v %v", batchPrev, err)
	}
	if len(batchPrev.TrackedOverwritePaths) != 2 || len(batchPrev.UntrackedDeletePaths) != 1 {
		t.Fatalf("unexpected preview breakdown: %+v", batchPrev)
	}

	batchRes, err := s.ExecuteDiscard(ctx, ExecuteDiscardRequest{
		RepositoryID:    repo.ID,
		WorktreeID:      wtID,
		ConfirmationID:  batchPrev.ConfirmationID,
		ClientRequestID: "discard-batch-1",
	})
	if err != nil {
		t.Fatalf("ExecuteDiscard batch: %v", err)
	}
	if batchRes.RecoveryOID == "" || !strings.Contains(batchRes.RecoveryCommand, batchRes.RecoveryOID) {
		t.Fatalf("missing recovery OID or command: %+v", batchRes)
	}
	// Verify sub/b.txt reverted to STAGED ("staged-b\n"), remove-me.txt restored, scratch.txt deleted, and Staged untouched!
	if got, _ := os.ReadFile(bPath); string(got) != "staged-b\n" {
		t.Fatalf("sub/b.txt should revert to staged content, got %q", string(got))
	}
	if idx := gitTest(t, dir, "cat-file", "-p", ":sub/b.txt"); idx != "staged-b" {
		t.Fatalf("staged index mutated by Discard: %q", idx)
	}
	if got, _ := os.ReadFile(delPath); string(got) != "restore-content\n" {
		t.Fatalf("deleted file not restored: %q", string(got))
	}
	if _, err := os.Stat(untrackedPath); !os.IsNotExist(err) {
		t.Fatalf("expected untracked file deleted, stat err=%v", err)
	}

	// Verify git-operations.json recorded RecoveryOID + RecoveryCommand!
	op, err := s.GetGitOperation(repo.ID, batchRes.OperationID)
	if err != nil || op.Kind != GitOpDiscard || op.RecoveryOID != batchRes.RecoveryOID {
		t.Fatalf("GetGitOperation discard: %+v %v", op, err)
	}

	// Prove recovery safety net works: restore BOTH overwritten sub/b.txt AND deleted untracked sub/scratch.txt from RecoveryOID!
	cmd := exec.Command("git", "restore", "--source="+batchRes.RecoveryOID, "--worktree", "--", "sub/b.txt", "sub/scratch.txt")
	cmd.Dir = dir
	if out, rErr := cmd.CombinedOutput(); rErr != nil {
		t.Fatalf("git restore from RecoveryOID failed: %v (%s)", rErr, string(out))
	}
	if got, _ := os.ReadFile(bPath); string(got) != "cli-agent-concurrent-write\n" {
		t.Fatalf("recovery failed for sub/b.txt: got %q", string(got))
	}
	if got, _ := os.ReadFile(untrackedPath); string(got) != "precious untracked work\n" {
		t.Fatalf("recovery failed for deleted untracked file: got %q", string(got))
	}

	// 3. Single-Hunk Discard and Sub-Hunk Selected-Lines Discard on a.txt!
	editedA := "h1\nnew-1\nnew-2\nnew-3\nnew-4\nnew-5\nmid1\nmid2\nmid3\nmid4\nbot-new\n"
	putFile(t, aPath, editedA)
	aDiff, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "a.txt")
	if err != nil || len(aDiff.Hunks) != 2 {
		t.Fatalf("expected 2 hunks on a.txt, got %+v %v", aDiff, err)
	}

	// Discard ONLY hunk 1 (bottom hunk: bot-new -> bot-old)
	hunkPrev, err := s.PreviewDiscard(ctx, DiscardPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        DiscardScopeHunks,
		Identity:     aDiff.Identity,
		HunkIDs:      []string{aDiff.Hunks[1].ID},
	})
	if err != nil || hunkPrev.ConfirmationID == "" {
		t.Fatalf("PreviewDiscard hunk: %+v %v", hunkPrev, err)
	}
	if _, err := s.ExecuteDiscard(ctx, ExecuteDiscardRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: hunkPrev.ConfirmationID,
	}); err != nil {
		t.Fatalf("ExecuteDiscard hunk: %v", err)
	}
	if got, _ := os.ReadFile(aPath); string(got) != "h1\nnew-1\nnew-2\nnew-3\nnew-4\nnew-5\nmid1\nmid2\nmid3\nmid4\nbot-old\n" {
		t.Fatalf("unexpected a.txt after single-hunk discard:\n%s", string(got))
	}

	// Now discard ONLY paired lines 2 & 4 (`old-2/new-2` and `old-4/new-4`) from hunk 0!
	aDiff2, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "a.txt")
	if err != nil || len(aDiff2.Hunks) != 1 {
		t.Fatalf("expected 1 hunk remaining on a.txt, got %+v %v", aDiff2, err)
	}
	var lineIndices []int
	for idx, l := range aDiff2.Hunks[0].Lines {
		if l.Text == "old-2" || l.Text == "new-2" || l.Text == "old-4" || l.Text == "new-4" {
			lineIndices = append(lineIndices, idx)
		}
	}
	linesPrev, err := s.PreviewDiscard(ctx, DiscardPreviewRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		Scope:          DiscardScopeLines,
		Identity:       aDiff2.Identity,
		LineSelections: []LineSelection{{HunkID: aDiff2.Hunks[0].ID, LineIndices: lineIndices}},
	})
	if err != nil || linesPrev.ConfirmationID == "" {
		t.Fatalf("PreviewDiscard lines: %+v %v", linesPrev, err)
	}
	if _, err := s.ExecuteDiscard(ctx, ExecuteDiscardRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: linesPrev.ConfirmationID,
	}); err != nil {
		t.Fatalf("ExecuteDiscard lines: %v", err)
	}
	wantA := "h1\nnew-1\nold-2\nnew-3\nold-4\nnew-5\nmid1\nmid2\nmid3\nmid4\nbot-old\n"
	if got, _ := os.ReadFile(aPath); string(got) != wantA {
		t.Fatalf("sub-hunk line discard mismatch:\ngot :\n%s\nwant:\n%s", string(got), wantA)
	}

	// 4. Active Session Guard: requires AcknowledgeSessions when a session is running!
	sess, err := s.CreateSession(ctx, CreateInput{Cwd: dir, Cmd: []string{"/bin/sh", "-c", "printf READY; read v"}})
	if err != nil {
		t.Fatal(err)
	}
	waitOutput(t, sess, "READY")
	sessPrev, err := s.PreviewDiscard(ctx, DiscardPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        DiscardScopeFiles,
		Paths:        []string{"a.txt"},
	})
	if err != nil || len(sessPrev.Sessions) != 1 {
		t.Fatalf("expected 1 active session in discard preview, got %+v %v", sessPrev, err)
	}
	if _, err := s.ExecuteDiscard(ctx, ExecuteDiscardRequest{
		RepositoryID:        repo.ID,
		WorktreeID:          wtID,
		ConfirmationID:      sessPrev.ConfirmationID,
		AcknowledgeSessions: false,
	}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked without AcknowledgeSessions, got %v", err)
	}
}
