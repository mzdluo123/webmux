package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStashCreateInspectApplySafePopAndDrop(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	putFile(t, filepath.Join(dir, "tracked"), "base line 1\nbase line 2\n")
	gitTest(t, dir, "add", "tracked")
	gitTest(t, dir, "commit", "-m", "feat: base tracked")

	// 1. Clean worktree -> PreviewStashCreate has nothing_to_stash blocker.
	cleanPrev, err := s.PreviewStashCreate(ctx, StashCreatePreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
	})
	if err != nil {
		t.Fatalf("PreviewStashCreate clean: %v", err)
	}
	if cleanPrev.ConfirmationID != "" || !hasBlocker(cleanPrev.Blockers, "nothing_to_stash") {
		t.Fatalf("expected nothing_to_stash blocker, got %+v", cleanPrev)
	}

	// 2. Untracked-only without IncludeUntracked -> blocked; with IncludeUntracked -> allowed!
	putFile(t, filepath.Join(dir, "notes.txt"), "untracked stash note\n")
	noUntrackedPrev, err := s.PreviewStashCreate(ctx, StashCreatePreviewRequest{
		RepositoryID:     repo.ID,
		WorktreeID:       wtID,
		IncludeUntracked: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if noUntrackedPrev.ConfirmationID != "" || !hasBlocker(noUntrackedPrev.Blockers, "nothing_to_stash") {
		t.Fatalf("expected nothing_to_stash without IncludeUntracked, got %+v", noUntrackedPrev)
	}

	// Add tracked modification + staged file + untracked file, then create stash with IncludeUntracked.
	putFile(t, filepath.Join(dir, "tracked"), "stashed line 1\nbase line 2\n")
	putFile(t, filepath.Join(dir, "staged.txt"), "staged content\n")
	gitTest(t, dir, "add", "staged.txt")

	createPrev, err := s.PreviewStashCreate(ctx, StashCreatePreviewRequest{
		RepositoryID:     repo.ID,
		WorktreeID:       wtID,
		Message:          "WIP: tracked + staged + untracked",
		IncludeUntracked: true,
	})
	if err != nil || createPrev.ConfirmationID == "" {
		t.Fatalf("PreviewStashCreate with untracked: %+v %v", createPrev, err)
	}
	if createPrev.StagedCount != 1 || createPrev.UnstagedCount != 1 || createPrev.UntrackedCount != 1 {
		t.Fatalf("unexpected counts in create preview: %+v", createPrev)
	}

	created, err := s.ExecuteStashCreate(ctx, ExecuteStashCreateRequest{
		RepositoryID:    repo.ID,
		WorktreeID:      wtID,
		ConfirmationID:  createPrev.ConfirmationID,
		ClientRequestID: "stash-create-1",
	})
	if err != nil {
		t.Fatalf("ExecuteStashCreate: %v", err)
	}
	if !created.Entry.HasUntracked || created.Entry.UntrackedOID == "" || len(created.Entry.Parents) != 3 {
		t.Fatalf("expected 3-parent stash with untracked commit, got %+v", created.Entry)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "notes.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected notes.txt removed from working tree after stash push -u, statErr=%v", statErr)
	}

	// 3. ListStashes & InspectStash MUST expose both tracked files AND 3rd-parent untracked files!
	stashes, err := s.ListStashes(ctx, repo.ID)
	if err != nil || len(stashes) != 1 {
		t.Fatalf("ListStashes: %+v %v", stashes, err)
	}
	detail, err := s.InspectStash(ctx, repo.ID, created.Entry.OID)
	if err != nil {
		t.Fatalf("InspectStash: %v", err)
	}
	if detail.TrackedCount != 2 || detail.UntrackedCount != 1 || len(detail.Files) != 3 {
		t.Fatalf("expected 2 tracked + 1 untracked files in StashDetail, got %+v", detail)
	}
	var sawUntrackedNotes bool
	for _, f := range detail.Files {
		if f.NewPath == "notes.txt" && f.Untracked {
			sawUntrackedNotes = true
		}
	}
	if !sawUntrackedNotes {
		t.Fatalf("expected notes.txt marked Untracked=true in StashDetail.Files: %+v", detail.Files)
	}

	// 4. Clean Apply keeps stash entry intact and restores ReinstateIndex when requested!
	applyPrev, err := s.PreviewStashApply(ctx, StashApplyPreviewRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		Mode:           "apply",
		StashRef:       created.Entry.StashRef,
		StashOID:       created.Entry.OID,
		ReinstateIndex: true,
	})
	if err != nil || applyPrev.ConfirmationID == "" {
		t.Fatalf("PreviewStashApply: %+v %v", applyPrev, err)
	}
	applyRes, err := s.ExecuteStashApply(ctx, ExecuteStashApplyRequest{
		RepositoryID:    repo.ID,
		WorktreeID:      wtID,
		ConfirmationID:  applyPrev.ConfirmationID,
		ClientRequestID: "stash-apply-1",
	})
	if err != nil {
		t.Fatalf("ExecuteStashApply: %v", err)
	}
	if applyRes.Outcome != StashApplyCompleted || applyRes.StashDropped || !applyRes.StashPreserved {
		t.Fatalf("expected clean apply with stash preserved, got %+v", applyRes)
	}
	stashesAfterApply, err := s.ListStashes(ctx, repo.ID)
	if err != nil || len(stashesAfterApply) != 1 {
		t.Fatalf("expected stash still present after Apply, got %+v %v", stashesAfterApply, err)
	}

	// Reset worktree cleanly and commit a conflicting edit to `tracked` so `Pop` hits a conflict!
	gitTest(t, dir, "reset", "--hard", "HEAD")
	gitTest(t, dir, "clean", "-fd")
	putFile(t, filepath.Join(dir, "tracked"), "main conflicting line 1\nbase line 2\n")
	gitTest(t, dir, "add", "tracked")
	gitTest(t, dir, "commit", "-m", "feat: conflicting tracked edit on main")

	// 5. Conflicting Pop MUST NOT drop the stash entry AND MUST wire into Unstash 3-way conflict view!
	popConflictPrev, err := s.PreviewStashApply(ctx, StashApplyPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Mode:         "pop",
		StashRef:     created.Entry.StashRef,
		StashOID:     created.Entry.OID,
	})
	if err != nil || popConflictPrev.ConfirmationID == "" {
		t.Fatalf("PreviewStashApply pop conflict: %+v %v", popConflictPrev, err)
	}
	popConflictRes, err := s.ExecuteStashPop(ctx, ExecuteStashApplyRequest{
		RepositoryID:    repo.ID,
		WorktreeID:      wtID,
		ConfirmationID:  popConflictPrev.ConfirmationID,
		ClientRequestID: "stash-pop-conflict-1",
	})
	if err != nil {
		t.Fatalf("ExecuteStashPop conflict: %v", err)
	}
	if popConflictRes.Outcome != StashApplyConflicted || popConflictRes.StashDropped || !popConflictRes.StashPreserved {
		t.Fatalf("CRITICAL: conflicting Pop must preserve stash without dropping! got %+v", popConflictRes)
	}
	if !popConflictRes.InProgress.Active || popConflictRes.InProgress.Kind != "unstash" || popConflictRes.InProgress.CanContinue || popConflictRes.InProgress.CanAbort {
		t.Fatalf("expected active unstash conflict state with CanContinue=false CanAbort=false, got %+v", popConflictRes.InProgress)
	}
	stashesAfterConflictPop, err := s.ListStashes(ctx, repo.ID)
	if err != nil || len(stashesAfterConflictPop) != 1 || stashesAfterConflictPop[0].OID != created.Entry.OID {
		t.Fatalf("stash entry was lost or altered after conflicting Pop! got %+v %v", stashesAfterConflictPop, err)
	}

	// InspectConflictFile during unstash: Left = stage 2 (current worktree branch), Right = stage 3 (stash).
	cDetail, err := s.InspectConflictFile(ctx, repo.ID, wtID, "tracked")
	if err != nil {
		t.Fatalf("InspectConflictFile unstash: %v", err)
	}
	if cDetail.OperationKind != "unstash" || cDetail.Left.Stage != 2 || cDetail.Right.Stage != 3 {
		t.Fatalf("unexpected unstash conflict sides: %+v", cDetail)
	}
	if !strings.Contains(cDetail.Left.Text, "main conflicting line 1") || !strings.Contains(cDetail.Right.Text, "stashed line 1") {
		t.Fatalf("unstash conflict sides inverted! left=%q right=%q", cDetail.Left.Text, cDetail.Right.Text)
	}

	// Resolve unstash conflict via ResolveConflict -> unstash ref clears automatically!
	resolved, err := s.ResolveConflict(ctx, ResolveConflictRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     cDetail.Identity,
		Resolution:   ResolutionText,
		ResultText:   "merged main + stashed line 1\nbase line 2\n",
	})
	if err != nil {
		t.Fatalf("ResolveConflict unstash: %v", err)
	}
	if resolved.InProgress.Active || resolved.InProgress.ConflictedCount != 0 {
		t.Fatalf("expected unstash in-progress cleared once all conflicts resolved, got %+v", resolved.InProgress)
	}

	// 6. Drop single stash entry with stale slot guard & pre-GC recovery command!
	dropPrev, err := s.PreviewStashDrop(ctx, StashDropPreviewRequest{
		RepositoryID: repo.ID,
		StashRef:     created.Entry.StashRef,
		StashOID:     created.Entry.OID,
	})
	if err != nil || dropPrev.ConfirmationID == "" {
		t.Fatalf("PreviewStashDrop: %+v %v", dropPrev, err)
	}
	if !strings.Contains(dropPrev.RecoveryCommand, created.Entry.OID) {
		t.Fatalf("expected recovery command with stash OID, got %q", dropPrev.RecoveryCommand)
	}
	dropRes, err := s.ExecuteStashDrop(ctx, ExecuteStashDropRequest{
		RepositoryID:    repo.ID,
		ConfirmationID:  dropPrev.ConfirmationID,
		ClientRequestID: "stash-drop-1",
	})
	if err != nil {
		t.Fatalf("ExecuteStashDrop: %v", err)
	}
	if len(dropRes.Stashes) != 0 || dropRes.StashOID != created.Entry.OID {
		t.Fatalf("expected empty stash list after drop, got %+v", dropRes)
	}

	// 7. Create a new stash and verify clean two-step Pop drops it when clean!
	gitTest(t, dir, "reset", "--hard", "HEAD")
	gitTest(t, dir, "clean", "-fd")
	putFile(t, filepath.Join(dir, "cleanpop.txt"), "pop cleanly\n")
	cpPrev, err := s.PreviewStashCreate(ctx, StashCreatePreviewRequest{
		RepositoryID:     repo.ID,
		WorktreeID:       wtID,
		Message:          "WIP: clean pop",
		IncludeUntracked: true,
	})
	if err != nil || cpPrev.ConfirmationID == "" {
		t.Fatal(err)
	}
	cpCreated, err := s.ExecuteStashCreate(ctx, ExecuteStashCreateRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: cpPrev.ConfirmationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	popCleanPrev, err := s.PreviewStashApply(ctx, StashApplyPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Mode:         "pop",
		StashRef:     cpCreated.Entry.StashRef,
		StashOID:     cpCreated.Entry.OID,
	})
	if err != nil || popCleanPrev.ConfirmationID == "" {
		t.Fatal(err)
	}
	popCleanRes, err := s.ExecuteStashPop(ctx, ExecuteStashApplyRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: popCleanPrev.ConfirmationID,
	})
	if err != nil {
		t.Fatalf("ExecuteStashPop clean: %v", err)
	}
	if popCleanRes.Outcome != StashApplyCompleted || !popCleanRes.StashDropped || popCleanRes.StashPreserved {
		t.Fatalf("expected clean Pop to drop stash entry, got %+v", popCleanRes)
	}
	remaining, err := s.ListStashes(ctx, repo.ID)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("expected 0 stashes after clean Pop, got %+v %v", remaining, err)
	}
}
