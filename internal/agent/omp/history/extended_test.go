package history

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	hs "github.com/rainchan/webmux/internal/historysource"
)

func TestExtendedNativeMessages(t *testing.T) {
	// Synthetic records retain the field shapes observed in native OMP sessions.
	tests := []struct {
		name, message, role, kind, text, status, tool, gap string
	}{
		{"developer", `{"role":"developer","content":[{"type":"text","text":"constraint"},{"type":"thinking","text":"PRIVATE"}],"attribution":"PRIVATE"}`, "developer", "message", "constraint", "", "", ""},
		{"custom", `{"role":"custom","content":"extension context","customType":"notice","display":false,"attribution":"PRIVATE"}`, "system", "message", "extension context", "", "", ""},
		{"bash", `{"role":"bashExecution","command":"echo result","output":"result","exitCode":0,"meta":{"secret":"PRIVATE"}}`, "tool", "tool_result", "echo result\nresult", "completed", "bash", ""},
		{"python", `{"role":"pythonExecution","code":"print(1)","output":"1","exitCode":1,"truncated":true}`, "tool", "tool_result", "print(1)\n1", "error", "python", "truncated_output"},
		{"cancelled", `{"role":"bashExecution","command":"sleep 10","output":"","cancelled":true}`, "tool", "tool_result", "sleep 10\n", "cancelled", "bash", ""},
		{"unknown_exit", `{"role":"bashExecution","command":"work","output":"partial"}`, "tool", "tool_result", "work\npartial", "unknown", "bash", ""},
		{"excluded", `{"role":"bashExecution","command":"PRIVATE","output":"PRIVATE","content":"PRIVATE","excludeFromContext":true}`, "", "metadata", "", "", "", "excluded_from_context"},
		{"file", `{"role":"fileMention","files":[{"path":"src/example.go","content":"PRIVATE","lineCount":1}]}`, "user", "message", "", "", "", "attachment_body_omitted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := "{\"type\":\"session\",\"id\":\"native\",\"version\":3}\n" +
				"{\"type\":\"message\",\"id\":\"root\",\"message\":{\"role\":\"user\",\"content\":\"task\"}}\n" +
				"{\"type\":\"message\",\"id\":\"extended\",\"parentId\":\"root\",\"message\":" + tt.message + "}\n"
			path := filepath.Join(t.TempDir(), "session.jsonl")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			doc, err := Read(context.Background(), hs.Request{Source: source(path, "native")})
			if err != nil {
				t.Fatal(err)
			}
			r := uniqueConsumerIndex(t, doc)["extended"]
			if r.ParentID != "root" || r.NativeID != "extended" || r.Role != tt.role || r.Kind != tt.kind || r.Text != tt.text || r.Status != tt.status || r.ToolName != tt.tool {
				t.Fatalf("extended projection = %+v", r)
			}
			if strings.Contains(r.Text, "PRIVATE") {
				t.Fatal("private payload exported")
			}
			if tt.gap != "" && (len(doc.Gaps) != 1 || doc.Gaps[0].Category != tt.gap || doc.Gaps[0].RecordID != "extended") {
				t.Fatalf("missing omission: %+v", doc.Gaps)
			}
			if tt.name == "file" && (len(r.Attachments) != 1 || r.Attachments[0].Name != "src/example.go") {
				t.Fatalf("lost file reference: %+v", r.Attachments)
			}
			if tt.name == "excluded" && r.Exportable() {
				t.Fatal("excluded execution became exportable")
			}
			frozen, err := Read(context.Background(), hs.Request{Source: doc.Source, Cursor: doc.Cursor})
			if err != nil || !reflect.DeepEqual(doc.Records, frozen.Records) {
				t.Fatalf("frozen extended records changed: %v", err)
			}
		})
	}
}

func TestUnsupportedRoleStillRejectsSelectedSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	data := "{\"type\":\"session\",\"id\":\"native\",\"version\":3}\n{\"type\":\"message\",\"id\":\"new\",\"message\":{\"role\":\"future_private_role\",\"content\":\"PRIVATE\"}}\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), hs.Request{Source: source(path, "native")}); !errors.Is(err, hs.ErrFormat) {
		t.Fatalf("unsupported role = %v", err)
	}
}
