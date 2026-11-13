package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageAndUnstageSelectedLinesFiveLineReplacementAndCommit(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	filePath := filepath.Join(dir, "block.txt")
	initial := "header\nold-1\nold-2\nold-3\nold-4\nold-5\nfooter\n"
	putFile(t, filePath, initial)
	gitTest(t, dir, "add", "block.txt")
	gitTest(t, dir, "commit", "-m", "add 5-line block")

	// Modify all 5 interior lines to form a contiguous 5 `-` + 5 `+` block.
	edited := "header\nnew-1\nnew-2\nnew-3\nnew-4\nnew-5\nfooter\n"
	putFile(t, filePath, edited)

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	diff, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "block.txt")
	if err != nil {
		t.Fatalf("InspectStagingFile: %v", err)
	}
	if !diff.HunkStageAvailable || len(diff.Hunks) != 1 {
		t.Fatalf("expected 1 stageable hunk, got %+v", diff)
	}
	hunk := diff.Hunks[0]

	// Locate indices for old-2/new-2 and old-4/new-4 inside hunk.Lines.
	var selIndices []int
	for idx, l := range hunk.Lines {
		switch l.Text {
		case "old-2", "new-2", "old-4", "new-4":
			selIndices = append(selIndices, idx)
		}
	}
	if len(selIndices) != 4 {
		t.Fatalf("expected 4 selected line indices (2 & 4 pairs), got %v in %+v", selIndices, hunk.Lines)
	}

	// Stage ONLY lines 2 & 4!
	if err := s.Stage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     diff.Identity,
		LineSelections: []LineSelection{
			{HunkID: hunk.ID, LineIndices: selIndices},
		},
	}); err != nil {
		t.Fatalf("Stage selected lines 2 & 4: %v", err)
	}

	// Verify .git/index blob has old-1, new-2, old-3, new-4, old-5 in-place!
	indexBlob := gitTest(t, dir, "cat-file", "-p", ":block.txt") + "\n"
	wantIndex := "header\nold-1\nnew-2\nold-3\nnew-4\nold-5\nfooter\n"
	if indexBlob != wantIndex {
		t.Fatalf("index blob mismatch:\ngot :\n%s\nwant:\n%s", indexBlob, wantIndex)
	}

	// Verify worktree file is 100% untouched byte-for-byte!
	diskRaw, err := os.ReadFile(filePath)
	if err != nil || string(diskRaw) != edited {
		t.Fatalf("worktree file mutated by Stage: got %q want %q (%v)", string(diskRaw), edited, err)
	}

	// Unstage ONLY line 4 (`old-4` / `new-4`) from Staged back to HEAD!
	stagedDiff, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaStaged, "block.txt")
	if err != nil {
		t.Fatalf("InspectStagingFile staged: %v", err)
	}
	var unstageSelections []LineSelection
	for _, sh := range stagedDiff.Hunks {
		var indices []int
		for idx, l := range sh.Lines {
			if l.Text == "old-4" || l.Text == "new-4" {
				indices = append(indices, idx)
			}
		}
		if len(indices) > 0 {
			unstageSelections = append(unstageSelections, LineSelection{
				HunkID:      sh.ID,
				LineIndices: indices,
			})
		}
	}
	if len(unstageSelections) == 0 {
		t.Fatalf("did not find line 4 in staged hunks: %+v", stagedDiff.Hunks)
	}
	if err := s.Unstage(ctx, StageRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		Identity:       stagedDiff.Identity,
		LineSelections: unstageSelections,
	}); err != nil {
		t.Fatalf("Unstage line 4: %v", err)
	}

	wantIndexAfterUnstage := "header\nold-1\nnew-2\nold-3\nold-4\nold-5\nfooter\n"
	indexAfterUnstage := gitTest(t, dir, "cat-file", "-p", ":block.txt") + "\n"
	if indexAfterUnstage != wantIndexAfterUnstage {
		t.Fatalf("index after partial unstage:\ngot :\n%s\nwant:\n%s", indexAfterUnstage, wantIndexAfterUnstage)
	}
	diskAfterUnstage, err := os.ReadFile(filePath)
	if err != nil || string(diskAfterUnstage) != edited {
		t.Fatalf("worktree mutated by partial Unstage: %q", string(diskAfterUnstage))
	}

	// Commit staged index and verify committed tree matches exact spliced lines!
	preview, err := s.PreviewCommit(ctx, repo.ID, wtID)
	if err != nil || preview.ConfirmationID == "" {
		t.Fatalf("PreviewCommit: %+v %v", preview, err)
	}
	commitRes, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: preview.ConfirmationID,
		Message:        "feat: commit only line 2",
	})
	if err != nil {
		t.Fatalf("ExecuteCommit: %v", err)
	}
	headTreeContent := gitTest(t, dir, "cat-file", "-p", commitRes.CommitOID+":block.txt") + "\n"
	if headTreeContent != wantIndexAfterUnstage {
		t.Fatalf("HEAD tree mismatch:\ngot :\n%s\nwant:\n%s", headTreeContent, wantIndexAfterUnstage)
	}
}

