package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHistoryScopesUnbornNoUpstreamAndPagination(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	// 1. Unborn / empty repository states.
	emptyDir := filepath.Join(t.TempDir(), "empty-repo")
	if err := os.MkdirAll(emptyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, emptyDir, "init", "-b", "main")
	s, _ := testService(t, t.TempDir(), emptyDir)
	emptyRepo := registerTest(t, s, emptyDir)

	allEmpty, err := s.History(ctx, HistoryQuery{RepositoryID: emptyRepo.ID, Scope: HistoryScopeAll})
	if err != nil {
		t.Fatalf("History all on empty repo: %v", err)
	}
	if allEmpty.Status != HistoryStatusEmpty || allEmpty.Reason == "" || len(allEmpty.Commits) != 0 {
		t.Fatalf("expected HistoryStatusEmpty, got %+v", allEmpty)
	}

	branchUnborn, err := s.History(ctx, HistoryQuery{
		RepositoryID: emptyRepo.ID,
		WorktreeID:   emptyRepo.Worktrees[0].ID,
		Scope:        HistoryScopeBranch,
	})
	if err != nil {
		t.Fatalf("History branch on unborn repo: %v", err)
	}
	if branchUnborn.Status != HistoryStatusUnborn || branchUnborn.Reason == "" {
		t.Fatalf("expected HistoryStatusUnborn, got %+v", branchUnborn)
	}

	noWTBranch, err := s.History(ctx, HistoryQuery{
		RepositoryID: emptyRepo.ID,
		Scope:        HistoryScopeBranch,
	})
	if err != nil || noWTBranch.Status != HistoryStatusNoWorktree {
		t.Fatalf("expected HistoryStatusNoWorktree, got %+v %v", noWTBranch, err)
	}

	// 2. Upstream states (no upstream vs configured upstream to-push / incoming).
	workDir := initRepo(t)
	repo := registerTest(t, s, workDir)
	wtID := repo.Worktrees[0].ID

	noUpToPush, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        HistoryScopeToPush,
	})
	if err != nil || noUpToPush.Status != HistoryStatusNoUpstream || noUpToPush.Reason == "" {
		t.Fatalf("expected HistoryStatusNoUpstream for to-push, got %+v %v", noUpToPush, err)
	}
	noUpIncoming, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        HistoryScopeIncoming,
	})
	if err != nil || noUpIncoming.Status != HistoryStatusNoUpstream || noUpIncoming.Reason == "" {
		t.Fatalf("expected HistoryStatusNoUpstream for incoming, got %+v %v", noUpIncoming, err)
	}

	// Attach a bare remote, push main, then create 3 local ahead commits and 1 fetched remote commit.
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remoteDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, remoteDir, "init", "--bare", "-b", "main")
	gitTest(t, workDir, "remote", "add", "origin", remoteDir)
	gitTest(t, workDir, "push", "-u", "origin", "main")

	peerDir := filepath.Join(t.TempDir(), "peer")
	if err := os.MkdirAll(peerDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, peerDir, "clone", "-b", "main", remoteDir, ".")
	putFile(t, filepath.Join(peerDir, "remote.txt"), "from peer\n")
	gitTest(t, peerDir, "add", "remote.txt")
	gitTest(t, peerDir, "commit", "-m", "peer upstream commit")
	peerOID := gitTest(t, peerDir, "rev-parse", "HEAD")
	gitTest(t, peerDir, "push", "origin", "HEAD:main")
	gitTest(t, workDir, "fetch", "origin")

	for i := 1; i <= 3; i++ {
		putFile(t, filepath.Join(workDir, "local.txt"), fmt.Sprintf("step %d\n", i))
		gitTest(t, workDir, "add", "local.txt")
		gitTest(t, workDir, "commit", "-m", fmt.Sprintf("local step %d", i))
	}

	toPush, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        HistoryScopeToPush,
	})
	if err != nil || toPush.Status != HistoryStatusReady || len(toPush.Commits) != 3 || toPush.LastFetchedAt == nil {
		t.Fatalf("unexpected to-push page: %+v %v", toPush, err)
	}
	incoming, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        HistoryScopeIncoming,
	})
	if err != nil || incoming.Status != HistoryStatusReady || len(incoming.Commits) != 1 || incoming.Commits[0].OID != peerOID {
		t.Fatalf("unexpected incoming page: %+v %v", incoming, err)
	}

	// 3. Continuous pagination without duplicates or gaps, plus refsChanged detection.
	page1, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        HistoryScopeBranch,
		Limit:        2,
	})
	if err != nil || len(page1.Commits) != 2 || page1.NextCursor == "" {
		t.Fatalf("page1: %+v %v", page1, err)
	}
	// Advance HEAD while paginating: page2 should still continue consistently from page1's anchor and report RefsChanged=true!
	putFile(t, filepath.Join(workDir, "after-page1.txt"), "new tip\n")
	gitTest(t, workDir, "add", "after-page1.txt")
	gitTest(t, workDir, "commit", "-m", "new tip after page1")

	page2, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Scope:        HistoryScopeBranch,
		Limit:        2,
		Cursor:       page1.NextCursor,
	})
	if err != nil || !page2.RefsChanged || len(page2.Commits) != 2 || page2.NextCursor != "" {
		t.Fatalf("page2: %+v %v", page2, err)
	}
	seen := map[string]bool{}
	for _, c := range append(page1.Commits, page2.Commits...) {
		if seen[c.OID] {
			t.Fatalf("duplicate commit OID across pages: %s", c.OID)
		}
		seen[c.OID] = true
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 distinct commits from anchored pagination, got %d", len(seen))
	}
}

