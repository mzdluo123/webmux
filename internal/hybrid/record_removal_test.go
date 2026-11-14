package hybrid

import (
	"context"
	"errors"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile/omp"
	"github.com/rainchan/webmux/internal/session"
)

// A list consumer must re-read the canonical record when removed arrives:
// the runtime notification alone can still observe a recoverable archive.
func removedRecord(t *testing.T, h *harness, sub *session.ListSubscriber, id string) error {
	t.Helper()
	ctx := ctxT(t)
	for {
		select {
		case event := <-sub.Out:
			if event.Type == "removed" && event.ID == id {
				_, err := h.svc.Record(id)
				return err
			}
		case <-sub.Slow:
			t.Fatal("session consumer fell behind")
		case <-ctx.Done():
			t.Fatal("no canonical removal notification", ctx.Err())
		}
	}
}

func TestArchiveOnlyRemovalNotifiesCanonicalConsumer(t *testing.T) {
	for _, method := range []string{"remove", "forget"} {
		t.Run(method, func(t *testing.T) {
			h := newHarness(t, 0)
			rec := testRecord("abcdefgh")
			rec.State = StateClosed
			seedRecords(t, h.svc.records, []Record{rec})
			sub := h.manager.Subscribe()
			defer h.manager.Unsubscribe(sub)
			var err error
			if method == "forget" {
				err = h.svc.Forget(ctxT(t), rec.ID)
			} else {
				err = h.svc.RemoveRecord(ctxT(t), rec.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := removedRecord(t, h, sub, rec.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("notified archive removal before canonical deletion: %v", err)
			}
			if strings.Contains(h.file(), rec.ID) {
				t.Fatal("notified archive removal before durable deletion")
			}
		})
	}
}

func TestDeletedRuntimeRemovalNotifiesCanonicalConsumer(t *testing.T) {
	h := newHarness(t, 0)
	id := h.create(h.input(omp.Name, SideACP)).Info().ID
	h.closeSession(id)
	if _, err := h.svc.Record(id); err != nil {
		t.Fatal("runtime close lost its recoverable archive", err)
	}
	sub := h.manager.Subscribe()
	defer h.manager.Unsubscribe(sub)
	if err := h.svc.Forget(ctxT(t), id); err != nil {
		t.Fatal(err)
	}
	if err := removedRecord(t, h, sub, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted runtime's removal retained its canonical archive: %v", err)
	}
}

func TestRecordRemovalCommitOrdersCanonicalNotifications(t *testing.T) {
	for _, failCommit := range []bool{false, true} {
		name := "committed"
		if failCommit {
			name = "failed"
		}
		t.Run(name, func(t *testing.T) {
			h, db := historyHarness(t)
			shell := h.create(h.input(omp.Name, SideTUI))
			id := shell.Info().ID
			a, err := h.svc.actor(id)
			if err != nil {
				t.Fatal(err)
			}
			binding, err := db.Binding(ctxT(t), id)
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Kill(shell.Identity().PID, syscall.SIGKILL); err != nil {
				t.Fatal(err)
			}
			waitFor(t, "interrupted record", func() bool { return h.record(id).State == StateInterrupted })
			sub := h.manager.Subscribe()
			defer h.manager.Unsubscribe(sub)

			// Gate only the removal write, after clear has retired the real
			// actor and runtime. Readers keep seeing the precommit archive.
			ctx, cancel := context.WithCancel(ctxT(t))
			defer cancel()
			started, release := make(chan struct{}), make(chan struct{})
			r := h.svc.records
			r.writeMu.Lock()
			r.mu.Lock()
			commit := r.commitStore
			r.commitStore = func(next storedRecords) error {
				if slices.ContainsFunc(next.Records, func(rec Record) bool { return rec.ID == id }) {
					return commit(next)
				}
				close(started)
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
				if failCommit {
					return errors.New("injected archive removal failure")
				}
				return commit(next)
			}
			r.mu.Unlock()
			r.writeMu.Unlock()
			result, exited := make(chan error, 1), make(chan struct{})
			go func() {
				defer close(exited)
				result <- h.svc.RemoveRecord(ctx, id)
			}()
			defer func() {
				cancel()
				<-exited
				r.writeMu.Lock()
				r.mu.Lock()
				r.commitStore = commit
				r.mu.Unlock()
				r.writeMu.Unlock()
			}()
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("removal did not reach archive commit", ctx.Err())
			}
			if err := removedRecord(t, h, sub, id); err != nil {
				t.Fatal("runtime removal should still expose its precommit archive", err)
			}
			if _, err := h.svc.Resume(ctx, id, ResumeInput{ClientRequestID: rid()}); !errors.Is(err, ErrSwitching) {
				t.Fatal("resume raced archive removal", err)
			}
			if err := h.svc.RemoveRecord(ctx, id); !errors.Is(err, ErrSwitching) {
				t.Fatal("second removal raced archive commit", err)
			}
			if releaseWriter, err := h.svc.ClaimConversation(binding.ConversationID, "competing-writer"); err == nil {
				releaseWriter()
				t.Fatal("archive removal released its conversation lease before commit")
			} else if !errors.Is(err, ErrSwitching) {
				t.Fatal(err)
			}
			close(release)
			select {
			case err = <-result:
			case <-ctx.Done():
				t.Fatal("archive removal deadlocked", ctx.Err())
			}
			if failCommit {
				if !errors.Is(err, ErrUnavailable) {
					t.Fatal("archive write failure was hidden", err)
				}
				if _, err := h.svc.Record(id); err != nil || !strings.Contains(h.file(), id) {
					t.Fatal("failed archive write lost the retained record", err)
				}
				for len(sub.Out) > 0 {
					if event := <-sub.Out; event.Type == "removed" && event.ID == id {
						t.Fatal("failed archive write advertised successful removal")
					}
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := removedRecord(t, h, sub, id); !errors.Is(err, ErrNotFound) {
					t.Fatal("postcommit consumer retained the precommit archive", err)
				}
				if strings.Contains(h.file(), id) {
					t.Fatal("postcommit notice preceded durable archive deletion")
				}
			}
			select {
			case <-a.quit:
			case <-ctx.Done():
				t.Fatal("retired actor did not stop", ctx.Err())
			}
			releaseWriter, err := h.svc.ClaimConversation(binding.ConversationID, "next-writer")
			if err != nil {
				t.Fatal("removal leaked its conversation lease", err)
			}
			releaseWriter()
			releaseRecord, err := h.svc.claim(id)
			if err != nil {
				t.Fatal("removal leaked its record claim", err)
			}
			releaseRecord()
		})
	}
}

func TestRecordRemovalNotifiesBeforeFailedConversationCleanup(t *testing.T) {
	h, db := historyHarness(t)
	rec := testRecord("abcdefgh")
	rec.State = StateClosed
	seedRecords(t, h.svc.records, []Record{rec})
	if err := h.svc.migrateConversations(); err != nil {
		t.Fatal(err)
	}
	r := h.svc.records
	commit := r.commitStore
	// Fail SQLite only after the real archive deletion has committed. The
	// canonical record is gone even though the operation reports cleanup failure.
	r.commitStore = func(next storedRecords) error {
		if err := commit(next); err != nil {
			return err
		}
		if err := db.Close(); err != nil {
			t.Error(err)
		}
		return nil
	}
	defer func() { r.commitStore = commit }()
	sub := h.manager.Subscribe()
	defer h.manager.Unsubscribe(sub)
	if err := h.svc.RemoveRecord(ctxT(t), rec.ID); !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "cleanup incomplete") {
		t.Fatal("expected postcommit conversation cleanup failure", err)
	}
	if err := removedRecord(t, h, sub, rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cleanup failure hid the committed canonical removal", err)
	}
	if strings.Contains(h.file(), rec.ID) {
		t.Fatal("canonical removal was not durable")
	}
}
