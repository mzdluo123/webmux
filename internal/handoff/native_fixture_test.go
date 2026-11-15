package handoff

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	claudehistory "github.com/rainchan/webmux/internal/agentprofile/claudecode/history"
	codexhistory "github.com/rainchan/webmux/internal/agentprofile/codex/history"
	omphistory "github.com/rainchan/webmux/internal/agentprofile/omp/history"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
)

type nativeFixtureMessage struct{ ID, Role, Text string }

func nativeTestServices(store *conversation.Service) (*historyimport.Service, *historyprojection.Service) {
	sources := historyimport.New(historyimport.Options{Store: store, Readers: map[string]historysource.ReadFunc{"omp": omphistory.Read, "codex": codexhistory.Read, "claudecode": claudehistory.Read}})
	return sources, historyprojection.New(historyprojection.Options{Source: sources, Preprocess: historyprojection.DefaultPreprocessConfig()})
}

// The isolated protocol agent persists its own wire transcript. Materialize that
// transcript as a native-format fixture before exercising the real JSONL reader.
func writeNativeFixture(t *testing.T, path, agent, id string, messages []nativeFixtureMessage) {
	t.Helper()
	var body []byte
	add := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, b...)
		body = append(body, '\n')
	}
	if agent == "codex" {
		add(map[string]any{"type": "session_meta", "payload": map[string]any{"id": id, "cli_version": "0.77.0", "cwd": "/example/project"}})
	} else if agent == "omp" {
		add(map[string]any{"type": "session", "version": 3, "id": id})
	}
	parent := ""
	for i, m := range messages {
		if m.ID == "" {
			m.ID = fmt.Sprintf("entry-%d", i)
		}
		role := m.Role
		if role == "agent" {
			role = "assistant"
		}
		if agent == "codex" {
			kind := "output_text"
			if role == "user" {
				kind = "input_text"
			}
			add(map[string]any{"type": "response_item", "payload": map[string]any{"type": "message", "id": m.ID, "role": role, "content": []map[string]string{{"type": kind, "text": m.Text}}}})
		} else if agent == "claudecode" {
			add(map[string]any{"type": role, "uuid": m.ID, "parentUuid": parent, "sessionId": id, "message": map[string]any{"role": role, "content": []map[string]string{{"type": "text", "text": m.Text}}}})
		} else {
			add(map[string]any{"type": "message", "id": m.ID, "parentId": parent, "message": map[string]any{"role": role, "content": []map[string]string{{"type": "text", "text": m.Text}}}})
		}
		parent = m.ID
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func appendNativeFixture(t *testing.T, path, kind, role, text string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parent := ""
	for _, line := range splitNativeLines(body) {
		var row struct{ ID string }
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatal(err)
		}
		parent = row.ID
	}
	id := fmt.Sprintf("entry-%d", len(body))
	row := map[string]any{"type": kind, "id": id, "parentId": parent}
	// The session header is not a graph entry.
	if len(splitNativeLines(body)) == 1 {
		row["parentId"] = ""
	}
	if kind == "compaction" {
		row["summary"] = text
	} else {
		row["message"] = map[string]any{"role": role, "content": []map[string]string{{"type": "text", "text": text}}}
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		t.Fatal(err)
	}
}

func splitNativeLines(body []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range body {
		if b == '\n' {
			if i > start {
				lines = append(lines, body[start:i])
			}
			start = i + 1
		}
	}
	return lines
}

func (h *deliveryHarness) syncNative() {
	h.t.Helper()
	ctx := context.Background()
	bindings, err := h.store.NativeBindings(ctx, h.source.ConversationID)
	if err != nil {
		h.t.Fatal(err)
	}
	for _, binding := range bindings {
		source := binding.Source
		if source.NativeID == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(h.agentStore, "sessions", source.NativeID+".json"))
		if err != nil {
			h.t.Fatal(err)
		}
		var saved struct{ Messages []nativeFixtureMessage }
		if err := json.Unmarshal(body, &saved); err != nil {
			h.t.Fatal(err)
		}
		if binding.ProviderThreadID == h.source.ProviderThreadID && source.Agent == "omp" {
			saved.Messages = append([]nativeFixtureMessage{{ID: "seed-user", Role: "user", Text: "original constraint 中文🙂"}, {ID: "seed-agent", Role: "agent", Text: "partial result"}}, saved.Messages...)
		}
		source.Path = filepath.Join(h.root, "native-"+source.NativeID+".jsonl")
		writeNativeFixture(h.t, source.Path, source.Agent, source.NativeID, saved.Messages)
		if err := h.service.options.NativeHistory.Bind(ctx, binding.ProviderThreadID, source); err != nil {
			h.t.Fatal(err)
		}
	}
	for _, execution := range h.completed {
		if err := h.service.options.NativeHistory.RecordDispatchCheckpoint(ctx, execution); err != nil {
			h.t.Fatal(err)
		}
	}
}

func (h *deliveryHarness) previewNative(ctx context.Context, id string, in Request) (conversation.Preview, error) {
	h.syncNative()
	return h.service.Preview(ctx, id, in)
}
