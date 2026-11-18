package history_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agy "github.com/rainchan/webmux/internal/agent/agy/history"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
)

const captureCascade = "00000004-0000-4000-8000-000000000002"
const captureTrajectory = "00000004-0000-4000-8000-000000000001"

func runtimeFixture(t *testing.T, root, name, fixture string) *sql.DB {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../testdata/native-history/agy", fixture))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(string(data)); err != nil {
		t.Fatal(err)
	}
	return db
}

func runtimeService(root string, id *string) *historyimport.Service {
	return historyimport.New(historyimport.Options{
		Readers:     map[string]historysource.ReadFunc{"agy": agy.Read},
		Discoverers: map[string]historyimport.DiscoverFunc{"agy": agy.Discover},
		Resolve: func(context.Context, string) (historyimport.Identity, error) {
			return historyimport.Identity{Agent: "agy", NativeID: *id, Root: root}, nil
		},
	})
}

func TestRuntimeCaptureDiscoveryAndFrozenContent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	// The filename deliberately differs from both native identities.
	db := runtimeFixture(t, root, "unrelated-name.db", "conversation-1.5.7.sql")
	neighbor := runtimeFixture(t, root, "future.db", "conversation-1.5.7.sql")
	if _, err := neighbor.Exec("UPDATE trajectory_meta SET cascade_id='future-cascade'; UPDATE steps SET step_format=99"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "unrecognized.db"), []byte("unsupported database"), 0600); err != nil {
		t.Fatal(err)
	}
	id := captureCascade
	svc := runtimeService(root, &id)
	source, err := svc.RuntimeSource(ctx, "runtime")
	if err != nil || source.NativeID != captureCascade || source.Path != filepath.Join(root, "unrelated-name.db") {
		t.Fatalf("runtime cascade mapping: %+v %v", source, err)
	}
	scope, err := svc.FreezeRuntime(ctx, "runtime")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := svc.Document(ctx, scope.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Records) != 2 || doc.Records[0].Role != "user" || doc.Records[1].Role != "assistant" {
		t.Fatalf("capture records: %+v", doc.Records)
	}
	for _, fact := range []string{"WMX-agy-capture", "蓝鹭", "48273"} {
		if !strings.Contains(doc.Records[0].Text, fact) {
			t.Fatalf("synthetic fact lost: %s", fact)
		}
	}
	if doc.Records[1].Text != "ACK WMX-agy-capture" || doc.Records[1].ParentID != doc.Records[0].ID || len(doc.RecordRefs) != 2 {
		t.Fatal("assistant response, ancestry or frozen references lost")
	}
	if _, err := db.Exec(`INSERT INTO steps (idx,step_type,status,has_subtrajectory,metadata,step_payload,step_format)
		SELECT 2,step_type,status,has_subtrajectory,metadata,step_payload,step_format FROM steps WHERE idx=1`); err != nil {
		t.Fatal(err)
	}
	frozen, err := svc.Document(ctx, scope.ID)
	if err != nil || len(frozen.Records) != 2 || frozen.Cursor != doc.Cursor || frozen.Fingerprint != doc.Fingerprint {
		t.Fatalf("append changed frozen content: %v", err)
	}
	var payload []byte
	if err := db.QueryRow("SELECT step_payload FROM steps WHERE idx=0").Scan(&payload); err != nil {
		t.Fatal(err)
	}
	changed := bytes.ReplaceAll(payload, []byte("48273"), []byte("48274"))
	if bytes.Equal(changed, payload) {
		t.Fatal("capture lacks synthetic port")
	}
	if _, err := db.Exec("UPDATE steps SET step_payload=? WHERE idx=0", changed); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Document(ctx, scope.ID); !errors.Is(err, historysource.ErrChanged) {
		t.Fatalf("content mutation accepted: %v", err)
	}
	if _, err := db.Exec("UPDATE steps SET step_payload=? WHERE idx=0", payload); err != nil {
		t.Fatal(err)
	}
	id = captureTrajectory
	if _, err := svc.RuntimeSource(ctx, "runtime"); !errors.Is(err, historysource.ErrIdentity) {
		t.Fatalf("trajectory accepted as runtime cascade: %v", err)
	}
	id = "future-cascade"
	if _, err := svc.RuntimeSource(ctx, "runtime"); !errors.Is(err, historysource.ErrFormat) {
		t.Fatalf("unsupported target misclassified: %v", err)
	}
	if _, err := svc.FreezeRuntime(ctx, "runtime"); !errors.Is(err, historysource.ErrFormat) {
		t.Fatalf("unsupported freeze misclassified: %v", err)
	}
	id = captureCascade
	if _, err := neighbor.Exec("UPDATE trajectory_meta SET cascade_id=?", captureCascade); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RuntimeSource(ctx, "runtime"); !errors.Is(err, historysource.ErrIdentity) {
		t.Fatalf("unsupported duplicate hidden: %v", err)
	}
}

