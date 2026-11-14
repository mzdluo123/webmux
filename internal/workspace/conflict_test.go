package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeConflictThreeWaySaveResolveAndContinue(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	// Prepare base file with non-conflicting top/bottom and a conflicting middle line,
	// separated by unchanged context lines so 3-way regions are distinct.
	putFile(t, filepath.Join(dir, "app.txt"), "top\nctx-1\nmid-base\nctx-2\nbottom\n")
	gitTest(t, dir, "add", "app.txt")
	gitTest(t, dir, "commit", "-m", "feat: base app.txt")
	baseOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// Create incoming branch editing mid-base and bottom.
	gitTest(t, dir, "checkout", "-b", "incoming", baseOID)
	putFile(t, filepath.Join(dir, "app.txt"), "top\nctx-1\nmid-incoming\nctx-2\nbottom-incoming\n")
	gitTest(t, dir, "add", "app.txt")
	gitTest(t, dir, "commit", "-m", "feat: incoming edits")
	incomingOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// Return to main and edit top (non-conflicting) + mid-base (conflicting).
	gitTest(t, dir, "checkout", "main")
	putFile(t, filepath.Join(dir, "app.txt"), "top-main\nctx-1\nmid-main\nctx-2\nbottom\n")
	gitTest(t, dir, "add", "app.txt")
	gitTest(t, dir, "commit", "-m", "feat: main edits")
	mainStartOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// 1. PreviewMerge checks explicit direction and blast radius.
	prev, err := s.PreviewMerge(ctx, DivergencePreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		RefKind:      "local",
		RefName:      "incoming",
	})
	if err != nil {
		t.Fatalf("PreviewMerge: %v", err)
	}
	if prev.ConfirmationID == "" || len(prev.Blockers) != 0 || prev.Commits.Total != 1 {
		t.Fatalf("expected unblocked merge preview with 1 commit, got %+v", prev)
	}
	if !strings.Contains(prev.DirectionSummary, "incoming") || !strings.Contains(prev.DirectionSummary, "main") {
		t.Fatalf("expected explicit direction summary, got %q", prev.DirectionSummary)
	}

	// 2. ExecuteMerge halts cleanly on conflict with outcome="conflicted".
	res, err := s.ExecuteMerge(ctx, ExecuteDivergenceRequest{
		RepositoryID:    repo.ID,
		WorktreeID:      wtID,
		ConfirmationID:  prev.ConfirmationID,
		ClientRequestID: "merge-req-1",
	})
	if err != nil {
		t.Fatalf("ExecuteMerge: %v", err)
	}
	if res.Outcome != DivergenceOutcomeConflicted || !res.InProgress.Active || res.InProgress.Kind != "merge" || res.InProgress.ConflictedCount != 1 {
		t.Fatalf("expected conflicted merge state, got %+v", res)
	}

	// Idempotent replay returns same OperationID and outcome.
	replayed, err := s.ExecuteMerge(ctx, ExecuteDivergenceRequest{
		RepositoryID:    repo.ID,
		WorktreeID:      wtID,
		ConfirmationID:  prev.ConfirmationID,
		ClientRequestID: "merge-req-1",
	})
	if err != nil || replayed.OperationID != res.OperationID || replayed.Outcome != DivergenceOutcomeConflicted {
		t.Fatalf("expected idempotent replay, got %+v err=%v", replayed, err)
	}

	// 3. U14: ContinueOperation MUST be rejected while unmerged entries remain!
	if _, err := s.ContinueOperation(ctx, ContinueOperationRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Kind:         "merge",
	}); !errors.Is(err, ErrConflicted) {
		t.Fatalf("expected ErrConflicted when Continue called with unresolved entries, got %v", err)
	}

	// 4. InspectConflictFile returns 3-way segments with Left=main(stage 2), Right=incoming(stage 3).
	detail, err := s.InspectConflictFile(ctx, repo.ID, wtID, "app.txt")
	if err != nil {
		t.Fatalf("InspectConflictFile: %v", err)
	}
	if !detail.TextMergeable || detail.Kind != ConflictKindText {
		t.Fatalf("expected text mergeable conflict, got %+v", detail)
	}
	if detail.Left.Stage != 2 || detail.Right.Stage != 3 || detail.Left.CommitOID != mainStartOID || detail.Right.CommitOID != incomingOID {
		t.Fatalf("unexpected merge side mapping: left=%+v right=%+v", detail.Left, detail.Right)
	}
	if !detail.DiskHasConflictMarker {
		t.Fatalf("expected conflict markers detected on disk before SaveConflictResult")
	}

	// Verify 3-way segments contain change_left, conflict, and change_right.
	var sawLeftChange, sawConflict, sawRightChange bool
	for _, seg := range detail.Segments {
		switch seg.Kind {
		case SegmentChangeLeft:
			sawLeftChange = true
		case SegmentConflict:
			sawConflict = true
		case SegmentChangeRight:
			sawRightChange = true
		}
	}
	if !sawLeftChange || !sawConflict || !sawRightChange {
		t.Fatalf("expected change_left + conflict + change_right segments, got %+v", detail.Segments)
	}

	// 5. SaveConflictResult writes draft to disk WITHOUT clearing unmerged index stages!
	mergedText := "top-main\nctx-1\nmid-main\nmid-incoming\nctx-2\nbottom-incoming\n"
	saved, err := s.SaveConflictResult(ctx, SaveConflictResultRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     detail.Identity,
		ResultText:   mergedText,
	})
	if err != nil {
		t.Fatalf("SaveConflictResult: %v", err)
	}
	stillInProg, err := s.InspectInProgressOperation(ctx, repo.ID, wtID)
	if err != nil || stillInProg.ConflictedCount != 1 || stillInProg.CanContinue {
		t.Fatalf("SaveConflictResult must NOT resolve index entry; got %+v err=%v", stillInProg, err)
	}

	// External disk tamper must reject stale SaveConflictResult / ResolveConflict!
	putFile(t, filepath.Join(dir, "app.txt"), "tampered externally\n")
	if _, err := s.ResolveConflict(ctx, ResolveConflictRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     saved.Identity,
		Resolution:   ResolutionText,
		ResultText:   mergedText,
	}); !errors.Is(err, ErrStale) {
		t.Fatalf("expected ErrStale after external disk tamper, got %v", err)
	}

	// Re-inspect after tamper, save, and Mark as Resolved!
	refreshed, err := s.InspectConflictFile(ctx, repo.ID, wtID, "app.txt")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := s.ResolveConflict(ctx, ResolveConflictRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     refreshed.Identity,
		Resolution:   ResolutionText,
		ResultText:   mergedText,
	})
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	if resolved.InProgress.ConflictedCount != 0 || !resolved.InProgress.CanContinue {
		t.Fatalf("expected 0 remaining conflicts and CanContinue=true, got %+v", resolved.InProgress)
	}

	// 6. ContinueOperation completes the merge commit!
	contRes, err := s.ContinueOperation(ctx, ContinueOperationRequest{
		RepositoryID:    repo.ID,
		WorktreeID:      wtID,
		Kind:            "merge",
		ClientRequestID: "continue-merge-1",
	})
	if err != nil {
		t.Fatalf("ContinueOperation: %v", err)
	}
	if contRes.Outcome != DivergenceOutcomeCompleted || contRes.InProgress.Active {
		t.Fatalf("expected merge completed, got %+v", contRes)
	}
	diskBytes, err := os.ReadFile(filepath.Join(dir, "app.txt"))
	if err != nil || string(diskBytes) != mergedText {
		t.Fatalf("final disk app.txt = %q, want %q (err=%v)", diskBytes, mergedText, err)
	}
}

