package hybrid

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeRootLaunchSnapshotAndRecovery(t *testing.T) {
	for _, key := range []string{"HOME", "CODEX_HOME", "CLAUDE_CONFIG_DIR", "KIMI_CODE_HOME", "XDG_DATA_HOME"} {
		t.Run(key, func(t *testing.T) {
			h := newHarness(t, 0)
			h.svc.nativeRoot = func(_ string, env map[string]string) string { return env[key] }
			in := h.input("omp", SideACP)
			in.Env[key] = "original-root"
			id := h.create(in).Info().ID
			rec := h.record(id)
			want := filepath.Join(rec.Cwd, "original-root")
			if rec.HistoryRoot != want {
				t.Fatalf("launch root not recorded: %+v", rec)
			}
			in.Env[key] = "replacement-root"
			if h.record(id).HistoryRoot != want {
				t.Fatal("launch edit changed historical root")
			}
			if strings.Contains(h.file(), "secret-value") || strings.Contains(h.file(), "API_KEY") {
				t.Fatal("record persisted launch secrets")
			}
			h.closeSession(id)
			h.mustResume(id, ResumeInput{Side: SideACP, Env: in.Env})
			if got := h.record(id).HistoryRoot; got != filepath.Join(rec.Cwd, "replacement-root") {
				t.Fatalf("recovery did not bind actual launch root: %q", got)
			}

		})
	}
}

func TestNativeRootRetainsRecordedIdentity(t *testing.T) {
	h := newHarness(t, 0)
	id := h.create(h.input("omp", SideACP)).Info().ID
	h.closeSession(id)
	h.svc.nativeRoot = func(string, map[string]string) string {
		t.Fatal("record read consulted current environment")
		return ""
	}
	if got := h.record(id).HistoryRoot; got != "" {
		t.Fatalf("root %q", got)
	}
	if got := (&Service{}).runtimeRoot("unknown", "/work", nil); got != "" {
		t.Fatal(got)
	}
}