func TestRuntimeCaptureFrozenIdentityMutation(t *testing.T) {
	for _, column := range []string{"cascade_id", "trajectory_id"} {
		t.Run(column, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			db := runtimeFixture(t, root, "capture.db", "conversation-1.5.7.sql")
			id := captureCascade
			svc := runtimeService(root, &id)
			scope, err := svc.FreezeRuntime(ctx, "runtime")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("UPDATE trajectory_meta SET " + column + "='rebound'"); err != nil {
				t.Fatal(err)
			}
			want := historysource.ErrChanged
			if column == "cascade_id" {
				want = historysource.ErrIdentity
			}
			if _, err := svc.Document(ctx, scope.ID); !errors.Is(err, want) {
				t.Fatalf("frozen identity mutation accepted: %v", err)
			}
		})
	}
}

func TestRuntimeInvalidMappingKeepsTargetCandidate(t *testing.T) {
	tests := []struct {
		name string
		give string
		want error
	}{
		{"empty-trajectory", "UPDATE trajectory_meta SET trajectory_id=''", historysource.ErrFormat},
		{"multiple-mappings", "INSERT INTO trajectory_meta VALUES('other-trajectory','other-cascade',4,17)", historysource.ErrFormat},
		{"duplicate-cascade", "INSERT INTO trajectory_meta VALUES('other-trajectory','" + captureCascade + "',4,17)", historysource.ErrIdentity},
		{"future-schema", "DROP TABLE steps", historysource.ErrFormat},
		{"future-identity-schema", "ALTER TABLE trajectory_meta RENAME COLUMN trajectory_id TO future_id", historysource.ErrFormat},
		{"missing-cascade", "UPDATE trajectory_meta SET cascade_id=''", historysource.ErrIdentity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			db := runtimeFixture(t, root, "capture.db", "conversation-1.5.7.sql")
			if _, err := db.Exec(tt.give); err != nil {
				t.Fatal(err)
			}
			id := captureCascade
			svc := runtimeService(root, &id)
			if _, err := svc.RuntimeSource(context.Background(), "runtime"); !errors.Is(err, tt.want) {
				t.Fatalf("runtime mapping: %v", err)
			}
			if _, err := svc.FreezeRuntime(context.Background(), "runtime"); !errors.Is(err, tt.want) {
				t.Fatalf("frozen mapping: %v", err)
			}
		})
	}
}

func TestRuntimeLegacyToolChain(t *testing.T) {
	root := t.TempDir()
	runtimeFixture(t, root, "legacy.db", "conversation.sql")
	id := "00000000-0000-4000-8000-000000000002"
	svc := runtimeService(root, &id)
	scope, err := svc.FreezeRuntime(context.Background(), "runtime")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := svc.Document(context.Background(), scope.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Records) != 7 || len(doc.RecordRefs) != 7 {
		t.Fatal("legacy tool records lost")
	}
	call, result, system := doc.Records[2], doc.Records[3], doc.Records[5]
	if call.Kind != "tool_call" || call.ToolName != "run_command" || call.NativeCallID != "fixture-id-0008" {
		t.Fatalf("normalized call: %+v", call)
	}
	if call.Text != `{"CommandLine":"[redacted]","WaitMsBeforeAsync":0,"toolSummary":"[redacted]"}` || result.Text != "[redacted]" {
		t.Fatal("tool arguments or result lost")
	}
	if result.CallID != call.CallID || system.CallID != call.CallID || call.ParentID != doc.Records[1].ID || result.ParentID != doc.Records[1].ID {
		t.Fatal("normalized tool chain lost")
	}
	if doc.Records[0].Role != "user" || doc.Records[6].Role != "assistant" || doc.Records[6].Text != "[redacted]" {
		t.Fatal("legacy message bodies lost")
	}
}
