package handoff

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	codexhistory "github.com/rainchan/webmux/internal/agent/codex/history"
	"github.com/rainchan/webmux/internal/agentprofile/agents"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/launch"
)

func TestFrozenProjectionAndIndependentFullFork(t *testing.T) {
	ctx := context.Background()
	store := conversation.Open(conversation.Options{StateDir: t.TempDir()})
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Unavailable(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "rollout.jsonl")
	body, err := os.ReadFile("../codexrollout/testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.Repeat("raw command output 你好\n", 500) + "RAW_END_MARKER"
	appendRecord := func(item map[string]any) {
		record, err := json.Marshal(map[string]any{"type": "response_item", "payload": item})
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, record...)
		body = append(body, '\n')
	}
	appendRecord(map[string]any{"type": "function_call", "id": "native-large-call", "call_id": "large-call", "name": "shell_command", "arguments": `{"command":"printf synthetic"}`})
	appendRecord(map[string]any{"type": "function_call_output", "id": "native-large-output", "call_id": "large-call", "output": raw})
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	const nativeID = "00000000-0000-4000-8000-000000000001"
	binding, err := store.EnsureRuntime(ctx, conversation.RuntimeInput{RuntimeID: "native-source", Cwd: root, AgentName: "codex", AgentSessionID: nativeID, HistoryRoot: root, Status: "closed", HistoryOrigin: "complete"})
	if err != nil {
		t.Fatal(err)
	}
	sources := historyimport.New(historyimport.Options{Store: store, Readers: map[string]historysource.ReadFunc{"codex": codexhistory.Read}})
	if err := sources.Bind(ctx, binding.ProviderThreadID, historysource.Source{Agent: "codex", NativeID: nativeID, Path: path}); err != nil {
		t.Fatal(err)
	}
	cfg := historyprojection.DefaultPreprocessConfig()
	cfg.OutputRunes = 40
	cfg.ArgumentRunes = 40
	cfg.CallRunes = 40
	projection := historyprojection.New(historyprojection.Options{Source: sources, Preprocess: cfg})
	launches := launch.Open(t.TempDir(), agents.Default())
	t.Cleanup(launches.Close)
	target, err := launches.Create(launch.Input{Name: "target", Cmd: []string{"omp"}, Hybrid: true})
	if err != nil {
		t.Fatal(err)
	}
	svc := New(Options{Conversations: store, Launch: launches, NativeHistory: sources, HistoryProjection: projection, TokenCap: 16000, MaxPromptBytes: 64 << 10, MaxPromptWireBytes: 1 << 20})
	t.Cleanup(svc.Close)
	preview, err := svc.Preview(ctx, binding.ConversationID, Request{ClientRequestID: "native-projection", ConfigID: target.ID, UserText: "CURRENT_TASK_ONLY"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.State != "ready" || strings.Contains(preview.RenderedText, raw) || strings.Contains(preview.RenderedText, "CURRENT_TASK_ONLY") || !strings.Contains(preview.RenderedText, `tool="shell_command"`) || !strings.Contains(preview.RenderedText, "mechanical-text/") {
		t.Fatalf("projection lost tool provenance, copied full output, or mixed the current task: state=%s", preview.State)
	}
	history, _, err := svc.nativeContext(ctx, preview)
	if err != nil {
		t.Fatal(err)
	}
	var delivered strings.Builder
	for _, item := range history.Items {
		delivered.WriteString(item.Text)
	}
	if strings.Contains(delivered.String(), raw) || strings.Contains(delivered.String(), "CURRENT_TASK_ONLY") || !strings.Contains(delivered.String(), `tool="shell_command"`) {
		t.Fatal("native delivery bypassed the reviewed mechanical projection")
	}
	appendRecord(map[string]any{"type": "message", "id": "late-native-entry", "role": "user", "content": []map[string]string{{"type": "input_text", "text": "LATE_NATIVE_APPEND"}}})
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	frozen, err := sources.Documents(ctx, preview.Request.SnapshotIDs)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range frozen.Records {
		if strings.Contains(record.Text, "LATE_NATIVE_APPEND") {
			t.Fatal("live append entered an already reviewed source scope")
		}
	}
	fork, err := svc.Preview(ctx, binding.ConversationID, Request{ClientRequestID: "native-full-fork", ConfigID: target.ID, UserText: "fork task", Fork: true})
	if err != nil {
		t.Fatal(err)
	}
	forkSource, err := sources.Documents(ctx, fork.Request.SnapshotIDs)
	if err != nil {
		t.Fatal(err)
	}
	appendRecord(map[string]any{"type": "message", "id": "after-fork-entry", "role": "user", "content": []map[string]string{{"type": "input_text", "text": "AFTER_FORK_APPEND"}}})
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.BindRuntime(ctx, "materialized-child", fork.TargetProviderThreadID, target.ID, "omp"); err != nil {
		t.Fatal(err)
	}
	if err := store.ForgetRuntime(ctx, "native-source"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	prefix, err := store.PrefixSnapshots(ctx, fork.TargetConversationID)
	if err != nil {
		t.Fatal(err)
	}
	child, err := sources.Documents(ctx, prefix)
	if err != nil {
		t.Fatal(err)
	}
	if len(child.Records) != len(forkSource.Records) {
		t.Fatalf("fork source scope changed: got %d records, want %d", len(child.Records), len(forkSource.Records))
	}
	for i, record := range child.Records {
		if record.ID != forkSource.Records[i].ID || record.Text != forkSource.Records[i].Text || strings.Contains(record.Text, "AFTER_FORK_APPEND") {
			t.Fatalf("fork source mutated after append, forget, or deletion: record %d", i)
		}
	}
	fullOutput, late := false, false
	for _, record := range child.Records {
		fullOutput = fullOutput || record.Text == raw
		late = late || record.Text == "LATE_NATIVE_APPEND"
	}
	if !fullOutput || !late {
		t.Fatalf("fork lost immutable source data: fullOutput=%t lateAppend=%t records=%d", fullOutput, late, len(child.Records))
	}
}
