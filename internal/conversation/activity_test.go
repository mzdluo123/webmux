package conversation

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func runTest(t *testing.T, s *Service, b Binding, key string) Run {
	t.Helper()
	r, err := s.StartRun(context.Background(), b.ProviderThreadID, key)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestVersionOneUpgradeKeepsIdentityAndOrdinals(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "conversations.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO conversation(id,cwd,history_origin,next_run_ordinal,created_at,updated_at) VALUES('existing','/workspace','unknown',23,1,1)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s := openTest(t, dir)
	c, err := s.Conversation(ctx, "existing")
	if err != nil || c.NextRunOrdinal != 23 || c.HistoryOrigin != "unknown" {
		t.Fatalf("upgrade changed identity: %+v %v", c, err)
	}
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("schema version: %d %v", version, err)
	}
}

func TestInterruptedRunsRecoverOnlyControlUncertainty(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest(t, dir)
	binding := bindTest(t, s, "runtime")
	pending := runTest(t, s, binding, "unfinished")
	finished := runTest(t, s, binding, "finished")
	if err := s.FinishRun(ctx, finished.ID, "cancelled"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTest(t, dir)
	for _, expected := range []struct{ id, status string }{{pending.ID, "interrupted"}, {finished.ID, "cancelled"}} {
		var status string
		if err := s.db.QueryRow("SELECT status FROM run WHERE id=?", expected.id).Scan(&status); err != nil || status != expected.status {
			t.Fatal("recovery changed terminal control state", status, err)
		}
	}
	conversation, err := s.Conversation(ctx, binding.ConversationID)
	if err != nil || !conversation.ReconcileRequired {
		t.Fatal("crash uncertainty lost", conversation, err)
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM history_gap WHERE provider_thread_id=? AND reason='crash' AND resolved=0", binding.ProviderThreadID).Scan(&count); err != nil || count != 1 {
		t.Fatal("crash recovery lost or duplicated gap", count, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTest(t, dir)
	if err := s.db.QueryRow("SELECT count(*) FROM history_gap WHERE provider_thread_id=? AND reason='crash' AND resolved=0", binding.ProviderThreadID).Scan(&count); err != nil || count != 1 {
		t.Fatal("reopening duplicated crash gap", count, err)
	}
}
