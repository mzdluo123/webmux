package history

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

func TestContentTextPreservesSupportedBlocksAndReportsGaps(t *testing.T) {
	tests := []struct {
		name string
		give string
		want string
		gap  bool
	}{
		{name: "empty", give: `null`},
		{name: "empty blocks", give: `[]`},
		{name: "text", give: `"正文\n  code  "`, want: "正文\n  code  "},
		{name: "text fragments", give: `[{"type":"text","text":"甲"},{"type":"text","text":"乙"}]`, want: "甲乙"},
		{name: "ACP content", give: `[{"type":"content","content":{"type":"text","text":"工具结果"}}]`, want: "工具结果"},
		{name: "resource text", give: `{"type":"resource","resource":{"uri":"fixture://script","mimeType":"text/x-shellscript","text":"echo fixture\n"}}`, want: "echo fixture\n"},
		{name: "diff", give: `{"type":"diff","path":"fixture.go","oldText":"old","newText":"new"}`, want: `{"type":"diff","path":"fixture.go","oldText":"old","newText":"new"}`},
		{name: "link", give: `{"type":"resource_link","uri":"fixture://document","name":"文档"}`, want: `{"type":"resource_link","uri":"fixture://document","name":"文档"}`},
		{name: "unknown block", give: `[{"type":"future","payload":"uninterpreted"},{"type":"text","text":"保留正文"}]`, want: "保留正文", gap: true},
		{name: "unknown scalar", give: `42`, gap: true},
		{name: "reasoning", give: `{"type":"thinking","thinking":"PRIVATE-THOUGHT"}`, gap: true},
		{name: "binary resource", give: `{"type":"resource","resource":{"blob":"PRIVATE-BINARY"}}`, gap: true},
		{name: "terminal reference", give: `{"type":"terminal","terminalId":"fixture-terminal"}`, gap: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, gap, err := contentText(json.RawMessage(tt.give))
			if err != nil || text != tt.want || gap != tt.gap {
				t.Fatalf("text=%q gap=%t err=%v", text, gap, err)
			}
		})
	}
}

func TestNativeToolArgumentsStateAndEmptyRecords(t *testing.T) {
	path, db := fixture(t)
	const callID = "exec:0#00000000000000000000000000000001"
	const args = `{"command":"echo fixture","limit":9007199254740993}`
	updates := []struct {
		id   int
		body string
	}{
		{6, `{"role":"user","content":[{"type":"text","text":"正文\n"},{"type":"text","text":"  缩进"}]}`},
		{8, `{"role":"assistant","content":"","tool_calls":[{"id":"` + callID + `","name":"exec","arguments":` + args + `}],"thinking":{"thinking":"PRIVATE-THOUGHT"}}`},
		{9, `{"role":"tool","tool_call_id":"` + callID + `","content":"native result"}`},
		{10, `{"role":"assistant","content":[]}`},
	}
	for _, update := range updates {
		if _, err := db.Exec(`UPDATE message_nodes SET chat_message=? WHERE node_id=?`, update.body, update.id); err != nil {
			t.Fatal(err)
		}
	}
	initial := `{"toolCallId":"` + callID + `","status":"pending","rawInput":` + args + `,"content":[{"type":"content","content":{"type":"resource","resource":{"text":"script body"}}}],"rawOutput":"initial observation"}`
	update := `{"toolCallId":"` + callID + `","status":"failed","content":[{"type":"content","content":{"type":"text","text":"state result"}},{"type":"terminal","terminalId":"fixture-terminal"}],"rawOutput":{"exitCode":7}}`
	if _, err := db.Exec(`UPDATE tool_call_state SET tool_call_json=?,tool_call_update_json=?`, initial, update); err != nil {
		t.Fatal(err)
	}
	req := historysource.Request{Source: historysource.Source{Agent: "devin", Path: path, NativeID: "fixture-session-001"}}
	doc, err := Read(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Records) != 7 {
		t.Fatalf("records: %+v", doc.Records)
	}
	wantText := map[string]bool{
		"正文\n  缩进": false, args: false, "script body": false,
		"initial observation": false, "state result": false, `{"exitCode":7}`: false, "native result": false,
	}
	for _, record := range doc.Records {
		if _, ok := wantText[record.Text]; !ok {
			t.Fatalf("unexpected body: %+v", record)
		}
		wantText[record.Text] = true
		if record.Role != "user" && (record.CallID != callID || record.NativeCallID != callID || record.ToolName != "exec" || record.Status != "failed") {
			t.Fatalf("tool association lost: %+v", record)
		}
		if record.ID == "8" || record.ID == "10" || strings.Contains(record.Text, "PRIVATE-THOUGHT") {
			t.Fatalf("empty or private record exported: %+v", record)
		}
		if record.Text == args && record.Kind != "tool-call" {
			t.Fatalf("arguments kind: %+v", record)
		}
		if record.Text == "script body" && record.Kind != "tool" {
			t.Fatalf("initial content kind: %+v", record)
		}
	}
	for text, found := range wantText {
		if !found {
			t.Fatalf("missing body %q", text)
		}
	}
	foundGap := false
	for _, gap := range doc.Gaps {
		foundGap = foundGap || gap.Category == "unsupported-tool-content"
	}
	if !foundGap {
		t.Fatal("terminal content gap missing")
	}
	req.Cursor = doc.Cursor
	if _, err := db.Exec(`UPDATE tool_call_state SET tool_call_update_json='{"status":"completed","rawOutput":"changed"}'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), req); !errors.Is(err, historysource.ErrChanged) {
		t.Fatalf("frozen state mutation: %v", err)
	}
}

func TestStateInputFallbackAndUnknownCallGap(t *testing.T) {
	path, db := fixture(t)
	if _, err := db.Exec(`UPDATE message_nodes SET chat_message='{"role":"assistant","content":null,"tool_calls":[{"id":"exec:0#00000000000000000000000000000001","name":"exec"},{"future":{"payload":"unknown"}}]}' WHERE node_id=8`); err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), historysource.Request{Source: historysource.Source{Agent: "devin", Path: path, NativeID: "fixture-session-001"}})
	if err != nil {
		t.Fatal(err)
	}
	foundInput, foundGap := false, false
	for _, record := range doc.Records {
		if record.ID == "8/call/0" {
			foundInput = record.Text == `{"command":"[redacted]"}`
		}
		if record.ID == "8" || record.ID == "8/call/1" {
			t.Fatalf("empty record exported: %+v", record)
		}
	}
	for _, gap := range doc.Gaps {
		foundGap = foundGap || gap.Category == "unsupported-tool-call"
	}
	if !foundInput || !foundGap {
		t.Fatalf("input=%t unknown gap=%t", foundInput, foundGap)
	}
}

func TestToolStateUnknownFieldsAreReported(t *testing.T) {
	state, gap, err := parseToolState(`{"status":"completed","rawOutput":{"value":42},"future":{"payload":"unrecognized"},"_meta":{"private":"PRIVATE-METADATA"}}`)
	if err != nil || !gap || state.Status != "completed" || string(state.RawOutput) != `{"value":42}` {
		t.Fatalf("state=%+v gap=%t err=%v", state, gap, err)
	}
	if _, _, err := parseToolState(`{"status":`); !errors.Is(err, historysource.ErrFormat) {
		t.Fatalf("invalid state: %v", err)
	}
}
