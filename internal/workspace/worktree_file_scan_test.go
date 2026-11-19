package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLargeSparseFile(t *testing.T, path string, size int64, prefix, suffix string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte(prefix), 0); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if len(suffix) > 0 {
		if _, err := f.WriteAt([]byte(suffix), size-int64(len(suffix))); err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func findStagingEntry(list []StagingEntry, relPath string) *StagingEntry {
	for i := range list {
		if list[i].Path == relPath {
			return &list[i]
		}
	}
	return nil
}

func findReviewFile(list []ReviewFileState, relPath string) *ReviewFileState {
	for i := range list {
		if list[i].Path == relPath {
			return &list[i]
		}
	}
	return nil
}

func TestLargeFilesNestedReposAndDirectoryReplacedTrackedPaths(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	const over32MiB = (33 << 20) + 17

	dir := initRepo(t)
	largeTrackedPath := filepath.Join(dir, "large-tracked.bin")
	dirReplacePath := filepath.Join(dir, "replaced.txt")

	writeLargeSparseFile(t, largeTrackedPath, over32MiB, "tracked-v1-head\n", "tracked-v1-tail\n")
	putFile(t, dirReplacePath, "was-a-file\n")
	gitTest(t, dir, "add", "large-tracked.bin", "replaced.txt")
	gitTest(t, dir, "commit", "-m", "initial with >32MiB tracked file")
	gitTest(t, dir, "branch", "feat/target")

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	// Record task baseline at initial HEAD before making dirty changes.
	if _, err := s.PutTaskBaseline(ctx, repo.ID, wtID, PutTaskBaselineRequest{}); err != nil {
		t.Fatalf("PutTaskBaseline: %v", err)
	}

	// 1. Mutate >32 MiB tracked file and create >32 MiB untracked file.
	largeUntrackedPath := filepath.Join(dir, "sub", "large-untracked.bin")
	wantTrackedV2Sum := writeLargeSparseFile(t, largeTrackedPath, over32MiB, "tracked-v2-head\n", "tracked-v2-tail\n")
	wantUntrackedSum := writeLargeSparseFile(t, largeUntrackedPath, over32MiB, "untracked-v1-head\n", "untracked-v1-tail\n")

	// 2. Create two untracked nested Git repositories: one unborn HEAD and one with a commit.
	nestedUnbornDir := filepath.Join(dir, "nested-unborn")
	if err := os.MkdirAll(nestedUnbornDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitTest(t, nestedUnbornDir, "init", "-b", "main")
	putFile(t, filepath.Join(nestedUnbornDir, "draft.txt"), "unborn nested file\n")

	nestedCommitDir := filepath.Join(dir, "nested-committed")
	if err := os.MkdirAll(nestedCommitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitTest(t, nestedCommitDir, "init", "-b", "main")
	putFile(t, filepath.Join(nestedCommitDir, "lib.txt"), "committed nested file\n")
	gitTest(t, nestedCommitDir, "add", "lib.txt")
	gitTest(t, nestedCommitDir, "commit", "-m", "nested init")
	wantNestedOID := gitTest(t, nestedCommitDir, "rev-parse", "HEAD")

	// 3. Delete tracked file `replaced.txt` on disk and create a directory with the same name.
	if err := os.Remove(dirReplacePath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dirReplacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(dirReplacePath, "child.txt"), "inside replacement directory\n")

	// ---- Verify canceled context propagates error in InspectStaging ----
	canceledCtx, cancelNow := context.WithCancel(ctx)
	cancelNow()
	if _, err := s.InspectStaging(canceledCtx, repo.ID, wtID); err == nil {
		t.Fatal("expected error when context is canceled")
	}

	// ---- Verify StagingOverview & InspectStagingFile ----
	overview, err := s.InspectStaging(ctx, repo.ID, wtID)
	if err != nil {
		t.Fatalf("InspectStaging failed: %v", err)
	}

	ltEntry := findStagingEntry(overview.Unstaged, "large-tracked.bin")
	if ltEntry == nil || ltEntry.Status != "M" || ltEntry.HunkStageAvailable ||
		ltEntry.Identity.TargetSHA256 != wantTrackedV2Sum ||
		ltEntry.DegradeReason != "文件超过 2 MiB 文本分块上限，仅支持整文件操作" {
		t.Fatalf("unexpected large-tracked.bin entry: %+v", ltEntry)
	}

	luEntry := findStagingEntry(overview.Unstaged, "sub/large-untracked.bin")
	if luEntry == nil || luEntry.Status != "?" || luEntry.HunkStageAvailable ||
		luEntry.Identity.TargetSHA256 != wantUntrackedSum ||
		luEntry.DegradeReason != "文件超过 2 MiB 文本分块上限，仅支持整文件操作" {
		t.Fatalf("unexpected sub/large-untracked.bin entry: %+v", luEntry)
	}

	nuEntry := findStagingEntry(overview.Unstaged, "nested-unborn")
	if nuEntry == nil || nuEntry.Status != "?" || nuEntry.HunkStageAvailable ||
		nuEntry.Identity.TargetMode != "160000" || !strings.HasPrefix(nuEntry.Identity.TargetSHA256, "unborn:") ||
		!strings.Contains(nuEntry.DegradeReason, "未跟踪嵌套 Git 仓库") {
		t.Fatalf("unexpected nested-unborn entry: %+v", nuEntry)
	}

	ncEntry := findStagingEntry(overview.Unstaged, "nested-committed")
	if ncEntry == nil || ncEntry.Status != "?" || ncEntry.HunkStageAvailable ||
		ncEntry.Identity.TargetMode != "160000" || ncEntry.Identity.TargetSHA256 != wantNestedOID ||
		!strings.Contains(ncEntry.DegradeReason, "未跟踪嵌套 Git 仓库") {
		t.Fatalf("unexpected nested-committed entry: %+v", ncEntry)
	}

	repEntry := findStagingEntry(overview.Unstaged, "replaced.txt")
	if repEntry == nil || repEntry.Status != "D" || repEntry.HunkStageAvailable ||
		repEntry.Identity.BaseMode != "100644" || repEntry.Identity.TargetMode != "" || repEntry.Identity.TargetSHA256 != "" {
		t.Fatalf("unexpected replaced.txt entry: %+v", repEntry)
	}
	repChild := findStagingEntry(overview.Unstaged, "replaced.txt/child.txt")
	if repChild == nil || repChild.Status != "?" || !repChild.HunkStageAvailable {
		t.Fatalf("expected replaced.txt/child.txt untracked entry, got %+v", repChild)
	}

	// InspectStagingFile must succeed for every one of these paths (no trailing-slash ErrInvalid or >32MiB ErrTooLarge).
	for _, p := range []string{"large-tracked.bin", "sub/large-untracked.bin", "nested-unborn", "nested-committed", "replaced.txt"} {
		diff, dErr := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, p)
		if dErr != nil {
			t.Fatalf("InspectStagingFile(%s): %v", p, dErr)
		}
		if diff.HunkStageAvailable {
			t.Fatalf("expected HunkStageAvailable=false for %s, got %+v", p, diff)
		}
	}

	// ---- Verify Reviews ("uncommitted" & "task") + mark & invalidation ----
	for _, scope := range []string{ReviewScopeUncommitted, ReviewScopeTask} {
		rev, rErr := s.GetReviews(ctx, repo.ID, wtID, scope)
		if rErr != nil {
			t.Fatalf("GetReviews(%s): %v", scope, rErr)
		}
		for _, p := range []string{"large-tracked.bin", "sub/large-untracked.bin", "nested-unborn", "nested-committed", "replaced.txt", "replaced.txt/child.txt"} {
			if findReviewFile(rev.Files, p) == nil {
				t.Fatalf("GetReviews(%s) missing %s in %+v", scope, p, rev.Files)
			}
		}
		repRev := findReviewFile(rev.Files, "replaced.txt")
		if repRev.Status != "D" || repRev.TargetMode != "" || repRev.TargetIdentity != "" {
			t.Fatalf("GetReviews(%s) replaced.txt unexpected state: %+v", scope, repRev)
		}
		nuRev := findReviewFile(rev.Files, "nested-unborn")
		marked, mErr := s.PutReviews(ctx, repo.ID, wtID, PutReviewsRequest{
			Scope:  scope,
			Action: ReviewActionMark,
			Item: &ReviewMarkIdentity{
				Scope:          nuRev.Scope,
				Area:           nuRev.Area,
				Path:           nuRev.Path,
				BaseMode:       nuRev.BaseMode,
				BaseIdentity:   nuRev.BaseIdentity,
				TargetMode:     nuRev.TargetMode,
				TargetIdentity: nuRev.TargetIdentity,
			},
		})
		if mErr != nil || !marked.Applied || marked.ReviewedFiles != 1 {
			t.Fatalf("PutReviews(%s) mark nested-unborn failed: %+v %v", scope, marked, mErr)
		}
	}

	// Creating a commit inside nested-unborn changes its HEAD from sentinel to commit OID and invalidates Review marks.
	gitTest(t, nestedUnbornDir, "add", "draft.txt")
	gitTest(t, nestedUnbornDir, "commit", "-m", "born commit")
	for _, scope := range []string{ReviewScopeUncommitted, ReviewScopeTask} {
		revAfter, rErr := s.GetReviews(ctx, repo.ID, wtID, scope)
		if rErr != nil {
			t.Fatalf("GetReviews(%s) after nested commit: %v", scope, rErr)
		}
		if revAfter.ReviewedFiles != 0 {
			t.Fatalf("expected nested-unborn review mark invalidated in %s, got %+v", scope, revAfter)
		}
	}

	// ---- Verify PreviewDiscard blockers for >32 MiB file, nested Git repo, and directory collision ----
	discLarge, err := s.PreviewDiscard(ctx, DiscardPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        DiscardScopeFiles,
		Paths:        []string{"large-tracked.bin"},
	})
	if err != nil || discLarge.ConfirmationID != "" || !containsIssueCode(discLarge.Blockers, "file_too_large") {
		t.Fatalf("expected file_too_large blocker on >32 MiB discard preview, got %+v %v", discLarge, err)
	}
	discNested, err := s.PreviewDiscard(ctx, DiscardPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        DiscardScopeFiles,
		Paths:        []string{"nested-committed"},
	})
	if err != nil || discNested.ConfirmationID != "" || !containsIssueCode(discNested.Blockers, "unsupported_submodule") {
		t.Fatalf("expected unsupported_submodule blocker on nested repo discard preview, got %+v %v", discNested, err)
	}
	discReplaced, err := s.PreviewDiscard(ctx, DiscardPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        DiscardScopeFiles,
		Paths:        []string{"replaced.txt"},
	})
	if err != nil || discReplaced.ConfirmationID != "" || !containsIssueCode(discReplaced.Blockers, "directory_collision") {
		t.Fatalf("expected directory_collision blocker on replaced.txt discard preview, got %+v %v", discReplaced, err)
	}

	// ---- Verify Stage rejects untracked nested Git repo (`160000` without index baseline) ----
	if err := s.Stage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     ncEntry.Identity,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict staging untracked nested Git repo, got %v", err)
	}

	// ---- Verify SmartCheckout blocker when untracked nested Git repo exists & IncludeUntracked=true ----
	scNestedBlocked, err := s.PreviewSmartCheckout(ctx, SmartCheckoutPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Target:       SwitchTarget{Kind: SwitchLocal, Branch: "feat/target"},
	})
	if err != nil || scNestedBlocked.ConfirmationID != "" || !containsIssueCode(scNestedBlocked.Blockers, "untracked_nested_repo") {
		t.Fatalf("expected untracked_nested_repo blocker on SmartCheckout preview, got %+v %v", scNestedBlocked, err)
	}
	includeFalse := false
	scWithoutUntracked, err := s.PreviewSmartCheckout(ctx, SmartCheckoutPreviewRequest{
		RepositoryID:     repo.ID,
		WorktreeID:       wtID,
		Target:           SwitchTarget{Kind: SwitchLocal, Branch: "feat/target"},
		IncludeUntracked: &includeFalse,
	})
	if err != nil || scWithoutUntracked.ConfirmationID == "" {
		t.Fatalf("expected SmartCheckout preview allowed when IncludeUntracked=false, got %+v %v", scWithoutUntracked, err)
	}

	// ---- Verify whole-file Stage & Unstage on directory-replaced tracked path + >32 MiB files ----
	if err := s.Stage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     repEntry.Identity,
	}); err != nil {
		t.Fatalf("Stage deleted replaced.txt: %v", err)
	}
	if err := s.Stage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     repChild.Identity,
	}); err != nil {
		t.Fatalf("Stage replaced.txt/child.txt: %v", err)
	}

	// Mutating >32 MiB file invalidates captured Identity -> Stage returns ErrStale.
	staleLargeID := ltEntry.Identity
	wantTrackedV3Sum := writeLargeSparseFile(t, largeTrackedPath, over32MiB, "tracked-v3-head\n", "tracked-v3-tail\n")
	if err := s.Stage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     staleLargeID,
	}); !errors.Is(err, ErrStale) {
		t.Fatalf("expected ErrStale when staging mutated >32 MiB file with stale identity, got %v", err)
	}

	freshLT, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "large-tracked.bin")
	if err != nil || freshLT.Identity.TargetSHA256 != wantTrackedV3Sum {
		t.Fatalf("InspectStagingFile fresh large-tracked.bin: %+v %v", freshLT, err)
	}
	if err := s.Stage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     freshLT.Identity,
	}); err != nil {
		t.Fatalf("whole-file Stage >32 MiB modified file: %v", err)
	}
	if err := s.Stage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     luEntry.Identity,
	}); err != nil {
		t.Fatalf("whole-file Stage >32 MiB untracked file: %v", err)
	}

	// InspectStaging and InspectStagingFile(StageAreaStaged) must succeed on >32 MiB staged blobs without ErrTooLarge!
	stagedOv, err := s.InspectStaging(ctx, repo.ID, wtID)
	if err != nil {
		t.Fatalf("InspectStaging with >32 MiB staged blobs: %v", err)
	}
	// Unstaging replaced.txt while replaced.txt/child.txt is still staged returns ErrConflict; unstaging child first succeeds cleanly.
	repStagedBefore := findStagingEntry(stagedOv.Staged, "replaced.txt")
	if repStagedBefore == nil {
		t.Fatalf("expected replaced.txt in Staged overview: %+v", stagedOv.Staged)
	}
	if uErr := s.Unstage(ctx, StageRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Identity:     repStagedBefore.Identity,
	}); !errors.Is(uErr, ErrConflict) {
		t.Fatalf("expected ErrConflict unstaging replaced.txt before replaced.txt/child.txt, got %v", uErr)
	}

	for _, p := range []string{"large-tracked.bin", "sub/large-untracked.bin", "replaced.txt/child.txt", "replaced.txt"} {
		stEntry := findStagingEntry(stagedOv.Staged, p)
		if stEntry == nil {
			t.Fatalf("expected %s in Staged overview, got %+v", p, stagedOv.Staged)
		}
		if _, dErr := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaStaged, p); dErr != nil {
			t.Fatalf("InspectStagingFile(staged, %s): %v", p, dErr)
		}
		if uErr := s.Unstage(ctx, StageRequest{
			RepositoryID: repo.ID,
			WorktreeID:   wtID,
			Identity:     stEntry.Identity,
		}); uErr != nil {
			t.Fatalf("Unstage(%s): %v", p, uErr)
		}
	}

	// ---- Verify SmartCheckout with >32 MiB modified + untracked files and ErrStale detection ----
	if err := os.RemoveAll(nestedUnbornDir); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(nestedCommitDir); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dirReplacePath); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "checkout", "--", "replaced.txt")

	scLargePrev, err := s.PreviewSmartCheckout(ctx, SmartCheckoutPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Target:       SwitchTarget{Kind: SwitchLocal, Branch: "feat/target"},
	})
	if err != nil || scLargePrev.ConfirmationID == "" {
		t.Fatalf("PreviewSmartCheckout with >32 MiB files: %+v %v", scLargePrev, err)
	}
	if scLargePrev.UnstagedCount != 1 || scLargePrev.UntrackedCount != 1 {
		t.Fatalf("expected UnstagedCount=1 UntrackedCount=1, got %+v", scLargePrev)
	}

	// Modify >32 MiB untracked file after preview -> ExecuteSmartCheckout must detect dirtyDigest mismatch and return ErrStale!
	wantUntrackedV2Sum := writeLargeSparseFile(t, largeUntrackedPath, over32MiB, "untracked-v2-head\n", "untracked-v2-tail\n")
	if _, err := s.ExecuteSmartCheckout(ctx, ExecuteSmartCheckoutRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: scLargePrev.ConfirmationID,
	}); !errors.Is(err, ErrStale) {
		t.Fatalf("expected ErrStale after mutating >32 MiB untracked file post-preview, got %v", err)
	}

	// Re-preview and execute SmartCheckout -> carries >32 MiB changes to feat/target!
	scLargePrev2, err := s.PreviewSmartCheckout(ctx, SmartCheckoutPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Target:       SwitchTarget{Kind: SwitchLocal, Branch: "feat/target"},
	})
	if err != nil || scLargePrev2.ConfirmationID == "" {
		t.Fatalf("PreviewSmartCheckout 2: %+v %v", scLargePrev2, err)
	}
	scLargeRes, err := s.ExecuteSmartCheckout(ctx, ExecuteSmartCheckoutRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: scLargePrev2.ConfirmationID,
	})
	if err != nil || scLargeRes.Outcome != SmartCheckoutCompleted {
		t.Fatalf("ExecuteSmartCheckout with >32 MiB files: %+v %v", scLargeRes, err)
	}
	if got := gitTest(t, dir, "branch", "--show-current"); got != "feat/target" {
		t.Fatalf("expected branch feat/target, got %q", got)
	}
	afterScan, err := s.InspectStagingFile(ctx, repo.ID, wtID, StageAreaUnstaged, "sub/large-untracked.bin")
	if err != nil || afterScan.Identity.TargetSHA256 != wantUntrackedV2Sum {
		t.Fatalf("expected >32 MiB untracked file carried intact to feat/target, got %+v %v", afterScan, err)
	}
}

