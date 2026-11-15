package history

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

func fixture(t *testing.T, name string) (string, *sql.DB) {
	t.Helper()
	raw, e := os.ReadFile(filepath.Join("../../testdata/native-history/opencode", name+".sql"))
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "opencode.db")
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
func TestNativeGenerationsFreeze(t *testing.T) {
	for _, gen := range []string{"legacy", "v2"} {
		t.Run(gen, func(t *testing.T) {
			path, db := fixture(t, gen)
			req := historysource.Request{Source: historysource.Source{Agent: "opencode", Path: path, NativeID: "ses_fixture_" + gen}}
			doc, e := Read(context.Background(), req)
			if e != nil {
				t.Fatal(e)
			}
			if len(doc.Records) != 4 {
				t.Fatalf("records: %#v", doc.Records)
			}
			calls, results := 0, 0
			for _, r := range doc.Records {
				if r.Kind == "tool-call" {
					calls++
				}
				if r.Kind == "tool-result" {
					results++
				}
			}
			if calls != 1 || results != 1 {
				t.Fatal(doc.Records)
			}
			if len(doc.Gaps) == 0 {
				t.Fatal("missing reasoning gap")
			}
			req.Cursor = doc.Cursor
			var insert, update string
			if gen == "legacy" {
				insert = `INSERT INTO message VALUES ('later','ses_fixture_legacy',0,0,'{"role":"user"}'); INSERT INTO part VALUES ('later-part','later','ses_fixture_legacy',0,0,'{"type":"text","text":"后续"}');`
				update = `UPDATE part SET data='{"type":"text","text":"冻结变化"}' WHERE id='prt_fixture_legacy_0001'`
			} else {
				insert = `INSERT INTO session_message VALUES ('later','ses_fixture_v2','user',999,0,0,'{"text":"后续"}')`
				update = `UPDATE session_message SET data='{"text":"冻结变化"}' WHERE id='msg_fixture_v2_0001'`
			}
			if _, e = db.Exec(insert); e != nil {
				t.Fatal(e)
			}
			frozen, e := Read(context.Background(), req)
			if e != nil || frozen.Fingerprint != doc.Fingerprint || len(frozen.Records) != len(doc.Records) {
				t.Fatalf("frozen: %v %#v", e, frozen)
			}
			if _, e = db.Exec(update); e != nil {
				t.Fatal(e)
			}
			if _, e = Read(context.Background(), req); !errors.Is(e, historysource.ErrChanged) {
				t.Fatalf("changed: %v", e)
			}
			sources, e := Discover(context.Background(), filepath.Dir(path))
			if e != nil || len(sources) != 1 || sources[0].NativeID != req.Source.NativeID {
				t.Fatalf("discover: %v %#v", e, sources)
			}
		})
	}
}
func TestIdentityCancellationAndParts(t *testing.T) {
	path, db := fixture(t, "legacy")
	req := historysource.Request{Source: historysource.Source{Agent: "opencode", Path: path, NativeID: "wrong"}}
	if _, e := Read(context.Background(), req); !errors.Is(e, historysource.ErrIdentity) {
		t.Fatal(e)
	}
	req.Source.NativeID = "ses_fixture_legacy"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Read(ctx, req); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := Discover(ctx, filepath.Dir(path)); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := db.Exec(`UPDATE part SET data='{"type":"text","text":"你好🦊"}' WHERE id='prt_fixture_legacy_0001'; INSERT INTO part VALUES ('compact','msg_fixture_legacy_0003','ses_fixture_legacy',1700000000099,0,'{"type":"compaction"}');`); e != nil {
		t.Fatal(e)
	}
	doc, e := Read(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	if doc.Records[0].Text != "你好🦊" {
		t.Fatal(doc.Records)
	}
	found := false
	for _, g := range doc.Gaps {
		found = found || g.Category == "compaction"
	}
	if !found {
		t.Fatal(doc.Gaps)
	}
	req.Source.Branch = "msg_fixture_legacy_0003"
	doc, e = Read(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range doc.Records {
		if strings.HasPrefix(r.ID, "prt_fixture_legacy_0004") {
			t.Fatal("sibling tool mixed into branch")
		}
	}
}
