package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	codexhistory "github.com/rainchan/webmux/internal/agentprofile/codex/history"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
)

// The fixture is a real provider-owned transcript. SQLite stores only bindings,
// immutable manifests, and explicitly retained exports, never a live mirror.
func nativeFile(t *testing.T, runtime string, texts ...string) historysource.Source {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "rollout.jsonl")
	const nativeID = "00000000-0000-4000-8000-000000000001"
	var body strings.Builder
	appendLine := func(value any) {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		body.Write(data)
		body.WriteByte('\n')
	}
	appendLine(map[string]any{"timestamp": "2025-12-01T00:00:00Z", "type": "session_meta", "payload": map[string]any{"id": nativeID, "cwd": root, "cli_version": "0.77.0", "timestamp": "2025-12-01T00:00:00Z"}})
	for index, text := range texts {
		appendLine(map[string]any{"timestamp": "2025-12-01T00:00:01Z", "type": "response_item", "payload": map[string]any{"type": "message", "id": fmt.Sprintf("%s-item-%d", runtime, index), "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text}}}})
	}
	if err := os.WriteFile(path, []byte(body.String()), 0600); err != nil {
		t.Fatal(err)
	}
	return historysource.Source{Agent: "codex", NativeID: nativeID, Path: path}
}

func nativeFixture(t *testing.T, s *Service, runtime string, texts ...string) (*historyimport.Service, Binding, historysource.Source) {
	t.Helper()
	ctx := context.Background()
	source := nativeFile(t, runtime, texts...)
	root := filepath.Dir(source.Path)
	binding, err := s.EnsureRuntime(ctx, RuntimeInput{RuntimeID: runtime, Cwd: root, AgentName: "codex", AgentSessionID: source.NativeID, HistoryRoot: root, Status: "closed", HistoryOrigin: "complete"})
	if err != nil {
		t.Fatal(err)
	}
	sources := historyimport.New(historyimport.Options{Store: s, Readers: map[string]historysource.ReadFunc{"codex": codexhistory.Read}})
	if err := sources.Bind(ctx, binding.ProviderThreadID, source); err != nil {
		t.Fatal(err)
	}
	return sources, binding, source
}

// Control-only tests still reserve a real native range: an empty transcript is
// not a synthetic Activity database or an invented record.
func controlSnapshots(t *testing.T, s *Service, owner string) []historysource.Snapshot {
	t.Helper()
	sources := historyimport.New(historyimport.Options{Store: s, Readers: map[string]historysource.ReadFunc{"codex": codexhistory.Read}})
	snapshot, err := sources.Freeze(context.Background(), nativeFile(t, owner), owner, true)
	if err != nil {
		t.Fatal(err)
	}
	return []historysource.Snapshot{snapshot}
}

func TestNativeFrozenReferencesKeepVisibleBodiesAndIsolateCleanup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest(t, dir)
	text := strings.Repeat("中文🙂 native body\n", 8000)
	sources, binding, source := nativeFixture(t, s, "source", text)
	snapshots, err := sources.FreezeConversation(ctx, binding.ConversationID, binding.ConversationID, true)
	if err != nil || len(snapshots) != 1 {
		t.Fatal(snapshots, err)
	}
	snapshot := snapshots[0]
	child := bindTest(t, s, "child")
	if err := sources.Retain(ctx, snapshot.ID, child.ConversationID); err != nil {
		t.Fatal(err)
	}
	otherSources, other, _ := nativeFixture(t, s, "other", "unrelated visible body")
	otherSnapshots, err := otherSources.FreezeConversation(ctx, other.ConversationID, other.ConversationID, true)
	if err != nil || len(otherSnapshots) != 1 {
		t.Fatal(otherSnapshots, err)
	}
	if err := s.ForgetRuntime(ctx, "source"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(source.Path); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTest(t, dir)
	// Deliberately no reader: retained bodies must be independent of the file.
	sources = historyimport.New(historyimport.Options{Store: s})
	var got strings.Builder
	request := historysource.PageRequest{Limit: 1, MaxBytes: 4001}
	for {
		page, err := sources.Page(ctx, snapshot.ID, request)
		if err != nil {
			t.Fatal(err)
		}
		for _, record := range page.Records {
			if !utf8.ValidString(record.Text) {
				t.Fatal("split UTF-8 rune")
			}
			got.WriteString(record.Text)
		}
		if !page.More {
			break
		}
		request.After, request.TextOffset = page.Next, page.TextOffset
	}
	if got.String() != text {
		t.Fatal("retained history lost native text")
	}
	if err := s.ForgetRuntime(ctx, "child"); err != nil {
		t.Fatal(err)
	}
	if _, err := sources.Document(ctx, snapshot.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("last owner deletion retained private body", err)
	}
	doc, err := sources.Document(ctx, otherSnapshots[0].ID)
	if err != nil || len(doc.Records) != 1 || doc.Records[0].Text != "unrelated visible body" {
		t.Fatal("cleanup damaged unrelated visible history", doc, err)
	}
}

func TestNativeFrozenLiveRangeAndConcurrentIdentity(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	sources, binding, source := nativeFixture(t, s, "source", "native body")
	const count = 8
	results := make(chan historysource.Snapshot, count)
	failures := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Go(func() {
			snapshot, err := sources.Freeze(ctx, source, binding.ConversationID, false)
			results <- snapshot
			failures <- err
		})
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var id string
	for snapshot := range results {
		if id != "" && id != snapshot.ID {
			t.Fatal("concurrent freeze duplicated immutable identity")
		}
		id = snapshot.ID
	}
	doc, err := sources.Document(ctx, id)
	if err != nil || len(doc.Records) != 1 || doc.Records[0].Text != "native body" {
		t.Fatal("live range cannot read actual native body", doc, err)
	}
	data, err := s.Frozen(ctx, id)
	if err != nil || data.Export != nil || data.Snapshot.Retained {
		t.Fatal("live range persisted a mirror", err)
	}
	wrong := source
	wrong.NativeID = "00000000-0000-4000-8000-000000000002"
	if err := sources.Bind(ctx, binding.ProviderThreadID, wrong); err == nil {
		t.Fatal("unverified native identity rebound provider")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := sources.Freeze(cancelled, source, "cancelled", true); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled read published history", err)
	}
	if err := os.Remove(source.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := sources.Document(ctx, id); err == nil {
		t.Fatal("missing native file fell back to a body mirror")
	}
}

func TestNativeImportRetainsVisiblePrefixWithoutLiveRuntime(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	sources, _, source := nativeFixture(t, s, "source", "imported native history")
	binding, snapshot, err := sources.Import(ctx, source, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(source.Path); err != nil {
		t.Fatal(err)
	}
	prefix, err := s.PrefixSnapshots(ctx, binding.ConversationID)
	if err != nil || len(prefix) != 1 || prefix[0] != snapshot.ID {
		t.Fatal("import did not transfer pending lifetime", prefix, err)
	}
	bindings, err := s.NativeBindings(ctx, binding.ConversationID)
	if err != nil || len(bindings) != 0 {
		t.Fatal("history-only import fabricated a runtime source", bindings, err)
	}
	doc, err := sources.Documents(ctx, prefix)
	if err != nil || len(doc.Records) != 1 || doc.Records[0].Text != "imported native history" {
		t.Fatal("imported prefix not visibly readable", doc, err)
	}
}

func TestSchema14ContainsNoLegacyBodyTables(t *testing.T) {
	s := openTest(t, t.TempDir())
	for _, table := range []string{"activity", "activity_chunk", "activity_heading_chunk", "activity_attachment", "native_source", "native_location", "native_record", "history_snapshot", "history_snapshot_item", "history_snapshot_gap", "history_snapshot_reference"} {
		var count int
		if err := s.db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE type='table' AND name=?", table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("legacy body table %s remains: %d %v", table, count, err)
		}
	}
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 14 {
		t.Fatal("unexpected schema", version, err)
	}
	rows, err := s.db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() || rows.Err() != nil {
		t.Fatal("schema has broken foreign keys", rows.Err())
	}
}

func TestNativeOptOutBlocksNewReadsWithoutChangingRetainedScope(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	sources, binding, source := nativeFixture(t, s, "private", "already retained public range")
	snapshot, err := sources.Freeze(ctx, source, binding.ConversationID, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddGap(ctx, binding.ProviderThreadID, "capture_disabled"); err != nil {
		t.Fatal(err)
	}
	if _, err := sources.Freeze(ctx, source, "new-owner", true); !errors.Is(err, ErrUnavailable) {
		t.Fatal("opted-out native body became visible to a new owner", err)
	}
	bindings, err := s.NativeBindings(ctx, binding.ConversationID)
	if err != nil || len(bindings) != 0 {
		t.Fatal("opted-out provider remained discoverable", bindings, err)
	}
	doc, err := sources.Document(ctx, snapshot.ID)
	if err != nil || len(doc.Records) != 1 || doc.Records[0].Text != "already retained public range" {
		t.Fatal("opt-out changed previously frozen source scope", doc, err)
	}
	owners, err := s.FrozenForOwner(ctx, "new-owner")
	if err != nil || len(owners) != 0 {
		t.Fatal("blocked capture leaked a retained reference", owners, err)
	}
}
