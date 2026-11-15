package historyimport_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	codexhistory "github.com/rainchan/webmux/internal/agent/codex/history"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
)

func importFixture(t *testing.T) (*conversation.Service, *sql.DB, historysource.Source, []byte) {
	t.Helper()
	state := t.TempDir()
	store := conversation.Open(conversation.Options{StateDir: state})
	if err := store.Unavailable(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db, err := sql.Open("sqlite", filepath.Join(state, "conversations.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	body, err := os.ReadFile("../agent/testdata/native-history/codex/session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return store, db, historysource.Source{Agent: "codex", NativeID: "00000001-0000-4000-8000-000000000003", Path: path}, body
}

func importService(store historyimport.Store) *historyimport.Service {
	return historyimport.New(historyimport.Options{Store: store, Readers: map[string]historysource.ReadFunc{"codex": codexhistory.Read}})
}

func importCount(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestImportCodexMetadataOnlyFrozenRange(t *testing.T) {
	for _, retain := range []bool{false, true} {
		t.Run(map[bool]string{false: "reference", true: "retained"}[retain], func(t *testing.T) {
			// No CLI or runtime registry is available. Only the real file reader
			// and the sanitized, actual #653 session recording are used.
			t.Setenv("PATH", t.TempDir())
			store, db, source, body := importFixture(t)
			svc := importService(store)
			ctx := context.Background()
			binding, snapshot, err := svc.Import(ctx, source, retain)
			if err != nil {
				t.Fatal(err)
			}
			if binding.ConversationID == "" || binding.ProviderThreadID == "" || binding.Source.NativeID != source.NativeID || binding.Source.Path != source.Path || binding.Source.Branch != source.NativeID || binding.Source.ProviderThreadID != binding.ProviderThreadID || binding.Root != "" || snapshot.Retained != retain {
				t.Fatalf("import identity: %+v %+v", binding, snapshot)
			}
			unchanged, err := os.ReadFile(source.Path)
			if err != nil || !bytes.Equal(unchanged, body) {
				t.Fatalf("import modified native source: %v", err)
			}
			var cwd, origin, status, config, root string
			if err := db.QueryRow(`SELECT c.cwd,c.history_origin,p.status,p.config_id,p.native_root FROM conversation c JOIN provider_thread p ON p.conversation_id=c.id WHERE c.id=?`, binding.ConversationID).Scan(&cwd, &origin, &status, &config, &root); err != nil {
				t.Fatal(err)
			}
			if cwd != "" || origin != "bounded_import" || status != "imported" || config != "" || root != "" {
				t.Fatalf("fabricated runtime metadata: %q %q %q %q %q", cwd, origin, status, config, root)
			}
			for _, table := range []string{"run", "runtime_binding", "native_source_binding"} {
				if count := importCount(t, db, "SELECT count(*) FROM "+table); count != 0 {
					t.Fatalf("import wrote %s: %d", table, count)
				}
			}
			if count := importCount(t, db, "SELECT count(*) FROM sqlite_schema WHERE type='table' AND name IN ('activity','activity_chunk','activity_attachment','native_record','native_source')"); count != 0 {
				t.Fatalf("obsolete live body tables remain: %d", count)
			}
			if count := importCount(t, db, "SELECT count(*) FROM frozen_export"); count != map[bool]int{false: 0, true: 1}[retain] {
				t.Fatalf("unexpected retained bodies: %d", count)
			}
			if count := importCount(t, db, "SELECT count(*) FROM frozen_reference WHERE owner LIKE 'import-pending:%'"); count != 0 {
				t.Fatalf("temporary owner leaked: %d", count)
			}
			ids, err := store.PrefixSnapshots(ctx, binding.ConversationID)
			if err != nil || !reflect.DeepEqual(ids, []string{snapshot.ID}) {
				t.Fatalf("missing frozen prefix: %v %v", ids, err)
			}
			owners, err := store.FrozenForOwner(ctx, binding.ConversationID)
			if err != nil || !reflect.DeepEqual(owners, ids) {
				t.Fatalf("missing conversation lifetime: %v %v", owners, err)
			}
			before, err := svc.Document(ctx, snapshot.ID)
			if err != nil || len(before.Records) == 0 {
				t.Fatalf("imported document: %+v %v", before, err)
			}
			// A genuine message envelope appended after import must never enter
			// either retained or reference imports' fixed prefix.
			const appended = "{\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"id\":\"after-import\",\"role\":\"user\",\"content\":[{\"type\":\"input_text\",\"text\":\"later suffix\"}]}}\n"
			if err := os.WriteFile(source.Path, append(body, []byte(appended)...), 0600); err != nil {
				t.Fatal(err)
			}
			after, err := svc.Documents(ctx, ids)
			if err != nil || !reflect.DeepEqual(after.Records, before.Records) {
				t.Fatalf("import grew with source: %+v %v", after, err)
			}
			if err := os.Remove(source.Path); err != nil {
				t.Fatal(err)
			}
			kept, err := svc.Document(ctx, snapshot.ID)
			if retain {
				if err != nil || !reflect.DeepEqual(kept.Records, before.Records) {
					t.Fatalf("retained import lost source body: %+v %v", kept, err)
				}
			} else if !errors.Is(err, historysource.ErrUnavailable) {
				t.Fatalf("missing reference source silently accepted: %v", err)
			}
		})
	}
}

func TestImportFailureRollsBackAndReleasesPending(t *testing.T) {
	store, db, source, _ := importFixture(t)
	if _, err := db.Exec(`CREATE TRIGGER fail_import BEFORE INSERT ON conversation_prefix BEGIN SELECT RAISE(ABORT,'import failure'); END`); err != nil {
		t.Fatal(err)
	}
	binding, snapshot, err := importService(store).Import(context.Background(), source, true)
	if err == nil || binding.ConversationID != "" || snapshot.ID != "" {
		t.Fatalf("failed import published identity: %+v %+v %v", binding, snapshot, err)
	}
	for _, table := range []string{"conversation", "provider_thread", "native_source_binding", "conversation_prefix", "frozen_reference", "frozen_snapshot", "frozen_export"} {
		if count := importCount(t, db, "SELECT count(*) FROM "+table); count != 0 {
			t.Fatalf("failed import leaked %s: %d", table, count)
		}
	}
}

type cancelImportStore struct {
	*conversation.Service
	cancel context.CancelFunc
}

func (s cancelImportStore) ImportFrozenSource(ctx context.Context, id, owner string) (historysource.Binding, error) {
	s.cancel()
	return s.Service.ImportFrozenSource(ctx, id, owner)
}

func TestImportCancellationReleasesPending(t *testing.T) {
	store, db, source, _ := importFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := importService(cancelImportStore{Service: store, cancel: cancel})
	if _, _, err := svc.Import(ctx, source, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled import: %v", err)
	}
	for _, table := range []string{"conversation", "provider_thread", "frozen_reference", "frozen_snapshot", "frozen_export"} {
		if count := importCount(t, db, "SELECT count(*) FROM "+table); count != 0 {
			t.Fatalf("cancelled import leaked %s: %d", table, count)
		}
	}
}

func TestImportFrozenSourceRequiresPendingManifest(t *testing.T) {
	store, db, source, _ := importFixture(t)
	ctx := context.Background()
	snapshot, err := importService(store).Freeze(ctx, source, "pending", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportFrozenSource(ctx, snapshot.ID, "wrong-owner"); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatalf("import without pending ownership: %v", err)
	}
	if _, err := db.Exec(`UPDATE frozen_snapshot SET manifest=? WHERE id=?`, []byte(`{"ID":"wrong-id"}`), snapshot.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportFrozenSource(ctx, snapshot.ID, "pending"); !errors.Is(err, conversation.ErrInvalid) {
		t.Fatalf("import ignored stored manifest identity: %v", err)
	}
	if count := importCount(t, db, "SELECT count(*) FROM conversation"); count != 0 {
		t.Fatalf("invalid manifest published conversation: %d", count)
	}
}
