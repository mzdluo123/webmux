package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/statefile"
)

func cloneTestService(t *testing.T, mode string) (*Service, string, string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("requires Git")
	}
	root, state := t.TempDir(), t.TempDir()
	calls := filepath.Join(root, "calls")
	t.Setenv("WEBMUX_CLONE_TEST_GIT", realGit)
	t.Setenv("WEBMUX_CLONE_TEST_CALLS", calls)
	t.Setenv("WEBMUX_CLONE_TEST_MODE", mode)
	t.Setenv("WEBMUX_CLONE_TEST_STATE", state)
	t.Setenv("WEBMUX_CLONE_TEST_TARGET", filepath.Join(root, "target"))
	script := `#!/bin/sh
clone=
for argument do
 if [ "$argument" = clone ]; then clone=yes; fi
 destination=$argument
done
if [ "$clone" = yes ]; then
 printf 'clone\n' >> "$WEBMUX_CLONE_TEST_CALLS"
 case "$WEBMUX_CLONE_TEST_MODE" in
 fail) printf 'SECRET_TOKEN=should-not-escape\n' >&2; exit 1 ;;
 block) sleep 60; exit 1 ;;
 occupied) mkdir "$WEBMUX_CLONE_TEST_TARGET"; printf preserved > "$WEBMUX_CLONE_TEST_TARGET/foreign" ;;
 esac
 "$WEBMUX_CLONE_TEST_GIT" init --initial-branch=main "$destination" || exit 1
 if [ "$WEBMUX_CLONE_TEST_MODE" = registration ]; then mkdir "$WEBMUX_CLONE_TEST_STATE/repositories.json"; fi
 exit 0
fi
exec "$WEBMUX_CLONE_TEST_GIT" "$@"
`
	wrapper := filepath.Join(root, "git-wrapper")
	if err := os.WriteFile(wrapper, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	service, _ := testService(t, state, root)
	service.gitPath = wrapper
	return service, root, state
}

func awaitProjectClone(t *testing.T, s *Service, id string) ProjectCloneOperation {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		op, err := s.ProjectClone(id)
		if err != nil {
			t.Fatal(err)
		}
		if op.State != "running" && op.State != "registering" {
			return op
		}
		select {
		case <-ctx.Done():
			t.Fatalf("clone did not complete: %+v", op)
		case <-ticker.C:
		}
	}
}

