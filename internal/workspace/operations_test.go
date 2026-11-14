package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

func TestOperationsLedgerDurabilityPermissionsRetentionAndRedaction(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	stateDir := t.TempDir()
	dir1 := initRepo(t)
	dir2 := initRepo(t)
	s, _ := testService(t, stateDir, dir1)
	repo1 := registerTest(t, s, dir1)
	repo2 := registerTest(t, s, dir2)
	wt1 := repo1.Worktrees[0].ID
	wt2 := repo2.Worktrees[0].ID

	putFile(t, filepath.Join(dir1, "one.txt"), "one\n")
	gitTest(t, dir1, "add", "one.txt")
	prev1, err := s.PreviewCommit(ctx, repo1.ID, wt1)
	if err != nil {
		t.Fatal(err)
	}
	res1, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:    repo1.ID,
		WorktreeID:      wt1,
		ConfirmationID:  prev1.ConfirmationID,
		ClientRequestID: "req-repo1-first",
		Message:         "feat: first durable commit",
	})
	if err != nil || res1.OperationID == "" {
		t.Fatalf("ExecuteCommit repo1: %+v %v", res1, err)
	}

	// Verify 0600 permissions on git-operations.json and git-operations.json.lock.
	for _, name := range []string{"git-operations.json", "git-operations.json.lock"} {
		info, statErr := os.Stat(filepath.Join(stateDir, name))
		if statErr != nil {
			t.Fatalf("stat %s: %v", name, statErr)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("%s mode = %04o, want 0600", name, perm)
		}
	}

	// Verify credential/token redaction and 4 KiB UTF-8 truncation.
	const rawSecretURL = "https://alice:super_secret_pw_123@example.com/org/repo.git"
	const rawGHPToken = "ghp_1234567890abcdef1234567890abcdef"
	const rawBearer = "Bearer tok_secret_bearer_999"
	noisy := fmt.Sprintf(
		"progress 10%%\rprogress 100%%\nfatal: auth failed for %s\nAuthorization: %s\ntoken=%s\nhttp.extraheader=AUTHORIZATION: basic dXNlcjpzZWNyZXQ=\n%s",
		rawSecretURL,
		rawBearer,
		rawGHPToken,
		strings.Repeat("中文日志摘要", 400),
	)
	sanitized := sanitizeOperationOutput(noisy)
	if len(sanitized) > MaxOperationOutputBytes {
		t.Fatalf("sanitized output %d bytes exceeds %d", len(sanitized), MaxOperationOutputBytes)
	}
	if !utf8.ValidString(sanitized) {
		t.Fatal("sanitized output is not valid UTF-8")
	}
	for _, secret := range []string{"super_secret_pw_123", rawGHPToken, "tok_secret_bearer_999", "dXNlcjpzZWNyZXQ=", "progress 10%"} {
		if strings.Contains(sanitized, secret) {
			t.Fatalf("secret or stale progress leaked in sanitized output: %q", secret)
		}
	}

	// Verify TTL eviction (>7 days) and per-repo cap (100).
	expiredAt := time.Now().UTC().Add(-OperationTTL - time.Hour)
	if err := s.recordOperationStart(GitOperation{
		ID:           "expired-op",
		RepositoryID: repo1.ID,
		WorktreeID:   wt1,
		Kind:         GitOpFetch,
		Status:       GitOpSucceeded,
		StartedAt:    expiredAt,
		FinishedAt:   &expiredAt,
		Summary:      "expired",
	}); err != nil {
		t.Fatal(err)
	}
	for i := range MaxOperationsPerRepo + 12 {
		ts := time.Now().UTC()
		if err := s.recordOperationStart(GitOperation{
			ID:           fmt.Sprintf("cap-op-%03d", i),
			RepositoryID: repo1.ID,
			WorktreeID:   wt1,
			Kind:         GitOpFetch,
			Status:       GitOpSucceeded,
			StartedAt:    ts,
			FinishedAt:   &ts,
			Summary:      fmt.Sprintf("fetch %d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}
	listed1, err := s.ListGitOperations(repo1.ID, "", MaxOperationsListLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed1) != MaxOperationsPerRepo {
		t.Fatalf("expected %d bounded operations for repo1, got %d", MaxOperationsPerRepo, len(listed1))
	}
	if _, err := s.GetGitOperation(repo1.ID, "expired-op"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected expired-op evicted, got %v", err)
	}

	// Record an operation on repo2, then Unregister repo1 and ensure repo1 records are purged on disk.
	putFile(t, filepath.Join(dir2, "two.txt"), "two\n")
	gitTest(t, dir2, "add", "two.txt")
	prev2, err := s.PreviewCommit(ctx, repo2.ID, wt2)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:   repo2.ID,
		WorktreeID:     wt2,
		ConfirmationID: prev2.ConfirmationID,
		Message:        "feat: repo2 commit",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Unregister(repo1.ID); err != nil {
		t.Fatalf("Unregister repo1: %v", err)
	}
	rawJSON, err := os.ReadFile(filepath.Join(stateDir, "git-operations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rawJSON), repo1.ID) {
		t.Fatalf("git-operations.json still contains unregistered repo1 ID %s", repo1.ID)
	}
	got2, err := s.GetGitOperation(repo2.ID, res2.OperationID)
	if err != nil || got2.ID != res2.OperationID {
		t.Fatalf("expected repo2 operation preserved, got %+v %v", got2, err)
	}
}

func TestOperationsIdempotencyConcurrentAndSequential(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	s, repo, root, remote := upstreamFixture(t)
	wtID := repo.Worktrees[0].ID

	countBefore := strings.TrimSpace(gitTest(t, root, "rev-list", "--count", "HEAD"))
	putFile(t, filepath.Join(root, "idem.txt"), "idempotent\n")
	gitTest(t, root, "add", "idem.txt")

	prev, err := s.PreviewCommit(ctx, repo.ID, wtID)
	if err != nil {
		t.Fatal(err)
	}

	// Slow down git commit slightly via hook so concurrent requests hit in-flight coalescing.
	hookEntered := make(chan struct{})
	releaseHook := make(chan struct{})
	var hookOnce sync.Once
	hookCtx := context.WithValue(ctx, afterCommitHookKey{}, func(string) {
		hookOnce.Do(func() { close(hookEntered) })
		<-releaseHook
	})

	req := ExecuteCommitRequest{
		RepositoryID:    repo.ID,
		WorktreeID:      wtID,
		ConfirmationID:  prev.ConfirmationID,
		ClientRequestID: "commit-req-1",
		Message:         "feat: idempotent commit",
	}

	type commitOut struct {
		res CommitResult
		err error
	}
	ch1 := make(chan commitOut, 1)
	ch2 := make(chan commitOut, 1)
	go func() {
		r, e := s.ExecuteCommit(hookCtx, req)
		ch1 <- commitOut{r, e}
	}()
	<-hookEntered
	go func() {
		r, e := s.ExecuteCommit(ctx, req)
		ch2 <- commitOut{r, e}
	}()
	// Also test in-flight conflict with mismatched message while first is still running!
	if _, conflictErr := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:    repo.ID,
		WorktreeID:      wtID,
		ConfirmationID:  prev.ConfirmationID,
		ClientRequestID: "commit-req-1",
		Message:         "feat: different message",
	}); !isSubkind(conflictErr, ErrIdempotency) {
		t.Fatalf("expected ErrIdempotency while in-flight, got %v", conflictErr)
	}
	close(releaseHook)

	o1, o2 := <-ch1, <-ch2
	if o1.err != nil || o2.err != nil {
		t.Fatalf("concurrent ExecuteCommit errors: %v / %v", o1.err, o2.err)
	}
	if o1.res.OperationID == "" || o1.res.OperationID != o2.res.OperationID || o1.res.CommitOID != o2.res.CommitOID {
		t.Fatalf("concurrent results diverged: %+v vs %+v", o1.res, o2.res)
	}

	// Sequential replay after completion: still returns exact CommitResult without running git commit again!
	seqRes, err := s.ExecuteCommit(ctx, req)
	if err != nil || seqRes.OperationID != o1.res.OperationID || seqRes.CommitOID != o1.res.CommitOID {
		t.Fatalf("sequential replay mismatch: %+v %v", seqRes, err)
	}

	countAfter := strings.TrimSpace(gitTest(t, root, "rev-list", "--count", "HEAD"))
	if countBefore != "1" || countAfter != "2" {
		t.Fatalf("expected commit count 1 -> 2 (strictly 1 commit created), got %s -> %s", countBefore, countAfter)
	}

	// Reusing clientRequestId for a different message or different operation kind -> ErrIdempotency.
	if _, err := s.ExecuteCommit(ctx, ExecuteCommitRequest{
		RepositoryID:    repo.ID,
		WorktreeID:      wtID,
		ConfirmationID:  prev.ConfirmationID,
		ClientRequestID: "commit-req-1",
		Message:         "another message",
	}); !isSubkind(err, ErrIdempotency) {
		t.Fatalf("expected ErrIdempotency on sequential parameter mismatch, got %v", err)
	}
	if _, err := s.FetchRemotesWithOptions(ctx, repo.ID, FetchRequest{ClientRequestID: "commit-req-1"}); !isSubkind(err, ErrIdempotency) {
		t.Fatalf("expected ErrIdempotency when reusing commit clientRequestId for fetch, got %v", err)
	}

	// Push idempotent replay.
	pushPrev, err := s.PreviewPush(ctx, repo.ID, "main", "", "")
	if err != nil || pushPrev.ConfirmationID == "" {
		t.Fatalf("PreviewPush: %+v %v", pushPrev, err)
	}
	pushReq := PushRequest{ConfirmationID: pushPrev.ConfirmationID, ClientRequestID: "push-req-1", WorktreeID: wtID}
	p1, err := s.Push(ctx, repo.ID, pushReq)
	if err != nil || p1.Status != remoteOK || p1.OperationID == "" {
		t.Fatalf("Push 1: %+v %v", p1, err)
	}
	p2, err := s.Push(ctx, repo.ID, pushReq)
	if err != nil || p2.OperationID != p1.OperationID || p2.Status != remoteOK {
		t.Fatalf("Push 2 idempotent replay: %+v %v", p2, err)
	}
	if strings.TrimSpace(gitTest(t, remote, "rev-parse", "refs/heads/main")) != o1.res.CommitOID {
		t.Fatal("remote main tip does not match pushed commit")
	}
}

func TestOperationsStartupReconciliationCommitThreeTimings(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	// Timing 1: Crash after writing running ledger entry, BEFORE git commit runs -> reconciled to "failed".
	t.Run("BeforeGitCommitConvergesToFailed", func(t *testing.T) {
		stateDir := t.TempDir()
		dir := initRepo(t)
		s1, _ := testService(t, stateDir, dir)
		repo := registerTest(t, s1, dir)
		wtID := repo.Worktrees[0].ID

		putFile(t, filepath.Join(dir, "t1.txt"), "timing1\n")
		gitTest(t, dir, "add", "t1.txt")
		prev, err := s1.PreviewCommit(ctx, repo.ID, wtID)
		if err != nil {
			t.Fatal(err)
		}
		var crashedOpID string
		crashCtx := context.WithValue(ctx, beforeOperationGitHookKey{}, func(opID string) bool {
			crashedOpID = opID
			return false
		})
		_, _ = s1.ExecuteCommit(crashCtx, ExecuteCommitRequest{
			RepositoryID:    repo.ID,
			WorktreeID:      wtID,
			ConfirmationID:  prev.ConfirmationID,
			ClientRequestID: "crash-before-git",
			Message:         "feat: interrupted before git",
		})
		s1.Close()

		s2, _ := testService(t, stateDir, dir)
		op, err := s2.GetGitOperation(repo.ID, crashedOpID)
		if err != nil {
			t.Fatalf("GetGitOperation after restart: %v", err)
		}
		if op.Status != GitOpFailed || op.FinishedAt == nil {
			t.Fatalf("expected failed status after restart before git commit, got %+v", op)
		}
		if got := strings.TrimSpace(gitTest(t, dir, "rev-list", "--count", "HEAD")); got != "1" {
			t.Fatalf("expected 1 commit (no auto-retry), got %s", got)
		}
	})

	// Timing 2a: Crash AFTER git commit updates HEAD & reflog, BEFORE ledger finalizes -> reconciled to "succeeded" and replays!
	t.Run("AfterGitCommitConvergesToSucceededAndReplays", func(t *testing.T) {
		stateDir := t.TempDir()
		dir := initRepo(t)
		s1, _ := testService(t, stateDir, dir)
		repo := registerTest(t, s1, dir)
		wtID := repo.Worktrees[0].ID

		putFile(t, filepath.Join(dir, "t2.txt"), "timing2\n")
		gitTest(t, dir, "add", "t2.txt")
		prev, err := s1.PreviewCommit(ctx, repo.ID, wtID)
		if err != nil {
			t.Fatal(err)
		}
		var crashedOpID string
		crashCtx := context.WithValue(ctx, afterOperationGitHookKey{}, func(opID string) bool {
			crashedOpID = opID
			return false
		})
		req := ExecuteCommitRequest{
			RepositoryID:    repo.ID,
			WorktreeID:      wtID,
			ConfirmationID:  prev.ConfirmationID,
			ClientRequestID: "crash-after-git",
			Message:         "feat: committed before crash",
		}
		_, _ = s1.ExecuteCommit(crashCtx, req)
		s1.Close()

		headOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))
		s2, _ := testService(t, stateDir, dir)
		op, err := s2.GetGitOperation(repo.ID, crashedOpID)
		if err != nil {
			t.Fatal(err)
		}
		if op.Status != GitOpSucceeded || op.ActualOID != headOID {
			t.Fatalf("expected succeeded with ActualOID=%s, got %+v", headOID, op)
		}

		// Re-sending identical clientRequestId after server restart replays matched CommitResult without creating a 2nd commit!
		replayed, err := s2.ExecuteCommit(ctx, req)
		if err != nil || replayed.Outcome != CommitOutcomeMatched || replayed.CommitOID != headOID || replayed.OperationID != crashedOpID {
			t.Fatalf("expected idempotent replay after restart, got %+v %v", replayed, err)
		}
		if got := strings.TrimSpace(gitTest(t, dir, "rev-list", "--count", "HEAD")); got != "2" {
			t.Fatalf("expected strictly 2 total commits, got %s", got)
		}
	})

	// Timing 2b: Crash AFTER git commit where pre-commit hook rewrote staged tree -> reconciled to "needs_review"!
	t.Run("AfterHookModifiedCommitConvergesToNeedsReview", func(t *testing.T) {
		stateDir := t.TempDir()
		dir := initRepo(t)
		s1, _ := testService(t, stateDir, dir)
		repo := registerTest(t, s1, dir)
		wtID := repo.Worktrees[0].ID

		hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
		hookScript := "#!/bin/sh\nprintf 'hooked\\n' > hook-added.txt\ngit add hook-added.txt\n"
		if err := os.WriteFile(hookPath, []byte(hookScript), 0o755); err != nil {
			t.Fatal(err)
		}
		putFile(t, filepath.Join(dir, "t2b.txt"), "timing2b\n")
		gitTest(t, dir, "add", "t2b.txt")
		prev, err := s1.PreviewCommit(ctx, repo.ID, wtID)
		if err != nil {
			t.Fatal(err)
		}
		var crashedOpID string
		crashCtx := context.WithValue(ctx, afterOperationGitHookKey{}, func(opID string) bool {
			crashedOpID = opID
			return false
		})
		_, _ = s1.ExecuteCommit(crashCtx, ExecuteCommitRequest{
			RepositoryID:   repo.ID,
			WorktreeID:     wtID,
			ConfirmationID: prev.ConfirmationID,
			Message:        "feat: hook modified tree before crash",
		})
		s1.Close()

		s2, _ := testService(t, stateDir, dir)
		op, err := s2.GetGitOperation(repo.ID, crashedOpID)
		if err != nil {
			t.Fatal(err)
		}
		if op.Status != GitOpNeedsReview || len(op.ReviewReasons) == 0 {
			t.Fatalf("expected needs_review after hook tree modification, got %+v", op)
		}
	})

	// Timing 3: Crash before git commit, but external CLI moved HEAD in the meantime -> reconciled to "uncertain" (never reset!).
	t.Run("ExternalHeadChangeWithoutOurReflogConvergesToUncertain", func(t *testing.T) {
		stateDir := t.TempDir()
		dir := initRepo(t)
		s1, _ := testService(t, stateDir, dir)
		repo := registerTest(t, s1, dir)
		wtID := repo.Worktrees[0].ID

		putFile(t, filepath.Join(dir, "t3.txt"), "timing3\n")
		gitTest(t, dir, "add", "t3.txt")
		prev, err := s1.PreviewCommit(ctx, repo.ID, wtID)
		if err != nil {
			t.Fatal(err)
		}
		var crashedOpID string
		crashCtx := context.WithValue(ctx, beforeOperationGitHookKey{}, func(opID string) bool {
			crashedOpID = opID
			return false
		})
		_, _ = s1.ExecuteCommit(crashCtx, ExecuteCommitRequest{
			RepositoryID:   repo.ID,
			WorktreeID:     wtID,
			ConfirmationID: prev.ConfirmationID,
			Message:        "feat: ambiguous crash",
		})
		s1.Close()

		// External terminal user commits while server is down.
		gitTest(t, dir, "commit", "-m", "external terminal commit")
		externalHead := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

		s2, _ := testService(t, stateDir, dir)
		op, err := s2.GetGitOperation(repo.ID, crashedOpID)
		if err != nil {
			t.Fatal(err)
		}
		if op.Status != GitOpUncertain {
			t.Fatalf("expected uncertain when external commit moved HEAD without webmux reflog, got %+v", op)
		}
		// Never git reset external commit!
		if got := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD")); got != externalHead {
			t.Fatalf("external HEAD was altered! got %s want %s", got, externalHead)
		}
	})
}

