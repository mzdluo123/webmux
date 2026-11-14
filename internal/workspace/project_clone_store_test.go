package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/statefile"
)

func TestProjectCloneURLPolicy(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"https://github.com/org/repo.git", "ssh://git@github.com/org/repo.git", "git@github.com:org/repo.git", "github.com:org/repo.git"} {
		if err := validateProjectURL(raw); err != nil {
			t.Errorf("rejected %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "-u", "file:///tmp/repo", "ext::sh -c id", "ext::id", "git@host:repo?token=secret", "file:/tmp/repo", "git://host/repo", "https://user:secret@host/repo", "https://token@host/repo", "https://host/repo?access_token=secret", "https://host/repo#secret", "ssh://git:secret@host/repo", "ssh://%0a@host/repo", "https://host/repo%0a", "git@host:repo\n", "../local", "/tmp/local"} {
		if err := validateProjectURL(raw); !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted %q: %v", raw, err)
		}
	}
}

func TestProjectCloneLedgerReconcilesWithoutStartingProcesses(t *testing.T) {
	t.Parallel()
	state, parent := t.TempDir(), t.TempDir()
	running := projectCloneRecord{ID: strings.Repeat("a", 32), RequestID: "request-running", URL: "https://host/running.git", Path: filepath.Join(parent, "running"), State: "running"}
	running.TemporaryPath = filepath.Join(parent, ".webmux-clone-"+running.ID+"-retained")
	if err := os.Mkdir(running.TemporaryPath, 0700); err != nil {
		t.Fatal(err)
	}
	registering := projectCloneRecord{ID: strings.Repeat("b", 32), RequestID: "request-registering", URL: "git@host:repo.git", Path: filepath.Join(parent, "published"), State: "registering"}
	if err := os.Mkdir(registering.Path, 0700); err != nil {
		t.Fatal(err)
	}
	s := New(state, nil, nil, parent)
	s.cloneSaveMu.Lock()
	err := s.commitProjectClones([]projectCloneRecord{running, registering})
	s.cloneSaveMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = New(state, nil, nil, parent)
	defer s.Close()
	if err := s.ProjectCloneStorageError(); err != nil {
		t.Fatal(err)
	}
	if len(s.clones) != 2 || s.clones[0].State != "interrupted" || s.clones[1].State != "registration_failed" {
		t.Fatalf("reconciliation: %+v", s.clones)
	}
	for _, path := range []string{running.TemporaryPath, registering.Path} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("removed retained content: %v", err)
		}
	}
	data, err := statefile.ReadSnapshot[projectCloneRegistry](state, statefile.ProjectClones, validateProjectClones)
	if err != nil || data.Operations[0].State != "interrupted" {
		t.Fatalf("recovery not durable: %+v %v", data, err)
	}
}

func TestProjectCloneStoreFailureIsIndependent(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	path := filepath.Join(state, "project-clones.json")
	bad := []byte(`{"version":99,"operations":[]}`)
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	s, _ := testService(t, state, t.TempDir())
	if s.ProjectCloneStorageError() == nil {
		t.Fatal("corrupt clone ledger accepted")
	}
	if _, _, err := s.RegisterDirectoryProject(t.TempDir()); err != nil {
		t.Fatalf("local project disabled: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, bad) {
		t.Fatalf("overwrote corruption: %v", err)
	}
}

func TestProjectCloneLedgerRejectsUnsafeTemporaryPath(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	record := projectCloneRecord{ID: strings.Repeat("a", 32), RequestID: "req-1", URL: "https://host/repo", Path: filepath.Join(parent, "target"), TemporaryPath: filepath.Join(parent, "someone-elses-files"), State: "running"}
	encoded, err := json.Marshal(projectCloneRegistry{Version: 1, Operations: []projectCloneRecord{record}})
	if err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "project-clones.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	s, _ := testService(t, state, parent)
	if s.ProjectCloneStorageError() == nil {
		t.Fatal("unowned temporary path accepted")
	}
}

func TestValidateProjectCloneLedgerBounds(t *testing.T) {
	t.Parallel()
	records := make([]projectCloneRecord, 0, maxProjectClones+1)
	for i := range maxProjectClones + 1 {
		records = append(records, projectCloneRecord{ID: fmt.Sprintf("%032x", i+1), RequestID: fmt.Sprintf("request-%d", i), URL: "https://host/repo.git", Path: fmt.Sprintf("/projects/p%d", i), State: "failed", Error: "clone failed"})
	}
	if err := validateProjectClones(projectCloneRegistry{Version: 1, Operations: records[:maxProjectClones]}); err != nil {
		t.Fatalf("full ledger: %v", err)
	}
	if err := validateProjectClones(projectCloneRegistry{Version: 1, Operations: records}); err == nil {
		t.Fatal("oversized ledger accepted")
	}
}

func TestProjectCloneValidationFailureDoesNotFreezeLedger(t *testing.T) {
	t.Parallel()
	s, _ := testService(t, t.TempDir(), t.TempDir())
	s.cloneSaveMu.Lock()
	defer s.cloneSaveMu.Unlock()
	invalid := projectCloneRecord{ID: "invalid", State: "failed"}
	if err := s.commitProjectClones([]projectCloneRecord{invalid}); err == nil {
		t.Fatal("invalid record accepted")
	}
	if err := s.ProjectCloneStorageError(); err != nil {
		t.Fatalf("pre-publication validation froze valid store: %v", err)
	}
	if len(s.clones) != 0 {
		t.Fatal("failed commit changed memory")
	}
	if err := s.commitProjectClones([]projectCloneRecord{}); err != nil {
		t.Fatalf("cannot retry valid commit: %v", err)
	}
}
