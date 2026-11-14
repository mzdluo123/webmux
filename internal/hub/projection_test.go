package hub

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/wire"
)

func TestProjectionContinuityAcrossSnapshotAndNotify(t *testing.T) {
	manager := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer manager.Shutdown(context.Background())
	process, err := manager.CreateShell(session.ShellInput{CreateInput: session.CreateInput{Cmd: []string{"driver"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := process.Info().ID
	var changed atomic.Int32
	project := func(info session.Info) contract.SessionInfo {
		out := wire.Session(info)
		work := "before"
		switch changed.Load() {
		case 1:
			work = "middle"
		case 2:
			work = "after"
		}
		out.Agent = &contract.SessionAgent{Name: "test", Work: work}
		return out
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	conn := newFakeMessageConn()
	// Three expected list frames plus periodic classification duplicates fit
	// without blocking the connection writer during the assertion.
	frames := make(chan contract.SessionEvent, 8)
	conn.writeHook = func(_ context.Context, kind MessageType, data []byte) error {
		if kind != MessageText {
			t.Errorf("unexpected list frame %v", kind)
			return nil
		}
		var event contract.SessionEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		select {
		case frames <- event:
		case <-ctx.Done():
			return ctx.Err()
		}
		if event.Type == "snapshot" {
			// Both commits land before the socket drains either increment.
			// Capturing only at socket-write time would collapse "middle".
			changed.Store(1)
			manager.Notify(id)
			changed.Store(2)
			manager.Notify(id)
		}
		return nil
	}
	done := make(chan struct{})
	go func() { defer close(done); List(ctx, manager, conn, Options{Project: project}) }()
	defer func() { cancel(); <-done }()
	select {
	case event := <-frames:
		if event.Type != "snapshot" || len(event.Sessions) != 1 || event.Sessions[0].Agent.Work != "before" {
			t.Fatalf("snapshot %+v", event)
		}
	case <-ctx.Done():
		t.Fatal("missing snapshot")
	}
	sawMiddle := false
	for {
		select {
		case event := <-frames:
			if event.Session == nil {
				continue
			}
			if event.Session.Agent.Work == "middle" {
				sawMiddle = true
				continue
			}
			if event.Session.Agent.Work != "after" {
				continue
			}
			if !sawMiddle {
				t.Fatal("source projection collapsed an ordered transition")
			}
			frame, _, err := encode(session.Message{Info: new(process.Info())}, Options{Project: project})
			if err != nil {
				t.Fatal(err)
			}
			var terminal contract.SessionInfo
			if err := json.Unmarshal(frame[1:], &terminal); err != nil {
				t.Fatal(err)
			}
			details := project(process.Info())
			if terminal.Agent.Work != details.Agent.Work || terminal.Agent.Work != event.Session.Agent.Work {
				t.Fatal("detail, terminal and list projections diverged")
			}
			return
		case <-ctx.Done():
			t.Fatal("lost committed metadata update")
		}
	}
}

func TestProjectedListDisconnectsWithoutBlockingPublication(t *testing.T) {
	sub := &projectedList{Out: make(chan contract.SessionEvent, 1), Slow: make(chan struct{})}
	sub.Observe(session.Event{Type: "snapshot"})
	for range projectedListQueueCapacity {
		sub.Observe(session.Event{Type: "updated", Session: &session.Info{ID: "runtime"}})
	}
	select {
	case <-sub.Slow:
	default:
		t.Fatal("full projection queue did not mark the consumer slow")
	}
	if len(sub.Out) != 1 {
		t.Fatal("overflow changed the already-queued snapshot")
	}
}
