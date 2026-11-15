package conversation

import (
	"context"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

func TestNativeRootPreservesExactProviderBindings(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	in := RuntimeInput{RuntimeID: "runtime", Cwd: "/work", AgentName: "codex", AgentSessionID: "native", HistoryRoot: "/original", Status: "closed"}
	binding, err := s.EnsureRuntime(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	original := historysource.Source{Agent: "codex", NativeID: "native", Path: "/original/sessions/exact.jsonl"}
	if err := s.BindNativeSource(ctx, binding.ProviderThreadID, original); err != nil {
		t.Fatal(err)
	}
	// Missing root metadata during migration must not erase a known binding.
	in.HistoryRoot = ""
	if err := s.SyncRuntime(ctx, in); err != nil {
		t.Fatal(err)
	}
	bindings, err := s.NativeBindings(ctx, binding.ConversationID)
	if err != nil || len(bindings) != 1 || bindings[0].Root != "/original" || bindings[0].Source != original {
		t.Fatalf("migration changed exact source: %+v %v", bindings, err)
	}
	// A process really launched with a different root gets a distinct provider
	// identity even when a provider reuses its session ID in both directories.
	in.HistoryRoot = "/new-launch"
	if err := s.SyncRuntime(ctx, in); err != nil {
		t.Fatal(err)
	}
	current, err := s.Binding(ctx, in.RuntimeID)
	if err != nil || current.ProviderThreadID == binding.ProviderThreadID || current.ConversationID != binding.ConversationID {
		t.Fatalf("root change reused identity: %+v %v", current, err)
	}
	bindings, err = s.NativeBindings(ctx, binding.ConversationID)
	if err != nil || len(bindings) != 2 || bindings[0].Root != "/original" || bindings[0].Source != original || bindings[1].Root != "/new-launch" || bindings[1].Source.Path != "" {
		t.Fatalf("lost historical native binding: %+v %v", bindings, err)
	}
}

func TestNativeRootDoesNotInferHistoricalUnknownDirectory(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	in := RuntimeInput{RuntimeID: "runtime", Cwd: "/work", AgentName: "codex", AgentSessionID: "native", Status: "closed"}
	binding, err := s.EnsureRuntime(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	in.HistoryRoot = "/actual-new-launch"
	if err := s.SyncRuntime(ctx, in); err != nil {
		t.Fatal(err)
	}
	bindings, err := s.NativeBindings(ctx, binding.ConversationID)
	if err != nil || len(bindings) != 2 || bindings[0].Root != "" || bindings[1].Root != in.HistoryRoot {
		t.Fatalf("guessed historical root: %+v %v", bindings, err)
	}
}

func TestNativeBindingsExcludeCaptureDisabledProviders(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	binding := bindTest(t, s, "runtime")
	if _, err := s.db.ExecContext(ctx, "INSERT INTO history_gap(id,conversation_id,provider_thread_id,reason,created_at) VALUES('private',?,?, 'capture_disabled',1)", binding.ConversationID, binding.ProviderThreadID); err != nil {
		t.Fatal(err)
	}
	bindings, err := s.NativeBindings(ctx, binding.ConversationID)
	if err != nil || len(bindings) != 0 {
		t.Fatalf("opted-out provider exposed: %+v %v", bindings, err)
	}
	in := RuntimeInput{RuntimeID: "runtime", Cwd: "/work", AgentName: "codex", AgentSessionID: "native-id", HistoryRoot: "/new", Status: "closed"}
	if err := s.SyncRuntime(ctx, in); err != nil {
		t.Fatal(err)
	}
	bindings, err = s.NativeBindings(ctx, binding.ConversationID)
	if err != nil || len(bindings) != 0 {
		t.Fatalf("replacement exposed opted-out lineage: %+v %v", bindings, err)
	}
}
