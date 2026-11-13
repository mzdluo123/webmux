package codeview

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitFileAndCommitDiffBoundaries(t *testing.T) {
	t.Parallel()
	cv, ws, _, base := setupGitContextFixture(t)
	ctx := context.Background()

	repoDir := filepath.Join(base, "repo")
	if err := os.MkdirAll(repoDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repoDir, "init", "-q", "-b", "main")
	write(t, repoDir, "app.go", "package main\nfunc v1() {}\n")
	write(t, repoDir, "old.txt", "rename me line 1\nrename me line 2\n")
	if err := os.WriteFile(filepath.Join(repoDir, "blob.bin"), []byte{0x00, 0x01, 0x02, 0x03}, 0o600); err != nil {
		t.Fatal(err)
	}
	write(t, repoDir, ".gitattributes", "*.enc filter=custom\n")
	write(t, repoDir, "secret.enc", "plain v1\n")
	gitTest(t, repoDir, "add", ".")
	gitTest(t, repoDir, "commit", "-q", "-m", "commit A")
	commitA := strings.TrimSpace(gitTest(t, repoDir, "rev-parse", "HEAD"))

	write(t, repoDir, "app.go", "package main\nfunc v2() {}\n")
	gitTest(t, repoDir, "mv", "old.txt", "new.txt")
	if err := os.WriteFile(filepath.Join(repoDir, "blob.bin"), []byte{0x00, 0x09, 0x08, 0x07}, 0o600); err != nil {
		t.Fatal(err)
	}
	write(t, repoDir, "secret.enc", "plain v2\n")
	write(t, repoDir, "added.txt", "brand new\n")
	gitTest(t, repoDir, "add", "-A")
	gitTest(t, repoDir, "commit", "-q", "-m", "commit B")
	commitB := strings.TrimSpace(gitTest(t, repoDir, "rev-parse", "HEAD"))

	// Stage a local modification in index so we can assert CommitFile/CommitDiff never touch HEAD or index.
	write(t, repoDir, "added.txt", "staged local edit\n")
	gitTest(t, repoDir, "add", "added.txt")
	indexBefore := gitTest(t, repoDir, "ls-files", "--stage")
	headBefore := strings.TrimSpace(gitTest(t, repoDir, "rev-parse", "HEAD"))

	repo, _, err := ws.Register(ctx, repoDir)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	target := Target{RepositoryID: repo.ID}
	repoCtx, err := cv.RepositoryContext(ctx, repo.ID)
	if err != nil {
		t.Fatalf("RepositoryContext: %v", err)
	}

	// 1. Reject revision expressions, option flags, non-existent OIDs, and invalid paths.
	invalidRevs := []string{
		"HEAD",
		"HEAD~1",
		"main",
		"--output=/tmp/pwn",
		commitA[:12],
		strings.ToUpper(commitA),
	}
	for _, bad := range invalidRevs {
		if _, err := cv.CommitFile(ctx, target, repoCtx.ID, bad, nodeID("app.go")); errorKind(err) != ErrInvalid {
			t.Errorf("CommitFile(%q) returned %v, want ErrInvalid", bad, err)
		}
		if _, err := cv.CommitChanges(ctx, target, repoCtx.ID, bad, commitB); errorKind(err) != ErrInvalid {
			t.Errorf("CommitChanges(from=%q) returned %v, want ErrInvalid", bad, err)
		}
		if _, err := cv.CommitChanges(ctx, target, repoCtx.ID, commitA, bad); errorKind(err) != ErrInvalid {
			t.Errorf("CommitChanges(to=%q) returned %v, want ErrInvalid", bad, err)
		}
	}
	missingOID := strings.Repeat("deadbeef", 5)
	if _, err := cv.CommitFile(ctx, target, repoCtx.ID, missingOID, nodeID("app.go")); errorKind(err) != ErrNotFound {
		t.Fatalf("CommitFile missing OID returned %v, want ErrNotFound", err)
	}
	for _, badPath := range []string{"../outside", "/etc/passwd", ".git/config", "a/../../b"} {
		if _, err := cv.CommitFile(ctx, target, repoCtx.ID, commitA, nodeID(badPath)); errorKind(err) != ErrInvalid {
			t.Errorf("CommitFile bad path %q returned %v, want ErrInvalid", badPath, err)
		}
	}

	// 2. Read historical CommitFile and stream via TargetRaw.
	fileA, err := cv.CommitFile(ctx, target, repoCtx.ID, commitA, nodeID("app.go"))
	if err != nil || fileA.Document.Text != "package main\nfunc v1() {}\n" {
		t.Fatalf("CommitFile A: %+v %v", fileA, err)
	}
	rawTr, err := cv.TargetRaw(ctx, target, fileA.Snapshot, "")
	if err != nil {
		t.Fatalf("TargetRaw commit blob: %v", err)
	}
	var streamed bytes.Buffer
	if err := rawTr.WriteTo(ctx, &streamed); err != nil {
		rawTr.Close()
		t.Fatalf("TargetRaw WriteTo: %v", err)
	}
	rawTr.Close()
	if streamed.String() != "package main\nfunc v1() {}\n" {
		t.Fatalf("streamed commit blob mismatch: %q", streamed.String())
	}

	// 3. Compare A -> B and B -> A (swapped direction).
	abChanges, err := cv.CommitChanges(ctx, target, repoCtx.ID, commitA, commitB)
	if err != nil {
		t.Fatalf("CommitChanges A->B: %v", err)
	}
	byNewPath := map[string]Change{}
	for _, entry := range abChanges.Entries {
		byNewPath[entry.NewPath] = entry
	}
	if byNewPath["added.txt"].Status != "A" {
		t.Fatalf("expected added.txt status A in A->B, got %+v", byNewPath["added.txt"])
	}
	if !strings.HasPrefix(byNewPath["new.txt"].Status, "R") || byNewPath["new.txt"].OldPath != "old.txt" {
		t.Fatalf("expected rename old.txt->new.txt in A->B, got %+v", byNewPath["new.txt"])
	}

	appDiffAB, err := cv.CommitDiff(ctx, target, repoCtx.ID, commitA, commitB, byNewPath["app.go"].ID)
	if err != nil {
		t.Fatalf("CommitDiff app.go A->B: %v", err)
	}
	if appDiffAB.Kind != "text" || appDiffAB.Old.Text != "package main\nfunc v1() {}\n" || appDiffAB.New.Text != "package main\nfunc v2() {}\n" {
		t.Fatalf("unexpected app.go A->B diff: %+v", appDiffAB)
	}

	// Binary file degrades to metadata.
	binDiff, err := cv.CommitDiff(ctx, target, repoCtx.ID, commitA, commitB, byNewPath["blob.bin"].ID)
	if err != nil {
		t.Fatalf("CommitDiff blob.bin: %v", err)
	}
	if binDiff.Kind != "metadata" {
		t.Fatalf("expected binary diff kind metadata, got %+v", binDiff)
	}

	// Filter/working-tree-encoding attribute degrades to metadata on both repo-only and worktree contexts.
	encDiffRepo, err := cv.CommitDiff(ctx, target, repoCtx.ID, commitA, commitB, byNewPath["secret.enc"].ID)
	if err != nil {
		t.Fatalf("CommitDiff secret.enc (repo): %v", err)
	}
	if encDiffRepo.Kind != "metadata" {
		t.Fatalf("expected filter path to degrade to metadata in repo context, got %+v", encDiffRepo)
	}

	wtTarget := Target{RepositoryID: repo.ID, WorktreeID: repo.Worktrees[0].ID}
	wtCtx, err := cv.WorktreeContext(ctx, repo.ID, repo.Worktrees[0].ID)
	if err != nil {
		t.Fatalf("WorktreeContext: %v", err)
	}
	encDiffWT, err := cv.CommitDiff(ctx, wtTarget, wtCtx.ID, commitA, commitB, byNewPath["secret.enc"].ID)
	if err != nil || encDiffWT.Kind != "metadata" {
		t.Fatalf("expected filter path to degrade to metadata in worktree context, got %+v %v", encDiffWT, err)
	}

	// Swap direction B -> A: added.txt becomes D, rename reverses to new.txt -> old.txt, app.go swaps sides.
	baChanges, err := cv.CommitChanges(ctx, target, repoCtx.ID, commitB, commitA)
	if err != nil {
		t.Fatalf("CommitChanges B->A: %v", err)
	}
	baByNew := map[string]Change{}
	for _, entry := range baChanges.Entries {
		baByNew[entry.NewPath] = entry
	}
	if baByNew["added.txt"].Status != "D" {
		t.Fatalf("expected added.txt status D in B->A, got %+v", baByNew["added.txt"])
	}
	if !strings.HasPrefix(baByNew["old.txt"].Status, "R") || baByNew["old.txt"].OldPath != "new.txt" {
		t.Fatalf("expected rename new.txt->old.txt in B->A, got %+v", baByNew["old.txt"])
	}
	appDiffBA, err := cv.CommitDiff(ctx, target, repoCtx.ID, commitB, commitA, baByNew["app.go"].ID)
	if err != nil {
		t.Fatalf("CommitDiff app.go B->A: %v", err)
	}
	if appDiffBA.Old.Text != "package main\nfunc v2() {}\n" || appDiffBA.New.Text != "package main\nfunc v1() {}\n" {
		t.Fatalf("unexpected app.go B->A diff: %+v", appDiffBA)
	}

	// 4. Verify HEAD and .git/index were never mutated!
	indexAfter := gitTest(t, repoDir, "ls-files", "--stage")
	headAfter := strings.TrimSpace(gitTest(t, repoDir, "rev-parse", "HEAD"))
	if indexAfter != indexBefore || headAfter != headBefore {
		t.Fatalf("HEAD or index mutated during CommitDiff/CommitFile:\nHEAD %s -> %s\nindex:\n%s\n->\n%s", headBefore, headAfter, indexBefore, indexAfter)
	}
}
