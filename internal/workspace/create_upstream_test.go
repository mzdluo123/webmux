package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/sessioncreate"
)

func upstreamConfirmationFor(t *testing.T, err error) WorktreeUpstream {
	t.Helper()
	var confirmation *UpstreamConfirmationError
	if !errors.As(err, &confirmation) || !errors.Is(err, sessioncreate.ErrUpstreamConfirmation) {
		t.Fatalf("expected upstream challenge, got %v", err)
	}
	return confirmation.WorktreeUpstream
}

func TestCreateWorktreeUpstreamChoices(t *testing.T) {
	for _, mode := range []string{"new", "existing"} {
		for _, useUpstream := range []bool{false, true} {
			t.Run(mode+map[bool]string{false: "/keep", true: "/upstream"}[useUpstream], func(t *testing.T) {
				isolateGit(t)
				s, r, root, remote := upstreamFixture(t)
				local := gitTest(t, root, "rev-parse", "HEAD")
				latest := advanceUpstream(t, root, remote, "latest")
				req := CreateRequest{Mode: mode, Branch: "topic", Path: filepath.Join(t.TempDir(), "tree")}
				if mode == "new" {
					req.Base = "HEAD"
				} else {
					gitTest(t, root, "branch", "--track", "topic", "origin/main")
				}
				_, err := s.CreateWorktree(context.Background(), r.ID, req)
				confirmation := upstreamConfirmationFor(t, err)
				if confirmation.LocalHead != local || confirmation.UpstreamHead != latest || !confirmation.CanUseUpstream {
					t.Fatalf("challenge: %+v", confirmation)
				}
				if _, err := os.Stat(req.Path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("challenge created path: %v", err)
				}
				if mode == "new" {
					gitTest(t, root, "show-ref", "--verify", "refs/heads/main")
				}
				req.UpstreamChoice = &UpstreamChoice{ConfirmationID: confirmation.ConfirmationID, UseUpstream: useUpstream}
				tree, err := s.CreateWorktree(context.Background(), r.ID, req)
				if err != nil {
					t.Fatal(err)
				}
				want := local
				if useUpstream {
					want = latest
				}
				if tree.Head == nil || *tree.Head != want {
					t.Fatalf("created head: %+v want %s", tree, want)
				}
				if gitTest(t, root, "rev-parse", "main") != local {
					t.Fatal("source branch changed")
				}
			})
		}
	}
}

func TestCreateWorktreeDivergence(t *testing.T) {
	for _, mode := range []string{"new", "existing"} {
		t.Run(mode, func(t *testing.T) {
			isolateGit(t)
			s, r, root, remote := upstreamFixture(t)
			latest := advanceUpstream(t, root, remote, "remote commit")
			gitTest(t, root, "commit", "--allow-empty", "-m", "local commit")
			local := gitTest(t, root, "rev-parse", "HEAD")
			req := CreateRequest{Mode: mode, Branch: "topic", Base: "main", Path: filepath.Join(t.TempDir(), "tree")}
			if mode == "existing" {
				gitTest(t, root, "branch", "topic", "main")
				gitTest(t, root, "branch", "--set-upstream-to=origin/main", "topic")
				req.Base = ""
			}
			_, err := s.CreateWorktree(context.Background(), r.ID, req)
			confirmation := upstreamConfirmationFor(t, err)
			if confirmation.CanUseUpstream != (mode == "new") {
				t.Fatalf("unsafe choice: %+v", confirmation)
			}
			req.UpstreamChoice = &UpstreamChoice{ConfirmationID: confirmation.ConfirmationID, UseUpstream: true}
			if mode == "existing" {
				_, err = s.CreateWorktree(context.Background(), r.ID, req)
				confirmation = upstreamConfirmationFor(t, err)
				if gitTest(t, root, "rev-parse", "topic") != local {
					t.Fatal("diverged branch lost local commits")
				}
				req.UpstreamChoice = &UpstreamChoice{ConfirmationID: confirmation.ConfirmationID}
			}
			tree, err := s.CreateWorktree(context.Background(), r.ID, req)
			if err != nil {
				t.Fatal(err)
			}
			want := latest
			if mode == "existing" {
				want = local
			}
			if tree.Head == nil || *tree.Head != want {
				t.Fatalf("head: %+v want %s", tree, want)
			}
		})
	}
}

