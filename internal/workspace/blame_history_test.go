package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileHistoryFollowRenameAndBlameReadOnlySafety(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	ctx := context.Background()

	dir := initRepo(t)
	// Configure a malicious textconv driver in .git/config to verify ReadGit neutralizes it during Blame!
	markerFile := filepath.Join(dir, "pwned-by-textconv")
	gitTest(t, dir, "config", "diff.evil.textconv", "touch "+markerFile)
	putFile(t, filepath.Join(dir, ".gitattributes"), "*.go diff=evil\n")
	putFile(t, filepath.Join(dir, "legacy.go"), "package engine\n\nfunc Run() string {\n\treturn \"v1\"\n}\n")
	gitTest(t, dir, "add", ".gitattributes", "legacy.go")
	gitTest(t, dir, "commit", "-m", "feat: add legacy.go")
	c1 := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// Rename legacy.go -> engine.go and modify line 4 so blame attributes lines across both commits!
	gitTest(t, dir, "mv", "legacy.go", "engine.go")
	putFile(t, filepath.Join(dir, "engine.go"), "package engine\n\nfunc Run() string {\n\treturn \"v2-renamed\"\n}\n")
	gitTest(t, dir, "add", "-A")
	gitTest(t, dir, "commit", "-m", "refactor: rename legacy.go to engine.go")
	c2 := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	// Add binary file + symlink to verify graceful Blame degradation.
	if err := os.WriteFile(filepath.Join(dir, "icon.bin"), []byte{0x00, 0x01, 0x02, 0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("engine.go", filepath.Join(dir, "engine.link")); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", "icon.bin", "engine.link")
	gitTest(t, dir, "commit", "-m", "chore: add binary and symlink")
	c3 := strings.TrimSpace(gitTest(t, dir, "rev-parse", "HEAD"))

	s, _ := testService(t, t.TempDir(), dir)
	repo := registerTest(t, s, dir)
	wtID := repo.Worktrees[0].ID

	// 1. FileHistory (`--follow`) walks across `git mv` rename and paginates accurately!
	page1, err := s.FileHistory(ctx, FileHistoryQuery{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Path:         "engine.go",
		Limit:        1,
	})
	if err != nil {
		t.Fatalf("FileHistory page1: %v", err)
	}
	if len(page1.Commits) != 1 || page1.NextCursor == "" {
		t.Fatalf("expected 1 commit and NextCursor on page1, got %+v", page1)
	}
	if page1.Commits[0].OID != c2 || page1.Commits[0].Status != "R" || page1.Commits[0].Path != "engine.go" || page1.Commits[0].OldPath != "legacy.go" {
		t.Fatalf("expected rename metadata (legacy.go -> engine.go) on commit c2, got %+v", page1.Commits[0])
	}

	page2, err := s.FileHistory(ctx, FileHistoryQuery{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Path:         "engine.go",
		Limit:        5,
		Cursor:       page1.NextCursor,
	})
	if err != nil {
		t.Fatalf("FileHistory page2: %v", err)
	}
	if len(page2.Commits) != 1 || page2.NextCursor != "" || page2.Commits[0].OID != c1 || page2.Commits[0].Path != "legacy.go" {
		t.Fatalf("expected pre-rename commit c1 with Path=legacy.go on page2, got %+v", page2)
	}

	// 2. Blame on engine.go at c3 attributes unchanged lines to c1 (with PreviousOID/OrigPath) and line 4 to c2!
	blameRes, err := s.Blame(ctx, BlameQuery{
		RepositoryID: repo.ID,
		WorktreeID:   wtID,
		Rev:          c3,
		Path:         "engine.go",
	})
	if err != nil {
		t.Fatalf("Blame engine.go: %v", err)
	}
	if _, statErr := os.Stat(markerFile); statErr == nil {
		t.Fatalf("SECURITY FAILURE: diff.evil.textconv executed during Blame!")
	}
	if blameRes.Kind != "text" || blameRes.TotalLines != 5 || len(blameRes.Lines) != 5 || len(blameRes.Commits) != 2 {
		t.Fatalf("unexpected BlameResult shape: %+v", blameRes)
	}
	if blameRes.Lines[0].CommitOID != c1 || blameRes.Lines[0].OrigPath != "legacy.go" {
		t.Fatalf("expected line 1 attributed to c1 (legacy.go), got %+v", blameRes.Lines[0])
	}
	if blameRes.Lines[3].CommitOID != c2 || blameRes.Lines[3].PreviousOID != c1 || blameRes.Lines[3].PreviousPath != "legacy.go" {
		t.Fatalf("expected line 4 attributed to c2 with PreviousOID=c1 PreviousPath=legacy.go, got %+v", blameRes.Lines[3])
	}

	// Re-blaming parent (`c1` at `legacy.go`) succeeds seamlessly!
	parentBlame, err := s.Blame(ctx, BlameQuery{
		RepositoryID: repo.ID,
		Rev:          blameRes.Lines[3].PreviousOID,
		Path:         blameRes.Lines[3].PreviousPath,
	})
	if err != nil || parentBlame.Kind != "text" || parentBlame.Lines[3].Text != "\treturn \"v1\"" {
		t.Fatalf("re-blame parent failed: %+v %v", parentBlame, err)
	}

	// 3. Binary & symlink degrade gracefully without error.
	binBlame, err := s.Blame(ctx, BlameQuery{
		RepositoryID: repo.ID,
		Rev:          c3,
		Path:         "icon.bin",
	})
	if err != nil || binBlame.Kind != "binary" || binBlame.DegradeReason == "" {
		t.Fatalf("expected binary degrade, got %+v %v", binBlame, err)
	}
	linkBlame, err := s.Blame(ctx, BlameQuery{
		RepositoryID: repo.ID,
		Rev:          c3,
		Path:         "engine.link",
	})
	if err != nil || linkBlame.Kind != "unsupported" || linkBlame.DegradeReason == "" {
		t.Fatalf("expected symlink unsupported degrade, got %+v %v", linkBlame, err)
	}
}
