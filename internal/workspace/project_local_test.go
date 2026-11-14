package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreatePlainProjectWithoutGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	parent := t.TempDir()
	s, _ := testService(t, t.TempDir(), parent)
	project, err := s.CreateProject(context.Background(), parent, "plain project", false)
	if err != nil || project.RepositoryID != "" {
		t.Fatalf("create: %+v %v", project, err)
	}
	if _, err := os.Stat(filepath.Join(project.Path, ".git")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected Git: %v", err)
	}
	p, created, err := s.OpenProject(context.Background(), project.Path)
	if err != nil || created || p.ID != project.ID {
		t.Fatalf("open again: %+v %v %v", p, created, err)
	}
	if _, err := s.CreateProject(context.Background(), parent, "plain project", false); !errors.Is(err, ErrConflict) {
		t.Fatalf("overwrite: %v", err)
	}
	if _, err := s.CreateProject(context.Background(), parent, "git project", true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing Git: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "git project")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("created without Git: %v", err)
	}
	for _, name := range []string{"", "..", ".git", "x/y", "x\\y", "name\n"} {
		if _, err := s.CreateProject(context.Background(), parent, name, false); !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted name %q: %v", name, err)
		}
	}
	if _, err := s.CreateProject(context.Background(), "", "unexpected", false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty parent: %v", err)
	}
	dirs, err := s.ProjectDirectories(parent + "/")
	if err != nil || len(dirs) != 1 || filepath.Base(dirs[0]) != "plain project" {
		t.Fatalf("browse: %v %v", dirs, err)
	}
}

func TestCreateGitProjectAndOpenSubdirectory(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	parent := t.TempDir()
	s, _ := testService(t, t.TempDir(), parent)
	project, err := s.CreateProject(context.Background(), parent, "new Git", true)
	if err != nil || project.RepositoryID == "" {
		t.Fatalf("initialize: %+v %v", project, err)
	}
	if branch := gitTest(t, project.Path, "symbolic-ref", "--short", "HEAD"); branch != "main" {
		t.Fatalf("branch %q", branch)
	}
	child := filepath.Join(project.Path, "subdirectory")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	opened, created, err := s.OpenProject(context.Background(), child)
	if err != nil || created || opened.ID != project.ID {
		t.Fatalf("subdirectory: %+v %v %v", opened, created, err)
	}
	// Creating a plain project inside a repository remains an explicit directory
	// project; reopening it cannot silently change it into the ancestor project.
	plain, err := s.CreateProject(context.Background(), project.Path, "plain", false)
	if err != nil {
		t.Fatal(err)
	}
	reopened, created, err := s.OpenProject(context.Background(), plain.Path)
	if err != nil || created || reopened.ID != plain.ID || reopened.RepositoryID != "" {
		t.Fatalf("plain changed: %+v %v", reopened, err)
	}
}

func TestProjectCreateRetainsDirectoryAfterSaveFailure(t *testing.T) {
	t.Parallel()
	state, parent := t.TempDir(), t.TempDir()
	s, _ := testService(t, state, parent)
	// Replace only the unpublished destination with a directory, forcing a
	// durable save failure without depending on the current process privileges.
	if err := os.Mkdir(filepath.Join(state, "projects.json"), 0700); err != nil {
		t.Fatal(err)
	}
	project, err := s.CreateProject(context.Background(), parent, "retained", false)
	if err == nil || project.Path == "" {
		t.Fatalf("missing recovery path: %+v %v", project, err)
	}
	if _, err := os.Stat(project.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(state, "projects.json")); err != nil {
		t.Fatal(err)
	}
	registered, _, err := s.OpenProject(context.Background(), project.Path)
	if err != nil || registered.ID == "" {
		t.Fatalf("register recovery: %+v %v", registered, err)
	}
}
