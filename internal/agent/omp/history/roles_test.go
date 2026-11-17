package history

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hs "github.com/rainchan/webmux/internal/historysource"
)

func TestLocalExecutionRoleAndToolArguments(t *testing.T) {
	const data = `{"type":"session","version":3,"id":"native"}
{"type":"message","id":"local","message":{"role":"bashExecution","command":"PRIVATE_COMMAND","output":"PRIVATE_OUTPUT","content":"PRIVATE_BODY"}}
{"type":"message","id":"call","parentId":"local","message":{"role":"assistant","content":[{"type":"toolCall","id":"call-1","name":"bash","arguments":{"command":"printf synthetic-fact"}}]}}
{"type":"message","id":"result","parentId":"call","message":{"role":"toolResult","toolCallId":"call-1","toolName":"bash","content":[{"type":"text","text":"synthetic-fact"}]}}
`
	path := filepath.Join(t.TempDir(), "roles.jsonl")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), hs.Request{Source: source(path, "native")})
	if err != nil {
		t.Fatal(err)
	}
	var call, result hs.Record
	for _, r := range doc.Records {
		if strings.Contains(r.Text, "PRIVATE") {
			t.Fatal("local execution payload escaped")
		}
		if r.Kind == "tool_call" {
			call = r
		}
		if r.Kind == "tool_result" {
			result = r
		}
	}
	if call.Text != `{"command":"printf synthetic-fact"}` || result.Text != "synthetic-fact" || call.CallID != result.CallID || result.ParentID != "call" {
		t.Fatalf("tool content/association: %+v %+v", call, result)
	}
	if len(doc.Gaps) != 1 || doc.Gaps[0].Category != "local_execution_omitted" || doc.Gaps[0].RecordID != "local" {
		t.Fatalf("local execution gap: %+v", doc.Gaps)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(data, "bashExecution", "futureRole", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), hs.Request{Source: source(path, "native")}); !errors.Is(err, hs.ErrFormat) {
		t.Fatalf("unknown role: %v", err)
	}
}
