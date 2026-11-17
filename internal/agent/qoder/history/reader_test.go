package history

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
)

// The input is deliberately named QoderCN: this is not an international capture.
func TestProvisionalChineseEditionFixture(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join("..", "..", "testdata", "native-history", "qodercn")
	sources, err := Discover(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	// Explicit expectations cover both captured writers without treating the
	// Chinese edition's captures as international runtime evidence.
	expected := map[string]struct {
		messages []string
		tools    bool
	}{
		"session-1.jsonl":            {messages: []string{"[REDACTED]", "[REDACTED]", "[REDACTED]"}},
		"session-2.jsonl":            {messages: []string{"[REDACTED]", "[REDACTED]"}},
		"session-1.1.81.jsonl":       {messages: []string{"合成项目蓝鹭，端口48273，校验码 WMX-qodercn-fixture。", "ACK WMX-qodercn-fixture"}},
		"session-tools-1.1.81.jsonl": {messages: []string{"仅使用 Read 读取 fact.txt，随后回复 ACK。", "ACK"}, tools: true},
	}
	for _, source := range sources {
		name := filepath.Base(source.Path)
		want, ok := expected[name]
		if !ok {
			t.Fatalf("unexpected or duplicate capture: %s", name)
		}
		delete(expected, name)
		t.Run(name, func(t *testing.T) {
			if source.Agent != "qoder" {
				t.Fatal(source)
			}
			doc, err := Read(ctx, historysource.Request{Source: source})
			if err != nil {
				t.Fatal(err)
			}
			if doc.Source.Agent != "qoder" || doc.ParserVersion != ParserVersion || doc.FormatVersion != "qoder-jsonl/unversioned" {
				t.Fatal(doc.Source, doc.ParserVersion, doc.FormatVersion)
			}
			svc := historyimport.New(historyimport.Options{Readers: map[string]historysource.ReadFunc{"qoder": Read}})
			scope, err := svc.FreezeScope(ctx, source)
			if err != nil {
				t.Fatal(err)
			}
			page, err := svc.Page(ctx, scope.ID, historysource.PageRequest{MaxBytes: 64 << 10})
			if err != nil || page.More {
				t.Fatalf("incomplete consumer page: %+v, %v", page, err)
			}
			var messages []string
			var call, result historysource.Record
			calls, results := 0, 0
			for _, record := range page.Records {
				switch record.Kind {
				case "message":
					messages = append(messages, record.Text)
				case "tool_arguments":
					call = record
					calls++
				case "tool_result":
					result = record
					results++
				default:
					t.Fatalf("unexpected exported record: %+v", record)
				}
			}
			if !reflect.DeepEqual(messages, want.messages) {
				t.Fatalf("conversation lost: got %q; want %q", messages, want.messages)
			}
			if !want.tools {
				if calls != 0 || results != 0 {
					t.Fatalf("invented tool records: %+v", page.Records)
				}
				return
			}
			if calls != 1 || results != 1 || len(page.Records) != 4 || page.Records[1].Kind != "tool_arguments" || page.Records[2].Kind != "tool_result" {
				t.Fatalf("tool chain missing or reordered: %+v", page.Records)
			}
			if call.Text != `{"file_path":"/workspace/native-fixture/fact.txt"}` || call.ToolName != "Read" || call.Role != "assistant" {
				t.Fatalf("tool input lost: %+v", call)
			}
			if result.Text != "1\tSYNTHETIC_TOOL_FACT=WMX-QODER-TOOL-FIXTURE\n2\tproject=白鹭\n3\tport=49369\n4\t" || result.ToolName != "Read" || result.Role != "tool" || result.Status != "completed" {
				t.Fatalf("tool result lost: %+v", result)
			}
			if call.CallID == "" || call.CallID != result.CallID || call.NativeCallID != "00000000-0000-0000-0000-00000000000b" || call.NativeCallID != result.NativeCallID {
				t.Fatalf("tool association lost: %+v, %+v", call, result)
			}
		})
	}
	if len(expected) != 0 {
		t.Fatalf("captures missing from discovery: %+v", expected)
	}
}

func TestProvisionalReaderRejectsUnobservedWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unsupported.jsonl")
	data := `{"type":"user","uuid":"user","sessionId":"fixture","version":"1.1.82","message":{"role":"user","content":"unsupported"}}` + "\n" +
		`{"type":"active-leaf","sessionId":"fixture","leafUuid":"user"}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	source := historysource.Source{Agent: "qoder", Path: path, NativeID: "fixture"}
	if _, err := Read(context.Background(), historysource.Request{Source: source}); !errors.Is(err, historysource.ErrFormat) {
		t.Fatalf("unobserved writer accepted: %v", err)
	}
}