func TestOperationsStartupReconciliationPushFastForwardSwitchFetch(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	stateDir := t.TempDir()
	s1, repo, root, remote := upstreamFixtureState(t, stateDir)
	wtID := repo.Worktrees[0].ID

	// 1. Push crashed after remoteGitOutput succeeded -> reconciles to succeeded.
	putFile(t, filepath.Join(root, "p.txt"), "push\n")
	gitTest(t, root, "add", "p.txt")
	gitTest(t, root, "commit", "-m", "local push commit")
	pushPrev, err := s1.PreviewPush(ctx, repo.ID, "main", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var pushOpID string
	_, _ = s1.Push(context.WithValue(ctx, afterOperationGitHookKey{}, func(id string) bool {
		pushOpID = id
		return false
	}), repo.ID, PushRequest{ConfirmationID: pushPrev.ConfirmationID, ClientRequestID: "push-crash-ok", WorktreeID: wtID})

	// 2. Fast-forward crashed after merge --ff-only succeeded -> reconciles to succeeded.
	upstreamHead := advanceUpstream(t, root, remote, "upstream ff")
	if _, err := s1.FetchRemotes(ctx, repo.ID); err != nil {
		t.Fatal(err)
	}
	ffPrev, err := s1.PreviewFastForward(ctx, repo.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	var ffOpID string
	_, _ = s1.FastForward(context.WithValue(ctx, afterOperationGitHookKey{}, func(id string) bool {
		ffOpID = id
		return false
	}), repo.ID, FastForwardRequest{ConfirmationID: ffPrev.ConfirmationID, ClientRequestID: "ff-crash-ok", WorktreeID: wtID})

	// 3. Switch crashed after git switch succeeded (use a separate linked worktree sowtID stays on main for ff check).
	gitTest(t, root, "branch", "feature-next", upstreamHead)
	linkedDir := filepath.Join(t.TempDir(), "linked-wt")
	linkedWt, err := s1.CreateWorktree(ctx, repo.ID, CreateRequest{
		Path:   linkedDir,
		Mode:   "new",
		Branch: "temp-wt-branch",
		Base:   upstreamHead,
	})
	if err != nil {
		t.Fatal(err)
	}
	swPrev, err := s1.PreviewSwitch(ctx, repo.ID, linkedWt.ID, SwitchTarget{Kind: SwitchLocal, Branch: "feature-next"})
	if err != nil {
		t.Fatal(err)
	}
	var swOpID string
	_, _ = s1.SwitchWorktreeResult(context.WithValue(ctx, afterOperationGitHookKey{}, func(id string) bool {
		swOpID = id
		return false
	}), repo.ID, linkedWt.ID, SwitchRequest{ConfirmationID: swPrev.ConfirmationID, ClientRequestID: "sw-crash-ok"})

	// 4. Fetch crashed mid-flight -> reconciles to uncertain, and retrying same clientRequestId yields ErrUncertain.
	var fetchOpID string
	_, _ = s1.FetchRemotesWithOptions(context.WithValue(ctx, beforeOperationGitHookKey{}, func(id string) bool {
		fetchOpID = id
		return false
	}), repo.ID, FetchRequest{ClientRequestID: "fetch-crash-uncertain", WorktreeID: wtID})
	s1.Close()

	s2, _ := testService(t, stateDir, root)
	for _, tc := range []struct {
		name string
		id   string
		want string
	}{
		{"push", pushOpID, GitOpSucceeded},
		{"fast_forward", ffOpID, GitOpSucceeded},
		{"switch", swOpID, GitOpSucceeded},
		{"fetch", fetchOpID, GitOpUncertain},
	} {
		op, gErr := s2.GetGitOperation(repo.ID, tc.id)
		if gErr != nil || op.Status != tc.want {
			t.Fatalf("%s reconcile: got %+v (%v), want status %s", tc.name, op, gErr, tc.want)
		}
	}
	if _, err := s2.FetchRemotesWithOptions(ctx, repo.ID, FetchRequest{ClientRequestID: "fetch-crash-uncertain", WorktreeID: wtID}); !isSubkind(err, ErrUncertain) {
		t.Fatalf("expected ErrUncertain replaying uncertain fetch, got %v", err)
	}
}

func upstreamFixtureState(t *testing.T, stateDir string) (*Service, Repository, string, string) {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "origin.git")
	gitTest(t, t.TempDir(), "init", "--bare", "-b", "main", remote)
	root := initRepo(t)
	gitTest(t, root, "remote", "add", "origin", remote)
	gitTest(t, root, "push", "-u", "origin", "main")
	s, _ := testService(t, stateDir, root)
	r := registerTest(t, s, root)
	return s, r, root, remote
}
