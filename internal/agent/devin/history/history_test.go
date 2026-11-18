package history

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

func fixture(t *testing.T) (string, *sql.DB) {
	t.Helper()
	raw, e := os.ReadFile("../../testdata/native-history/devin/sessions.sql")
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "sessions.db")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if _, e = db.Exec(string(raw)); e != nil {
		t.Fatal(e)
	}
	return path, db
}
func TestCommittedNativeChain(t *testing.T) {
	path, db := fixture(t)
	req := historysource.Request{Source: historysource.Source{Agent: "devin", Path: path, NativeID: "fixture-session-001"}}
	if _, e := db.Exec(`INSERT INTO message_nodes(session_id,node_id,parent_node_id,chat_message,created_at) VALUES ('fixture-session-001',100,7,'{"role":"assistant","content":"INFERENCE-SIBLING"}',0)`); e != nil {
		t.Fatal(e)
	}
	doc, e := Read(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	if len(doc.Records) != 7 {
		t.Fatalf("records: %#v", doc.Records)
	}
	for _, r := range doc.Records {
		if r.Text == "INFERENCE-SIBLING" {
			t.Fatal("inference sibling leaked")
		}
		if r.Kind == "tool-call" && r.Status != "completed" {
			t.Fatal(r)
		}
	}
	if len(doc.Gaps) == 0 {
		t.Fatal("system and compaction gaps missing")
	}
	req.Cursor = doc.Cursor
	if _, e = db.Exec(`INSERT INTO message_nodes(session_id,node_id,parent_node_id,chat_message,created_at) VALUES ('fixture-session-001',11,10,'{"role":"user","content":"新增🦊"}',0); UPDATE sessions SET main_chain_id=11`); e != nil {
		t.Fatal(e)
	}
	frozen, e := Read(context.Background(), req)
	if e != nil || len(frozen.Records) != 7 || frozen.Fingerprint != doc.Fingerprint {
		t.Fatalf("freeze: %v %#v", e, frozen)
	}
	req.Cursor = ""
	latest, e := Read(context.Background(), req)
	if e != nil || latest.Records[len(latest.Records)-1].Text != "新增🦊" {
		t.Fatalf("unicode append: %v %#v", e, latest)
	}
	req.Cursor = doc.Cursor
	if _, e = db.Exec(`UPDATE message_nodes SET chat_message='{"role":"user","content":"changed"}' WHERE node_id=6`); e != nil {
		t.Fatal(e)
	}
	if _, e = Read(context.Background(), req); !errors.Is(e, historysource.ErrChanged) {
		t.Fatal(e)
	}
	req.Cursor = ""
	req.Source.Branch = "100"
	if _, e = Read(context.Background(), req); !errors.Is(e, historysource.ErrIdentity) {
		t.Fatal(e)
	}
	req.Source.Branch = "9"
	branch, e := Read(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range branch.Records {
		if r.ID == "10" || r.ID == "11" {
			t.Fatal("later descendant in branch")
		}
	}
	sources, e := Discover(context.Background(), filepath.Dir(path))
	if e != nil || len(sources) != 1 || sources[0].NativeID != "fixture-session-001" {
		t.Fatalf("discover: %v %#v", e, sources)
	}
}
func TestIdentityCancellationAndMissingAncestor(t *testing.T) {
	path, db := fixture(t)
	req := historysource.Request{Source: historysource.Source{Agent: "devin", Path: path, NativeID: "independent-atif-session"}}
	if _, e := Read(context.Background(), req); !errors.Is(e, historysource.ErrIdentity) {
		t.Fatal(e)
	}
	req.Source.NativeID = "fixture-session-001"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Read(ctx, req); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := Discover(ctx, filepath.Dir(path)); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := db.Exec(`DELETE FROM message_nodes WHERE node_id=5`); e != nil {
		t.Fatal(e)
	}
	doc, e := Read(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, g := range doc.Gaps {
		found = found || g.Category == "missing-ancestor"
	}
	if !found {
		t.Fatal(doc.Gaps)
	}
}