func TestStageSelectedLinesCRLFNoTrailingNewlineUnbornAndValidation(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	// 1. CRLF + no trailing newline + merged 1-line gap + unpaired `+` / `-` selection.
	dir := initRepo(t)
	crlfPath := filepath.Join(dir, "win.bat")
	baseCRLF := "rem top\r\ndel-a\r\nkeep-gap\r\ndel-b\r\ntail-no-nl"
	if err := os.WriteFile(crlfPath, []byte(baseCRLF), 0o755); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", "--chmod=+x", "win.bat")
	gitTest(t, dir, "commit", "-m", "add crlf script")

	// Edit del-a -> add-a1 + add-a2, keep-gap unchanged (1-line gap merges into 1 hunk!), del-b deleted, tail-no-nl -> TAIL-EDITED (no trailing newline).
	targetCRLF := "rem top\r\nadd-a1\r\nadd-a2\r\nkeep-gap\r\nTAIL-EDITED"
	if err := os.WriteFile(crlfPath, []byte(targetCRLF), 0o755); err != nil {
		t.Fatal(err)
	}

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	diff, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "win.bat")
	if err != nil || len(diff.Hunks) != 1 {
		t.Fatalf("expected 1 merged CRLF hunk, got %+v %v", diff, err)
	}
	h := diff.Hunks[0]

	// Validation: mixing HunkIDs and LineSelections must return ErrInvalid.
	if err := s.Stage(ctx, StageRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		Identity:       diff.Identity,
		HunkIDs:        []string{h.ID},
		LineSelections: []LineSelection{{HunkID: h.ID, LineIndices: []int{1}}},
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid on mixed HunkIDs + LineSelections, got %v", err)
	}

	// Validation: selecting a context line index must return ErrInvalid.
	ctxIdx := -1
	var pickIndices []int
	for idx, l := range h.Lines {
		if l.Kind == "context" && ctxIdx < 0 {
			ctxIdx = idx
		}
		// Select ONLY `add-a2` (leaving `del-a` kept in index!) and `tail-no-nl` + `TAIL-EDITED` (leaving `del-b` kept in index!).
		if l.Text == "add-a2" || l.Text == "tail-no-nl" || l.Text == "TAIL-EDITED" {
			pickIndices = append(pickIndices, idx)
		}
	}
	if ctxIdx >= 0 {
		if err := s.Stage(ctx, StageRequest{
			RepositoryID:   repo.ID,
			WorktreeID:     wtID,
			Identity:       diff.Identity,
			LineSelections: []LineSelection{{HunkID: h.ID, LineIndices: []int{ctxIdx}}},
		}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid selecting context line, got %v", err)
		}
	}

	// Stage unpaired `add-a2` + `tail-no-nl -> TAIL-EDITED`!
	if err := s.Stage(ctx, StageRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		Identity:       diff.Identity,
		LineSelections: []LineSelection{{HunkID: h.ID, LineIndices: pickIndices}},
	}); err != nil {
		t.Fatalf("Stage CRLF sub-hunk lines: %v", err)
	}

	rawStaged, err := ReadGit(ctx, dir, 1<<20, "cat-file", "blob", ":win.bat")
	if err != nil {
		t.Fatal(err)
	}
	wantCRLF := "rem top\r\ndel-a\r\nadd-a2\r\nkeep-gap\r\ndel-b\r\nTAIL-EDITED"
	if string(rawStaged) != wantCRLF {
		t.Fatalf("CRLF/no-nl staged bytes mismatch:\ngot : %q\nwant: %q", string(rawStaged), wantCRLF)
	}
	lsStage := gitTest(t, dir, "ls-files", "--stage", "--", "win.bat")
	if !strings.HasPrefix(lsStage, "100755 ") {
		t.Fatalf("expected 100755 preserved, got %q", lsStage)
	}

	// Stale snapshot check: external edit on disk must fail with ErrStale.
	diffBeforeEdit, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "win.bat")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(crlfPath, []byte(targetCRLF+"\r\nrem agent edit"), 0o755); err != nil {
		t.Fatal(err)
	}
	var firstChanged int
	for idx, l := range diffBeforeEdit.Hunks[0].Lines {
		if l.Kind != "context" {
			firstChanged = idx
			break
		}
	}
	if err := s.Stage(ctx, StageRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		Identity:       diffBeforeEdit.Identity,
		LineSelections: []LineSelection{{HunkID: diffBeforeEdit.Hunks[0].ID, LineIndices: []int{firstChanged}}},
	}); !errors.Is(err, ErrStale) {
		t.Fatalf("expected ErrStale after external file modification, got %v", err)
	}

	// 2. Unborn HEAD + newly added file: stage 1 line of 2, then unstage that 1 line -> file removed from index!
	unbornDir := filepath.Join(t.TempDir(), "unborn-lines")
	if err := os.MkdirAll(unbornDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, unbornDir, "init", "-b", "main")
	newFile := filepath.Join(unbornDir, "notes.txt")
	putFile(t, newFile, "alpha\nbeta\ngamma\n")

	uRepo := registerTest(t, s, unbornDir)
	uWT := uRepo.Worktrees[0].ID
	uDiff, err := s.InspectStagingFile(ctx, uRepo.ID, uWT, StageAreaUnstaged, "notes.txt")
	if err != nil || len(uDiff.Hunks) != 1 {
		t.Fatalf("unborn InspectStagingFile: %+v %v", uDiff, err)
	}
	// Stage only line index 1 ("beta")
	if err := s.Stage(ctx, StageRequest{
		RepositoryID:   uRepo.ID,
		WorktreeID:     uWT,
		Identity:       uDiff.Identity,
		LineSelections: []LineSelection{{HunkID: uDiff.Hunks[0].ID, LineIndices: []int{1}}},
	}); err != nil {
		t.Fatalf("Stage 1 line of untracked file on unborn HEAD: %v", err)
	}
	if got := gitTest(t, unbornDir, "cat-file", "-p", ":notes.txt"); got != "beta" {
		t.Fatalf("expected only 'beta' staged in index, got %q", got)
	}
	// Unstage that 1 remaining staged line -> file must be removed from index while worktree keeps all 3 lines!
	uStaged, err := s.InspectStagingFile(ctx, uRepo.ID, uWT, StageAreaStaged, "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Unstage(ctx, StageRequest{
		RepositoryID:   uRepo.ID,
		WorktreeID:     uWT,
		Identity:       uStaged.Identity,
		LineSelections: []LineSelection{{HunkID: uStaged.Hunks[0].ID, LineIndices: []int{0}}},
	}); err != nil {
		t.Fatalf("Unstage last line of newly added file: %v", err)
	}
	if ls := gitTest(t, unbornDir, "ls-files", "--", "notes.txt"); ls != "" {
		t.Fatalf("expected notes.txt removed from index after unstaging all lines, got %q", ls)
	}
	if disk, _ := os.ReadFile(newFile); string(disk) != "alpha\nbeta\ngamma\n" {
		t.Fatalf("worktree file altered: %q", string(disk))
	}
}
