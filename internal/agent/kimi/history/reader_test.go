package history

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
)

func fixture(t *testing.T) historysource.Source {
	t.Helper()
	return fixtureSession(t, "wd_fixture_0001", "session_00000000-0000-4000-8000-000000000001")
}

func fixture15(t *testing.T) historysource.Source {
	t.Helper()
	return fixtureSession(t, "wd_fixture_0002", "session_00000000-0000-4000-8000-000000000002")
}

func fixtureSession(t *testing.T, workspace, id string) historysource.Source {
	t.Helper()
	return fixtureSessionAtRoot(t, t.TempDir(), workspace, id)
}

func fixtureSessionAtRoot(t *testing.T, root, workspace, id string) historysource.Source {
	t.Helper()
	source := filepath.Join("..", "..", "testdata", "native-history", "kimi", workspace, id)
	home := filepath.Join(root, workspace, id, "agents", "main")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	wire, err := os.ReadFile(filepath.Join(source, "agents", "main", "wire.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(filepath.Join(source, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(state, &m); err != nil {
		t.Fatal(err)
	}
	m["agents"].(map[string]any)["main"].(map[string]any)["homedir"] = home
	state, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, workspace, id, "state.json"), state, 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "wire.jsonl")
	if err := os.WriteFile(path, wire, 0600); err != nil {
		t.Fatal(err)
	}
	return historysource.Source{Agent: "kimi", Path: path, NativeID: id}
}

func appendWire(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeWire(t *testing.T) {
	s := fixture(t)
	doc, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil {
		t.Fatal(err)
	}
	if doc.FormatVersion != "kimi-wire/1.4" || doc.ParserVersion != ParserVersion {
		t.Fatal(doc.FormatVersion, doc.ParserVersion)
	}
	users, calls, results, errorsFound := 0, 0, 0, 0
	names := map[string]string{}
	for _, r := range doc.Records {
		if r.Role == "user" {
			users++
		}
		if r.Kind == "tool_arguments" {
			calls++
			names[r.CallID] = r.ToolName
			if !json.Valid([]byte(r.Text)) || r.ParentID == "" {
				t.Fatal("lost tool arguments or step linkage", r)
			}
		}
		if r.Kind == "tool_result" {
			results++
			if r.ParentID != r.CallID || r.Text == "" {
				t.Fatal("lost result body or parent linkage", r)
			}
			if names[r.CallID] != r.ToolName || r.ToolName == "" {
				t.Fatal("broken tool closure", r)
			}
			if r.Status == "error" {
				errorsFound++
			}
		}
	}
	if users != 1 || calls != 7 || results != 7 || errorsFound != 1 {
		t.Fatal(users, calls, results, errorsFound)
	}
	if last := doc.Records[len(doc.Records)-1]; last.Kind != "status" || last.Status != "end_turn" {
		t.Fatal(last)
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(s.Path)))))
	sources, err := Discover(context.Background(), root)
	if err != nil || len(sources) != 1 || sources[0].NativeID != s.NativeID {
		t.Fatal(sources, err)
	}
	s.NativeID = "wrong"
	if _, err := Read(context.Background(), historysource.Request{Source: s}); !errors.Is(err, historysource.ErrIdentity) {
		t.Fatal(err)
	}
}

