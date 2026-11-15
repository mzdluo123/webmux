package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStageDualHunksPreservesCRLFNoNewlineExecModeAndWorktreeIsolation(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	scriptPath := filepath.Join(dir, "run.sh")
	// Executable 100755 script with CRLF line endings and NO trailing newline on the last line!
	initialContent := "line1\r\nkeep2\r\nkeep3\r\nkeep4\r\nkeep5\r\nlast-no-nl"
	if err := os.WriteFile(scriptPath, []byte(initialContent), 0o755); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", "--chmod=+x", "run.sh")
	gitTest(t, dir, "commit", "-m", "add executable crlf script")

	// Modify top hunk and bottom hunk while leaving keep2..keep5 untouched.
	editedContent := "LINE1-EDITED\r\nkeep2\r\nkeep3\r\nkeep4\r\nkeep5\r\nLAST-EDITED-NO-NL"
	if err := os.WriteFile(scriptPath, []byte(editedContent), 0o755); err != nil {
		t.Fatal(err)
	}

	// Add a linked worktree B to verify per-worktree write gate isolation!
	wtBPath := filepath.Join(t.TempDir(), "wt-b")
	gitTest(t, dir, "worktree", "add", "-b", "feature-b", wtBPath, "HEAD")
	putFile(t, filepath.Join(wtBPath, "tracked"), "worktree B edit\n")

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	mainWT, err := findTree(repo, worktreeID(repo.ID, dir))
	if err != nil {
		t.Fatal(err)
	}
	linkedWT, err := findTree(repo, worktreeID(repo.ID, wtBPath))
	if err != nil {
		t.Fatal(err)
	}

	// Hold mainWT's write gate manually and verify linkedWT staging still completes immediately!
	mainLoc, err := s.LocateWorktree(ctx, repo.ID, mainWT.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.acquireWorktreeWrite(ctx, mainLoc.Path); err != nil {
		t.Fatal(err)
	}
	bCtx, bCancel := context.WithTimeout(ctx, 2*time.Second)
	bDiff, bErr := s.InspectStagingFile(bCtx, repo.ID, linkedWT.ID, StageAreaUnstaged, "tracked")
	if bErr == nil {
		bErr = s.Stage(bCtx, StageRequest{
			RepositoryID: repo.ID,
			WorktreeID:   linkedWT.ID,
			Identity:     bDiff.Identity,
		})
	}
	bCancel()
	s.releaseWorktreeWrite(mainLoc.Path)
	if bErr != nil {
		t.Fatalf("linked worktree B blocked by main worktree write gate: %v", bErr)
	}

	// Inspect run.sh in mainWT: expect 2 hunks, CRLF + none line endings.
	fileDiff, err := s.InspectStagingFile(ctx, repo.ID, mainWT.ID, StageAreaUnstaged, "run.sh")
	if err != nil {
		t.Fatalf("InspectStagingFile run.sh: %v", err)
	}
	if !fileDiff.HunkStageAvailable || len(fileDiff.Hunks) != 2 {
		t.Fatalf("expected 2 stageable hunks, got available=%v len=%d (%+v)", fileDiff.HunkStageAvailable, len(fileDiff.Hunks), fileDiff)
	}

	// Stage ONLY the first hunk!
	if err := s.Stage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   mainWT.ID,
		Identity:     fileDiff.Identity,
		HunkIDs:      []string{fileDiff.Hunks[0].ID},
	}); err != nil {
		t.Fatalf("Stage hunk 0: %v", err)
	}

	// Verify real Git index has ONLY hunk 0, keeps 100755, CRLF, and no trailing newline!
	lsStage := gitTest(t, dir, "ls-files", "--stage", "--", "run.sh")
	if !strings.HasPrefix(lsStage, "100755 ") {
		t.Fatalf("expected 100755 mode preserved in index, got %q", lsStage)
	}
	stagedBlob := gitTest(t, dir, "cat-file", "blob", ":run.sh")
	// Note: gitTest strips a single trailing '\n', and our file has NO trailing '\n'!
	expectedIndexContent := "LINE1-EDITED\r\nkeep2\r\nkeep3\r\nkeep4\r\nkeep5\r\nlast-no-nl"
	if stagedBlob != expectedIndexContent {
		t.Fatalf("unexpected staged blob bytes:\ngot : %q\nwant: %q", stagedBlob, expectedIndexContent)
	}

	// Worktree file on disk MUST still contain BOTH edits untouched!
	diskBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(diskBytes) != editedContent {
		t.Fatalf("worktree file modified unexpectedly: %q", string(diskBytes))
	}

	// InspectStaging MUST list run.sh in BOTH Staged and Unstaged!
	overview, err := s.InspectStaging(ctx, repo.ID, mainWT.ID)
	if err != nil {
		t.Fatalf("InspectStaging: %v", err)
	}
	hasStaged, hasUnstaged := false, false
	for _, e := range overview.Staged {
		if e.Path == "run.sh" {
			hasStaged = true
		}
	}
	for _, e := range overview.Unstaged {
		if e.Path == "run.sh" {
			hasUnstaged = true
		}
	}
	if !hasStaged || !hasUnstaged {
		t.Fatalf("expected run.sh in both Staged (%v) and Unstaged (%v): %+v", hasStaged, hasUnstaged, overview)
	}

	// Unstage hunk 0 back to HEAD via hunk-level Unstage, keeping worktree intact!
	stagedDiff, err := s.InspectStagingFile(ctx, repo.ID, mainWT.ID, StageAreaStaged, "run.sh")
	if err != nil {
		t.Fatalf("InspectStagingFile staged: %v", err)
	}
	if len(stagedDiff.Hunks) != 1 {
		t.Fatalf("expected 1 staged hunk, got %d", len(stagedDiff.Hunks))
	}
	if err := s.Unstage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   mainWT.ID,
		Identity:     stagedDiff.Identity,
		HunkIDs:      []string{stagedDiff.Hunks[0].ID},
	}); err != nil {
		t.Fatalf("Unstage hunk 0: %v", err)
	}
	if cached := gitTest(t, dir, "diff", "--cached", "--name-only"); cached != "" {
		t.Fatalf("expected clean staged diff after unstaging hunk 0, got %q", cached)
	}
	diskAfterUnstage, err := os.ReadFile(scriptPath)
	if err != nil || string(diskAfterUnstage) != editedContent {
		t.Fatalf("worktree bytes altered by Unstage: %q %v", string(diskAfterUnstage), err)
	}
}

