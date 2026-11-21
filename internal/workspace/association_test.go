package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/hybridresume"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

type associationObserver struct {
	service *Service
	seen    chan Association
}

func (o *associationObserver) Observe(event session.Event) {
	if event.Session == nil {
		return
	}
	value := o.service.Association(event.Session.ID)
	if value == nil {
		return
	}
	select {
	case o.seen <- *value:
	default:
	}
}

func TestAssociationCommitAndSourceCaptureHaveNoLockInversion(t *testing.T) {
	manager := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer manager.Shutdown(context.Background())
	process, err := manager.CreateShell(session.ShellInput{CreateInput: session.CreateInput{Cmd: []string{"driver"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	info := process.Info()
	service := &Service{manager: manager, entries: []*repositoryState{{entry: registration{ID: "repository"}}}}
	const updates = 128
	// Two concurrent committers plus unrelated notifications fit without
	// making this cache-only source observer block manager publication.
	observer := &associationObserver{service: service, seen: make(chan Association, 4*updates+16)}
	stop := manager.Observe(observer)
	defer stop()
	var workers sync.WaitGroup
	for worker := range 2 {
		workers.Go(func() {
			for index := range updates {
				association := &Association{RepositoryID: "repository", WorktreeID: strconv.Itoa(worker*updates + index), WorktreePath: info.Cwd}
				service.commitAssociation(info, association, false)
			}
		})
	}
	workers.Go(func() {
		for range updates {
			manager.Notify(info.ID)
		}
	})
	done := make(chan struct{})
	go func() { workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("manager publication inverted a feature cache lock")
	}
	seen := make(map[string]bool)
	for len(observer.seen) > 0 {
		value := <-observer.seen
		seen[value.WorktreeID] = true
	}
	for index := range 2 * updates {
		if !seen[strconv.Itoa(index)] {
			t.Fatalf("committed association %d was collapsed before capture", index)
		}
	}
}

func TestAssociationEndsWithRemovedSession(t *testing.T) {
	isolateGit(t)
	root := initRepo(t)
	manager := session.NewManager(session.Options{DefaultCwd: root})
	t.Cleanup(func() { manager.Shutdown(context.Background()) })
	service := New(t.TempDir(), manager, nil, root)
	t.Cleanup(service.Close)
	if _, _, err := service.Register(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	item, err := manager.Create(session.CreateInput{Cwd: root, Cmd: []string{"/bin/sh", "-c", "exit 0"}})
	if err != nil {
		t.Fatal(err)
	}
	<-item.Done()
	service.reassociate(context.Background())
	info := item.Info()
	association := service.Association(info.ID)
	if association == nil {
		t.Fatal("exited sessions retain their repository association")
	}
	if err := manager.Remove(info.ID); err != nil {
		t.Fatal(err)
	}
	if service.Association(info.ID) != nil {
		t.Fatal("removed session retained repository metadata")
	}
	service.commitAssociation(info, association, false)
	if service.Association(info.ID) != nil {
		t.Fatal("late association resolution resurrected removed metadata")
	}
}

type recordedAssociationEvent struct {
	eventType   string
	sessionID   string
	association *Association
}

type associationEventObserver struct {
	service *Service
	mu      sync.Mutex
	events  []recordedAssociationEvent
}

func (o *associationEventObserver) Observe(event session.Event) {
	if event.Session == nil {
		return
	}
	var association *Association
	if current := o.service.Association(event.Session.ID); current != nil {
		value := *current
		association = &value
	}
	o.mu.Lock()
	o.events = append(o.events, recordedAssociationEvent{
		eventType:   event.Type,
		sessionID:   event.Session.ID,
		association: association,
	})
	o.mu.Unlock()
}

func (o *associationEventObserver) forSession(id string) []recordedAssociationEvent {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []recordedAssociationEvent
	for _, item := range o.events {
		if item.sessionID == id {
			out = append(out, item)
		}
	}
	return out
}

func TestResumePublishesWorkspaceAssociationOnCreatedEvent(t *testing.T) {
	h := newQuickHarness(t, "")
	tree := h.repo.Worktrees[0]
	id := h.archive(tree.Path)

	observer := &associationEventObserver{service: h.s}
	stop := h.manager.Observe(observer)
	defer stop()

	resumer := hybridresume.New(h.hybrids, h.launches, hybridresume.Options{ResumeLocations: h.s})
	ctx, cancel := context.WithTimeout(context.Background(), quickTimeout)
	defer cancel()
	if _, err := resumer.Resume(ctx, id, hybridresume.Input{ClientRequestID: "resume-staged-001"}); err != nil {
		t.Fatalf("resume: %v", err)
	}

	events := observer.forSession(id)
	if len(events) == 0 || events[0].eventType != "created" {
		t.Fatalf("observed events = %+v, want initial created event", events)
	}
	want := Association{RepositoryID: h.repo.ID, WorktreeID: tree.ID, WorktreePath: tree.Path}
	for i, event := range events {
		if event.association == nil || *event.association != want {
			t.Fatalf("event %d (%s) association = %+v, want %+v", i, event.eventType, event.association, want)
		}
	}

	// Closing and deleting the agent session forces a failed resume; the
	// staged association must be cleaned up when resume aborts.
	if err := h.manager.Close(id); err != nil {
		t.Fatal(err)
	}
	waitForCondition(t, "closed record", func() bool {
		rec, err := h.hybrids.Record(id)
		return err == nil && rec.State == hybrid.StateClosed
	})
	rec, err := h.hybrids.Record(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(h.store, "sessions", rec.AgentSessionID+".json")); err != nil {
		t.Fatal(err)
	}
	if _, err := resumer.Resume(ctx, id, hybridresume.Input{ClientRequestID: "resume-missing-001"}); !errors.Is(err, hybrid.ErrSessionMissing) {
		t.Fatalf("resume missing session err = %v, want ErrSessionMissing", err)
	}
	if got := h.s.Association(id); got != nil {
		t.Fatalf("failed resume left staged association %+v", got)
	}
}

func TestCreatePublishesWorkspaceAssociationOnCreatedEvent(t *testing.T) {
	h := newQuickHarness(t, "")
	tree := h.repo.Worktrees[0]
	ref := &sessioncreate.Reference{RepositoryID: h.repo.ID, WorktreeID: tree.ID}
	want := Association{RepositoryID: h.repo.ID, WorktreeID: tree.ID, WorktreePath: tree.Path}

	observer := &associationEventObserver{service: h.s}
	stop := h.manager.Observe(observer)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), quickTimeout)
	defer cancel()

	chat, err := h.createSession(ctx, sessioncreate.Input{ConfigID: &h.configID, Side: string(hybrid.SideACP), Workspace: ref})
	if err != nil {
		t.Fatalf("create chat: %v", err)
	}
	plain, err := h.createSession(ctx, sessioncreate.Input{Cmd: []string{"/bin/sh", "-c", "sleep 30"}, Workspace: ref})
	if err != nil {
		t.Fatalf("create terminal: %v", err)
	}

	for name, id := range map[string]string{"chat": chat.Info().ID, "terminal": plain.Info().ID} {
		events := observer.forSession(id)
		if len(events) == 0 || events[0].eventType != "created" {
			t.Fatalf("%s observed events = %+v, want initial created event", name, events)
		}
		if events[0].association == nil || *events[0].association != want {
			t.Fatalf("%s created association = %+v, want %+v", name, events[0].association, want)
		}
	}
}

func TestStageAssociationRollbackRespectsRegistration(t *testing.T) {
	manager := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer manager.Shutdown(context.Background())
	service := &Service{manager: manager, entries: []*repositoryState{{entry: registration{ID: "repo1"}}}}
	want := &Association{RepositoryID: "repo1", WorktreeID: "tree1", WorktreePath: "/tmp/repo"}

	rollback := service.stageAssociation(session.Info{ID: "unreg001"}, want)
	if rollback == nil {
		t.Fatal("stageAssociation returned nil rollback for registered repo")
	}
	if got := service.Association("unreg001"); got == nil || *got != *want {
		t.Fatalf("staged association = %+v, want %+v", got, *want)
	}
	rollback()
	if got := service.Association("unreg001"); got != nil {
		t.Fatalf("rollback kept unregistered association %+v", got)
	}

	item, err := manager.CreateShell(session.ShellInput{CreateInput: session.CreateInput{Cmd: []string{"driver"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	info := item.Info()
	keep := service.stageAssociation(info, want)
	keep()
	if got := service.Association(info.ID); got == nil || *got != *want {
		t.Fatalf("rollback dropped registered session association: %+v", got)
	}
}

func waitForCondition(t *testing.T, label string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(quickTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", label)
}