func TestHistoryMergeGraphBoundaryIndirectEdgesAndCommitDetail(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := filepath.Join(t.TempDir(), "graph-repo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Root commit A touches cmd/main.go
	putFile(t, filepath.Join(dir, "cmd/main.go"), "package main\n// A\n")
	gitTest(t, dir, "add", "cmd/main.go")
	gitTestEnv(t, dir, []string{
		"GIT_AUTHOR_DATE=2026-01-01T10:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T10:00:00Z",
	}, "commit", "-m", "commit A")
	oidA := gitTest(t, dir, "rev-parse", "HEAD")

	// Branch topic from A and commit T1
	gitTest(t, dir, "checkout", "-b", "topic")
	putFile(t, filepath.Join(dir, "topic.txt"), "topic work\n")
	if err := os.WriteFile(filepath.Join(dir, "icon.bin"), []byte{0x00, 0x01, 0x02}, 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", "topic.txt", "icon.bin")
	gitTestEnv(t, dir, []string{
		"GIT_AUTHOR_DATE=2026-01-01T11:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T11:00:00Z",
	}, "commit", "-m", "topic T1")
	oidT1 := gitTest(t, dir, "rev-parse", "HEAD")

	// Switch back to main: commit B touches other.txt (NOT cmd/main.go), commit C touches cmd/main.go
	gitTest(t, dir, "checkout", "main")
	putFile(t, filepath.Join(dir, "other.txt"), "middle B\n")
	gitTest(t, dir, "add", "other.txt")
	gitTestEnv(t, dir, []string{
		"GIT_AUTHOR_DATE=2026-01-01T12:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T12:00:00Z",
	}, "commit", "-m", "commit B")
	oidB := gitTest(t, dir, "rev-parse", "HEAD")

	putFile(t, filepath.Join(dir, "cmd/main.go"), "package main\n// C\n")
	gitTest(t, dir, "add", "cmd/main.go")
	gitTestEnv(t, dir, []string{
		"GIT_AUTHOR_DATE=2026-01-01T13:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T13:00:00Z",
	}, "commit", "-m", "commit C")
	oidC := gitTest(t, dir, "rev-parse", "HEAD")

	// Merge topic into main with distinct author vs committer!
	gitTestEnv(t, dir, []string{
		"GIT_AUTHOR_NAME=Alice Author",
		"GIT_AUTHOR_EMAIL=alice@example.invalid",
		"GIT_AUTHOR_DATE=2026-01-01T14:00:00Z",
		"GIT_COMMITTER_NAME=Bob Committer",
		"GIT_COMMITTER_EMAIL=bob@example.invalid",
		"GIT_COMMITTER_DATE=2026-01-01T14:05:00Z",
	}, "merge", "--no-ff", "topic", "-m", "Merge branch 'topic'\n\nDetailed merge body.")
	oidM := gitTest(t, dir, "rev-parse", "HEAD")

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)

	// 1. Paginate with Limit=2: in --topo-order, merge commit M (parents [C, T1])
	// and second parent T1 appear on page 1, while first parent C appears on page 2!
	p1, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		Scope:        HistoryScopeAll,
		Limit:        2,
	})
	if err != nil {
		t.Fatalf("History page 1: %v", err)
	}
	if len(p1.Commits) != 2 || p1.Commits[0].OID != oidM || p1.Commits[1].OID != oidT1 {
		t.Fatalf("unexpected page 1 commits: %+v", p1.Commits)
	}
	mRow := p1.Commits[0]
	if len(mRow.Parents) != 2 || mRow.Parents[0] != oidC || mRow.Parents[1] != oidT1 {
		t.Fatalf("merge row parents mismatch: %+v", mRow.Parents)
	}
	var edgeToC, edgeToT1 *HistoryEdge
	for i := range mRow.Edges {
		switch mRow.Edges[i].TargetOID {
		case oidC:
			edgeToC = &mRow.Edges[i]
		case oidT1:
			edgeToT1 = &mRow.Edges[i]
		}
	}
	if edgeToT1 == nil || edgeToT1.Kind != "direct" {
		t.Fatalf("expected direct edge from M to T1 in page 1, got %+v", edgeToT1)
	}
	if edgeToC == nil || edgeToC.Kind != "boundary" {
		t.Fatalf("expected boundary edge from M to C across page 1->2, got %+v", edgeToC)
	}

	p2, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		Scope:        HistoryScopeAll,
		Limit:        3,
		Cursor:       p1.NextCursor,
	})
	if err != nil {
		t.Fatalf("History page 2: %v", err)
	}
	var cRow *HistoryRow
	for i := range p2.Commits {
		if p2.Commits[i].OID == oidC {
			cRow = &p2.Commits[i]
			break
		}
	}
	if cRow == nil || cRow.Lane != edgeToC.ToLane {
		t.Fatalf("expected C in page 2 to continue lane %d, got %+v", edgeToC.ToLane, cRow)
	}

	// 2. Path filter `cmd/main.go` on C -> B -> A: B is omitted!
	// C MUST still carry real parent B, mark DirectParentsMissing=true, and emit an "indirect" edge to A (never "direct").
	filtered, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		Branch:       "main",
		Scope:        HistoryScopeBranch,
		Path:         "cmd/main.go",
	})
	if err != nil {
		t.Fatalf("History path-filtered: %v", err)
	}
	if len(filtered.Commits) != 2 || filtered.Commits[0].OID != oidC || filtered.Commits[1].OID != oidA {
		t.Fatalf("unexpected filtered commits: %+v", filtered.Commits)
	}
	cFiltered := filtered.Commits[0]
	if len(cFiltered.Parents) != 1 || cFiltered.Parents[0] != oidB {
		t.Fatalf("filtered commit C lost real parent B (%s): %+v", oidB, cFiltered.Parents)
	}
	if !cFiltered.DirectParentsMissing {
		t.Fatalf("expected DirectParentsMissing=true on filtered commit C")
	}
	if len(cFiltered.Edges) != 1 || cFiltered.Edges[0].TargetOID != oidA || cFiltered.Edges[0].Kind != "indirect" {
		t.Fatalf("expected indirect edge C -> A when B is filtered out, got %+v", cFiltered.Edges)
	}

	// 3. CommitDetail: root commit A, merge commit M against default parent 1 (C) vs explicit parent 2 (T1),
	// and distinct author vs committer.
	rootDetail, err := s.CommitDetail(ctx, repo.ID, oidA, "")
	if err != nil {
		t.Fatalf("CommitDetail root A: %v", err)
	}
	if len(rootDetail.Parents) != 0 || len(rootDetail.Files) != 1 || rootDetail.Files[0].NewPath != "cmd/main.go" || rootDetail.Files[0].Status != "A" {
		t.Fatalf("unexpected root commit detail: %+v", rootDetail)
	}

	mergeDefault, err := s.CommitDetail(ctx, repo.ID, oidM, "")
	if err != nil {
		t.Fatalf("CommitDetail M default parent: %v", err)
	}
	if !mergeDefault.DistinctCommitter || mergeDefault.AuthorName != "Alice Author" || mergeDefault.CommitterName != "Bob Committer" {
		t.Fatalf("expected distinct author/committer on merge M, got %+v", mergeDefault)
	}
	if mergeDefault.SelectedParent != oidC {
		t.Fatalf("expected default SelectedParent=%s, got %s", oidC, mergeDefault.SelectedParent)
	}
	// Relative to C (parent 1), M brought in topic.txt and icon.bin (binary).
	defaultFiles := map[string]CommitFileChange{}
	for _, f := range mergeDefault.Files {
		defaultFiles[f.NewPath] = f
	}
	if defaultFiles["topic.txt"].Status != "A" || !defaultFiles["icon.bin"].Binary {
		t.Fatalf("unexpected M vs P1 files: %+v", defaultFiles)
	}

	// Relative to T1 (parent 2), M brought in other.txt and modified cmd/main.go.
	mergeP2, err := s.CommitDetail(ctx, repo.ID, oidM, oidT1)
	if err != nil {
		t.Fatalf("CommitDetail M parent 2: %v", err)
	}
	if mergeP2.SelectedParent != oidT1 {
		t.Fatalf("expected SelectedParent=%s, got %s", oidT1, mergeP2.SelectedParent)
	}
	p2Files := map[string]CommitFileChange{}
	for _, f := range mergeP2.Files {
		p2Files[f.NewPath] = f
	}
	if p2Files["other.txt"].Status != "A" || p2Files["cmd/main.go"].Status != "M" {
		t.Fatalf("unexpected M vs P2 files: %+v", p2Files)
	}
	if _, hasTopic := p2Files["topic.txt"]; hasTopic {
		t.Fatalf("topic.txt should not appear in T1 -> M diff: %+v", p2Files)
	}
}