func TestRebaseLeftRightNonInversionMultiStepAndAbort(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	putFile(t, filepath.Join(dir, "cfg.txt"), "v0\n")
	gitTest(t, dir, "add", "cfg.txt")
	gitTest(t, dir, "commit", "-m", "feat: v0 cfg")
	baseOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// Advance main (`onto` target for rebase).
	putFile(t, filepath.Join(dir, "cfg.txt"), "v-main-target\n")
	gitTest(t, dir, "add", "cfg.txt")
	gitTest(t, dir, "commit", "-m", "feat: main target")
	mainTargetOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// Checkout feature branch from baseOID and make a conflicting commit + binary conflict file.
	gitTest(t, dir, "checkout", "-b", "feat/rebase-me", baseOID)
	putFile(t, filepath.Join(dir, "cfg.txt"), "v-feature-branch\n")
	gitTest(t, dir, "add", "cfg.txt")
	gitTest(t, dir, "commit", "-m", "feat: feature change")
	featureStartOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// PreviewRebase onto main.
	prev, err := s.PreviewRebase(ctx, DivergencePreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		RefKind:      "local",
		RefName:      "main",
	})
	if err != nil {
		t.Fatalf("PreviewRebase: %v", err)
	}
	if prev.ConfirmationID == "" || prev.Commits.Total != 1 {
		t.Fatalf("expected unblocked rebase preview, got %+v", prev)
	}

	res, err := s.ExecuteRebase(ctx, ExecuteDivergenceRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: prev.ConfirmationID,
	})
	if err != nil {
		t.Fatalf("ExecuteRebase: %v", err)
	}
	if res.Outcome != DivergenceOutcomeConflicted || res.InProgress.Kind != "rebase" || res.InProgress.StepCurrent != 1 || res.InProgress.StepTotal != 1 {
		t.Fatalf("expected rebase conflict at step 1/1, got %+v", res)
	}

	// CRITICAL git4idea rule: During rebase, Left MUST be feat/rebase-me (stage 3)
	// and Right MUST be main target base (stage 2), NEVER inverted!
	detail, err := s.InspectConflictFile(ctx, repo.ID, wtID, "cfg.txt")
	if err != nil {
		t.Fatalf("InspectConflictFile rebase: %v", err)
	}
	if detail.Left.Stage != 3 || detail.Right.Stage != 2 {
		t.Fatalf("expected Left.Stage=3 and Right.Stage=2 during rebase, got left=%d right=%d", detail.Left.Stage, detail.Right.Stage)
	}
	if detail.Left.Text != "v-feature-branch\n" || detail.Right.Text != "v-main-target\n" {
		t.Fatalf("rebase columns inverted! Left.Text=%q Right.Text=%q", detail.Left.Text, detail.Right.Text)
	}
	if !strings.Contains(detail.Left.Label, "feat/rebase-me") || detail.Right.CommitOID != mainTargetOID {
		t.Fatalf("expected real branch/OID labels on rebase sides, got left=%+v right=%+v", detail.Left, detail.Right)
	}

	// PreviewAbortOperation and AbortOperation must restore exact featureStartOID!
	abortPrev, err := s.PreviewAbortOperation(ctx, repo.ID, wtID)
	if err != nil {
		t.Fatalf("PreviewAbortOperation: %v", err)
	}
	if abortPrev.ConfirmationID == "" || abortPrev.RestoreOID != featureStartOID || len(abortPrev.AffectedPaths) == 0 {
		t.Fatalf("unexpected abort preview: %+v (want restore %s)", abortPrev, featureStartOID)
	}
	abortRes, err := s.AbortOperation(ctx, ExecuteAbortOperationRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: abortPrev.ConfirmationID,
	})
	if err != nil {
		t.Fatalf("AbortOperation: %v", err)
	}
	if abortRes.Outcome != DivergenceOutcomeAborted || abortRes.HeadOID != featureStartOID || abortRes.InProgress.Active {
		t.Fatalf("expected clean abort restoring %s, got %+v", featureStartOID, abortRes)
	}
}

