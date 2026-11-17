package codeview

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/workspace"
)

func setupGitContextFixture(t *testing.T) (*Service, *workspace.Service, *session.Manager, string) {
	t.Helper()
	base := t.TempDir()
	base, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(base, "state")
	m := session.NewManager(session.Options{DefaultCwd: base})
	ws := workspace.New(stateDir, m, nil, base)
	cv := New(m, nil)
	cv.UseWorkspace(ws)
	t.Cleanup(func() {
		cv.Close()
		ws.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	return cv, ws, m, base
}

func initRepoWithCommit(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "README.md", "# Demo\nline 2\n")
	write(t, dir, "src.txt", "alpha\nbeta\n")
	gitTest(t, dir, "add", "README.md", "src.txt")
	gitTest(t, dir, "commit", "-q", "-m", "initial")
	return strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))
}

func TestZeroSessionRepositoryAndWorktreeContexts(t *testing.T) {
	t.Parallel()
	cv, ws, m, base := setupGitContextFixture(t)
	ctx := context.Background()

	repoDir := filepath.Join(base, "repo")
	initRepoWithCommit(t, repoDir)
	repo, created, err := ws.Register(ctx, repoDir)
	if err != nil || !created {
		t.Fatalf("Register: created=%v err=%v", created, err)
	}
	if len(m.List()) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(m.List()))
	}

	repoCtx, err := cv.RepositoryContext(ctx, repo.ID)
	if err != nil {
		t.Fatalf("RepositoryContext: %v", err)
	}
	if repoCtx.Kind != "repository" || repoCtx.Browse || repoCtx.Diff || repoCtx.Write || repoCtx.Reason == "" {
		t.Fatalf("unexpected repository context: %+v", repoCtx)
	}
	if _, err := cv.TargetTree(ctx, Target{RepositoryID: repo.ID}, repoCtx.ID, "", false, ""); errorKind(err) != ErrConflict {
		t.Fatalf("expected browse conflict on repository-only context, got %v", err)
	}

	mainWT := repo.Worktrees[0]
	wtTarget := Target{RepositoryID: repo.ID, WorktreeID: mainWT.ID}
	wtCtx, err := cv.WorktreeContext(ctx, repo.ID, mainWT.ID)
	if err != nil {
		t.Fatalf("WorktreeContext: %v", err)
	}
	if wtCtx.Kind != "worktree" || !wtCtx.Browse || !wtCtx.Diff || !wtCtx.Ignore || !wtCtx.Write {
		t.Fatalf("unexpected worktree context: %+v", wtCtx)
	}

	tree, err := cv.TargetTree(ctx, wtTarget, wtCtx.ID, "", false, "")
	if err != nil || len(tree.Children) != 2 {
		t.Fatalf("TargetTree: %+v %v", tree, err)
	}
	fileSnap, err := cv.TargetFile(ctx, wtTarget, wtCtx.ID, nodeID("README.md"))
	if err != nil || fileSnap.Document.Text != "# Demo\nline 2\n" {
		t.Fatalf("TargetFile: %+v %v", fileSnap, err)
	}
	tr, err := cv.TargetRaw(ctx, wtTarget, fileSnap.Snapshot, "")
	if err != nil {
		t.Fatalf("TargetRaw: %v", err)
	}
	var rawBuf bytes.Buffer
	if err := tr.WriteTo(ctx, &rawBuf); err != nil {
		tr.Close()
		t.Fatalf("TargetRaw WriteTo: %v", err)
	}
	tr.Close()
	if rawBuf.String() != "# Demo\nline 2\n" {
		t.Fatalf("unexpected raw content: %q", rawBuf.String())
	}

	if len(m.List()) != 0 {
		t.Fatalf("contexts created hidden sessions: %d", len(m.List()))
	}
}