func TestHistoryReadOnlyNoExtDiffTextconvOrIndexTouch(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	marker := filepath.Join(t.TempDir(), "driver-ran")
	helperScript := filepath.Join(t.TempDir(), "evil-helper.sh")
	scriptBody := fmt.Sprintf("#!/bin/sh\nprintf 'ran' > %q\nexit 0\n", marker)
	if err := os.WriteFile(helperScript, []byte(scriptBody), 0o700); err != nil {
		t.Fatal(err)
	}

	// Configure diff.external and diff.*.textconv in repo config, plus dirty unstaged file.
	gitTest(t, dir, "config", "diff.external", helperScript)
	gitTest(t, dir, "config", "diff.custom.textconv", helperScript)
	putFile(t, filepath.Join(dir, ".gitattributes"), "* diff=custom\n")
	gitTest(t, dir, "add", ".gitattributes")
	gitTest(t, dir, "commit", "-m", "configure custom diff")
	headOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)

	// Dirty tracked file AFTER registration (since Register runs git status),
	// without refreshing index stat, and record .git/index bytes + mtime.
	putFile(t, filepath.Join(dir, "tracked"), "dirty unstaged content\n")
	indexPath := filepath.Join(dir, ".git", "index")
	oldTime := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(indexPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	beforeStat, err := os.Stat(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeBytes, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		Scope:        HistoryScopeAll,
		Path:         "tracked",
	}); err != nil {
		t.Fatalf("History: %v", err)
	}
	if _, err := s.CommitDetail(ctx, repo.ID, headOID, ""); err != nil {
		t.Fatalf("CommitDetail: %v", err)
	}

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("custom diff/textconv helper executed during read-only history/detail! err=%v", err)
	}
	afterStat, err := os.Stat(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	afterBytes, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !afterStat.ModTime().Equal(beforeStat.ModTime()) || string(afterBytes) != string(beforeBytes) {
		t.Fatalf(".git/index was modified by read-only history/detail: mtime %v -> %v", beforeStat.ModTime(), afterStat.ModTime())
	}
}

