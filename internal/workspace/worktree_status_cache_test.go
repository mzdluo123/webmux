package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeStatusCacheReusesDormantAndRefreshesOnChangeOrExplicitRefresh(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)
	tree := linkedTest(t, s, r, "dormant")

	// Pure-Go common-dir resolution works for both main and linked worktrees.
	for _, p := range []string{root, tree.Path} {
		gitDir, commonDir, ok := worktreeCommonDir(p)
		if !ok || gitDir == "" || commonDir != r.CommonDir {
			t.Fatalf("worktreeCommonDir(%q) = (%q, %q, %v), want common=%q", p, gitDir, commonDir, ok, r.CommonDir)
		}
	}

	// Wrap git to count status and rev-parse invocations.
	logFile := filepath.Join(t.TempDir(), "git.log")
	realGit := s.gitPath
	wrapper := filepath.Join(t.TempDir(), "git")
	putFile(t, wrapper, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$3\" >> %q\nexec %q \"$@\"\n", logFile, realGit))
	if err := os.Chmod(wrapper, 0o700); err != nil {
		t.Fatal(err)
	}
	s.gitPath = wrapper

	countOp := func(op string) int {
		data, err := os.ReadFile(logFile)
		if err != nil {
			return 0
		}
		n := 0
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == op {
				n++
			}
		}
		return n
	}
	resetLog := func() { _ = os.Remove(logFile) }

	// Clear any status entries populated during register/create so the first
	// List exercises the cold path (2 status calls, 1 anchor rev-parse).
	repoState, err := s.repository(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	repoState.statusCacheMu.Lock()
	clear(repoState.statusCache)
	repoState.statusCacheMu.Unlock()

	resetLog()
	if _, err := s.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := countOp("status"); got != 2 {
		t.Fatalf("cold List status calls = %d, want 2", got)
	}
	if got := countOp("rev-parse"); got != 1 {
		t.Fatalf("cold List rev-parse calls = %d, want 1 (anchor only)", got)
	}

	// Second List immediately reuses cached status for untouched dormant worktrees.
	resetLog()
	if _, err := s.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := countOp("status"); got != 0 {
		t.Fatalf("warm List status calls = %d, want 0", got)
	}

	// Explicit Refresh bypasses cache and observes external edits immediately.
	putFile(t, filepath.Join(tree.Path, "tracked"), "external-edit\n")
	resetLog()
	refreshed, err := s.Refresh(context.Background(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := countOp("status"); got != 2 {
		t.Fatalf("explicit Refresh status calls = %d, want 2", got)
	}
	updated, err := findTree(refreshed, tree.ID)
	if err != nil || updated.Dirty == nil || !*updated.Dirty {
		t.Fatalf("explicit Refresh missed dirty state: %+v %v", updated, err)
	}

	// AnchorPath and WorktreeInfo avoid spawning git status completely.
	resetLog()
	anchor, err := s.AnchorPath(r.ID)
	if err != nil || anchor != r.AnchorPath {
		t.Fatalf("AnchorPath = (%q, %v), want %q", anchor, err, r.AnchorPath)
	}
	repoInfo, treeInfo, err := s.WorktreeInfo(context.Background(), r.ID, tree.ID)
	if err != nil || repoInfo.ID != r.ID || treeInfo == nil || treeInfo.ID != tree.ID {
		t.Fatalf("WorktreeInfo = (%+v, %+v, %v)", repoInfo, treeInfo, err)
	}
	if got := countOp("status"); got != 0 {
		t.Fatalf("AnchorPath+WorktreeInfo status calls = %d, want 0", got)
	}
}
