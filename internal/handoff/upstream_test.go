package handoff

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

func awaitUpstream(t *testing.T, s *Service, id string) Operation {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		o, err := s.Delivery(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if o.State == "awaiting_upstream" {
			return o
		}
		if o.FinishedAt != nil {
			t.Fatalf("delivery finished before confirmation: %+v", o)
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("delivery did not request upstream confirmation")
	return Operation{}
}

func TestUpstreamConfirmationResumesFrozenDeliveryOnce(t *testing.T) {
	for _, latest := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "latest"}[latest], func(t *testing.T) {
			s, b := newTestService(t)
			manager := session.NewManager(session.Options{})
			t.Cleanup(func() { manager.Shutdown(context.Background()) })
			var created atomic.Int32
			var stale atomic.Bool
			stale.Store(true)
			s.options.Create = func(_ context.Context, in sessioncreate.Input) (sessioncreate.Result, error) {
				choice := in.NewWorktree.UpstreamChoice
				if choice == nil || stale.Swap(false) {
					id := "first"
					if choice != nil {
						id = "changed-tip"
					}
					return sessioncreate.Result{}, &sessioncreate.Error{Step: sessioncreate.StepWorktree, Err: &sessioncreate.UpstreamConfirmationError{WorktreeUpstream: sessioncreate.WorktreeUpstream{ConfirmationID: id, Branch: "main", LocalHead: "local", Upstream: "origin/main", UpstreamHead: id, CanUseUpstream: true}}}
				}
				if choice.ConfirmationID != "changed-tip" || choice.UseUpstream != latest {
					return sessioncreate.Result{}, errors.New("wrong creation decision")
				}
				item, err := manager.CreateShell(session.ShellInput{CreateInput: session.CreateInput{Cmd: []string{"test-agent"}, Cwd: t.TempDir()}}, nil)
				if err != nil {
					return sessioncreate.Result{}, err
				}
				created.Add(1)
				b.mu.Lock()
				b.records[item.Info().ID] = hybrid.Record{ID: item.Info().ID, ConfigID: "config", Agent: "test", AgentSessionID: "new-native", State: hybrid.StateActive, Side: hybrid.SideACP}
				b.mu.Unlock()
				return sessioncreate.Result{Session: item}, nil
			}
			p, err := s.Preview(context.Background(), "source", Request{ConfigID: "config", Format: "text", Location: Location{RepositoryID: "repo", NewWorktreeBranch: "task", NewWorktreeBase: "main"}, UserText: "current task"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
				t.Fatal(err)
			}
			first := awaitUpstream(t, s, p.OperationID)
			if created.Load() != 0 || first.TargetSessionID != "" || first.FinishedAt != nil {
				t.Fatalf("resources before decision: %+v", first)
			}
			if _, err := s.ConfirmUpstream(context.Background(), p.ID, p.OperationID, sessioncreate.UpstreamChoice{ConfirmationID: first.WorktreeUpstream.ConfirmationID, UseUpstream: latest}); err != nil {
				t.Fatal(err)
			}
			second := awaitUpstream(t, s, p.OperationID)
			if second.WorktreeUpstream.ConfirmationID != "changed-tip" || created.Load() != 0 {
				t.Fatalf("stale decision created resources: %+v", second)
			}
			choice := sessioncreate.UpstreamChoice{ConfirmationID: second.WorktreeUpstream.ConfirmationID, UseUpstream: latest}
			var wg sync.WaitGroup
			for range 16 {
				wg.Go(func() {
					if _, err := s.ConfirmUpstream(context.Background(), p.ID, p.OperationID, choice); err != nil {
						t.Errorf("repeat decision: %v", err)
					}
				})
			}
			wg.Wait()
			o := awaitOperation(t, s, p.OperationID)
			b.mu.Lock()
			defer b.mu.Unlock()
			if o.State != "succeeded" || created.Load() != 1 || b.promptCalls != 1 || b.text != p.Text {
				t.Fatalf("delivery=%+v sessions=%d prompts=%d frozen=%v", o, created.Load(), b.promptCalls, b.text == p.Text)
			}
		})
	}
}

func TestPendingUpstreamCancellationAndExpiryDoNotCreate(t *testing.T) {
	for _, expire := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "expire"}[expire], func(t *testing.T) {
			s, b := newTestService(t)
			var calls atomic.Int32
			s.options.Create = func(context.Context, sessioncreate.Input) (sessioncreate.Result, error) {
				calls.Add(1)
				return sessioncreate.Result{}, &sessioncreate.UpstreamConfirmationError{WorktreeUpstream: sessioncreate.WorktreeUpstream{ConfirmationID: "check", CanUseUpstream: true}}
			}
			p, err := s.Preview(context.Background(), "source", Request{ConfigID: "config", Format: "text", Location: Location{RepositoryID: "repo", NewWorktreeBranch: "task", NewWorktreeBase: "main"}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
				t.Fatal(err)
			}
			awaitUpstream(t, s, p.OperationID)
			if expire {
				s.mu.Lock()
				s.now = func() time.Time { return p.ExpiresAt }
				s.mu.Unlock()
			} else if _, err := s.Cancel(context.Background(), p.OperationID); err != nil {
				t.Fatal(err)
			}
			o := awaitOperation(t, s, p.OperationID)
			if _, err := s.ConfirmUpstream(context.Background(), p.ID, p.OperationID, sessioncreate.UpstreamChoice{ConfirmationID: "check", UseUpstream: true}); !errors.Is(err, ErrConflict) {
				t.Fatalf("late confirmation: %v", err)
			}
			b.mu.Lock()
			defer b.mu.Unlock()
			if o.State != "failed" || calls.Load() != 1 || b.promptCalls != 0 {
				t.Fatalf("delivery=%+v creates=%d prompts=%d", o, calls.Load(), b.promptCalls)
			}
		})
	}
}
