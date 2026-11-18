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
	return historysource.Request{Source: historysource.Source{Agent: "agy", Path: path, NativeID: "00000000-0000-4000-8000-000000000002"}}, db
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
	if call.Text != `{"CommandLine":"[redacted]","WaitMsBeforeAsync":0,"toolSummary":"[redacted]"}` || result.Text != "[redacted]" {
		t.Fatal("tool arguments or result lost")
	}
	if call.ParentID != doc.Records[1].ID || result.ParentID != doc.Records[1].ID {
		t.Fatal("tool ancestry lost")
	}
	if !strings.Contains(call.ID, req.Source.NativeID+"/trajectory/00000000-0000-4000-8000-000000000001/") {
		t.Fatal("record identity does not bind cascade and trajectory")
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

func TestIdentityValidationAndDiscoveryCandidates(t *testing.T) {
	tests := []struct {
		name       string
		give       string
		candidates int
	}{
		{"missing-row", "DELETE FROM trajectory_meta", 0},
		{"empty-cascade", "UPDATE trajectory_meta SET cascade_id=''", 0},
		{"blank-cascade", "UPDATE trajectory_meta SET cascade_id='  '", 0},
		{"null-cascade", "UPDATE trajectory_meta SET cascade_id=NULL", 0},
		{"binary-cascade", "UPDATE trajectory_meta SET cascade_id=x'6162'", 0},
		{"long-cascade", "UPDATE trajectory_meta SET cascade_id=printf('%1025s','x')", 0},
		{"empty-trajectory", "UPDATE trajectory_meta SET trajectory_id=''", 1},
		{"blank-trajectory", "UPDATE trajectory_meta SET trajectory_id='  '", 1},
		{"null-trajectory", "UPDATE trajectory_meta SET trajectory_id=NULL", 1},
		{"binary-trajectory", "UPDATE trajectory_meta SET trajectory_id=x'6162'", 1},
		{"long-trajectory", "UPDATE trajectory_meta SET trajectory_id=printf('%1025s','x')", 1},
		{"multiple-rows", "INSERT INTO trajectory_meta VALUES('another-trajectory','another-cascade',4,17)", 2},
		{"duplicate-cascade", "INSERT INTO trajectory_meta VALUES('another-trajectory','00000000-0000-4000-8000-000000000002',4,17)", 2},
		{"missing-trajectory-column", "ALTER TABLE trajectory_meta RENAME COLUMN trajectory_id TO future_id", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, db := fixture(t)
			if _, err := db.Exec(tt.give); err != nil {
				t.Fatal(err)
			}
			if _, err := Read(context.Background(), req); !errors.Is(err, historysource.ErrFormat) {
				t.Fatalf("invalid mapping: %v", err)
			}
			sources, err := Discover(context.Background(), filepath.Dir(req.Source.Path))
			if err != nil || len(sources) != tt.candidates {
				t.Fatalf("candidates=%+v err=%v", sources, err)
			}
		})
	}
}

func TestFrozenIdentityBinding(t *testing.T) {
	for _, column := range []string{"cascade_id", "trajectory_id"} {
		t.Run(column, func(t *testing.T) {
			req, db := fixture(t)
			first, err := Read(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("UPDATE trajectory_meta SET " + column + "='rebound'"); err != nil {
				t.Fatal(err)
			}
			req.Cursor = first.Cursor
			want := historysource.ErrChanged
			if column == "cascade_id" {
				want = historysource.ErrIdentity
			}
			if _, err := Read(context.Background(), req); !errors.Is(err, want) {
				t.Fatalf("frozen mapping mutation: %v", err)
			}
			if column == "cascade_id" {
				req.Source.NativeID = "rebound"
				if _, err := Read(context.Background(), req); !errors.Is(err, historysource.ErrChanged) {
					t.Fatalf("rebound source reused cursor: %v", err)
				}
			}
			req.Cursor = ""
			latest, err := Read(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if latest.Fingerprint == first.Fingerprint || latest.Records[0].ID == first.Records[0].ID {
				t.Fatal("identity rebinding reused record IDs or digest")
			}
		})
	}
}

func TestTrajectoryCannotIdentifyRuntime(t *testing.T) {
	req, _ := fixture(t)
	req.Source.NativeID = "00000000-0000-4000-8000-000000000001"
	if _, err := Read(context.Background(), req); !errors.Is(err, historysource.ErrIdentity) {
		t.Fatalf("trajectory accepted as runtime identity: %v", err)
	}
}

func TestDiscoveryPreservesDirectoryAndLimitErrors(t *testing.T) {
	if _, err := Discover(context.Background(), filepath.Join(t.TempDir(), "absent")); !errors.Is(err, historysource.ErrUnavailable) {
		t.Fatalf("directory failure: %v", err)
	}
	t.Run("database-size", func(t *testing.T) {
		req, db := fixture(t)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Truncate(req.Source.Path, historysource.MaxBytes+1); err != nil {
			t.Fatal(err)
		}
		if _, err := Discover(context.Background(), filepath.Dir(req.Source.Path)); !errors.Is(err, historysource.ErrLimit) {
			t.Fatalf("database limit: %v", err)
		}
	})
	t.Run("identity-count", func(t *testing.T) {
		req, db := fixture(t)
		if _, err := db.Exec(`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<1000)
			INSERT INTO trajectory_meta SELECT printf('trajectory-%d',x),printf('cascade-%d',x),4,17 FROM n`); err != nil {
			t.Fatal(err)
		}
		if _, err := Discover(context.Background(), filepath.Dir(req.Source.Path)); !errors.Is(err, historysource.ErrLimit) {
			t.Fatalf("identity limit: %v", err)
		}
	})
}