func TestBranchesTagsAndHistoryTagFiltering(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	firstOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))
	gitTest(t, dir, "tag", "v0.1.0-light")

	putFile(t, filepath.Join(dir, "second.txt"), "second\n")
	gitTest(t, dir, "add", "second.txt")
	gitTest(t, dir, "commit", "-m", "second release commit")
	secondOID := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))
	gitTest(t, dir, "tag", "-a", "v1.0.0", "-m", "Annotated v1.0.0")

	putFile(t, filepath.Join(dir, "third.txt"), "third\n")
	gitTest(t, dir, "add", "third.txt")
	gitTest(t, dir, "commit", "-m", "post release work")
	headBefore := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)

	branches, err := s.Branches(ctx, repo.ID)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	tagMap := map[string]Tag{}
	for _, tag := range branches.Tags {
		tagMap[tag.Name] = tag
	}
	if tagMap["v0.1.0-light"].Head != firstOID {
		t.Fatalf("lightweight tag head = %q, want %q", tagMap["v0.1.0-light"].Head, firstOID)
	}
	if tagMap["v1.0.0"].Head != secondOID {
		t.Fatalf("annotated tag peeled head = %q, want %q", tagMap["v1.0.0"].Head, secondOID)
	}

	// Filtering history by annotated tag returns commits up to secondOID without mutating HEAD.
	tagHist, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		Scope:        HistoryScopeAll,
		Branch:       "v1.0.0",
	})
	if err != nil {
		t.Fatalf("History by tag: %v", err)
	}
	if tagHist.Status != HistoryStatusReady || len(tagHist.Commits) != 2 || tagHist.Commits[0].OID != secondOID {
		t.Fatalf("unexpected tag history: %+v", tagHist)
	}

	// Requesting upstream delta scope (to-push) on a tag returns HistoryStatusNoUpstream with clear reason.
	tagToPush, err := s.History(ctx, HistoryQuery{
		RepositoryID: repo.ID,
		Scope:        HistoryScopeToPush,
		Branch:       "v1.0.0",
	})
	if err != nil {
		t.Fatalf("History to-push by tag: %v", err)
	}
	if tagToPush.Status != HistoryStatusNoUpstream || tagToPush.Reason == "" {
		t.Fatalf("expected HistoryStatusNoUpstream for tag to-push, got %+v", tagToPush)
	}

	headAfter := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))
	if headAfter != headBefore {
		t.Fatalf("HEAD changed during tag history filter: %s -> %s", headBefore, headAfter)
	}
}
