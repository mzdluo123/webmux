package hybrid

import (
	"errors"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agent/omp"
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
