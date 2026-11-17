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
	data := `{"type":"session","id":"native","version":3}
{"type":"message","id":"root","message":{"role":"user","content":"question"}}
{"type":"message","id":"mixed","parentId":"root","message":{"role":"assistant","content":[{"type":"text","text":"answer"},{"type":"toolCall","id":"call-1","name":"Bash"},{"type":"toolCall","id":"call-2","name":"Read"}]}}
{"type":"message","id":"result-1","parentId":"mixed","message":{"role":"toolResult","toolCallId":"call-1","toolName":"Bash","content":[{"type":"text","text":"first"}]}}
{"type":"message","id":"result-2","parentId":"result-1","message":{"role":"toolResult","toolCallId":"call-2","toolName":"Read","isError":true,"content":[{"type":"text","text":"second"}]}}
{"type":"message","id":"block:bWl4ZWQ:1","parentId":"result-2","message":{"role":"user","content":"collision"}}
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
	if len(byID) != 7 || byID["mixed"].Text != "answer" || byID["mixed"].ParentID != "root" {
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
	for _, tc := range []struct{ name, id string }{{"session", "00000001-0000-4000-8000-000000000001"}, {"compaction", "00000002-0000-4000-8000-000000000001"}} {
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