func TestBareRepositoryAndInvalidWorktreeContexts(t *testing.T) {
	t.Parallel()
	cv, ws, _, base := setupGitContextFixture(t)
	ctx := context.Background()

	srcDir := filepath.Join(base, "src")
	headOID := initRepoWithCommit(t, srcDir)
	bareDir := filepath.Join(base, "bare.git")
	gitTest(t, base, "clone", "-q", "--bare", srcDir, bareDir)

	bareRepo, _, err := ws.Register(ctx, bareDir)
	if err != nil {
		t.Fatalf("Register bare: %v", err)
	}
	repoCtx, err := cv.RepositoryContext(ctx, bareRepo.ID)
	if err != nil {
		t.Fatalf("RepositoryContext bare: %v", err)
	}
	if repoCtx.Kind != "bare" || repoCtx.Browse || repoCtx.Diff || repoCtx.Write || repoCtx.Reason == "" {
		t.Fatalf("unexpected bare repository context: %+v", repoCtx)
	}
	bareFile, err := cv.CommitFile(ctx, Target{RepositoryID: bareRepo.ID}, repoCtx.ID, headOID, nodeID("README.md"))
	if err != nil || bareFile.Document.Text != "# Demo\nline 2\n" {
		t.Fatalf("CommitFile on bare repo: %+v %v", bareFile, err)
	}

	otherDir := filepath.Join(base, "other")
	initRepoWithCommit(t, otherDir)
	otherRepo, _, err := ws.Register(ctx, otherDir)
	if err != nil {
		t.Fatalf("Register other: %v", err)
	}
	// Cross-repository worktree ID must be rejected.
	if _, err := cv.WorktreeContext(ctx, bareRepo.ID, otherRepo.Worktrees[0].ID); errorKind(err) != ErrNotFound {
		t.Fatalf("cross-repo worktree ID returned %v, want ErrNotFound", err)
	}
	// Unregistered raw path passed as IDs must be rejected.
	unregistered := filepath.Join(base, "unregistered")
	if err := os.MkdirAll(unregistered, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := cv.RepositoryContext(ctx, unregistered); errorKind(err) != ErrNotFound {
		t.Fatalf("unregistered repository ID returned %v, want ErrNotFound", err)
	}
}

func TestWorktreeIsolationSessionDeletionAndDirectoryReplacement(t *testing.T) {
	t.Parallel()
	cv, ws, m, base := setupGitContextFixture(t)
	ctx := context.Background()

	repoDir := filepath.Join(base, "repo")
	initRepoWithCommit(t, repoDir)
	wtBPath := filepath.Join(base, "wt-b")
	gitTest(t, repoDir, "worktree", "add", "-q", "-b", "feat/b", wtBPath, "HEAD")

	repo, _, err := ws.Register(ctx, repoDir)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	var wtA, wtB workspace.Worktree
	for _, wt := range repo.Worktrees {
		switch wt.Path {
		case repoDir:
			wtA = wt
		case wtBPath:
			wtB = wt
		}
	}
	if wtA.ID == "" || wtB.ID == "" {
		t.Fatalf("missing worktrees: %+v", repo.Worktrees)
	}

	// Create and later delete a session on worktree A.
	sess, err := m.Create(session.CreateInput{Cwd: repoDir, Cmd: []string{"/bin/sh"}})
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}
	sessID := sess.Info().ID

	targetA := Target{RepositoryID: repo.ID, WorktreeID: wtA.ID}
	targetB := Target{RepositoryID: repo.ID, WorktreeID: wtB.ID}
	ctxA, err := cv.WorktreeContext(ctx, repo.ID, wtA.ID)
	if err != nil {
		t.Fatalf("WorktreeContext A: %v", err)
	}
	ctxB, err := cv.WorktreeContext(ctx, repo.ID, wtB.ID)
	if err != nil {
		t.Fatalf("WorktreeContext B: %v", err)
	}

	// Modify src.txt in worktree A and capture a diff snapshot.
	write(t, repoDir, "src.txt", "alpha\nchanged in A\n")
	changesA, err := cv.TargetChanges(ctx, targetA, ctxA.ID)
	if err != nil || len(changesA.Entries) != 1 {
		t.Fatalf("TargetChanges A: %+v %v", changesA, err)
	}
	diffA, err := cv.TargetDiff(ctx, targetA, ctxA.ID, changesA.Entries[0].ID)
	if err != nil {
		t.Fatalf("TargetDiff A: %v", err)
	}

	// Cross-worktree and cross-session snapshot reads MUST be rejected.
	if _, err := cv.TargetSnapshotFile(ctx, targetB, diffA.Snapshot, "new"); errorKind(err) != ErrInvalid {
		t.Fatalf("cross-worktree snapshot access returned %v, want ErrInvalid", err)
	}
	if _, err := cv.SnapshotFile(ctx, sessID, diffA.Snapshot, "new"); errorKind(err) != ErrInvalid {
		t.Fatalf("cross-session snapshot access returned %v, want ErrInvalid", err)
	}

	// Delete the last session in worktree A; wait past sweep interval (>1.2s).
	if err := m.Close(sessID); err != nil {
		t.Fatalf("Close session: %v", err)
	}
	select {
	case <-sess.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("session exit timeout")
	}
	if err := m.Remove(sessID); err != nil {
		t.Fatalf("Remove session: %v", err)
	}
	time.Sleep(1300 * time.Millisecond)

	// Worktree A's independent context and snapshot MUST still be alive!
	snapAfter, err := cv.TargetSnapshotFile(ctx, targetA, diffA.Snapshot, "new")
	if err != nil || snapAfter.Document.Text != "alpha\nchanged in A\n" {
		t.Fatalf("worktree snapshot evicted on session removal: %+v %v", snapAfter, err)
	}

	// Replace worktree B directory externally (e.g. symlink forgery or new dir) -> old context conflicts,
	// while repository-level context remains readable.
	movedB := filepath.Join(base, "wt-b-moved")
	if err := os.Rename(wtBPath, movedB); err != nil {
		t.Fatal(err)
	}
	fakeTarget := filepath.Join(base, "fake-dir")
	if err := os.MkdirAll(fakeTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fakeTarget, wtBPath); err != nil {
		t.Fatal(err)
	}
	if _, err := cv.TargetTree(ctx, targetB, ctxB.ID, "", false, ""); errorKind(err) != ErrConflict {
		t.Fatalf("symlink-replaced worktree B returned %v, want ErrConflict", err)
	}
	// Rebinding worktree B while symlinked also fails with ErrConflict without reading fakeTarget.
	cv.mu.Lock()
	delete(cv.roots, targetScope(targetB))
	cv.mu.Unlock()
	if _, err := cv.WorktreeContext(ctx, repo.ID, wtB.ID); !errors.Is(errorKind(err), ErrConflict) {
		t.Fatalf("rebinding symlinked worktree B returned %v, want ErrConflict", err)
	}

	// Repository-level context remains readable despite broken worktree B.
	repoCtx, err := cv.RepositoryContext(ctx, repo.ID)
	if err != nil || repoCtx.Kind != "repository" {
		t.Fatalf("RepositoryContext after broken worktree B: %+v %v", repoCtx, err)
	}
}
