package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

func TestCreateGitProjectPromotesRetainedDirectory(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	parent := t.TempDir()
	target := filepath.Join(parent, "retained")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	s, _ := testService(t, t.TempDir(), parent)
	original, _, err := s.RegisterDirectoryProject(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	project, err := s.CreateProject(context.Background(), parent, "retained", true)
	if err != nil || project.RepositoryID == "" || project.ID != original.ID {
		t.Fatalf("init masked by retained record: %+v %v", project, err)
	}
	repositories, err := s.List(context.Background())
	if err != nil || len(repositories) != 1 || repositories[0].ID != project.RepositoryID {
		t.Fatalf("repository not registered: %+v %v", repositories, err)
	}
}

func TestOpenProjectRejectsFileBeforeGit(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	s, _ := testService(t, t.TempDir(), t.TempDir())
	// A Git executable is deliberately present; the directory validation must
	// run before invoking it and return the same invalid-path category.
	s.gitPath = "/definitely-not-a-git-command"
	if _, _, err := s.OpenProject(context.Background(), path); !errors.Is(err, ErrInvalid) {
		t.Fatalf("file classified as Git failure: %v", err)
	}
}

func TestCreateProjectPermissionFailureDoesNotPublish(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	s, _ := testService(t, t.TempDir(), parent)
	// A per-service filesystem port also tests EACCES when the suite runs as root.
	s.mkdirProject = func(path string) error { return &os.PathError{Op: "mkdir", Path: path, Err: os.ErrPermission} }
	if _, err := s.CreateProject(context.Background(), parent, "denied", false); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("permission failure: %v", err)
	}
	projects, err := s.ListProjects()
	if err != nil || len(projects) != 0 {
		t.Fatalf("permission failure published: %+v %v", projects, err)
	}
	if _, err := os.Stat(filepath.Join(parent, "denied")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected directory: %v", err)
	}
}

func TestOpenProjectDoesNotReadPostCommitStatus(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("requires Git")
	}
	marker := filepath.Join(t.TempDir(), "status-was-read")
	t.Setenv("WEBMUX_TEST_GIT", realGit)
	t.Setenv("WEBMUX_TEST_STATUS_MARKER", marker)
	dir := initRepo(t)
	wrapper := filepath.Join(t.TempDir(), "git-wrapper")
	script := `#!/bin/sh
for arg do
 if [ "$arg" = status ]; then
  printf visited > "$WEBMUX_TEST_STATUS_MARKER"
  exit 1
 fi
done
exec "$WEBMUX_TEST_GIT" "$@"
`
	if err := os.WriteFile(wrapper, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	s, _ := testService(t, t.TempDir(), dir)
	s.gitPath = wrapper
	project, created, err := s.OpenProject(context.Background(), dir)
	if err != nil || !created || project.RepositoryID == "" {
		t.Fatalf("registration: %+v %v %v", project, created, err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("registration read Git status after commit: %v", err)
	}
}