func TestCreateWorktreeFetchFailureAndStaleConfirmation(t *testing.T) {
	isolateGit(t)
	s, r, root, remote := upstreamFixture(t)
	req := CreateRequest{Mode: "new", Branch: "topic", Base: "main", Path: filepath.Join(t.TempDir(), "tree")}
	advanceUpstream(t, root, remote, "first")
	_, err := s.CreateWorktree(context.Background(), r.ID, req)
	first := upstreamConfirmationFor(t, err)
	latest := advanceUpstream(t, root, remote, "second")
	req.UpstreamChoice = &UpstreamChoice{ConfirmationID: first.ConfirmationID, UseUpstream: true}
	_, err = s.CreateWorktree(context.Background(), r.ID, req)
	second := upstreamConfirmationFor(t, err)
	if second.ConfirmationID == first.ConfirmationID || second.UpstreamHead != latest {
		t.Fatalf("stale confirmation reused: %+v", second)
	}
	req.UpstreamChoice = &UpstreamChoice{ConfirmationID: second.ConfirmationID, UseUpstream: true}
	if _, err := s.CreateWorktree(context.Background(), r.ID, req); err != nil {
		t.Fatal(err)
	}

	gitTest(t, root, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	req = CreateRequest{Mode: "new", Branch: "offline", Base: "main", Path: filepath.Join(t.TempDir(), "offline")}
	_, err = s.CreateWorktree(context.Background(), r.ID, req)
	failed := upstreamConfirmationFor(t, err)
	if failed.CanUseUpstream || failed.Warning == "" {
		t.Fatalf("fetch failure was silently accepted: %+v", failed)
	}
	req.UpstreamChoice = &UpstreamChoice{ConfirmationID: failed.ConfirmationID}
	if _, err := s.CreateWorktree(context.Background(), r.ID, req); err != nil {
		t.Fatal(err)
	}
}

func TestCreateWorktreeUnchangedAndExactBases(t *testing.T) {
	for _, kind := range []string{"equal", "ahead", "no-upstream", "historical", "orphan"} {
		t.Run(kind, func(t *testing.T) {
			isolateGit(t)
			s, r, root, remote := upstreamFixture(t)
			original := gitTest(t, root, "rev-parse", "HEAD")
			req := CreateRequest{Mode: "new", Branch: "topic", Base: "HEAD", Path: filepath.Join(t.TempDir(), "tree")}
			switch kind {
			case "ahead":
				gitTest(t, root, "commit", "--allow-empty", "-m", "ahead")
			case "no-upstream":
				gitTest(t, root, "branch", "--unset-upstream", "main")
				advanceUpstream(t, root, remote, "remote")
			case "historical":
				gitTest(t, root, "commit", "--allow-empty", "-m", "later")
				advanceUpstream(t, root, remote, "remote")
				req.Base = original + "^{commit}"
			case "orphan":
				req.Mode, req.Base = "orphan", ""
				advanceUpstream(t, root, remote, "remote")
			}
			tree, err := s.CreateWorktree(context.Background(), r.ID, req)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "historical" && (tree.Head == nil || *tree.Head != original) {
				t.Fatalf("historical base changed: %+v", tree)
			}
		})
	}
}

func TestSessionCreateConfirmationRetrySameRequestID(t *testing.T) {
	isolateGit(t)
	s, r, root, remote := upstreamFixture(t)
	advanceUpstream(t, root, remote, "remote")
	creator := sessioncreate.New(s.manager, sessioncreate.Options{DefaultCwd: root, Locations: s})
	t.Cleanup(creator.Close)
	in := sessioncreate.Input{ClientRequestID: "upstream-retry", Cmd: []string{"/bin/sh", "-c", "exit 0"}, NewWorktree: &sessioncreate.NewWorktree{RepositoryID: r.ID, Mode: "new", Branch: "topic", Base: "HEAD", Path: filepath.Join(t.TempDir(), "tree")}}
	_, err := creator.Create(context.Background(), in)
	confirmation := upstreamConfirmationFor(t, err)
	in.NewWorktree.UpstreamChoice = &UpstreamChoice{ConfirmationID: confirmation.ConfirmationID}
	result, err := creator.Create(context.Background(), in)
	if err != nil || result.Location == nil {
		t.Fatalf("challenge poisoned request ID: %+v %v", result, err)
	}
}

func TestCreateWorktreeCancelledBeforeFetchHasNoSideEffects(t *testing.T) {
	isolateGit(t)
	s, r, _, _ := upstreamFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "cancelled")
	_, err := s.CreateWorktree(ctx, r.ID, CreateRequest{Mode: "new", Branch: "cancelled", Base: "HEAD", Path: path})
	if !errors.Is(err, context.Canceled) && !errors.Is(err, ErrTimeout) {
		t.Fatalf("cancellation: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancel created path: %v", err)
	}
}

