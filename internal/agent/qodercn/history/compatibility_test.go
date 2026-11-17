package history

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
)

const liveFixtureID = "00000000-0000-0000-0000-000000000001"

func appendRecords(t *testing.T, source historysource.Source, text string) {
	t.Helper()
	file, err := os.OpenFile(source.Path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(text); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

// Use the actual consumer, not just Record.Text: tool_use is not exportable.
func TestCapturedWriterConsumerPages(t *testing.T) {
	for _, capture := range []struct{ file, id string }{
		{"session-1.1.81.jsonl", liveFixtureID},
		{"session-2.jsonl", "54b3d76a-0976-5482-94ae-367317ee052e"},
	} {
		t.Run(capture.file, func(t *testing.T) {
			ctx := context.Background()
			source := fixture(t, capture.file, capture.id)
			original, err := Read(ctx, historysource.Request{Source: source})
			if err != nil {
				t.Fatal(err)
			}
			if capture.file == "session-1.1.81.jsonl" {
				var messages []string
				for _, record := range original.Records {
					if record.Kind == "message" {
						messages = append(messages, record.Text)
					}
				}
				want := []string{"合成项目蓝鹭，端口48273，校验码 WMX-qodercn-fixture。", "ACK WMX-qodercn-fixture"}
				if !reflect.DeepEqual(messages, want) || len(original.Gaps) != 0 {
					t.Fatalf("capture text or ancestry lost: %q, %+v", messages, original.Gaps)
				}
			}
			// These explicitly synthetic tool/attachment mutations extend both
			// real captures without claiming native tool capture evidence.
			appendRecords(t, source, `{"type":"assistant","uuid":"call","parentUuid":"`+original.Source.Branch+`","message":{"role":"assistant","content":[{"type":"thinking","thinking":"PRIVATE_REASONING","signature":"PRIVATE_SIGNATURE"},{"type":"text","text":"读取蓝鹭🙂"},{"type":"tool_use","id":"read-1","name":"Read","input":{"file_path":"/workspace/蓝鹭.txt","offset":1}},{"type":"tool_use","id":"read-2","name":"Read","input":{"file_path":"/workspace/missing.txt"}}]}}`+"\n"+
				`{"type":"user","uuid":"result","parentUuid":"call","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"read-1","content":[{"type":"text","text":"蓝鹭🙂"},{"type":"image","source":{"data":"PRIVATE_BINARY"}},{"type":"text","text":"结果"}]},{"type":"tool_result","tool_use_id":"read-2","content":"没有文件","is_error":true}]}}`+"\n"+
				`{"type":"user","uuid":"meta","parentUuid":"result","isMeta":true,"message":{"role":"user","content":"PRIVATE_META"}}`+"\n"+
				`{"type":"attachment","uuid":"environment","parentUuid":"meta","attachment":{"type":"environment","content":"PRIVATE_CONFIG"}}`+"\n"+
				`{"type":"assistant","uuid":"answer","parentUuid":"environment","message":{"role":"assistant","content":[{"type":"redacted_thinking","data":"PRIVATE_ENCRYPTED"},{"type":"text","text":"完成🙂"}]}}`+"\n"+
				`{"type":"active-leaf","leafUuid":"answer"}`+"\n")
			svc := historyimport.New(historyimport.Options{Readers: map[string]historysource.ReadFunc{"qodercn": Read}})
			scope, err := svc.FreezeScope(ctx, source)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := svc.ScopeDocument(ctx, scope.ID)
			if err != nil {
				t.Fatal(err)
			}
			var want []historysource.Record
			for _, record := range doc.Records {
				if record.Exportable() {
					want = append(want, record)
				}
			}
			// Append a new branch and an unfinished line after freezing. Neither
			// can enter the selected range, including continued UTF-8 pages.
			appendRecords(t, source, `{"type":"assistant","uuid":"later","parentUuid":"answer","message":{"role":"assistant","content":"LATER_BRANCH"}}`+"\n"+`{"type":"active-leaf","leafUuid":"later"}`+"\n"+`{"partial":`)
			var got []historysource.Record
			request := historysource.PageRequest{Limit: 2, MaxBytes: 7}
			for pages := 0; ; pages++ {
				if pages > 500 {
					t.Fatal("pagination did not terminate")
				}
				page, err := svc.Page(ctx, scope.ID, request)
				if err != nil {
					t.Fatal(err)
				}
				for _, record := range page.Records {
					if !utf8.ValidString(record.Text) || strings.Contains(record.Text, "PRIVATE") {
						t.Fatalf("unsafe or invalid page: %+v", record)
					}
					if len(got) > 0 && got[len(got)-1].ID == record.ID {
						got[len(got)-1].Text += record.Text
					} else {
						record.ByteLength = 0
						got = append(got, record)
					}
				}
				if !page.More {
					break
				}
				if page.Next == request.After && page.TextOffset == request.TextOffset {
					t.Fatal("pagination made no progress")
				}
				request.After, request.TextOffset = page.Next, page.TextOffset
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("consumer lost records: got %+v; want %+v", got, want)
			}
			calls := make(map[string]historysource.Record)
			results := 0
			for _, record := range got {
				if strings.Contains(record.Text, "PRIVATE") || strings.Contains(record.Text, "LATER_BRANCH") {
					t.Fatalf("private or unfrozen content exported: %+v", record)
				}
				switch record.Kind {
				case "tool_arguments":
					calls[record.CallID] = record
				case "tool_result":
					call, ok := calls[record.CallID]
					if !ok || call.ToolName != "Read" || record.ToolName != call.ToolName || call.NativeCallID != record.NativeCallID || record.Role != "tool" {
						t.Fatalf("broken tool association: %+v, %+v", call, record)
					}
					if record.NativeCallID == "read-1" && (call.Text != `{"file_path":"/workspace/蓝鹭.txt","offset":1}` || record.Text != "蓝鹭🙂结果" || record.Status != "completed") {
						t.Fatalf("successful tool body lost: %+v, %+v", call, record)
					}
					if record.NativeCallID == "read-2" && (call.Text != `{"file_path":"/workspace/missing.txt"}` || record.Text != "没有文件" || record.Status != "error") {
						t.Fatalf("failed tool body lost: %+v, %+v", call, record)
					}
					results++
				}
			}
			if len(calls) != 2 || results != 2 {
				t.Fatalf("tools missing from consumer: calls=%d results=%d", len(calls), results)
			}
		})
	}
}

func TestUnsupportedWriterAndContent(t *testing.T) {
	for _, version := range []string{"1.1.80", "1.1.82", "1.1.85", "2.0.0", ""} {
		t.Run("version-"+version, func(t *testing.T) {
			source := fixture(t, "session-1.1.81.jsonl", liveFixtureID)
			data, err := os.ReadFile(source.Path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source.Path, []byte(strings.ReplaceAll(string(data), `"version":"1.1.81"`, `"version":"`+version+`"`)), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Read(context.Background(), historysource.Request{Source: source}); !errors.Is(err, historysource.ErrFormat) {
				t.Fatalf("unsupported writer accepted: %v", err)
			}
		})
	}
	for name, content := range map[string]string{
		"unknown-block":        `[{"type":"future_payload","text":"do not silently drop"}]`,
		"null-message":         `null`,
		"object-message":       `{"text":"wrong shape"}`,
		"missing-call-id":      `[{"type":"tool_use","name":"Read","input":{}}]`,
		"missing-call-name":    `[{"type":"tool_use","id":"tool","input":{}}]`,
		"missing-call-input":   `[{"type":"tool_use","id":"tool","name":"Read"}]`,
		"null-call-input":      `[{"type":"tool_use","id":"tool","name":"Read","input":null}]`,
		"array-call-input":     `[{"type":"tool_use","id":"tool","name":"Read","input":[]}]`,
		"duplicate-call-id":    `[{"type":"tool_use","id":"tool","name":"Read","input":{}},{"type":"tool_use","id":"tool","name":"Read","input":{}}]`,
		"missing-result-id":    `[{"type":"tool_result","content":"done"}]`,
		"null-result":          `[{"type":"tool_result","tool_use_id":"tool","content":null}]`,
		"unknown-result-block": `[{"type":"tool_result","tool_use_id":"tool","content":[{"type":"future_payload","text":"unknown"}]}]`,
	} {
		t.Run(name, func(t *testing.T) {
			source := fixture(t, "session-1.1.81.jsonl", liveFixtureID)
			appendRecords(t, source, `{"type":"assistant","uuid":"invalid","message":{"role":"assistant","content":`+content+`}}`+"\n"+`{"type":"active-leaf","leafUuid":"invalid"}`+"\n")
			if _, err := Read(context.Background(), historysource.Request{Source: source}); !errors.Is(err, historysource.ErrFormat) {
				t.Fatalf("unsupported content accepted: %v", err)
			}
		})
	}
}

func TestFrozenParserAndBranchBinding(t *testing.T) {
	source := fixture(t, "session-1.1.81.jsonl", liveFixtureID)
	doc, err := Read(context.Background(), historysource.Request{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	source.Branch = "different-branch"
	if _, err := Read(context.Background(), historysource.Request{Source: source, Cursor: doc.Cursor}); !errors.Is(err, historysource.ErrChanged) {
		t.Fatalf("changed branch accepted: %v", err)
	}
	source.Branch = ""
	data, err := base64.RawURLEncoding.DecodeString(doc.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	var cursor boundary
	if err := json.Unmarshal(data, &cursor); err != nil {
		t.Fatal(err)
	}
	for _, parser := range []string{"", "qodercn-jsonl/1", "qodercn-jsonl/future"} {
		cursor.Parser = parser
		data, err := json.Marshal(cursor)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Read(context.Background(), historysource.Request{Source: source, Cursor: base64.RawURLEncoding.EncodeToString(data)}); !errors.Is(err, historysource.ErrChanged) {
			t.Fatalf("changed parser accepted: %v", err)
		}
	}
}

func TestCapturedReadToolConsumer(t *testing.T) {
	ctx := context.Background()
	source := fixture(t, "session-tools-1.1.81.jsonl", liveFixtureID)
	svc := historyimport.New(historyimport.Options{Readers: map[string]historysource.ReadFunc{"qodercn": Read}})
	scope, err := svc.FreezeScope(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	page, err := svc.Page(ctx, scope.ID, historysource.PageRequest{MaxBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.More || len(page.Records) != 4 || len(page.Snapshot.Gaps) != 0 {
		t.Fatalf("incomplete captured tool chain: %+v", page)
	}
	user, call, result, answer := page.Records[0], page.Records[1], page.Records[2], page.Records[3]
	if user.Kind != "message" || user.Role != "user" || user.Text != "仅使用 Read 读取 fact.txt，随后回复 ACK。" || answer.Kind != "message" || answer.Role != "assistant" || answer.Text != "ACK" {
		t.Fatalf("captured conversation lost: %+v", page.Records)
	}
	if call.Kind != "tool_arguments" || call.Role != "assistant" || call.ToolName != "Read" || call.Text != `{"file_path":"/workspace/native-fixture/fact.txt"}` {
		t.Fatalf("captured arguments not exported: %+v", call)
	}
	wantResult := "1\tSYNTHETIC_TOOL_FACT=WMX-QODER-TOOL-FIXTURE\n2\tproject=白鹭\n3\tport=49369\n4\t"
	if result.Kind != "tool_result" || result.Role != "tool" || result.Text != wantResult || result.Status != "completed" || result.ToolName != "Read" {
		t.Fatalf("captured result not exported: %+v", result)
	}
	if call.CallID == "" || call.CallID != result.CallID || call.NativeCallID != "00000000-0000-0000-0000-00000000000b" || result.NativeCallID != call.NativeCallID {
		t.Fatalf("captured call association lost: %+v, %+v", call, result)
	}
}

func TestMissingToolCallRemainsVisibleGap(t *testing.T) {
	source := fixture(t, "session-1.1.81.jsonl", liveFixtureID)
	appendRecords(t, source, `{"type":"user","uuid":"orphan","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"outside","content":"available result"}]}}`+"\n"+`{"type":"active-leaf","leafUuid":"orphan"}`+"\n")
	doc, err := Read(context.Background(), historysource.Request{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Records) != 1 || doc.Records[0].Text != "available result" || doc.Records[0].CallID != "outside" || len(doc.Gaps) != 1 || doc.Gaps[0].Category != "missing-tool-call" {
		t.Fatalf("missing call was invented or result dropped: %+v", doc)
	}
}