func TestProjectCloneDetachedIdempotencyAndRegistrationRecovery(t *testing.T) {
	s, root, _ := cloneTestService(t, "registration")
	req := ProjectCloneRequest{RequestID: "retry-request", URL: "https://example.invalid/repository.git", Path: filepath.Join(root, "target")}
	ctx, cancel := context.WithCancel(context.Background())
	op, err := s.StartProjectClone(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	cancel() // Browser departure must not cancel the durable worker.
	failed := awaitProjectClone(t, s, op.ID)
	if failed.State != "registration_failed" {
		t.Fatalf("expected retained clone: %+v", failed)
	}
	if _, err := os.Stat(filepath.Join(req.Path, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(s.stateDir, "repositories.json")); err != nil {
		t.Fatal(err)
	}
	retried, err := s.RetryProjectCloneRegistration(context.Background(), op.ID)
	if err != nil || retried.ID != op.ID {
		t.Fatalf("retry: %+v %v", retried, err)
	}
	done := awaitProjectClone(t, s, op.ID)
	if done.State != "succeeded" || done.ProjectID == "" {
		t.Fatalf("registration: %+v", done)
	}
	duplicate, err := s.StartProjectClone(context.Background(), req)
	if err != nil || duplicate.ID != op.ID || duplicate.State != "succeeded" {
		t.Fatalf("duplicate: %+v %v", duplicate, err)
	}
	calls, err := os.ReadFile(filepath.Join(root, "calls"))
	if err != nil || string(calls) != "clone\n" {
		t.Fatalf("cloned more than once: %q %v", calls, err)
	}
	req.URL = "https://example.invalid/other.git"
	if _, err := s.StartProjectClone(context.Background(), req); !errors.Is(err, ErrConflict) {
		t.Fatalf("request reused for other source: %v", err)
	}
}

func TestProjectCloneDoesNotReplaceRacingTarget(t *testing.T) {
	s, root, _ := cloneTestService(t, "occupied")
	op, err := s.StartProjectClone(context.Background(), ProjectCloneRequest{RequestID: "occupied", URL: "git@example.invalid:repo.git", Path: filepath.Join(root, "target")})
	if err != nil {
		t.Fatal(err)
	}
	done := awaitProjectClone(t, s, op.ID)
	if done.State != "failed" {
		t.Fatalf("publication overwrote target: %+v", done)
	}
	data, err := os.ReadFile(filepath.Join(root, "target", "foreign"))
	if err != nil || string(data) != "preserved" {
		t.Fatalf("foreign files changed: %q %v", data, err)
	}
}

func TestProjectCloneFailureRedactsGitOutputAndCleansOwnDirectory(t *testing.T) {
	s, root, _ := cloneTestService(t, "fail")
	op, err := s.StartProjectClone(context.Background(), ProjectCloneRequest{RequestID: "failure", URL: "https://example.invalid/repo.git", Path: filepath.Join(root, "target")})
	if err != nil {
		t.Fatal(err)
	}
	done := awaitProjectClone(t, s, op.ID)
	s.StopProjectClones() // Join deferred cleanup before inspecting the directory.
	if done.State != "failed" || done.Error == "" || strings.Contains(done.Error, "SECRET_TOKEN") {
		t.Fatalf("unsafe failure: %+v", done)
	}
	entries, err := filepath.Glob(filepath.Join(root, ".webmux-clone-*"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("owned temporary directories leaked: %v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(root, "target")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed clone published target: %v", err)
	}
}

func TestProjectCloneStopJoinsProcessGroupAndRejectsNewWork(t *testing.T) {
	s, root, state := cloneTestService(t, "block")
	req := ProjectCloneRequest{RequestID: "stop", URL: "https://example.invalid/repo.git", Path: filepath.Join(root, "target")}
	op, err := s.StartProjectClone(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(filepath.Join(root, "calls")); err == nil {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("worker never started")
		case <-ticker.C:
		}
	}
	finished := make(chan struct{})
	go func() { s.Close(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not cancel/join Git process group")
	}
	stored, err := statefile.ReadSnapshot[projectCloneRegistry](state, statefile.ProjectClones, validateProjectClones)
	if err != nil || len(stored.Operations) != 1 || stored.Operations[0].ID != op.ID || stored.Operations[0].State != "interrupted" {
		t.Fatalf("shutdown state: %+v %v", stored, err)
	}
	if _, err := s.StartProjectClone(context.Background(), req); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("accepted work after close: %v", err)
	}
}

func TestConcurrentProjectCloneAdmission(t *testing.T) {
	s, root, _ := cloneTestService(t, "success")
	req := ProjectCloneRequest{RequestID: "concurrent", URL: "https://example.invalid/repo.git", Path: filepath.Join(root, "target")}
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	for range 12 {
		wg.Go(func() {
			op, err := s.StartProjectClone(context.Background(), req)
			if err != nil {
				t.Errorf("admit: %v", err)
				return
			}
			ids <- op.ID
		})
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatalf("duplicate operation IDs: %s %s", first, id)
		}
	}
	if done := awaitProjectClone(t, s, first); done.State != "succeeded" {
		t.Fatalf("clone: %+v", done)
	}
	calls, err := os.ReadFile(filepath.Join(root, "calls"))
	if err != nil || string(calls) != "clone\n" {
		t.Fatalf("concurrent clone duplicated: %q %v", calls, err)
	}
	s.Close()
	restored := New(s.stateDir, nil, nil, root)
	defer restored.Close()
	replay, err := restored.StartProjectClone(context.Background(), req)
	if err != nil || replay.ID != first || replay.State != "succeeded" {
		t.Fatalf("durable replay: %+v %v", replay, err)
	}
}

func TestProjectCloneAdmissionBoundsAndDoesNotRunAtLimit(t *testing.T) {
	s, root, _ := cloneTestService(t, "success")
	records := make([]projectCloneRecord, 0, maxProjectClones)
	for i := range maxProjectClones {
		records = append(records, projectCloneRecord{ID: fmt.Sprintf("%032x", i+1), RequestID: fmt.Sprintf("previous-%d", i), URL: "https://example.invalid/repo.git", Path: filepath.Join(root, fmt.Sprintf("previous-%d", i)), State: "failed"})
	}
	s.cloneMu.Lock()
	s.clones = records
	s.cloneMu.Unlock()
	if _, err := s.StartProjectClone(context.Background(), ProjectCloneRequest{RequestID: "over-limit", URL: "https://example.invalid/repo.git", Path: filepath.Join(root, "target")}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("accepted operation beyond limit: %v", err)
	}
	if err := s.ProjectCloneStorageError(); err != nil {
		t.Fatalf("refusal disabled clone ledger: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "calls")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ran Git despite refusal: %v", err)
	}
}

func TestProjectCloneDoesNotStartAfterLedgerCommitFailure(t *testing.T) {
	s, root, state := cloneTestService(t, "success")
	if err := os.Mkdir(filepath.Join(state, "project-clones.json"), 0700); err != nil {
		t.Fatal(err)
	}
	op, err := s.StartProjectClone(context.Background(), ProjectCloneRequest{RequestID: "cannot-save", URL: "https://example.invalid/repo.git", Path: filepath.Join(root, "target")})
	if err == nil || op.ID != "" {
		t.Fatalf("admitted uncommitted operation: %+v %v", op, err)
	}
	if s.ProjectCloneStorageError() != nil {
		t.Fatal("failed pre-publication commit froze the ledger")
	}
	if len(s.clones) != 0 {
		t.Fatal("failed admission published memory")
	}
	if _, err := os.Stat(filepath.Join(root, "calls")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Git started without a durable receipt: %v", err)
	}
	if _, _, err := s.RegisterDirectoryProject(root); err != nil {
		t.Fatalf("clone failure disabled directory projects: %v", err)
	}
}

func TestProjectClonePreservesConfiguredSSHTransport(t *testing.T) {
	s, root, _ := cloneTestService(t, "success")
	t.Setenv("GIT_SSH_COMMAND", "deployment-key-wrapper")
	env := s.projectCloneGitEnv(context.Background(), root)
	for _, value := range env {
		if strings.HasPrefix(value, "GIT_SSH_COMMAND=") {
			t.Fatalf("overrode inherited transport: %s", value)
		}
	}
	t.Setenv("GIT_SSH_COMMAND", "")
	t.Setenv("GIT_SSH", "")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(root, "gitconfig"))
	if out, err := exec.Command(os.Getenv("WEBMUX_CLONE_TEST_GIT"), "config", "--global", "core.sshCommand", "deployment-key-wrapper").CombinedOutput(); err != nil {
		t.Fatalf("config: %s %v", out, err)
	}
	env = s.projectCloneGitEnv(context.Background(), root)
	for _, value := range env {
		if strings.HasPrefix(value, "GIT_SSH_COMMAND=") {
			t.Fatalf("overrode configured transport: %s", value)
		}
	}
}