func TestFrozenUnicodeReasoningAndDedupe(t *testing.T) {
	s := fixture(t)
	event := `{"type":"context.append_loop_event","event":{"type":"content.part","uuid":"unicode","part":{"type":"text","text":"你好 🌍"}}}` + "\n"
	appendWire(t, s.Path, event+event+`{"type":"context.append_loop_event","event":{"type":"content.part","uuid":"thought","part":{"type":"think","think":"SECRET_REASONING"}}}`+"\n")
	doc, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, r := range doc.Records {
		if r.Text == "你好 🌍" {
			count++
		}
		if strings.Contains(r.Text, "SECRET") {
			t.Fatal("reasoning leak")
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
	appendWire(t, s.Path, `{"type":"context.append_message","message":{"role":"user","content":[{"type":"text","text":"new"}]}}`+"\n"+`{"partial":`)
	frozen, err := Read(context.Background(), historysource.Request{Source: s, Cursor: doc.Cursor})
	if err != nil || len(frozen.Records) != len(doc.Records) || frozen.Fingerprint != doc.Fingerprint {
		t.Fatal(err)
	}
	latest, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil || !latest.Pending {
		t.Fatal(err, latest.Pending)
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "[redacted]", "[mutated!]", 1))
	if err := os.WriteFile(s.Path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), historysource.Request{Source: s, Cursor: doc.Cursor}); !errors.Is(err, historysource.ErrChanged) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(ctx, historysource.Request{Source: s}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := Discover(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestStateBindingAndUnknownMutation(t *testing.T) {
	s := fixture(t)
	appendWire(t, s.Path, `{"type":"context.compact","body":"SECRET"}`+"\n")
	doc, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil || len(doc.Gaps) != 1 {
		t.Fatal(doc.Gaps, err)
	}
	statePath := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(s.Path))), "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "[redacted]", "changed", 1))
	if err := os.WriteFile(statePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), historysource.Request{Source: s, Cursor: doc.Cursor}); !errors.Is(err, historysource.ErrChanged) {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), filepath.Dir(s.Path), "/wrong/session/agents/main", 1))
	if err := os.WriteFile(statePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), historysource.Request{Source: s}); !errors.Is(err, historysource.ErrIdentity) {
		t.Fatal(err)
	}
}

func TestMessageAndLoopMirrorsUseNativeIDs(t *testing.T) {
	s := fixture(t)
	appendWire(t, s.Path,
		`{"type":"context.append_message","message":{"id":"00000000-0000-4000-8000-000000000026","role":"assistant","content":[{"type":"text","text":"DUPLICATE_MIRROR"}]}}`+"\n"+
			`{"type":"context.append_message","message":{"id":"early-message","role":"assistant","content":[{"type":"text","text":"EARLY_MIRROR"}]}}`+"\n"+
			`{"type":"context.append_loop_event","event":{"type":"content.part","uuid":"part","stepUuid":"step","part":{"type":"text","text":"canonical"}}}`+"\n"+
			`{"type":"context.append_loop_event","event":{"type":"step.end","uuid":"step","messageId":"early-message","finishReason":"end_turn"}}`+"\n")
	doc, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil {
		t.Fatal(err)
	}
	canonical := 0
	for _, r := range doc.Records {
		if strings.Contains(r.Text, "MIRROR") {
			t.Fatal("duplicated context representations", r)
		}
		if r.Text == "canonical" {
			canonical++
		}
	}
	if canonical != 1 {
		t.Fatal(canonical)
	}
}

func TestProtocol15CanonicalTranscript(t *testing.T) {
	s := fixture15(t)
	doc, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil {
		t.Fatal(err)
	}
	if doc.FormatVersion != "kimi-wire/1.5" || doc.ParserVersion != ParserVersion || len(doc.Gaps) != 0 {
		t.Fatal(doc, err)
	}
	if len(doc.Records) != 3 {
		t.Fatalf("mirrors, injections or bookkeeping became history: %#v", doc.Records)
	}
	if doc.Records[0].Role != "user" || !strings.Contains(doc.Records[0].Text, "项目蓝鹭，端口48273") {
		t.Fatal("missing user body", doc.Records[0])
	}
	if doc.Records[1].Role != "assistant" || doc.Records[1].Text != "ACK FIXTURE-KIMI-15" || doc.Records[1].ParentID == "" {
		t.Fatal("missing assistant body or step linkage", doc.Records[1])
	}
	if doc.Records[2].Kind != "status" || doc.Records[2].Status != "end_turn" {
		t.Fatal(doc.Records[2])
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(s.Path)))))
	sources, err := Discover(context.Background(), root)
	if err != nil || len(sources) != 1 || sources[0].NativeID != s.NativeID {
		t.Fatal(sources, err)
	}
	appendWire(t, s.Path, `{"type":"context.append_message","agentId":"child","message":{"role":"user","content":[{"type":"text","text":"HIDDEN_CHILD"}]}}`+"\n")
	latest, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil || !reflect.DeepEqual(doc.Records, latest.Records) {
		t.Fatal("child body leaked", latest.Records, err)
	}
	appendWire(t, s.Path, `{"type":"context.append_message","agentId":"main","message":{"id":"new-user","role":"user","origin":{"kind":"user"},"content":[{"type":"text","text":"later"}]}}`+"\n"+`{"partial":`)
	frozen, err := Read(context.Background(), historysource.Request{Source: s, Cursor: doc.Cursor})
	if err != nil || !reflect.DeepEqual(doc.Records, frozen.Records) || frozen.Fingerprint != doc.Fingerprint {
		t.Fatal("frozen boundary moved", frozen, err)
	}
	latest, err = Read(context.Background(), historysource.Request{Source: s})
	if err != nil || !latest.Pending || len(latest.Records) != 4 {
		t.Fatal(latest, err)
	}
}

