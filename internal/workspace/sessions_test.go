package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

func TestMultiplePTYsKeepRealChildCwdAndIndependentBuffers(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, m := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)
	tree := linkedTest(t, s, r, "sessions")
	child := filepath.Join(tree.Path, "packages", "app")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	first, err := s.CreateSession(context.Background(), sessioncreate.Input{Cwd: child, Cmd: []string{"/bin/sh", "-c", "printf FIRST; read value; printf 'GOT:%s' \"$value\""}, Name: "first", Workspace: &sessioncreate.Reference{RepositoryID: r.ID, WorktreeID: tree.ID}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateSession(context.Background(), sessioncreate.Input{Cwd: child, Cmd: []string{"/bin/sh", "-c", "printf SECOND; read value"}, Name: "second"})
	if err != nil {
		t.Fatal(err)
	}
	waitOutput(t, first, "FIRST")
	waitOutput(t, second, "SECOND")
	if first.Info().Cwd != child || s.Association(first.Info().ID) == nil || s.Association(second.Info().ID) == nil || first.Info().ID == second.Info().ID {
		t.Fatalf("independent launch metadata: %+v %+v", first.Info(), second.Info())
	}
	if err := first.Write([]byte("one\n")); err != nil {
		t.Fatal(err)
	}
	waitDone(t, first)
	if !strings.Contains(string(first.Buffer()), "GOT:one") || strings.Contains(string(second.Buffer()), "GOT:one") || second.Info().Status == session.StatusExited {
		t.Fatal("PTY input/buffer/exit isolation failed")
	}
	if err := m.Remove(first.Info().ID); err != nil {
		t.Fatal(err)
	}
	assertExists(t, tree.Path)
	// Workspace metadata updates are delivered through the existing list stream.
	sub := m.Subscribe()
	defer m.Unsubscribe(sub)
	<-sub.Out
	if err := s.Unregister(r.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-sub.Out:
		if event.Type != "updated" || event.Session == nil || event.Session.ID != second.Info().ID || s.Association(event.Session.ID) != nil {
			t.Fatalf("regroup event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("workspace regroup event missing")
	}
	if second.Info().Status == session.StatusExited {
		t.Fatal("regroup stopped independent PTY")
	}
}
func TestExplicitWorkspaceRequiresKnownRepositoryBeforeCreatingSession(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, m := testService(t, t.TempDir(), root)
	registerTest(t, s, root)
	creator := newCreateFixture(s, nil, nil)
	t.Cleanup(creator.Close)
	for _, tc := range []struct {
		name      string
		workspace *sessioncreate.Reference
		worktree  *sessioncreate.NewWorktree
	}{
		{name: "empty workspace", workspace: &sessioncreate.Reference{}},
		{name: "empty new worktree", worktree: &sessioncreate.NewWorktree{Mode: "new", Base: "HEAD", Branch: "must-not-exist"}},
		{name: "unknown workspace", workspace: &sessioncreate.Reference{RepositoryID: "missing"}},
		{name: "unknown new worktree", worktree: &sessioncreate.NewWorktree{RepositoryID: "missing", Mode: "new", Base: "HEAD", Branch: "must-not-exist"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "started")
			result, err := creator.Create(context.Background(), sessioncreate.Input{
				Cmd:         []string{"/bin/sh", "-c", `printf started > "$1"`, "sh", marker},
				Workspace:   tc.workspace,
				NewWorktree: tc.worktree,
			})
			failed := quickFailure(t, err, sessioncreate.StepPrecheck, ErrNotFound)
			if result.Session != nil || result.Location != nil || failed.SessionID != "" || failed.Location != nil {
				t.Fatalf("rejected creation returned created resources: %+v, %+v", result, failed)
			}
			if len(m.List()) != 0 {
				t.Fatalf("rejected creation started sessions: %+v", m.List())
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("rejected creation ran command: %v", err)
			}
			if trees := strings.Count(gitTest(t, root, "worktree", "list", "--porcelain"), "worktree "); trees != 1 {
				t.Fatalf("rejected creation made worktrees: %d", trees)
			}
			if branches := gitTest(t, root, "branch", "--list", "must-not-exist"); strings.TrimSpace(branches) != "" {
				t.Fatalf("rejected creation made branch: %s", branches)
			}
		})
	}
	direct, err := creator.Create(context.Background(), sessioncreate.Input{
		Cwd: root, Cmd: []string{"/bin/sh", "-c", "printf CWD_ONLY"},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, direct.Session)
	if direct.Session.Info().Cwd != root || !strings.Contains(string(direct.Session.Buffer()), "CWD_ONLY") || len(m.List()) != 1 {
		t.Fatalf("cwd-only creation did not run normally: %+v", direct.Session.Info())
	}
}

func TestExplicitWorkspaceNeverFallsBackOrMatchesNestedRepository(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)
	tree := linkedTest(t, s, r, "explicit")
	nested := filepath.Join(tree.Path, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, nested, "init")
	for _, cwd := range []string{root, nested} {
		if _, err := s.CreateSession(context.Background(), sessioncreate.Input{Cwd: cwd, Cmd: []string{"/bin/sh"}, Workspace: &sessioncreate.Reference{RepositoryID: r.ID, WorktreeID: tree.ID}}); err == nil {
			t.Fatalf("conflicting cwd accepted: %q", cwd)
		}
	}
	direct, err := s.CreateSession(context.Background(), sessioncreate.Input{Cwd: nested, Cmd: []string{"/bin/sh", "-c", "printf NESTED"}})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, direct)
	if s.Association(direct.Info().ID) != nil {
		t.Fatal("nested unregistered repository matched outer worktree")
	}
	prior, err := s.CreateSession(context.Background(), sessioncreate.Input{Cwd: tree.Path, Cmd: []string{"/bin/sh", "-c", "printf OLD; read value"}})
	if err != nil {
		t.Fatal(err)
	}
	waitOutput(t, prior, "OLD")
	if err := os.Rename(tree.Path, tree.Path+"-gone"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSession(context.Background(), sessioncreate.Input{Cmd: []string{"/bin/sh"}, Workspace: &sessioncreate.Reference{RepositoryID: r.ID, WorktreeID: tree.ID}}); err == nil {
		t.Fatal("missing selected tree fell back to default")
	}
	if _, err := s.Refresh(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	if s.Association(prior.Info().ID) == nil || s.Association(prior.Info().ID).WorktreeID != tree.ID || prior.Info().Status == session.StatusExited {
		t.Fatal("unavailable workspace session lost association or process")
	}
}
func TestCloseTimeoutDoesNotDeleteTreeAndReleasesReservation(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)
	tree := linkedTest(t, s, r, "close-timeout")
	item, err := s.CreateSession(context.Background(), sessioncreate.Input{Cwd: tree.Path, Cmd: []string{"/bin/bash", "-c", `trap '' HUP; printf READY; while :; do sleep 1; done`}})
	if err != nil {
		t.Fatal(err)
	}
	waitOutput(t, item, "READY")
	preview := previewTest(t, s, r, tree)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = s.RemoveWorktree(ctx, r.ID, tree.ID, forceRequest(preview))
	if errorKind(err) != ErrTimeout {
		t.Fatalf("close timeout incorrectly succeeded: %v", err)
	}
	assertExists(t, tree.Path)
	waitDone(t, item)
	next, err := s.CreateSession(context.Background(), sessioncreate.Input{Cwd: tree.Path, Cmd: []string{"/bin/sh", "-c", "printf RELEASED"}})
	if err != nil {
		t.Fatal("timeout retained reservation", err)
	}
	waitDone(t, next)
}
