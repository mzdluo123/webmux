package history

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

func fixture(t *testing.T) historysource.Source {
	t.Helper()
	root := t.TempDir()
	id := "session_00000000-0000-4000-8000-000000000001"
	source := filepath.Join("..", "..", "testdata", "native-history", "kimi", "wd_fixture_0001", id)
	home := filepath.Join(root, "wd_fixture_0001", id, "agents", "main")
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
	if err := os.WriteFile(filepath.Join(root, "wd_fixture_0001", id, "state.json"), state, 0600); err != nil {
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
		if r.Kind == "tool_use" {
			calls++
			names[r.CallID] = r.ToolName
		}
		if r.Kind == "tool_result" {
			results++
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
