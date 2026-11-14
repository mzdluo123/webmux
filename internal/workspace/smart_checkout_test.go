package workspace

import (
	"context"
	"errors"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSmartCheckoutCleanRollbackAndTargetConflict(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	putFile(t, filepath.Join(dir, "shared.txt"), "base-shared\n")
	putFile(t, filepath.Join(dir, "staged.txt"), "base-staged\n")
	gitTest(t, dir, "add", "shared.txt", "staged.txt")
	gitTest(t, dir, "commit", "-m", "initial baseline")

	// Create clean target branch `feat/clean` and conflicting target branch `feat/conflict`.
	gitTest(t, dir, "checkout", "-b", "feat/clean")
	putFile(t, filepath.Join(dir, "feature-only.txt"), "from feat/clean\n")
	gitTest(t, dir, "add", "feature-only.txt")
	gitTest(t, dir, "commit", "-m", "feat/clean commit")

	gitTest(t, dir, "checkout", "main")
	gitTest(t, dir, "checkout", "-b", "feat/conflict")
	putFile(t, filepath.Join(dir, "shared.txt"), "target-branch-conflict\n")
	gitTest(t, dir, "add", "shared.txt")
	gitTest(t, dir, "commit", "-m", "feat/conflict commit")

	gitTest(t, dir, "checkout", "main")

	// Also create a pre-existing user stash to prove Smart Checkout never drops unrelated stashes!
	putFile(t, filepath.Join(dir, "shared.txt"), "user-manual-stash\n")
	gitTest(t, dir, "stash", "push", "-m", "manual-stash-keep")

	stateDir := t.TempDir()
	s, _ := testService(t, stateDir, dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	// Make staged + unstaged + untracked changes on `main`.
	putFile(t, filepath.Join(dir, "staged.txt"), "my-staged-change\n")
	gitTest(t, dir, "add", "staged.txt")
	putFile(t, filepath.Join(dir, "shared.txt"), "my-unstaged-change\n")
	putFile(t, filepath.Join(dir, "untracked-wip.txt"), "my-untracked-wip\n")

	// Plain SwitchPreview MUST report `changes` blocker.
	plainPrev, err := s.PreviewSwitch(ctx, repo.ID, wtID, SwitchTarget{Kind: SwitchLocal, Branch: "feat/clean"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsIssueCode(plainPrev.Blockers, "changes") {
		t.Fatalf("expected plain switch blocked by changes, got %+v", plainPrev.Blockers)
	}

	// 1. Active session acknowledgement guard: start a session, verify AcknowledgeSessions is enforced!
	sess, err := s.CreateSession(ctx, sessioncreate.Input{Cwd: dir, Cmd: []string{"/bin/sh", "-c", "printf READY; read v"}})
	if err != nil {
		t.Fatal(err)
	}
	waitOutput(t, sess, "READY")

	scPrev1, err := s.PreviewSmartCheckout(ctx, SmartCheckoutPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Target:       SwitchTarget{Kind: SwitchLocal, Branch: "feat/clean"},
	})
	if err != nil || scPrev1.ConfirmationID == "" || len(scPrev1.Sessions) != 1 {
		t.Fatalf("PreviewSmartCheckout with session: %+v %v", scPrev1, err)
	}
	if _, err := s.ExecuteSmartCheckout(ctx, ExecuteSmartCheckoutRequest{
		RepositoryID:        repo.ID,
		WorktreeID:          wtID,
		ConfirmationID:      scPrev1.ConfirmationID,
		AcknowledgeSessions: false,
	}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked when AcknowledgeSessions=false, got %v", err)
	}

	// 2. Phase 2 Switch Failure Auto-Rollback: inject `failSwitch` fault -> must stay on `main`, restore all dirty changes, and drop temp stash!
	scPrevRollback, err := s.PreviewSmartCheckout(ctx, SmartCheckoutPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Target:       SwitchTarget{Kind: SwitchLocal, Branch: "feat/clean"},
	})
	if err != nil {
		t.Fatal(err)
	}
	failCtx := context.WithValue(ctx, smartCheckoutFaultHookKey{}, smartCheckoutFaultHooks{
		failSwitch: func(opID, stashOID string) error {
			return errors.New("injected switch failure")
		},
	})
	rbRes, err := s.ExecuteSmartCheckout(failCtx, ExecuteSmartCheckoutRequest{
		RepositoryID:        repo.ID,
		WorktreeID:          wtID,
		ConfirmationID:      scPrevRollback.ConfirmationID,
		AcknowledgeSessions: true,
	})
	if err != nil {
		t.Fatalf("ExecuteSmartCheckout rollback returned unexpected error: %v", err)
	}
	if rbRes.Outcome != SmartCheckoutRolledBack || !rbRes.StashRestored || !rbRes.StashDropped {
		t.Fatalf("expected clean auto-rollback, got %+v", rbRes)
	}
	if got := gitTest(t, dir, "branch", "--show-current"); got != "main" {
		t.Fatalf("expected branch still main after rollback, got %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "shared.txt")); string(got) != "my-unstaged-change\n" {
		t.Fatalf("unstaged change lost on rollback: %q", string(got))
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "untracked-wip.txt")); string(got) != "my-untracked-wip\n" {
		t.Fatalf("untracked change lost on rollback: %q", string(got))
	}
	if got := gitTest(t, dir, "cat-file", "-p", ":staged.txt"); got != "my-staged-change" {
		t.Fatalf("staged index change lost on rollback: %q", got)
	}

	// 3. Clean 3-Phase Smart Checkout to `feat/clean`!
	scPrevClean, err := s.PreviewSmartCheckout(ctx, SmartCheckoutPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Target:       SwitchTarget{Kind: SwitchLocal, Branch: "feat/clean"},
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanRes, err := s.ExecuteSmartCheckout(ctx, ExecuteSmartCheckoutRequest{
		RepositoryID:        repo.ID,
		WorktreeID:          wtID,
		ConfirmationID:      scPrevClean.ConfirmationID,
		ClientRequestID:     "sc-clean-1",
		AcknowledgeSessions: true,
	})
	if err != nil {
		t.Fatalf("ExecuteSmartCheckout clean: %v", err)
	}
	if cleanRes.Outcome != SmartCheckoutCompleted || cleanRes.Phase != SmartCheckoutPhaseCompleted || !cleanRes.StashDropped {
		t.Fatalf("unexpected clean result: %+v", cleanRes)
	}
	if got := gitTest(t, dir, "branch", "--show-current"); got != "feat/clean" {
		t.Fatalf("expected branch feat/clean, got %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "shared.txt")); string(got) != "my-unstaged-change\n" {
		t.Fatalf("unstaged change missing on feat/clean: %q", string(got))
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "untracked-wip.txt")); string(got) != "my-untracked-wip\n" {
		t.Fatalf("untracked change missing on feat/clean: %q", string(got))
	}
	if got := gitTest(t, dir, "cat-file", "-p", ":staged.txt"); got != "my-staged-change" {
		t.Fatalf("staged index not preserved on feat/clean: %q", got)
	}
	// Pre-existing user stash MUST still be intact!
	stashes, err := s.ListStashes(ctx, repo.ID)
	if err != nil || len(stashes) != 1 || !strings.Contains(stashes[0].Summary, "manual-stash-keep") {
		t.Fatalf("pre-existing manual stash disturbed: %+v %v", stashes, err)
	}

	// 4. Phase 3 Target-Branch Conflict (`feat/conflict`): MUST preserve controlled stash + wire 3-way conflict view!
	scPrevConflict, err := s.PreviewSmartCheckout(ctx, SmartCheckoutPreviewRequest{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Target:       SwitchTarget{Kind: SwitchLocal, Branch: "feat/conflict"},
	})
	if err != nil {
		t.Fatal(err)
	}
	confRes, err := s.ExecuteSmartCheckout(ctx, ExecuteSmartCheckoutRequest{
		RepositoryID:        repo.ID,
		WorktreeID:          wtID,
		ConfirmationID:      scPrevConflict.ConfirmationID,
		AcknowledgeSessions: true,
	})
	if err != nil {
		t.Fatalf("ExecuteSmartCheckout conflict: %v", err)
	}
	if confRes.Outcome != SmartCheckoutConflict || !confRes.StashPreserved || confRes.StashDropped || confRes.StashOID == "" {
		t.Fatalf("expected preserved stash on conflict, got %+v", confRes)
	}
	if typ := gitTest(t, dir, "cat-file", "-t", confRes.StashOID); typ != "commit" {
		t.Fatalf("preserved stashCommitOid %s missing from git objects: %q", confRes.StashOID, typ)
	}
	inProg, err := s.InspectInProgressOperation(ctx, repo.ID, wtID)
	if err != nil || !inProg.Active || inProg.Kind != "unstash" || !strings.Contains(inProg.SourceLabel, "Smart Checkout") {
		t.Fatalf("expected Smart Checkout unstash conflict state, got %+v %v", inProg, err)
	}
	confFile, err := s.InspectConflictFile(ctx, repo.ID, wtID, "shared.txt")
	if err != nil || !confFile.TextMergeable {
		t.Fatalf("InspectConflictFile shared.txt: %+v %v", confFile, err)
	}
}

func TestSmartCheckoutCrashReconciliationAcrossAllPhases(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	for _, phaseCrash := range []string{"afterStash", "afterSwitch", "beforeUnstash", "afterUnstash"} {
		t.Run(phaseCrash, func(t *testing.T) {
			dir := initRepo(t)
			putFile(t, filepath.Join(dir, "app.txt"), "v1\n")
			gitTest(t, dir, "add", "app.txt")
			gitTest(t, dir, "commit", "-m", "v1")
			gitTest(t, dir, "branch", "feat/next")
			putFile(t, filepath.Join(dir, "app.txt"), "dirty-v2\n")

			stateDir := t.TempDir()
			s1, mgr1 := testService(t, stateDir, dir)
			repo := registerTest(t, s1, dir)
			wtID := repo.Worktrees[0].ID

			prev, err := s1.PreviewSmartCheckout(ctx, SmartCheckoutPreviewRequest{
				RepositoryID: repo.ID,
				WorktreeID:   wtID,
				Target:       SwitchTarget{Kind: SwitchLocal, Branch: "feat/next"},
			})
			if err != nil {
				t.Fatal(err)
			}

			var capturedOpID, capturedStashOID string
			errSimulatedCrash := errors.New("simulated mid-operation server crash")
			hooks := smartCheckoutFaultHooks{}
			switch phaseCrash {
			case "afterStash":
				hooks.afterStash = func(opID, stashOID string) error {
					capturedOpID, capturedStashOID = opID, stashOID
					return errSimulatedCrash
				}
			case "afterSwitch":
				hooks.afterSwitch = func(opID, stashOID string) error {
					capturedOpID, capturedStashOID = opID, stashOID
					return errSimulatedCrash
				}
			case "beforeUnstash":
				hooks.beforeUnstash = func(opID, stashOID string) error {
					capturedOpID, capturedStashOID = opID, stashOID
					return errSimulatedCrash
				}
			case "afterUnstash":
				hooks.afterUnstash = func(opID, stashOID string) error {
					capturedOpID, capturedStashOID = opID, stashOID
					return errSimulatedCrash
				}
			}

			crashCtx := context.WithValue(ctx, smartCheckoutFaultHookKey{}, hooks)
			_, execErr := s1.ExecuteSmartCheckout(crashCtx, ExecuteSmartCheckoutRequest{
				RepositoryID:   repo.ID,
				WorktreeID:     wtID,
				ConfirmationID: prev.ConfirmationID,
			})
			if !errors.Is(execErr, errSimulatedCrash) {
				t.Fatalf("expected simulated crash error, got %v", execErr)
			}
			s1.Close()
			mgr1.Shutdown(context.Background())

			// Restart service on the same stateDir: reconcileInterruptedOperations runs!
			s2, mgr2 := testService(t, stateDir, dir)
			defer func() {
				s2.Close()
				mgr2.Shutdown(context.Background())
			}()

			op, err := s2.GetGitOperation(repo.ID, capturedOpID)
			if err != nil {
				t.Fatalf("GetGitOperation after restart: %v", err)
			}
			if op.Status == GitOpRunning || op.Status == GitOpPending {
				t.Fatalf("operation stayed running after crash reconciliation: %+v", op)
			}
			if op.StashOID != capturedStashOID {
				t.Fatalf("expected stashCommitOid %s preserved across crash, got %q", capturedStashOID, op.StashOID)
			}
			if phaseCrash == "afterStash" || phaseCrash == "afterSwitch" || phaseCrash == "beforeUnstash" {
				if op.Status != GitOpNeedsReview || !strings.Contains(op.RecoveryCommand, capturedStashOID) {
					t.Fatalf("expected needs_review + recovery command for %s, got %+v", phaseCrash, op)
				}
				// Verify the pinned stash commit still exists in Git so zero work is lost!
				if typ := gitTest(t, dir, "cat-file", "-t", capturedStashOID); typ != "commit" {
					t.Fatalf("pinned smart-checkout stash commit %s lost: %q", capturedStashOID, typ)
				}
			} else {
				if op.Status != GitOpSucceeded {
					t.Fatalf("expected succeeded after afterUnstash reconciliation, got %+v", op)
				}
			}
		})
	}
}

func containsIssueCode(issues []Issue, code string) bool {
	for _, it := range issues {
		if it.Code == code {
			return true
		}
	}
	return false
}
