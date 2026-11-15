package workspace

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/session"
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
		t.Fatal("manager publication inverted a plugin cache lock")
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
