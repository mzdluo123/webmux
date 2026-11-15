package workspace

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func fakeRegistrations(n int) []registration {
	out := make([]registration, 0, n)
	for i := range n {
		out = append(out, registration{ID: fmt.Sprintf("id%04d", i), CommonDir: fmt.Sprintf("/repos/r%04d/.git", i), AnchorPath: fmt.Sprintf("/repos/r%04d", i), DisplayName: "r"})
	}
	return out
}

func TestRegistryRefusesMoreThanMaxRepositories(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	s.mu.Lock()
	for _, entry := range fakeRegistrations(MaxRepositories) {
		s.entries = append(s.entries, newRepository(entry))
	}
	s.mu.Unlock()
	if _, _, err := s.Register(context.Background(), root); !errors.Is(err, ErrInvalid) {
		t.Fatalf("register past the limit: %v, want ErrInvalid", err)
	}
	s.mu.Lock()
	n := len(s.entries)
	s.mu.Unlock()
	if n != MaxRepositories || !s.Capability().RegistryAvailable {
		t.Fatalf("registry changed by a refused registration: %d entries", n)
	}
}

func TestValidateRegistryBoundsRepositoryCount(t *testing.T) {
	t.Parallel()
	if err := validateRegistry(registry{Version: 1, Repositories: fakeRegistrations(MaxRepositories)}); err != nil {
		t.Fatalf("a full registry: %v", err)
	}
	if err := validateRegistry(registry{Version: 1, Repositories: fakeRegistrations(MaxRepositories + 1)}); err == nil {
		t.Fatal("a registry past the limit loaded")
	}
}

func fakeDirectoryProjects(n int) []directoryProject {
	out := make([]directoryProject, 0, n)
	for i := range n {
		out = append(out, directoryProject{ID: fmt.Sprintf("directory:id%04d", i), Name: "project", Path: fmt.Sprintf("/projects/p%04d", i)})
	}
	return out
}

func TestValidateProjectsBoundsDirectoryCount(t *testing.T) {
	t.Parallel()
	if err := validateProjects(projectRegistry{Version: 1, Projects: fakeDirectoryProjects(MaxProjects)}); err != nil {
		t.Fatalf("full catalog: %v", err)
	}
	if err := validateProjects(projectRegistry{Version: 1, Projects: fakeDirectoryProjects(MaxProjects + 1)}); err == nil {
		t.Fatal("oversized catalog accepted")
	}
}

func TestDirectoryProjectsRefuseMoreThanMaxProjects(t *testing.T) {
	t.Parallel()
	s, _ := testService(t, t.TempDir(), t.TempDir())
	s.projectMu.Lock()
	s.projects = fakeDirectoryProjects(MaxProjects)
	s.projectMu.Unlock()
	if _, created, err := s.RegisterDirectoryProject(t.TempDir()); !errors.Is(err, ErrInvalid) || created {
		t.Fatalf("registration past limit: created=%v err=%v", created, err)
	}
	projects, err := s.ListProjects()
	if err != nil || len(projects) != MaxProjects || s.ProjectStorageError() != nil {
		t.Fatalf("refusal changed catalog: count=%d err=%v", len(projects), err)
	}
}
