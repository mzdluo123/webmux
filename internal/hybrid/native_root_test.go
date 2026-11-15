package hybrid

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeRootLaunchSnapshotAndRecovery(t *testing.T) {
	for _, key := range []string{"HOME", "CODEX_HOME", "CLAUDE_CONFIG_DIR", "KIMI_CODE_HOME", "XDG_DATA_HOME"} {
		t.Run(key, func(t *testing.T) {
			h, db := historyHarness(t)
			h.svc.nativeRoot = func(_ string, env map[string]string) string { return env[key] }
			in := h.input("omp", SideACP)
			in.Env[key] = "original-root"
			id := h.create(in).Info().ID
			rec := h.record(id)
			want := filepath.Join(rec.Cwd, "original-root")
			if rec.HistoryRoot != want || conversationInput(rec, "").HistoryRoot != want {
				t.Fatalf("launch root not recorded: %+v", rec)
			}
			binding, err := db.Binding(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			// Editing the saved launch environment is not a historical rebind.
			in.Env[key] = "replacement-root"
			h.svc.syncConversation(h.record(id))
			bindings, err := db.NativeBindings(context.Background(), binding.ConversationID)
			if err != nil || len(bindings) != 1 || bindings[0].Root != want {
				t.Fatalf("config edit rewrote history: %+v %v", bindings, err)
			}
			if strings.Contains(h.file(), "secret-value") || strings.Contains(h.file(), "API_KEY") {
				t.Fatal("record persisted launch secrets")
			}
			h.closeSession(id)
			h.mustResume(id, ResumeInput{Side: SideACP, Env: in.Env})
			if got := h.record(id).HistoryRoot; got != filepath.Join(rec.Cwd, "replacement-root") {
				t.Fatalf("recovery did not bind actual launch root: %q", got)
			}
			bindings, err = db.NativeBindings(context.Background(), binding.ConversationID)
			if err != nil || len(bindings) != 2 || bindings[0].Root != want || bindings[1].Root != filepath.Join(rec.Cwd, "replacement-root") {
				t.Fatalf("recovery lost old/native identity: %+v %v", bindings, err)
			}
		})
	}
}

func TestNativeRootMigrationNeverUsesCurrentLaunchEnvironment(t *testing.T) {
	h, db := historyHarness(t)
	id := h.create(h.input("omp", SideACP)).Info().ID
	h.closeSession(id)
	h.svc.nativeRoot = func(_ string, _ map[string]string) string {
		t.Fatal("migration tried to guess a historical root")
		return ""
	}
	if err := h.svc.migrateConversations(); err != nil {
		t.Fatal(err)
	}
	binding, err := db.Binding(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := db.NativeBindings(context.Background(), binding.ConversationID)
	if err != nil || len(bindings) != 1 || bindings[0].Root != "" {
		t.Fatalf("migration guessed root: %+v %v", bindings, err)
	}
	if got := (&Service{}).runtimeRoot("unknown", "/work", nil); got != "" {
		t.Fatalf("missing capability synthesized a root: %q", got)
	}
}