func TestBinaryAndDeleteModifyConflictsCherryPickAndRevert(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	// Create binary file + delete/modify candidate on main.
	if err := os.WriteFile(filepath.Join(dir, "logo.bin"), []byte{0x89, 0x50, 0x4e, 0x00, 0x01}, 0o644); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(dir, "delmod.txt"), "initial\n")
	gitTest(t, dir, "add", "logo.bin", "delmod.txt")
	gitTest(t, dir, "commit", "-m", "feat: add binary and delmod")
	baseOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// Create branch `other` modifying both binary and delmod.txt.
	gitTest(t, dir, "checkout", "-b", "other", baseOID)
	if err := os.WriteFile(filepath.Join(dir, "logo.bin"), []byte{0x89, 0x50, 0x4e, 0x00, 0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(dir, "delmod.txt"), "modified on other\n")
	gitTest(t, dir, "add", "logo.bin", "delmod.txt")
	gitTest(t, dir, "commit", "-m", "feat: other binary and delmod")
	otherCommitOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// Back on main: modify binary differently and delete delmod.txt.
	gitTest(t, dir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(dir, "logo.bin"), []byte{0x89, 0x50, 0x4e, 0x00, 0xaa}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "delmod.txt")); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", "-A")
	gitTest(t, dir, "commit", "-m", "feat: main binary and delete delmod")

	// Cherry-pick `otherCommitOID` onto `main` -> triggers binary + delete_modify conflicts!
	cpPrev, err := s.PreviewCherryPick(ctx, DivergencePreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		CommitOID:    otherCommitOID,
	})
	if err != nil || cpPrev.ConfirmationID == "" {
		t.Fatalf("PreviewCherryPick: %+v %v", cpPrev, err)
	}
	cpRes, err := s.ExecuteCherryPick(ctx, ExecuteDivergenceRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: cpPrev.ConfirmationID,
	})
	if err != nil || cpRes.Outcome != DivergenceOutcomeConflicted || cpRes.InProgress.ConflictedCount != 2 {
		t.Fatalf("expected 2 conflicts on cherry-pick, got %+v err=%v", cpRes, err)
	}

	// Inspect binary conflict: TextMergeable MUST be false; choose_right resolves cleanly without corrupting bytes!
	binDetail, err := s.InspectConflictFile(ctx, repo.ID, wtID, "logo.bin")
	if err != nil {
		t.Fatalf("InspectConflictFile binary: %v", err)
	}
	if binDetail.TextMergeable || binDetail.Kind != ConflictKindBinary {
		t.Fatalf("expected non-text binary conflict, got %+v", binDetail)
	}
	if _, err := s.ResolveConflict(ctx, ResolveConflictRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     binDetail.Identity,
		Resolution:   ResolutionChooseRight,
	}); err != nil {
		t.Fatalf("ResolveConflict binary choose_right: %v", err)
	}
	gotBin, err := os.ReadFile(filepath.Join(dir, "logo.bin"))
	if err != nil || len(gotBin) != 5 || gotBin[4] != 0xff {
		t.Fatalf("binary bytes corrupted or wrong side: %v err=%v", gotBin, err)
	}

	// Inspect delete_modify conflict: choose delete!
	dmDetail, err := s.InspectConflictFile(ctx, repo.ID, wtID, "delmod.txt")
	if err != nil {
		t.Fatalf("InspectConflictFile delete_modify: %v", err)
	}
	if dmDetail.TextMergeable || dmDetail.Kind != ConflictKindDeleteModify {
		t.Fatalf("expected delete_modify conflict, got %+v", dmDetail)
	}
	dmRes, err := s.ResolveConflict(ctx, ResolveConflictRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     dmDetail.Identity,
		Resolution:   ResolutionDelete,
	})
	if err != nil || !dmRes.InProgress.CanContinue {
		t.Fatalf("ResolveConflict delete_modify delete: %+v %v", dmRes, err)
	}

	// Continue cherry-pick to completion!
	doneCP, err := s.ContinueOperation(ctx, ContinueOperationRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Kind:         "cherry-pick",
	})
	if err != nil || doneCP.Outcome != DivergenceOutcomeCompleted {
		t.Fatalf("Continue cherry-pick: %+v %v", doneCP, err)
	}

	// Cleanly Revert the cherry-picked commit!
	revPrev, err := s.PreviewRevert(ctx, DivergencePreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		CommitOID:    doneCP.HeadOID,
	})
	if err != nil || revPrev.ConfirmationID == "" {
		t.Fatalf("PreviewRevert: %+v %v", revPrev, err)
	}
	revRes, err := s.ExecuteRevert(ctx, ExecuteDivergenceRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: revPrev.ConfirmationID,
	})
	if err != nil || revRes.Outcome != DivergenceOutcomeCompleted {
		t.Fatalf("ExecuteRevert: %+v %v", revRes, err)
	}
}
