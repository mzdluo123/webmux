package conversation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
)

// This fixture is deliberately pre-migration SQL: the production opener will
// eventually stop creating these tables, but upgrades must keep reading them.
func legacyFrozenFixture(t *testing.T) *Service {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	legacyFrozenExec(t, db, `PRAGMA foreign_keys=ON;
 CREATE TABLE conversation(id TEXT PRIMARY KEY,parent_id TEXT REFERENCES conversation(id) ON DELETE SET NULL,relationship TEXT NOT NULL DEFAULT '',history_origin TEXT NOT NULL DEFAULT 'unknown');
 CREATE TABLE provider_thread(id TEXT PRIMARY KEY,conversation_id TEXT REFERENCES conversation(id) ON DELETE CASCADE,agent_session_id TEXT NOT NULL DEFAULT '');
 CREATE TABLE activity(id TEXT PRIMARY KEY,conversation_id TEXT REFERENCES conversation(id) ON DELETE CASCADE,position INTEGER,kind TEXT,role TEXT,status TEXT,heading_length INTEGER NOT NULL DEFAULT 0,byte_length INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE activity_chunk(activity_id TEXT REFERENCES activity(id) ON DELETE CASCADE,byte_offset INTEGER,byte_length INTEGER,text BLOB,PRIMARY KEY(activity_id,byte_offset));
 CREATE TABLE activity_heading_chunk(activity_id TEXT REFERENCES activity(id) ON DELETE CASCADE,byte_offset INTEGER,byte_length INTEGER,text BLOB,PRIMARY KEY(activity_id,byte_offset));
 CREATE TABLE activity_attachment(activity_id TEXT REFERENCES activity(id) ON DELETE CASCADE,position INTEGER,name TEXT,mime_type TEXT,reference TEXT,size INTEGER);
 CREATE TABLE history_gap(id TEXT PRIMARY KEY,conversation_id TEXT REFERENCES conversation(id) ON DELETE CASCADE,reason TEXT,resolved INTEGER,created_at INTEGER);
 CREATE TABLE native_source(id TEXT PRIMARY KEY,conversation_id TEXT REFERENCES conversation(id) ON DELETE CASCADE);
 CREATE TABLE history_snapshot(id TEXT PRIMARY KEY,source_id TEXT REFERENCES native_source(id) ON DELETE CASCADE,diagnostics BLOB);
 CREATE TABLE history_snapshot_item(snapshot_id TEXT REFERENCES history_snapshot(id) ON DELETE CASCADE,activity_id TEXT REFERENCES activity(id) ON DELETE CASCADE,position INTEGER);
 CREATE TABLE history_snapshot_gap(snapshot_id TEXT REFERENCES history_snapshot(id) ON DELETE CASCADE,position INTEGER,id TEXT,reason TEXT);
 CREATE TABLE history_snapshot_reference(snapshot_id TEXT REFERENCES history_snapshot(id) ON DELETE CASCADE,owner TEXT);
 CREATE TABLE context_handoff(id TEXT PRIMARY KEY,source_conversation_id TEXT REFERENCES conversation(id) ON DELETE CASCADE,target_conversation_id TEXT REFERENCES conversation(id) ON DELETE CASCADE,created_at INTEGER);
 CREATE TABLE handoff_preview(handoff_id TEXT PRIMARY KEY REFERENCES context_handoff(id) ON DELETE CASCADE,source_position INTEGER,state TEXT);
 CREATE TABLE handoff_item(handoff_id TEXT REFERENCES context_handoff(id) ON DELETE CASCADE,item_id TEXT,selected INTEGER,position INTEGER);
 CREATE TABLE handoff_delivery(id TEXT PRIMARY KEY,handoff_id TEXT REFERENCES context_handoff(id) ON DELETE CASCADE);
 CREATE TABLE handoff_execution(id TEXT PRIMARY KEY,delivery_id TEXT REFERENCES handoff_delivery(id) ON DELETE CASCADE,call_started INTEGER);
 CREATE TABLE inline_receipt(execution_id TEXT PRIMARY KEY,target_provider_thread_id TEXT REFERENCES provider_thread(id) ON DELETE CASCADE,user_text TEXT,provider_item_id TEXT);
 CREATE TABLE native_injection(delivery_id TEXT PRIMARY KEY,target_provider_thread_id TEXT REFERENCES provider_thread(id) ON DELETE CASCADE,state TEXT,call_started INTEGER);
 CREATE TABLE native_injection_item(delivery_id TEXT REFERENCES native_injection(delivery_id),item_id TEXT,provider_item_id TEXT);
 CREATE TABLE dispatch_receipt(execution_id TEXT PRIMARY KEY,target_provider_thread_id TEXT REFERENCES provider_thread(id) ON DELETE CASCADE,status TEXT,call_started INTEGER,delivery_id TEXT);
 `)
	legacyFrozenExec(t, db, frozenSchema)
	return &Service{db: db}
}

func legacyFrozenExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}
func legacyFrozenAdd(t *testing.T, s *Service, owner, id, kind, role, text string, position any) {
	t.Helper()
	legacyFrozenExec(t, s.db, `INSERT INTO activity(id,conversation_id,position,kind,role,status,byte_length) VALUES(?,?,?,?,?,'completed',?)`, id, owner, position, kind, role, len(text))
	// Split inside UTF-8 code points, as historical chunk storage could do.
	for start := 0; start < len(text); {
		end := min(len(text), start+65533)
		legacyFrozenExec(t, s.db, `INSERT INTO activity_chunk VALUES(?,?,?,?)`, id, start, end-start, []byte(text[start:end]))
		start = end
	}
}
func legacyFrozenMigrate(t *testing.T, s *Service) {
	t.Helper()
	if err := s.transact(context.Background(), func(tx *sql.Tx) error { return migrateLegacyFrozen(context.Background(), tx) }); err != nil {
		t.Fatal(err)
	}
}
func legacyFrozenDocument(t *testing.T, s *Service, id string) historysource.Document {
	t.Helper()
	doc, err := historyimport.New(historyimport.Options{Store: s}).Document(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
func legacyFrozenPrefix(t *testing.T, s *Service, owner string) []string {
	t.Helper()
	ids, err := s.PrefixSnapshots(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	return ids
}
func legacyFrozenHasGap(doc historysource.Document, category string) bool {
	for _, gap := range doc.Gaps {
		if gap.Category == category {
			return true
		}
	}
	return false
}

func TestLegacyFrozenLongUnicodeAndPrivateKinds(t *testing.T) {
	s := legacyFrozenFixture(t)
	legacyFrozenExec(t, s.db, `INSERT INTO conversation(id) VALUES('source'); INSERT INTO provider_thread(id,conversation_id) VALUES('provider','source')`)
	body := strings.Repeat("中文🙂\n", 100000)
	legacyFrozenAdd(t, s, "source", "public", "message", "agent", body, 1)
	legacyFrozenAdd(t, s, "source", "thought", "reasoning", "agent", "private reasoning", 2)
	legacyFrozenAdd(t, s, "source", "permission", "permission", "user", "private permission", 3)
	legacyFrozenAdd(t, s, "source", "system", "message", "system", "private system", 4)
	legacyFrozenAdd(t, s, "source", "draft", "message", "user", "uncommitted", nil)
	legacyFrozenAdd(t, s, "source", "tool", "tool", "tool", "original tool result", 5)
	legacyFrozenExec(t, s.db, `INSERT INTO activity_attachment VALUES('public',0,'image.png','image/png','sha256:verified-reference',12); INSERT INTO history_gap VALUES('gap','source','capture_disabled',0,1)`)
	legacyFrozenMigrate(t, s)
	ids := legacyFrozenPrefix(t, s, "source")
	if len(ids) != 1 {
		t.Fatalf("prefixes: %v", ids)
	}
	doc := legacyFrozenDocument(t, s, ids[0])
	if len(doc.Records) != 2 || doc.Records[0].ID != "public" || doc.Records[0].Text != body || doc.Records[1].Role != "tool" || doc.Records[1].Kind != "tool" || doc.Records[1].Offset != 5 {
		t.Fatal("original content, roles or order lost")
	}
	if doc.Source.Agent != "legacy-webmux" || doc.Source.NativeID != "source" || doc.Generation != "legacy-activity/v1" || !legacyFrozenHasGap(doc, "legacy_history_origin_unknown") || !legacyFrozenHasGap(doc, "capture_disabled") || !legacyFrozenHasGap(doc, "legacy_attachment_body_unavailable") {
		t.Fatalf("legacy provenance missing: %+v", doc.Source)
	}
	if got := doc.Records[0].Attachments; len(got) != 1 || got[0].Reference != "sha256:verified-reference" {
		t.Fatalf("attachment reference: %+v", got)
	}
	data, err := s.Frozen(context.Background(), ids[0])
	if err != nil {
		t.Fatal(err)
	}
	doc.Pending = false
	doc.RecordRefs = nil
	doc.References = nil
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(encoded)
	if data.Snapshot.ContentHash != hex.EncodeToString(sum[:]) {
		t.Fatal("document hash not compatible with SourceService")
	}
	recordBytes, _ := json.Marshal(doc.Records[0])
	recordHash := sha256.Sum256(recordBytes)
	if data.Snapshot.Records[0].Hash != hex.EncodeToString(recordHash[:]) {
		t.Fatal("record hash not compatible with SourceService")
	}
	// Retained reads must not depend on either legacy text table or an adapter.
	legacyFrozenExec(t, s.db, `DELETE FROM activity`)
	if legacyFrozenDocument(t, s, ids[0]).Records[0].Text != body {
		t.Fatal("retained export depended on Activity")
	}
}

func TestLegacyFrozenOldSnapshotSurvivesParentDeletionAndPreservesUnknown(t *testing.T) {
	s := legacyFrozenFixture(t)
	legacyFrozenExec(t, s.db, `INSERT INTO conversation(id) VALUES('parent'); INSERT INTO conversation(id,parent_id,relationship) VALUES('child','parent','fork'); INSERT INTO provider_thread(id,conversation_id,agent_session_id) VALUES('target','child','native-child')`)
	legacyFrozenAdd(t, s, "parent", "early", "message", "user", "old frozen body", 1)
	legacyFrozenAdd(t, s, "parent", "later", "message", "agent", "must not widen old snapshot", 2)
	legacyFrozenAdd(t, s, "child", "own", "message", "user", "child request", 1)
	legacyFrozenExec(t, s.db, `INSERT INTO native_source VALUES('native','parent'); INSERT INTO history_snapshot VALUES('old-snapshot','native',?); INSERT INTO history_snapshot_item VALUES('old-snapshot','early',1); INSERT INTO history_snapshot_reference VALUES('old-snapshot','child'); INSERT INTO history_snapshot_gap VALUES('old-snapshot',0,'old-gap','missing_tool_result'); INSERT INTO native_injection(delivery_id,target_provider_thread_id,state,call_started) VALUES('delivery','target','unknown',1); INSERT INTO native_injection_item VALUES('delivery','injected-id','native-item'); INSERT INTO inline_receipt(execution_id,target_provider_thread_id,user_text,provider_item_id) VALUES('execution','target','unchanged new request','current-native-item'); INSERT INTO dispatch_receipt VALUES('execution','target','unknown',1,'delivery')`, []byte("pending_tail"))
	legacyFrozenMigrate(t, s)
	childPrefix := legacyFrozenPrefix(t, s, "child")
	if len(childPrefix) != 1 {
		t.Fatalf("ambiguous parent became projected prefix: %v", childPrefix)
	}
	childDoc := legacyFrozenDocument(t, s, childPrefix[0])
	if len(childDoc.Records) != 1 || childDoc.Records[0].ID != "own" || !legacyFrozenHasGap(childDoc, "legacy_fork_boundary_unknown") {
		t.Fatal("ambiguous fork invented inherited context")
	}
	doc := legacyFrozenDocument(t, s, "old-snapshot")
	if len(doc.Records) != 1 || doc.Records[0].ID != "early" || !legacyFrozenHasGap(doc, "missing_tool_result") || !legacyFrozenHasGap(doc, "pending_tail") {
		t.Fatal("old snapshot widened or lost diagnostics")
	}
	legacyFrozenExec(t, s.db, `DELETE FROM conversation WHERE id='parent'`)
	if legacyFrozenDocument(t, s, "old-snapshot").Records[0].Text != "old frozen body" {
		t.Fatal("parent deletion damaged old retained snapshot")
	}
	owners, err := s.FrozenForOwner(context.Background(), "child")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, id := range owners {
		if id == "old-snapshot" {
			found = true
		}
	}
	if !found {
		t.Fatal("old owner reference lost")
	}
	var state, request, item, status string
	if err := s.db.QueryRow(`SELECT n.state,i.user_text,m.provider_item_id,d.status FROM native_injection n JOIN native_injection_item m ON m.delivery_id=n.delivery_id JOIN inline_receipt i ON i.target_provider_thread_id=n.target_provider_thread_id JOIN dispatch_receipt d ON d.execution_id=i.execution_id`).Scan(&state, &request, &item, &status); err != nil {
		t.Fatal(err)
	}
	if state != "unknown" || status != "unknown" || request != "unchanged new request" || item != "native-item" {
		t.Fatal("target-owned receipt or uncertainty changed")
	}
}

func TestLegacyFrozenExactHandoffAndUnexecutedDraft(t *testing.T) {
	s := legacyFrozenFixture(t)
	legacyFrozenExec(t, s.db, `INSERT INTO conversation(id) VALUES('source'); INSERT INTO conversation(id,parent_id,relationship) VALUES('child','source','fork'),('draft','source','fork'); INSERT INTO provider_thread(id,conversation_id,agent_session_id) VALUES('child-thread','child','native-child'),('draft-thread','draft',''); INSERT INTO context_handoff VALUES('handoff','source','child',1),('draft-handoff','source','draft',2); INSERT INTO handoff_preview VALUES('handoff',1,'complete'),('draft-handoff',1,'preparing'); INSERT INTO handoff_item(handoff_id,item_id,selected,position) VALUES('handoff','first',1,1),('draft-handoff','first',1,1)`)
	legacyFrozenExec(t, s.db, `INSERT INTO handoff_delivery VALUES('delivered','handoff'); INSERT INTO handoff_execution VALUES('started','delivered',1)`)
	legacyFrozenAdd(t, s, "source", "first", "message", "user", "frozen boundary", 1)
	legacyFrozenAdd(t, s, "source", "second", "message", "agent", "newer content", 2)
	legacyFrozenMigrate(t, s)
	ids, err := s.HandoffSnapshots(context.Background(), "handoff")
	if err != nil || len(ids) != 1 {
		t.Fatalf("handoff sources: %v %v", ids, err)
	}
	doc := legacyFrozenDocument(t, s, ids[0])
	if len(doc.Records) != 1 || doc.Records[0].ID != "first" {
		t.Fatal("handoff range widened")
	}
	var item, hash, state string
	var selected int
	if err := s.db.QueryRow(`SELECT item_id,selected,source_hash FROM handoff_item WHERE handoff_id='handoff'`).Scan(&item, &selected, &hash); err != nil {
		t.Fatal(err)
	}
	if item != "first" || selected != 1 || hash == "" {
		t.Fatal("selected identity was not preserved")
	}
	if err := s.db.QueryRow(`SELECT state FROM handoff_preview WHERE handoff_id='draft-handoff'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "preparing" || len(legacyFrozenPrefix(t, s, "draft")) != 1 {
		t.Fatal("unexecuted fork draft changed lifecycle or inherited history")
	}
	child := legacyFrozenPrefix(t, s, "child")
	if len(child) != 2 || child[0] != ids[0] {
		t.Fatalf("executed child lacks exact parent prefix: %v", child)
	}
	legacyFrozenExec(t, s.db, `DELETE FROM conversation WHERE id='source'`)
	if legacyFrozenDocument(t, s, child[0]).Records[0].Text != "frozen boundary" {
		t.Fatal("child depends on deleted parent")
	}
}

func TestLegacyFrozenResourceFailureRollsBackAllPublishedBodies(t *testing.T) {
	s := legacyFrozenFixture(t)
	legacyFrozenExec(t, s.db, `INSERT INTO conversation(id) VALUES('a-valid'),('z-overlimit')`)
	legacyFrozenAdd(t, s, "a-valid", "valid", "message", "user", "must roll back too", 1)
	// Declared size alone exceeds the resource boundary; migration must fail
	// before attempting to allocate it or making an empty/placeholder export.
	legacyFrozenExec(t, s.db, `INSERT INTO activity(id,conversation_id,position,kind,role,status,byte_length) VALUES('oversize','z-overlimit',1,'message','user','completed',?)`, historysource.MaxRecordBytes+1)
	err := s.transact(context.Background(), func(tx *sql.Tx) error { return migrateLegacyFrozen(context.Background(), tx) })
	if !errors.Is(err, historysource.ErrLimit) {
		t.Fatalf("limit: %v", err)
	}
	if ids := legacyFrozenPrefix(t, s, "a-valid"); len(ids) != 0 {
		t.Fatalf("partial prefix committed: %v", ids)
	}
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM frozen_export`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("partial body survived rollback")
	}
	var body []byte
	if err := s.db.QueryRow(`SELECT text FROM activity_chunk WHERE activity_id='valid'`).Scan(&body); err != nil || string(body) != "must roll back too" {
		t.Fatal("rollback destroyed source data", err)
	}
}
