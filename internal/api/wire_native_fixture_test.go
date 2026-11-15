package api

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historysource"
)

func wireEmptyNativeSource(f *wireFixture, runtime, cwd string) conversation.Binding {
	f.t.Helper()
	nativeID := "wire-native-" + runtime
	binding, err := f.conversations.EnsureRuntime(f.ctx, conversation.RuntimeInput{RuntimeID: runtime, Cwd: cwd, AgentName: "codex", AgentSessionID: nativeID, Status: "closed", HistoryOrigin: "complete"})
	if err != nil {
		f.t.Fatal(err)
	}
	path := filepath.Join(f.root, runtime+".jsonl")
	header, err := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]string{"id": nativeID, "cwd": cwd, "cli_version": "0.77.0"}})
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, append(header, '\n'), 0600); err != nil {
		f.t.Fatal(err)
	}
	sources, _ := nativeFixture(f.conversations)
	if err := sources.Bind(f.ctx, binding.ProviderThreadID, historysource.Source{Agent: "codex", NativeID: nativeID, Path: path}); err != nil {
		f.t.Fatal(err)
	}
	return binding
}