func TestStreamWorktreeBlobWriteStaleGuardAndMediumLargeFileDiscard(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	const threeMiB = (3 << 20) + 123

	dir := initRepo(t)
	medPath := filepath.Join(dir, "medium-large.bin")
	wantV1Sum := writeLargeSparseFile(t, medPath, threeMiB, "med-v1-head\n", "med-v1-tail\n")
	gitTest(t, dir, "add", "medium-large.bin")
	gitTest(t, dir, "commit", "-m", "commit 3MiB file")

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	wantV2Sum := writeLargeSparseFile(t, medPath, threeMiB, "med-v2-head\n", "med-v2-tail\n")

	// Direct verification of streamWorktreeBlobWrite TeeReader digest mismatch -> ErrStale.
	if _, err := s.streamWorktreeBlobWrite(ctx, dir, "medium-large.bin", wantV1Sum, true); !errors.Is(err, ErrStale) {
		t.Fatalf("expected ErrStale from streamWorktreeBlobWrite on mismatched SHA-256, got %v", err)
	}

	// Whole-file Discard on 3 MiB (> maxHunkFileBytes, <= maxWorkspaceGitOutput) streams recovery snapshot and restores index blob.
	prev, err := s.PreviewDiscard(ctx, DiscardPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        DiscardScopeFiles,
		Paths:        []string{"medium-large.bin"},
	})
	if err != nil || prev.ConfirmationID == "" {
		t.Fatalf("PreviewDiscard 3MiB file: %+v %v", prev, err)
	}
	discRes, err := s.ExecuteDiscard(ctx, ExecuteDiscardRequest{
		RepositoryID:   repo.ID,
		WorktreeID:     wtID,
		ConfirmationID: prev.ConfirmationID,
	})
	if err != nil || discRes.RecoveryOID == "" {
		t.Fatalf("ExecuteDiscard 3MiB file: %+v %v", discRes, err)
	}

	// Working tree must match committed v1 digest.
	scanRestored, err := s.scanWorktreeFile(ctx, dir, "medium-large.bin", "100644")
	if err != nil || scanRestored.sum != wantV1Sum {
		t.Fatalf("expected restored 3MiB digest %s, got %+v %v", wantV1Sum, scanRestored, err)
	}

	// Recovery commit must contain the exact 3MiB pre-discard v2 content.
	gitTest(t, dir, "restore", "--source="+discRes.RecoveryOID, "--worktree", "--", "medium-large.bin")
	scanRecovered, err := s.scanWorktreeFile(ctx, dir, "medium-large.bin", "100644")
	if err != nil || scanRecovered.sum != wantV2Sum {
		t.Fatalf("expected recovered 3MiB digest %s, got %+v %v", wantV2Sum, scanRecovered, err)
	}
}
