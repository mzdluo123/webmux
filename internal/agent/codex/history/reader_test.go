package history

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/codexrollout"
	"github.com/rainchan/webmux/internal/historysource"
)

func TestRealNativeFixtures(t *testing.T) {
	tests := []struct {
		file, id, version        string
		messages, calls, results int
	}{
		{"session.jsonl", "00000001-0000-4000-8000-000000000003", "0.162.1", 3, 3, 3},
		{"compaction.jsonl", "00000002-0000-4000-8000-000000000003", "0.157.1", 8, 0, 0},
		{"current-0.169.0.jsonl", "00000003-0000-4000-8000-000000000003", "0.169.0", 2, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			source := historysource.Source{Agent: "codex", Path: filepath.Join("..", "..", "testdata", "native-history", "codex", tt.file), NativeID: tt.id}
			doc, err := Read(context.Background(), historysource.Request{Source: source})
			if err != nil {
				t.Fatal(err)
			}
			var messages, calls, results int
			callIDs := map[string]bool{}
			for _, item := range doc.Records {
				if strings.Contains(item.Text, "OPAQUE") || strings.Contains(item.Text, "encrypted_content") {
					t.Fatal("reasoning escaped source boundary")
				}
				switch item.Kind {
				case "message":
					messages++
				case "tool", "tool_arguments":
					calls++
					callIDs[item.CallID] = true
				case "tool_result":
					results++
					if !callIDs[item.CallID] {
						t.Fatalf("result lost call identity: %+v", item)
					}
				}
			}
			if messages != tt.messages || calls != tt.calls || results != tt.results || doc.FormatVersion != tt.version || doc.Source.Branch != tt.id {
				t.Fatalf("native format mismatch: messages=%d calls=%d results=%d version=%s branch=%s", messages, calls, results, doc.FormatVersion, doc.Source.Branch)
			}
		})
	}
}