func TestToolBodiesReachFrozenConsumer(t *testing.T) {
	for _, tc := range []struct {
		version string
		fixture func(*testing.T) historysource.Source
		calls   int
		notes   int
	}{
		{"1.4", fixture, 7, 1},
		{"1.5", fixture15Tool, 1, 0},
	} {
		t.Run(tc.version, func(t *testing.T) {
			s := tc.fixture(t)
			ctx := context.Background()
			service := historyimport.New(historyimport.Options{Readers: map[string]historysource.ReadFunc{"kimi": Read}})
			scope, err := service.FreezeScope(ctx, s)
			if err != nil {
				t.Fatal(err)
			}
			if scope.Snapshot.FormatVersion != "kimi-wire/"+tc.version {
				t.Fatal(scope.Snapshot.FormatVersion)
			}
			appendWire(t, s.Path, `{"type":"context.append_message","message":{"role":"user","content":[{"type":"text","text":"NOT_IN_FROZEN_EXPORT"}]}}`+"\n")
			page, err := service.Page(ctx, scope.ID, historysource.PageRequest{Limit: 100, MaxBytes: 65536})
			if err != nil || page.More {
				t.Fatal(page, err)
			}
			calls := map[string]historysource.Record{}
			results, notes := 0, 0
			for _, r := range page.Records {
				if strings.Contains(r.Text, "NOT_IN_FROZEN_EXPORT") {
					t.Fatal("consumer escaped frozen prefix", r)
				}
				switch r.Kind {
				case "tool_arguments":
					if !json.Valid([]byte(r.Text)) || r.ToolName == "" {
						t.Fatal("consumer lost tool arguments", r)
					}
					calls[r.CallID] = r
					if tc.version == "1.5" && r.Text != `{"path":"fact.txt"}` {
						t.Fatal("real 1.5 arguments changed", r)
					}
				case "tool_result":
					results++
					call, ok := calls[r.CallID]
					if !ok || call.ToolName != r.ToolName || r.ParentID != call.ID || r.Text == "" {
						t.Fatal("consumer lost result or association", r)
					}
					if r.Text == "[redacted]\n[redacted]" {
						notes++
					}
					if tc.version == "1.5" && r.Text != "1\tSYNTHETIC_TOOL_FACT=FIXTURE-KIMI-TOOL\n2\tproject=青鹭\n3\tport=49367" {
						t.Fatal("real 1.5 result changed", r)
					}
				}
			}
			if len(calls) != tc.calls || results != tc.calls || notes != tc.notes {
				t.Fatal(len(calls), results, notes)
			}
		})
	}
}

func TestProtocol15ConsumerFiltersAndFreeze(t *testing.T) {
	s := fixture15(t)
	ctx := context.Background()
	service := historyimport.New(historyimport.Options{Readers: map[string]historysource.ReadFunc{"kimi": Read}})
	scope, err := service.FreezeScope(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.Page(ctx, scope.ID, historysource.PageRequest{})
	if err != nil {
		t.Fatal(err)
	}
	users, assistants := 0, 0
	for _, r := range page.Records {
		if strings.Contains(r.Text, "HIDDEN") || strings.Contains(r.Text, "[redacted]") {
			t.Fatal("private runtime metadata leaked into consumer", r)
		}
		if r.Role == "user" {
			users++
		}
		if r.Role == "assistant" {
			assistants++
		}
	}
	if users != 1 || assistants != 1 {
		t.Fatal("consumer lost or duplicated messages", page)
	}
	statePath := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(s.Path))), "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"archived":false`, `"archived":true`, 1))
	if err := os.WriteFile(statePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Page(ctx, scope.ID, historysource.PageRequest{}); !errors.Is(err, historysource.ErrChanged) {
		t.Fatal("consumer accepted changed state", err)
	}
}

