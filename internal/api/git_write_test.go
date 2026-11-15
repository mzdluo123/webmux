package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestGitWriteAPIStagingHunksAndLocks(t *testing.T) {
	env := newGitAPITestEnv(t)
	wtURL := "/api/repositories/" + env.repo.ID + "/worktrees/" + env.mainWT.ID + "/git"

	// 401 & 403 boundaries on staging/write routes.
	for _, methodPath := range [][2]string{
		{"GET", wtURL + "/staging"},
		{"POST", wtURL + "/stage"},
		{"POST", wtURL + "/unstage"},
		{"POST", wtURL + "/commit-preview"},
		{"POST", wtURL + "/commit"},
	} {
		r401 := env.doRaw(methodPath[0], methodPath[1], "{}", nil, "")
		r401.Body.Close()
		if r401.StatusCode != 401 {
			t.Fatalf("%s %s without token: got %d want 401", methodPath[0], methodPath[1], r401.StatusCode)
		}
		for _, badOrigin := range []string{"https://evil.example", "null"} {
			r403 := env.doRaw(methodPath[0], methodPath[1], "{}", map[string]string{
				"Authorization": "Bearer secret",
				"Origin":        badOrigin,
			}, "")
			r403.Body.Close()
			if r403.StatusCode != 403 {
				t.Fatalf("%s %s origin %q: got %d want 403", methodPath[0], methodPath[1], badOrigin, r403.StatusCode)
			}
		}
	}

	// 404 unknown repo / worktree / missing path.
	env.doJSON("GET", "/api/repositories/missing/worktrees/"+env.mainWT.ID+"/git/staging", nil, 404)
	env.doJSON("GET", "/api/repositories/"+env.repo.ID+"/worktrees/missing/git/staging", nil, 404)
	env.doJSON("GET", wtURL+"/staging/diff?area=unstaged&path=nonexistent.txt", nil, 404)

	// 409 readonly on bare worktree.
	bareDir := filepath.Join(t.TempDir(), "bare.git")
	env.gitOut(env.root, "clone", "--bare", env.root, bareDir)
	var bareRepo contract.Repository
	if err := json.Unmarshal(env.doJSON("POST", "/api/repositories", map[string]string{"path": bareDir}, 201), &bareRepo); err != nil {
		t.Fatal(err)
	}
	if len(bareRepo.Worktrees) == 0 {
		t.Fatal("expected bare worktree entry")
	}
	bareWTURL := "/api/repositories/" + bareRepo.ID + "/worktrees/" + bareRepo.Worktrees[0].ID + "/git"
	for _, call := range [][2]string{
		{"GET", bareWTURL + "/staging"},
		{"POST", bareWTURL + "/commit-preview"},
	} {
		var conflict contract.GitConflict
		if err := json.Unmarshal(env.doJSON(call[0], call[1], map[string]any{}, 409), &conflict); err != nil || conflict.Reason != "readonly" {
			t.Fatalf("%s %s: expected readonly conflict, got %+v (%v)", call[0], call[1], conflict, err)
		}
	}

	// Prepare a 14-line file so two edits separated by >6 lines yield 2 independent hunks.
	var initialLines []string
	for i := 1; i <= 14; i++ {
		initialLines = append(initialLines, fmt.Sprintf("line-%02d", i))
	}
	multiPath := filepath.Join(env.root, "multi.txt")
	if err := os.WriteFile(multiPath, []byte(strings.Join(initialLines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "multi.txt")
	env.gitOut(env.root, "commit", "-m", "feat: add multi.txt")

	editedLines := append([]string(nil), initialLines...)
	editedLines[1] = "line-02-hunk-one"
	editedLines[12] = "line-13-hunk-two"
	if err := os.WriteFile(multiPath, []byte(strings.Join(editedLines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "fresh.txt"), []byte("brand new\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var overview contract.GitStagingOverview
	if err := json.Unmarshal(env.doJSON("GET", wtURL+"/staging", nil, 200), &overview); err != nil {
		t.Fatal(err)
	}
	if len(overview.Unstaged) != 2 || len(overview.Staged) != 0 {
		t.Fatalf("unexpected staging overview: %+v", overview)
	}

	var unstagedDiff contract.GitStagingFileDiff
	if err := json.Unmarshal(env.doJSON("GET", wtURL+"/staging/diff?area=unstaged&path=multi.txt", nil, 200), &unstagedDiff); err != nil {
		t.Fatal(err)
	}
	if !unstagedDiff.HunkStageAvailable || len(unstagedDiff.Hunks) != 2 {
		t.Fatalf("expected 2 stageable hunks, got %+v", unstagedDiff)
	}

	// External .git/index.lock -> 409 busy, and lock file is NEVER deleted.
	lockPath := filepath.Join(env.root, ".git", "index.lock")
	const lockSentinel = "external-cli-pid-999"
	if err := os.WriteFile(lockPath, []byte(lockSentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	var busyConflict contract.GitConflict
	stageReq := contract.GitStageRequest{
		Identity: unstagedDiff.Identity,
		HunkIDs:  []string{unstagedDiff.Hunks[0].ID},
	}
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/stage", stageReq, 409), &busyConflict); err != nil {
		t.Fatal(err)
	}
	if busyConflict.Reason != "busy" {
		t.Fatalf("expected busy reason when index.lock exists, got %+v", busyConflict)
	}
	lockBytes, err := os.ReadFile(lockPath)
	if err != nil || string(lockBytes) != lockSentinel {
		t.Fatalf("index.lock was modified or deleted: %q %v", lockBytes, err)
	}
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}

	// Stage only hunk 0; hunk 1 must remain unstaged!
	env.doJSON("POST", wtURL+"/stage", stageReq, 204)

	var stagedDiff, remainingUnstaged contract.GitStagingFileDiff
	if err := json.Unmarshal(env.doJSON("GET", wtURL+"/staging/diff?area=staged&path=multi.txt", nil, 200), &stagedDiff); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(env.doJSON("GET", wtURL+"/staging/diff?area=unstaged&path=multi.txt", nil, 200), &remainingUnstaged); err != nil {
		t.Fatal(err)
	}
	if len(stagedDiff.Hunks) != 1 || len(remainingUnstaged.Hunks) != 1 {
		t.Fatalf("expected 1 staged hunk and 1 remaining unstaged hunk, got %d and %d", len(stagedDiff.Hunks), len(remainingUnstaged.Hunks))
	}

	// Re-using the stale identity from before hunk 0 was staged must fail with 409 stale.
	var staleConflict contract.GitConflict
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/stage", stageReq, 409), &staleConflict); err != nil || staleConflict.Reason != "stale" {
		t.Fatalf("expected stale conflict on outdated identity, got %+v (%v)", staleConflict, err)
	}

	// Unstage hunk 0 back out of index.
	env.doJSON("POST", wtURL+"/unstage", contract.GitStageRequest{
		Identity: stagedDiff.Identity,
		HunkIDs:  []string{stagedDiff.Hunks[0].ID},
	}, 204)

	// Whole-file stage & unstage of untracked fresh.txt.
	var freshOverview contract.GitStagingOverview
	if err := json.Unmarshal(env.doJSON("GET", wtURL+"/staging", nil, 200), &freshOverview); err != nil {
		t.Fatal(err)
	}
	var freshIdentity *contract.GitStageFileIdentity
	for i := range freshOverview.Unstaged {
		if freshOverview.Unstaged[i].Path == "fresh.txt" {
			freshIdentity = &freshOverview.Unstaged[i].Identity
		}
	}
	if freshIdentity == nil {
		t.Fatal("missing fresh.txt in unstaged")
	}
	env.doJSON("POST", wtURL+"/stage", contract.GitStageRequest{Identity: *freshIdentity}, 204)

	if err := json.Unmarshal(env.doJSON("GET", wtURL+"/staging", nil, 200), &freshOverview); err != nil {
		t.Fatal(err)
	}
	var stagedFresh *contract.GitStageFileIdentity
	for i := range freshOverview.Staged {
		if freshOverview.Staged[i].Path == "fresh.txt" {
			stagedFresh = &freshOverview.Staged[i].Identity
		}
	}
	if stagedFresh == nil {
		t.Fatalf("expected fresh.txt in staged: %+v", freshOverview.Staged)
	}
	env.doJSON("POST", wtURL+"/unstage", contract.GitStageRequest{Identity: *stagedFresh}, 204)
}

func TestGitWriteAPICommitLifecycleDisconnectAndPrivacy(t *testing.T) {
	var logBuf syncBuffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	env := newGitAPITestEnv(t)
	readBase := "/api/repositories/" + env.repo.ID + "/git"
	wtURL := "/api/repositories/" + env.repo.ID + "/worktrees/" + env.mainWT.ID + "/git"

	const (
		secretFileLine   = "TOP_SECRET_FILE_LINE_98765"
		secretDiffToken  = "TOP_SECRET_DIFF_TOKEN_54321"
		secretCommitBody = "TOP_SECRET_COMMIT_BODY_112233"
	)

	// 1. Empty index commit-preview reports blockers and omits confirmationId.
	var blockedPreview contract.GitCommitPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/commit-preview", nil, 200), &blockedPreview); err != nil {
		t.Fatal(err)
	}
	if len(blockedPreview.Blockers) == 0 || blockedPreview.ConfirmationID != "" || blockedPreview.ExpiresAt != nil {
		t.Fatalf("expected blocked preview without confirmationId, got %+v", blockedPreview)
	}

	// 2. Pre-staged file from CLI is visible in commit-preview; external mutation invalidates confirmationId.
	if err := os.WriteFile(filepath.Join(env.root, "secret.txt"), []byte(secretFileLine+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "secret.txt")
	if err := os.WriteFile(filepath.Join(env.root, "secret.txt"), []byte(secretFileLine+"\n"+secretDiffToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Read file & diff to verify log privacy across read/diff paths too.
	var wtCtx contract.GitContext
	if err := json.Unmarshal(env.doJSON("GET", readBase+"/context?worktreeId="+env.mainWT.ID, nil, 200), &wtCtx); err != nil {
		t.Fatal(err)
	}
	env.doJSON("GET", readBase+"/file?contextId="+wtCtx.ID+"&nodeId="+encodeNodeID("secret.txt"), nil, 200)
	env.doJSON("GET", wtURL+"/staging/diff?area=unstaged&path="+url.QueryEscape("secret.txt"), nil, 200)

	var preview1 contract.GitCommitPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/commit-preview", map[string]any{}, 200), &preview1); err != nil {
		t.Fatal(err)
	}
	if preview1.ConfirmationID == "" || len(preview1.StagedFiles) != 1 || preview1.StagedFiles[0].NewPath != "secret.txt" {
		t.Fatalf("unexpected commit preview: %+v", preview1)
	}

	// Mutate index externally after preview -> 409 stale.
	if err := os.WriteFile(filepath.Join(env.root, "extra.txt"), []byte("extra\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "extra.txt")
	var staleCommit contract.GitConflict
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/commit", contract.GitCommitRequest{
		ConfirmationID: preview1.ConfirmationID,
		Message:        "feat: stale attempt\n\n" + secretCommitBody,
	}, 409), &staleCommit); err != nil || staleCommit.Reason != "stale" {
		t.Fatalf("expected stale commit conflict, got %+v (%v)", staleCommit, err)
	}

	// 3. Clean matched commit.
	var preview2 contract.GitCommitPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/commit-preview", map[string]any{}, 200), &preview2); err != nil {
		t.Fatal(err)
	}
	var commitRes contract.GitCommitResult
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/commit", contract.GitCommitRequest{
		ConfirmationID: preview2.ConfirmationID,
		Message:        "feat: matched commit\n\n" + secretCommitBody,
	}, 200), &commitRes); err != nil {
		t.Fatal(err)
	}
	if commitRes.Outcome != "matched" || commitRes.CommitOID == "" || commitRes.TreeOID != commitRes.ExpectedTreeOID {
		t.Fatalf("expected matched commit result, got %+v", commitRes)
	}

	// 4. pre-commit hook mutates index -> outcome == "needs_review".
	hooksDir := filepath.Join(env.root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho hooked > hook-touched.txt\ngit add hook-touched.txt\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "secret.txt")
	var previewReview contract.GitCommitPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/commit-preview", map[string]any{}, 200), &previewReview); err != nil {
		t.Fatal(err)
	}
	var reviewRes contract.GitCommitResult
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/commit", contract.GitCommitRequest{
		ConfirmationID: previewReview.ConfirmationID,
		Message:        "feat: hook modifies index",
	}, 200), &reviewRes); err != nil {
		t.Fatal(err)
	}
	if reviewRes.Outcome != "needs_review" || len(reviewRes.ReviewReasons) == 0 || reviewRes.TreeOID == reviewRes.ExpectedTreeOID {
		t.Fatalf("expected needs_review when hook modifies index, got %+v", reviewRes)
	}

	// 5. Failing pre-commit hook -> 409 commit_failed.
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho 'lint failed' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "fail.txt"), []byte("fail\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "fail.txt")
	var previewFail contract.GitCommitPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/commit-preview", map[string]any{}, 200), &previewFail); err != nil {
		t.Fatal(err)
	}
	var failedCommit contract.GitConflict
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/commit", contract.GitCommitRequest{
		ConfirmationID: previewFail.ConfirmationID,
		Message:        "feat: blocked by hook",
	}, 409), &failedCommit); err != nil || failedCommit.Reason != "commit_failed" {
		t.Fatalf("expected commit_failed conflict, got %+v (%v)", failedCommit, err)
	}

	// 6. Client disconnects mid-commit while slow hook runs -> commit still completes cleanly on disk!
	marker := filepath.Join(t.TempDir(), "hook-started")
	hookScript := fmt.Sprintf("#!/bin/sh\nprintf ok > %q\nsleep 0.35\n", marker)
	if err := os.WriteFile(hookPath, []byte(hookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	headBefore := env.gitOut(env.root, "rev-parse", "HEAD")
	var previewDisc contract.GitCommitPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/commit-preview", map[string]any{}, 200), &previewDisc); err != nil {
		t.Fatal(err)
	}
	reqBody, _ := json.Marshal(contract.GitCommitRequest{
		ConfirmationID: previewDisc.ConfirmationID,
		Message:        "feat: survives client disconnect\n\n" + secretCommitBody,
	})
	reqCtx, cancelReq := context.WithCancel(context.Background())
	go func() {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if _, sErr := os.Stat(marker); sErr == nil {
				cancelReq()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	httpReq, err := http.NewRequestWithContext(reqCtx, "POST", env.server.URL+wtURL+"/commit", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	httpReq.Header.Set("Authorization", "Bearer secret")
	httpReq.Header.Set("Content-Type", "application/json")
	if resp, doErr := env.server.Client().Do(httpReq); doErr == nil {
		resp.Body.Close()
	}
	cancelReq()

	deadline := time.Now().Add(3 * time.Second)
	for env.gitOut(env.root, "rev-parse", "HEAD") == headBefore && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	headAfter := env.gitOut(env.root, "rev-parse", "HEAD")
	if headAfter == headBefore {
		t.Fatal("expected commit to complete on disk even after client disconnected")
	}

	// 7. Log privacy assertion: no file contents, diff lines, or commit message body leaked to slog.
	logs := logBuf.String()
	for _, secret := range []string{secretFileLine, secretDiffToken, secretCommitBody} {
		if strings.Contains(logs, secret) {
			t.Fatalf("sensitive payload %q leaked in server logs: %s", secret, logs)
		}
	}
}