func TestStageUnbornHeadStaleSnapshotAndSpecialDegradation(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	// 1. Unborn HEAD repository: initial hunk stage + unstage without `git reset HEAD` failure!
	unbornDir := filepath.Join(t.TempDir(), "unborn-stage")
	if err := os.MkdirAll(unbornDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, unbornDir, "init", "-b", "main")
	putFile(t, filepath.Join(unbornDir, "hello.txt"), "first\nsecond\n")

	s, _ := testService(t, t.TempDir(), unbornDir)
	unbornRepo := registerTest(t, s, unbornDir)
	unbornWTID := unbornRepo.Worktrees[0].ID

	unbornDiff, err := s.InspectStagingFile(ctx, unbornRepo.ID, unbornWTID, StageAreaUnstaged, "hello.txt")
	if err != nil || !unbornDiff.HunkStageAvailable || len(unbornDiff.Hunks) != 1 {
		t.Fatalf("unborn InspectStagingFile: %+v %v", unbornDiff, err)
	}
	if err := s.Stage(ctx, StageRequest{
		RepositoryID: unbornRepo.ID,
		WorktreeID:   unbornWTID,
		Identity:     unbornDiff.Identity,
		HunkIDs:      []string{unbornDiff.Hunks[0].ID},
	}); err != nil {
		t.Fatalf("unborn Stage: %v", err)
	}
	if ls := gitTest(t, unbornDir, "ls-files"); ls != "hello.txt" {
		t.Fatalf("expected hello.txt staged in unborn repo, got %q", ls)
	}

	unbornStaged, err := s.InspectStagingFile(ctx, unbornRepo.ID, unbornWTID, StageAreaStaged, "hello.txt")
	if err != nil {
		t.Fatalf("unborn staged inspect: %v", err)
	}
	if err := s.Unstage(ctx, StageRequest{
		RepositoryID: unbornRepo.ID,
		WorktreeID:   unbornWTID,
		Identity:     unbornStaged.Identity,
	}); err != nil {
		t.Fatalf("unborn whole-file Unstage: %v", err)
	}
	if ls := gitTest(t, unbornDir, "ls-files"); ls != "" {
		t.Fatalf("expected empty index after unborn Unstage, got %q", ls)
	}
	if data, err := os.ReadFile(filepath.Join(unbornDir, "hello.txt")); err != nil || string(data) != "first\nsecond\n" {
		t.Fatalf("unborn Unstage touched worktree file: %q %v", string(data), err)
	}

	// 2. External modification invalidates stale snapshot; binary / rename / filter / conflict degradation.
	dir := initRepo(t)
	putFile(t, filepath.Join(dir, ".gitattributes"), "filtered.txt filter=lfs\n")
	putFile(t, filepath.Join(dir, "filtered.txt"), "v1\n")
	putFile(t, filepath.Join(dir, "rename-me.txt"), "same content\n")
	if err := os.WriteFile(filepath.Join(dir, "img.png"), []byte{0x89, 'P', 'N', 'G', 0x00, 0x01}, 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-m", "setup fixtures")

	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	// Stale snapshot rejection when background agent modifies tracked file!
	putFile(t, filepath.Join(dir, "tracked"), "user edit\n")
	staleSnap, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "tracked")
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(dir, "tracked"), "agent concurrent edit\n")
	err = s.Stage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     staleSnap.Identity,
		HunkIDs:      []string{staleSnap.Hunks[0].ID},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on stale snapshot, got %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "tracked")); string(got) != "agent concurrent edit\n" {
		t.Fatalf("agent edit was overwritten: %q", string(got))
	}

	// Binary file & filter=lfs file degrade to whole-file only!
	if err := os.WriteFile(filepath.Join(dir, "img.png"), []byte{0x89, 'P', 'N', 'G', 0x00, 0x02}, 0o600); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(dir, "filtered.txt"), "v2\n")

	binDiff, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "img.png")
	if err != nil || binDiff.HunkStageAvailable || !binDiff.Binary || binDiff.DegradeReason == "" {
		t.Fatalf("expected binary degradation, got %+v %v", binDiff, err)
	}
	filterDiff, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "filtered.txt")
	if err != nil || filterDiff.HunkStageAvailable || filterDiff.DegradeReason == "" {
		t.Fatalf("expected filter degradation, got %+v %v", filterDiff, err)
	}
	// Whole-file Stage still works for both degraded files!
	if err := s.Stage(ctx, StageRequest{RepositoryID: repo.ID, WorktreeID: wtID, Identity: binDiff.Identity}); err != nil {
		t.Fatalf("whole-file Stage binary: %v", err)
	}
	if err := s.Stage(ctx, StageRequest{RepositoryID: repo.ID, WorktreeID: wtID, Identity: filterDiff.Identity}); err != nil {
		t.Fatalf("whole-file Stage filtered: %v", err)
	}

	// Staged rename degrades to whole-file and whole-file Unstage restores both paths!
	gitTest(t, dir, "mv", "rename-me.txt", "renamed.txt")
	ov, err := s.InspectStaging(ctx, repo.ID, wtID)
	if err != nil {
		t.Fatal(err)
	}
	var renameEntry *StagingEntry
	for i := range ov.Staged {
		if ov.Staged[i].Path == "renamed.txt" {
			renameEntry = &ov.Staged[i]
			break
		}
	}
	if renameEntry == nil || renameEntry.HunkStageAvailable || renameEntry.OldPath != "rename-me.txt" {
		t.Fatalf("expected staged rename degraded to whole-file, got %+v", renameEntry)
	}
	if err := s.Unstage(ctx, StageRequest{RepositoryID: repo.ID, WorktreeID: wtID, Identity: renameEntry.Identity}); err != nil {
		t.Fatalf("whole-file Unstage rename: %v", err)
	}

	// Unmerged conflict entry (stage 1..3) rejects ordinary Stage!
	gitTest(t, dir, "checkout", "-f", "main")
	gitTest(t, dir, "checkout", "-b", "conflict-side")
	putFile(t, filepath.Join(dir, "tracked"), "side branch\n")
	gitTest(t, dir, "add", "tracked")
	gitTest(t, dir, "commit", "-m", "side")
	gitTest(t, dir, "checkout", "main")
	putFile(t, filepath.Join(dir, "tracked"), "main branch\n")
	gitTest(t, dir, "add", "tracked")
	gitTest(t, dir, "commit", "-m", "main")
	// Trigger merge conflict (expected non-zero exit).
	_, _, _ = s.gitInputOutput(ctx, dir, nil, nil, "merge", "conflict-side")

	conflictDiff, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "tracked")
	if err != nil || !conflictDiff.Conflicted || conflictDiff.HunkStageAvailable {
		t.Fatalf("expected conflicted file diff, got %+v %v", conflictDiff, err)
	}
	if err := s.Stage(ctx, StageRequest{RepositoryID: repo.ID, WorktreeID: wtID, Identity: conflictDiff.Identity}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict when staging unmerged conflict path, got %v", err)
	}
}
