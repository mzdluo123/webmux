package history

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

func fixture(t *testing.T) (historysource.Request, *sql.DB) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/native-history/agy/conversation.sql")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "conversation.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err = db.Exec(string(data)); err != nil {
		t.Fatal(err)
	}
	return historysource.Request{Source: historysource.Source{Agent: "agy", Path: path, NativeID: "00000000-0000-4000-8000-000000000001"}}, db
}

func TestNativeFixtureToolChain(t *testing.T) {
	req, _ := fixture(t)
	doc, err := Read(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Records) != 7 {
		t.Fatalf("records=%d", len(doc.Records))
	}
	call, result, system := doc.Records[2], doc.Records[3], doc.Records[5]
	if call.Kind != "tool_call" || call.ToolName != "run_command" || call.CallID != "fixture-id-0008" {
		t.Fatalf("call=%+v", call)
	}
	if result.Kind != "tool_result" || result.CallID != call.CallID || system.CallID != call.CallID {
		t.Fatal("tool chain lost")
	}
	if doc.Records[0].Role != "user" || doc.Records[6].Role != "assistant" || doc.Records[6].Text != "[redacted]" {
		t.Fatal("messages lost")
	}
	if doc.Pending || doc.Cursor == "" || doc.Fingerprint == "" || !hasGap(doc, "reasoning-omitted") || !hasGap(doc, "opaque-payload-omitted") {
		t.Fatalf("metadata=%+v", doc)
	}
	sources, err := Discover(context.Background(), filepath.Dir(req.Source.Path))
	if err != nil || len(sources) != 1 || sources[0] != req.Source {
		t.Fatalf("discover=%+v %v", sources, err)
	}
	req.Source.NativeID = "wrong"
	if _, err = Read(context.Background(), req); !errors.Is(err, historysource.ErrIdentity) {
		t.Fatal(err)
	}
}

func hasGap(d historysource.Document, category string) bool {
	for _, g := range d.Gaps {
		if g.Category == category {
			return true
		}
	}
	return false
}

func field(n int, s string) []byte {
	b := binary.AppendUvarint(nil, uint64(n<<3|2))
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}
func response(text string) []byte {
	b := []byte{8, 15, 32, 3}
	return append(b, field(20, string(field(1, text)))...)
}

func TestFrozenAppendAndMutation(t *testing.T) {
	req, db := fixture(t)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	first, err := Read(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO steps(idx,step_type,status,step_payload) VALUES(6,15,3,?)", response("你好 🌍")); err != nil {
		t.Fatal(err)
	}
	req.Cursor = first.Cursor
	frozen, err := Read(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(frozen.Records) != len(first.Records) || frozen.Fingerprint != first.Fingerprint {
		t.Fatal("append changed frozen prefix")
	}
	req.Cursor = ""
	latest, err := Read(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Records[len(latest.Records)-1].Text != "你好 🌍" {
		t.Fatal("Unicode lost")
	}
	if _, err = db.Exec("UPDATE steps SET step_payload=? WHERE idx=5", response("changed")); err != nil {
		t.Fatal(err)
	}
	req.Cursor = first.Cursor
	if _, err = Read(context.Background(), req); !errors.Is(err, historysource.ErrChanged) {
		t.Fatalf("mutation: %v", err)
	}
}

func TestUnknownAndPrivateFields(t *testing.T) {
	req, db := fixture(t)
	planner := append(field(1, "public"), field(3, "PRIVATE REASONING")...)
	planner = append(planner, field(777, "SECRET UNKNOWN")...)
	blob := append([]byte{8, 15, 32, 3}, field(20, string(planner))...)
	if _, err := db.Exec("UPDATE steps SET step_payload=? WHERE idx=5", blob); err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !hasGap(doc, "unknown-protobuf-field") {
		t.Fatal("unknown field unreported")
	}
	for _, r := range doc.Records {
		if strings.Contains(r.Text, "PRIVATE") || strings.Contains(r.Text, "SECRET") {
			t.Fatal("private field leaked")
		}
	}
	if doc.Records[len(doc.Records)-1].Text != "public" {
		t.Fatal("public response lost")
	}
}

func TestCancellationMalformedAndLimits(t *testing.T) {
	req, db := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := Discover(ctx, filepath.Dir(req.Source.Path)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, blob := range [][]byte{{0}, {8, 15, 32, 3, 162, 1, 255}, {8, 15, 32, 3, 11}} {
		if _, err := db.Exec("UPDATE steps SET step_payload=? WHERE idx=5", blob); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(context.Background(), req); !errors.Is(err, historysource.ErrFormat) {
			t.Fatalf("malformed %x: %v", blob, err)
		}
	}
	if _, err := db.Exec("UPDATE steps SET step_payload=zeroblob(?) WHERE idx=5", historysource.MaxRecordBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), req); !errors.Is(err, historysource.ErrLimit) {
		t.Fatal(err)
	}
}

func TestExternalParentsAndMissingSteps(t *testing.T) {
	req, db := fixture(t)
	if _, err := db.Exec("DELETE FROM steps WHERE idx=3; INSERT INTO parent_references VALUES(0,x'00'); UPDATE steps SET has_subtrajectory=1 WHERE idx=2"); err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range []string{"missing-step", "external-parent-unresolved", "external-subtrajectory"} {
		if !hasGap(doc, category) {
			t.Fatal(category)
		}
	}
	req.Source.Branch = "unverified"
	if _, err = Read(context.Background(), req); !errors.Is(err, historysource.ErrFormat) {
		t.Fatal(err)
	}
}

func TestWireBounds(t *testing.T) {
	for _, b := range [][]byte{{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 1}, {10, 255, 255, 255, 255, 255, 255, 255, 255, 127}, {13, 1}} {
		if _, err := decode(b, nil); !errors.Is(err, historysource.ErrFormat) {
			t.Fatalf("%x: %v", b, err)
		}
	}
}