func TestExistingRolloutParserAndFrozenAppend(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "codexrollout", "testdata", "basic.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	source := historysource.Source{Agent: "codex", Path: path, NativeID: "00000000-0000-4000-8000-000000000001"}
	before, err := Read(context.Background(), historysource.Request{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("{\"type\":\"future\",\"payload\":{}}\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := Read(context.Background(), historysource.Request{Source: before.Source, Cursor: before.Cursor})
	if err != nil || before.Fingerprint != after.Fingerprint || len(before.Records) != len(after.Records) {
		t.Fatalf("frozen prefix changed after append: %v", err)
	}
	source.NativeID = "other-session"
	if _, err := Read(context.Background(), historysource.Request{Source: source}); !errors.Is(err, historysource.ErrIdentity) {
		t.Fatalf("wrong native identity accepted: %v", err)
	}
}

func currentCapture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "native-history", "codex", "current-0.169.0.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCurrentCaptureBodyPrivacyAndFrozenIdentity(t *testing.T) {
	data := currentCapture(t)
	root := t.TempDir()
	dir := filepath.Join(root, "sessions")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-misleading-filename-id.jsonl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	sources, err := Discover(context.Background(), root)
	if err != nil || len(sources) != 1 || sources[0].NativeID != "00000003-0000-4000-8000-000000000003" {
		t.Fatalf("metadata discovery: %+v, %v", sources, err)
	}
	before, err := Read(context.Background(), historysource.Request{Source: sources[0]})
	if err != nil {
		t.Fatal(err)
	}
	var messages []string
	for _, record := range before.Records {
		if record.Kind == "message" {
			messages = append(messages, record.Role+":"+record.Text)
			if record.NativeID == "" || record.ID != before.Source.NativeID+":"+record.NativeID {
				t.Fatalf("lost message identity: %+v", record)
			}
		}
		if strings.Contains(record.Text, "REDACTED") {
			t.Fatal("environment, instruction or reasoning body escaped")
		}
	}
	want := []string{"user:Synthetic handoff code WMX-codex-capture; project Heron; port 48273.", "agent:ACK WMX-codex-capture"}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("body duplicated or lost: %q", messages)
	}
	categories := map[string]bool{}
	for _, gap := range before.Gaps {
		categories[gap.Category] = true
		if strings.HasPrefix(gap.Category, "unknown_") {
			t.Fatalf("current capture format unrecognized: %s", gap.Category)
		}
	}
	for _, category := range []string{"nonexportable_environment", "nonexportable_provider_state", "nonexportable_tool_definitions", "nonexportable_reasoning", "mirror_pairing_unverified"} {
		if !categories[category] {
			t.Fatalf("missing privacy/mirror category %s", category)
		}
	}
	tail := `{"timestamp":"2000-01-01T00:01:00Z","type":"response_item","payload":{"type":"message","id":"later-message","role":"assistant","content":[{"type":"output_text","text":"Later answer"}]}}` + "\n"
	if err := os.WriteFile(path, append(append([]byte{}, data...), tail...), 0600); err != nil {
		t.Fatal(err)
	}
	frozen, err := Read(context.Background(), historysource.Request{Source: before.Source, Cursor: before.Cursor})
	if err != nil || frozen.Fingerprint != before.Fingerprint || !reflect.DeepEqual(frozen.Records, before.Records) {
		t.Fatalf("frozen capture changed: %v", err)
	}
	latest, err := Read(context.Background(), historysource.Request{Source: before.Source})
	if err != nil || len(latest.Records) != len(before.Records)+1 {
		t.Fatalf("new capture lost append: %v", err)
	}
	for _, mismatch := range []historysource.Source{
		{Agent: "codex", Path: path, NativeID: "misleading-filename-id"},
		{Agent: "codex", Path: path, NativeID: before.Source.NativeID, Branch: "other-branch"},
	} {
		if _, err := Read(context.Background(), historysource.Request{Source: mismatch}); !errors.Is(err, historysource.ErrIdentity) {
			t.Fatalf("wrong identity accepted: %v", err)
		}
	}
}

func TestNewerWriterToolEnvelopeCompatibility(t *testing.T) {
	// Combine a synthetic writer version with current messages and native tool envelopes.
	data := []byte(strings.Replace(string(currentCapture(t)), "0.169.0", "9.99.0", 1))
	older, err := os.ReadFile(filepath.Join("..", "..", "testdata", "native-history", "codex", "session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var want []historysource.Record
	for _, line := range strings.Split(strings.TrimSpace(string(older)), "\n") {
		var envelope struct {
			Payload struct {
				Type, ID, Name, Input, Arguments string
				CallID                           string `json:"call_id"`
				Output                           json.RawMessage
			}
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(envelope.Payload.Type, "tool_call") && !strings.HasPrefix(envelope.Payload.Type, "function_call") {
			continue
		}
		p := envelope.Payload
		record := historysource.Record{NativeID: p.ID, CallID: p.CallID, ToolName: p.Name}
		switch p.Type {
		case "custom_tool_call":
			record.Kind, record.Text = "tool", p.Input
		case "function_call":
			record.Kind, record.Text = "tool_arguments", p.Arguments
		default:
			record.Kind = "tool_result"
			if json.Unmarshal(p.Output, &record.Text) != nil {
				var parts []struct{ Text string }
				if err := json.Unmarshal(p.Output, &parts); err != nil {
					t.Fatal(err)
				}
				for _, part := range parts {
					record.Text += part.Text
				}
			}
		}
		want = append(want, record)
		data = append(data, line...)
		data = append(data, '\n')
	}
	path := filepath.Join(t.TempDir(), "current-with-legacy-tools.jsonl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), historysource.Request{Source: historysource.Source{Agent: "codex", Path: path, NativeID: "00000003-0000-4000-8000-000000000003"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []historysource.Record
	calls := map[string]bool{}
	for _, record := range doc.Records {
		if record.CallID == "" {
			continue
		}
		if record.Kind == "tool_result" {
			if !calls[record.CallID] {
				t.Fatalf("tool result lost association: %+v", record)
			}
		} else {
			calls[record.CallID] = true
		}
		got = append(got, historysource.Record{NativeID: record.NativeID, CallID: record.CallID, ToolName: record.ToolName, Kind: record.Kind, Text: record.Text})
	}
	if len(got) != 6 || len(calls) != 3 || !reflect.DeepEqual(got, want) {
		t.Fatalf("tool arguments/results changed: got %+v want %+v", got, want)
	}
}

func TestDiscoveryRetainsUnsupportedTargetIdentity(t *testing.T) {
	header, _, _ := strings.Cut(string(currentCapture(t)), "\n")
	for _, version := range []string{"0.76.0", "invalid", ""} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "sessions")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "rollout-not-the-session-id.jsonl")
			data := strings.Replace(header, "0.169.0", version, 1) + "\nnot a supported body\n"
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			sources, err := Discover(context.Background(), root)
			if err != nil || len(sources) != 1 || sources[0].NativeID != "00000003-0000-4000-8000-000000000003" {
				t.Fatalf("unsupported candidate lost: %+v %v", sources, err)
			}
			doc, err := Read(context.Background(), historysource.Request{Source: sources[0]})
			var parseError *historysource.ParseError
			if !errors.Is(err, historysource.ErrFormat) || !errors.Is(err, codexrollout.ErrVersion) || !errors.As(err, &parseError) || parseError.Offset != 0 || len(doc.Records) != 0 {
				t.Fatalf("unsupported target lost format cause: %v", err)
			}
		})
	}
}