func TestCreateWorktreeFetchDoesNotHoldRepositoryGate(t *testing.T) {
	isolateGit(t)
	s, r, _, _ := upstreamFixture(t)
	realGit := s.gitPath
	dir := t.TempDir()
	started, unblock := filepath.Join(dir, "started"), filepath.Join(dir, "unblock")
	wrapper := filepath.Join(dir, "git")
	putFile(t, wrapper, fmt.Sprintf("#!/bin/sh\nif [ \"$3\" = fetch ]; then\n  : > %q\n  while [ ! -e %q ]; do sleep 0.02; done\nfi\nexec %q \"$@\"\n", started, unblock, realGit))
	if err := os.Chmod(wrapper, 0700); err != nil {
		t.Fatal(err)
	}
	s.gitPath = wrapper
	defer func() { _ = os.WriteFile(unblock, nil, 0600) }()
	done := make(chan error, 1)
	go func() {
		_, err := s.CreateWorktree(context.Background(), r.ID, CreateRequest{Mode: "new", Branch: "topic", Base: "HEAD", Path: filepath.Join(dir, "tree")})
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("creation did not fetch")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := s.Branches(ctx, r.ID); err != nil {
		t.Fatalf("creation fetch held repository gate: %v", err)
	}
	if err := os.WriteFile(unblock, nil, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("creation did not finish")
	}
}

func TestSessionCreateGeneratedLocationSurvivesConfirmationAcrossSeconds(t *testing.T) {
	isolateGit(t)
	s, r, root, remote := upstreamFixture(t)
	latest := advanceUpstream(t, root, remote, "upstream update")
	creator := sessioncreate.New(s.manager, sessioncreate.Options{DefaultCwd: root, Locations: s})
	t.Cleanup(creator.Close)
	in := sessioncreate.Input{
		ClientRequestID: "generated-location-retry",
		Cmd:             []string{"/bin/sh", "-c", "exit 0"},
		NewWorktree:     &sessioncreate.NewWorktree{RepositoryID: r.ID, Mode: "new"},
	}
	_, err := creator.Create(context.Background(), in)
	challenge := upstreamConfirmationFor(t, err)
	s.mu.Lock()
	original := s.upstreamConfirmations[challenge.ConfirmationID]
	s.mu.Unlock()
	if original.branch == "" {
		t.Fatal("challenge did not retain its generated branch")
	}
	// Cross the generated name's clock boundary: a fresh plan must not choose
	// the current second's name and invalidate the caller's confirmation.
	for generatedBranch(time.Now(), func(string) bool { return true }) == original.branch {
		time.Sleep(10 * time.Millisecond)
	}
	in.NewWorktree.UpstreamChoice = &UpstreamChoice{ConfirmationID: challenge.ConfirmationID, UseUpstream: true}
	result, err := creator.Create(context.Background(), in)
	if err != nil || result.Location == nil {
		t.Fatalf("generated location confirmation could not resume: %+v %v", result, err)
	}
	tree, ok := result.Location.Detail.(Worktree)
	expectedPath := filepath.Join(root+".worktrees", filepath.Base(original.branch))
	if !ok || tree.Branch == nil || *tree.Branch != original.branch || tree.Path != expectedPath || tree.Head == nil || *tree.Head != latest {
		t.Fatalf("generated location or chosen baseline changed: %+v, expected %s at %s", tree, original.branch, expectedPath)
	}
}

func TestCreateWorktreeLatestUpstream(t *testing.T) {
	for _, kind := range []string{"behind", "ahead", "diverged", "offline"} {
		t.Run(kind, func(t *testing.T) {
			isolateGit(t)
			s, r, root, remote := upstreamFixture(t)
			latest := gitTest(t, root, "rev-parse", "origin/main")
			if kind == "behind" || kind == "diverged" {
				latest = advanceUpstream(t, root, remote, "remote")
			}
			if kind == "ahead" || kind == "diverged" {
				gitTest(t, root, "commit", "--allow-empty", "-m", "local")
			}
			if kind == "offline" {
				gitTest(t, root, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
			}
			req := CreateRequest{Mode: "new", Branch: "task", Base: "refs/heads/main", Path: filepath.Join(t.TempDir(), "tree"), LatestUpstream: true}
			tree, err := s.CreateWorktree(context.Background(), r.ID, req)
			if kind == "offline" {
				if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "无法获取最新上游提交，请重试") {
					t.Fatalf("expected fetch failure, got %v", err)
				}
				return
			}
			if err != nil || tree.Head == nil || *tree.Head != latest {
				t.Fatalf("expected upstream %s, got %+v, %v", latest, tree, err)
			}
		})
	}
}

func TestCreateWorktreeKeepLocalConfirmationSkipsSecondFetch(t *testing.T) {
	isolateGit(t)
	s, r, root, remote := upstreamFixture(t)
	local := gitTest(t, root, "rev-parse", "HEAD")
	advanceUpstream(t, root, remote, "remote ahead")

	realGit := s.gitPath
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "git")
	countFile := filepath.Join(dir, "fetches.log")
	putFile(t, wrapper, "#!/bin/sh\nif [ \"$3\" = fetch ]; then printf 'fetch\\n' >> '"+countFile+"'; fi\nexec '"+realGit+"' \"$@\"\n")
	if err := os.Chmod(wrapper, 0700); err != nil {
		t.Fatal(err)
	}
	s.gitPath = wrapper

	req := CreateRequest{Mode: "new", Branch: "keep-local", Base: "main", Path: filepath.Join(t.TempDir(), "tree")}
	_, err := s.CreateWorktree(context.Background(), r.ID, req)
	confirmation := upstreamConfirmationFor(t, err)

	req.UpstreamChoice = &UpstreamChoice{ConfirmationID: confirmation.ConfirmationID, UseUpstream: false}
	tree, err := s.CreateWorktree(context.Background(), r.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Head == nil || *tree.Head != local {
		t.Fatalf("head = %+v, want local %s", tree, local)
	}
	raw, err := os.ReadFile(countFile)
	if err != nil {
		t.Fatal(err)
	}
	if fetches := len(strings.Fields(string(raw))); fetches != 1 {
		t.Fatalf("keep-local confirmation ran %d fetches, want 1", fetches)
	}
}
