package history

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	hs "github.com/rainchan/webmux/internal/historysource"
)

// Synthetic mixed-block graph models the consumer's unique-ID index, including
// multiple calls/results and a native ID deliberately colliding with a block ID.
func TestProjectedRecordIdentity(t *testing.T) {
	data := `{"type":"user","uuid":"root","sessionId":"native","message":{"role":"user","content":"question"}}
{"type":"assistant","uuid":"mixed","parentUuid":"root","sessionId":"native","message":{"role":"assistant","content":[{"type":"text","text":"answer"},{"type":"tool_use","id":"call-1","name":"Bash"},{"type":"tool_use","id":"call-2","name":"Read"}]}}
{"type":"user","uuid":"results","parentUuid":"mixed","sessionId":"native","message":{"role":"user","content":[{"type":"text","text":"comment"},{"type":"tool_result","tool_use_id":"call-1","content":"first"},{"type":"tool_result","tool_use_id":"call-2","content":"second","is_error":true}]}}
{"type":"user","uuid":"block:bWl4ZWQ:1","parentUuid":"results","sessionId":"native","message":{"role":"user","content":"collision"}}
`
	path := filepath.Join(t.TempDir(), "mixed.jsonl")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), hs.Request{Source: source(path, "native")})
	if err != nil {
		t.Fatal(err)
	}
	byID := uniqueConsumerIndex(t, doc)
	if len(byID) != 8 || byID["mixed"].Text != "answer" || byID["mixed"].ParentID != "root" {
		t.Fatalf("lost native node or projected blocks: %+v", doc.Records)
	}
	calls := map[string]hs.Record{}
	results := map[string]hs.Record{}
	for _, r := range doc.Records {
		if r.Kind == "tool_call" {
			calls[r.CallID] = r
			if r.ParentID != "root" {
				t.Fatalf("rewritten native parent: %+v", r)
			}
		}
		if r.Kind == "tool_result" {
			results[r.CallID] = r
		}
	}
	for id, name := range map[string]string{"call-1": "Bash", "call-2": "Read"} {
		if calls[id].ToolName != name || results[id].ToolName != name || calls[id].ID == results[id].ID || calls[id].ID == "mixed" {
			t.Fatalf("lost call/result identity: %v %v", calls, results)
		}
	}
	if results["call-2"].Status != "error" {
		t.Fatal("lost result error")
	}
	replay, err := Read(context.Background(), hs.Request{Source: doc.Source, Cursor: doc.Cursor})
	if err != nil || !reflect.DeepEqual(doc.Records, replay.Records) {
		t.Fatalf("unstable block IDs: %v", err)
	}
}

// Real #653 excerpts must also survive a consumer keyed by Record.ID.
func TestProjectedRecordIdentityNativeFixtures(t *testing.T) {
	for _, tc := range []struct{ name, id string }{{"session", "00000001-0000-4000-8000-000000000004"}, {"compaction", "00000002-0000-4000-8000-000000000006"}} {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Read(context.Background(), hs.Request{Source: source(fixture(tc.name), tc.id)})
			if err != nil {
				t.Fatal(err)
			}
			uniqueConsumerIndex(t, doc)
		})
	}
}

func uniqueConsumerIndex(t *testing.T, doc hs.Document) map[string]hs.Record {
	t.Helper()
	byID := make(map[string]hs.Record, len(doc.Records))
	for _, r := range doc.Records {
		if _, exists := byID[r.ID]; exists || r.ID == "" {
			t.Fatalf("consumer would overwrite record %q", r.ID)
		}
		byID[r.ID] = r
	}
	return byID
}
