package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rainchan/webmux/internal/statefile"
)

func TestDirectoryProjectsSurviveRestartWithoutGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	state, dir := t.TempDir(), t.TempDir()
	s := New(state, nil, nil, dir)
	if s.Capability().GitAvailable {
		t.Fatal("test needs Git unavailable")
	}
	p, created, err := s.RegisterDirectoryProject(dir)
	if err != nil || !created {
		t.Fatalf("register: %+v %v %v", p, created, err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}
	duplicate, created, err := s.RegisterDirectoryProject(alias)
	if err != nil || created || duplicate.ID != p.ID {
		t.Fatalf("duplicate: %+v %v %v", duplicate, created, err)
	}
	s.Close()
	s = New(state, nil, nil, dir)
	defer s.Close()
	projects, err := s.ListProjects()
	if err != nil || len(projects) != 1 || projects[0].ID != p.ID {
		t.Fatalf("reload: %+v %v", projects, err)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	projects, err = s.ListProjects()
	if err != nil || projects[0].Availability != "unavailable" || projects[0].Error == "" {
		t.Fatalf("missing: %+v %v", projects, err)
	}
}

func TestProjectsOverlayLegacyRepositoriesWithoutRewriting(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	state, dir := t.TempDir(), initRepo(t)
	s, manager := testService(t, state, dir)
	repository := registerTest(t, s, dir)
	original, err := os.ReadFile(filepath.Join(state, "repositories.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = New(state, manager, nil, dir)
	defer s.Close()
	projects, err := s.ListProjects()
	if err != nil || len(projects) != 1 || projects[0].RepositoryID != repository.ID {
		t.Fatalf("legacy: %+v %v", projects, err)
	}
	id := projects[0].ID
	duplicate, created, err := s.RegisterDirectoryProject(dir)
	if err != nil || created || duplicate.ID != id {
		t.Fatalf("duplicate repo: %+v %v %v", duplicate, created, err)
	}
	current, err := os.ReadFile(filepath.Join(state, "repositories.json"))
	if err != nil || !bytes.Equal(original, current) {
		t.Fatalf("legacy bytes changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "projects.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("overlay wrote project file: %v", err)
	}
}

func TestDirectoryProjectKeepsIdentityWhenRepositoryAdded(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	dir := initRepo(t)
	s, _ := testService(t, t.TempDir(), dir)
	project, _, err := s.RegisterDirectoryProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	repo := registerTest(t, s, dir)
	projects, err := s.ListProjects()
	if err != nil || len(projects) != 1 || projects[0].ID != project.ID || projects[0].RepositoryID != repo.ID {
		t.Fatalf("overlay: %+v %v", projects, err)
	}
	if err := s.Unregister(repo.ID); err != nil {
		t.Fatal(err)
	}
	projects, err = s.ListProjects()
	if err != nil || len(projects) != 1 || projects[0].ID != project.ID || projects[0].RepositoryID != "" {
		t.Fatalf("unregister: %+v %v", projects, err)
	}
}

func TestProjectCommitFailureDoesNotPublish(t *testing.T) {
	t.Parallel()
	state, dir := t.TempDir(), t.TempDir()
	s, _ := testService(t, state, dir)
	if _, _, err := s.RegisterDirectoryProject(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "projects.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// A non-regular replacement makes Commit fail before publication, even
	// when tests run with privileges that bypass filesystem mode bits.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, created, err := s.RegisterDirectoryProject(t.TempDir()); err == nil || created {
		t.Fatal("failed save reported success")
	}
	projects, err := s.ListProjects()
	if err != nil || len(projects) != 1 {
		t.Fatalf("failed save published: %+v %v", projects, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptProjectStoreDoesNotOverwriteOrDisableRepositories(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	path := filepath.Join(state, "projects.json")
	bad := []byte(`{"version":9,"projects":[]}`)
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	s, _ := testService(t, state, t.TempDir())
	if s.ProjectStorageError() == nil {
		t.Fatal("corrupt store available")
	}
	if _, err := s.ListProjects(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("list: %v", err)
	}
	if _, _, err := s.RegisterDirectoryProject(t.TempDir()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("register: %v", err)
	}
	if _, err := s.List(context.Background()); err != nil {
		t.Fatalf("unrelated repositories: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, bad) {
		t.Fatalf("overwrote corruption: %v", err)
	}
}

func TestConcurrentProjectRegistrationDeduplicates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, _ := testService(t, t.TempDir(), dir)
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if _, _, err := s.RegisterDirectoryProject(dir); err != nil {
				t.Errorf("register: %v", err)
			}
		})
	}
	wg.Wait()
	projects, err := s.ListProjects()
	if err != nil || len(projects) != 1 {
		t.Fatalf("duplicate projects: %+v %v", projects, err)
	}
	s.Close()
	if _, _, err := s.RegisterDirectoryProject(dir); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("register after close: %v", err)
	}
}

func TestProjectStoreLifetimeLock(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	s, _ := testService(t, state, t.TempDir())
	second := New(state, nil, nil, t.TempDir())
	defer second.Close()
	if !statefile.IsKind(second.ProjectStorageError(), statefile.Locked) {
		t.Fatalf("missing lifetime lock: %v", second.ProjectStorageError())
	}
	s.Close()
	third := New(state, nil, nil, t.TempDir())
	defer third.Close()
	if err := third.ProjectStorageError(); err != nil {
		t.Fatalf("lock not released: %v", err)
	}
}