func TestUnknownProtocolAndStateRejected(t *testing.T) {
	for _, version := range []string{"1.3", "1.6", "2.0", ""} {
		t.Run("wire-"+version, func(t *testing.T) {
			s := fixture15(t)
			data, err := os.ReadFile(s.Path)
			if err != nil {
				t.Fatal(err)
			}
			data = []byte(strings.Replace(string(data), `"protocol_version":"1.5"`, `"protocol_version":"`+version+`"`, 1))
			if err := os.WriteFile(s.Path, data, 0600); err != nil {
				t.Fatal(err)
			}
			doc, err := Read(context.Background(), historysource.Request{Source: s})
			if !errors.Is(err, historysource.ErrFormat) || len(doc.Records) != 0 {
				t.Fatal(doc, err)
			}
		})
	}
	for _, tc := range []struct {
		name, key string
		value     any
		want      error
	}{
		{"unknown-state", "version", 3, historysource.ErrFormat},
		{"malformed-state-version", "version", "future", historysource.ErrFormat},
		{"wrong-state-id", "id", "session-other", historysource.ErrIdentity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture15(t)
			path := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(s.Path))), "state.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var state map[string]any
			if err := json.Unmarshal(data, &state); err != nil {
				t.Fatal(err)
			}
			state[tc.key] = tc.value
			data, err = json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Read(context.Background(), historysource.Request{Source: s}); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
			root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(s.Path)))))
			wantDiscover := tc.want
			if tc.key == "version" {
				wantDiscover = nil
			}
			if _, err := Discover(context.Background(), root); !errors.Is(err, wantDiscover) {
				t.Fatal(err)
			}
		})
	}
}

func TestPreviousParserCursorRejected(t *testing.T) {
	s := fixture15(t)
	doc, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.RawURLEncoding.DecodeString(doc.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	var old boundary
	if err := json.Unmarshal(data, &old); err != nil {
		t.Fatal(err)
	}
	old.Parser = ""
	data, err = json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	cursor := base64.RawURLEncoding.EncodeToString(data)
	if _, err := Read(context.Background(), historysource.Request{Source: s, Cursor: cursor}); !errors.Is(err, historysource.ErrChanged) {
		t.Fatal(err)
	}
}

func TestProtocol15MutationGapsAndRewrite(t *testing.T) {
	s := fixture15(t)
	appendWire(t, s.Path,
		`{"type":"context.compact","body":"HIDDEN_MUTATION"}`+"\n"+
			`{"type":"context.append_loop_event","agentId":"main","event":{"type":"future.event","uuid":"future-event","body":"HIDDEN_EVENT"}}`+"\n")
	doc, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil || len(doc.Gaps) != 2 || len(doc.Records) != 3 {
		t.Fatal(doc, err)
	}
	if doc.Gaps[0].Category != "unsupported-context-mutation" || doc.Gaps[1].Category != "unsupported-event" {
		t.Fatal(doc.Gaps)
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the prefix length unchanged: fingerprinting must detect edits, not
	// merely appends or truncation.
	data = []byte(strings.Replace(string(data), "ACK FIXTURE-KIMI-15", "ACK FIXTURE-KIMI-16", 1))
	if err := os.WriteFile(s.Path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), historysource.Request{Source: s, Cursor: doc.Cursor}); !errors.Is(err, historysource.ErrChanged) {
		t.Fatal("changed frozen prefix accepted", err)
	}
}

func fixture15Tool(t *testing.T) historysource.Source {
	t.Helper()
	return fixtureSession(t, "wd_fixture_0003", "session_00000000-0000-4000-8000-000000000003")
}

func TestProtocol15RealToolTranscript(t *testing.T) {
	s := fixture15Tool(t)
	doc, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil || doc.FormatVersion != "kimi-wire/1.5" || len(doc.Gaps) != 0 || len(doc.Records) != 6 {
		t.Fatal(doc, err)
	}
	if doc.Records[0].Role != "user" || doc.Records[0].Text != "读取 fact.txt 后只回复 ACK。" {
		t.Fatal("lost real prompt", doc.Records[0])
	}
	call, result := doc.Records[1], doc.Records[2]
	if call.Kind != "tool_arguments" || !call.Exportable() || call.ToolName != "Read" || call.Text != `{"path":"fact.txt"}` || call.ParentID == "" {
		t.Fatal("lost exportable arguments", call)
	}
	// Real 1.5 uses a call event UUID distinct from its toolCallId. Result
	// parentUuid refers to the event UUID, not to the toolCallId.
	if call.ID == call.CallID || result.CallID != call.CallID || result.ParentID != call.ID || result.ToolName != call.ToolName {
		t.Fatal("lost native tool association", call, result)
	}
	if result.Kind != "tool_result" || result.Status != "completed" || result.Text != "1\tSYNTHETIC_TOOL_FACT=FIXTURE-KIMI-TOOL\n2\tproject=青鹭\n3\tport=49367" {
		t.Fatal("lost tool output", result)
	}
	if doc.Records[3].Kind != "status" || doc.Records[3].Status != "tool_use" || doc.Records[3].ID != call.ParentID {
		t.Fatal("lost tool step", doc.Records[3])
	}
	if doc.Records[4].Role != "assistant" || doc.Records[4].Text != "ACK" || doc.Records[5].Status != "end_turn" {
		t.Fatal("lost final assistant answer", doc.Records[4:])
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(s.Path)))))
	sources, err := Discover(context.Background(), root)
	if err != nil || len(sources) != 1 || sources[0].NativeID != s.NativeID {
		t.Fatal(sources, err)
	}
}

func TestDiscoverMixedStateVersions(t *testing.T) {
	root := t.TempDir()
	target := fixtureSessionAtRoot(t, root, "wd_fixture_0001", "session_00000000-0000-4000-8000-000000000001")
	unsupported := fixtureSessionAtRoot(t, root, "wd_fixture_0002", "session_00000000-0000-4000-8000-000000000002")
	path := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(unsupported.Path))), "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	state["version"] = 3
	data, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sources, err := Discover(ctx, root)
	if err != nil || len(sources) != 2 {
		t.Fatal("unsupported state poisoned discovery", sources, err)
	}
	found := make(map[string]bool)
	for _, source := range sources {
		found[source.NativeID] = true
		doc, err := Read(ctx, historysource.Request{Source: source})
		switch source.NativeID {
		case target.NativeID:
			if err != nil || len(doc.Records) == 0 {
				t.Fatal("supported target became unreadable", doc, err)
			}
		case unsupported.NativeID:
			if !errors.Is(err, historysource.ErrFormat) || len(doc.Records) != 0 {
				t.Fatal("unsupported state lost precise rejection", doc, err)
			}
		default:
			t.Fatal("unexpected source", source)
		}
	}
	if !found[target.NativeID] || !found[unsupported.NativeID] {
		t.Fatal("discovery silently omitted a source", sources)
	}
}

func TestUnknownUserOriginProducesSafeGap(t *testing.T) {
	s := fixture15(t)
	info, err := os.Stat(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	appendWire(t, s.Path,
		`{"type":"context.append_message","agentId":"main","message":{"id":"future-origin","role":"user","origin":{"kind":"future-origin"},"content":[{"type":"text","text":"HIDDEN_UNKNOWN_ORIGIN"}]}}`+"\n"+
			`{"type":"context.append_message","agentId":"main","message":{"id":"known-injection","role":"user","origin":{"kind":"injection"},"content":[{"type":"text","text":"HIDDEN_KNOWN_INJECTION"}]}}`+"\n")
	ctx := context.Background()
	doc, err := Read(ctx, historysource.Request{Source: s})
	if err != nil || len(doc.Records) != 3 || len(doc.Gaps) != 1 {
		t.Fatal(doc, err)
	}
	gap := doc.Gaps[0]
	if gap.RecordID != "future-origin" || gap.Category != "unsupported-message-origin" || gap.Offset != info.Size() {
		t.Fatal("unknown origin was silently discarded", gap)
	}
	service := historyimport.New(historyimport.Options{Readers: map[string]historysource.ReadFunc{"kimi": Read}})
	scope, err := service.FreezeScope(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.Page(ctx, scope.ID, historysource.PageRequest{})
	if err != nil || len(page.Snapshot.Gaps) != 1 {
		t.Fatal("consumer lost unsupported-origin gap", page, err)
	}
	for _, record := range page.Records {
		if strings.Contains(record.Text, "HIDDEN") {
			t.Fatal("unknown origin or injection exported as body", record)
		}
	}
}
