package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/handoff"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/session"
)

type handoffRuntime struct{ calls atomic.Int32 }

func (*handoffRuntime) Record(id string) (hybrid.Record, error) {
	return hybrid.Record{ID: id, Agent: "omp", AgentSessionID: "native", HistoryRoot: "/native", Cwd: "/work", ConfigID: "cfg", Side: hybrid.SideACP, State: hybrid.StateActive}, nil
}
func (r *handoffRuntime) Prompt(context.Context, string, hybrid.PromptInput) (hybrid.PromptResult, error) {
	r.calls.Add(1)
	return hybrid.PromptResult{TurnID: "turn"}, nil
}

type handoffConfig struct{}

func (handoffConfig) Get(string) (launch.Config, error) {
	return launch.Config{ID: "cfg", Cmd: []string{"omp"}, Hybrid: true, HybridSupport: agentprofile.Support{Supported: true, Agent: "omp"}}, nil
}
func TestNativeHandoffHTTP(t *testing.T) {
	runtime := &handoffRuntime{}
	native := historyimport.New(historyimport.Options{
		Resolve: func(context.Context, string) (historyimport.Identity, error) {
			return historyimport.Identity{Agent: "omp", NativeID: "native", Root: "/native"}, nil
		},
		Discoverers: map[string]historyimport.DiscoverFunc{"omp": func(context.Context, string) ([]historysource.Source, error) {
			return []historysource.Source{{Agent: "omp", NativeID: "native", Path: "/native/session"}}, nil
		}},
		Readers: map[string]historysource.ReadFunc{"omp": func(ctx context.Context, r historysource.Request) (historysource.Document, error) {
			return historysource.Document{Source: r.Source, Cursor: "fixed", Generation: "1", Fingerprint: "file", FormatVersion: "v1", ParserVersion: "v1", Records: []historysource.Record{{ID: "message", Kind: "prose", Role: "user", Text: "原文\n  code"}}}, nil
		}},
	})
	projection := historyprojection.New(historyprojection.Options{Source: native, Preprocess: historyprojection.DefaultPreprocessConfig()})
	handoffs := handoff.New(handoff.Options{Hybrid: runtime, Launch: handoffConfig{}, NativeHistory: native, HistoryProjection: projection})
	t.Cleanup(handoffs.Close)
	manager := session.NewManager(session.Options{})
	handler := New(testDeps(t, testServices{Config: config.Config{Token: "secret"}, Manager: manager, Handoffs: handoffs, NativeHistory: native, HistoryProjection: projection}))
	request := func(method, path, body, token, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://webmux.example"+path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	body := `{"configId":"","targetSessionId":"target","agentConfig":{},"userText":"task\\n  code","location":{"cwd":"","repositoryId":"","worktreeId":"","newWorktreeBase":"","newWorktreeBranch":""}}`
	path := "/api/sessions/source/handoff-previews"
	if w := request("POST", path, body, "", ""); w.Code != 401 {
		t.Fatalf("auth %d", w.Code)
	}
	if w := request("POST", path, body, "secret", "https://evil.example"); w.Code != 403 {
		t.Fatalf("Origin %d: %s", w.Code, w.Body.String())
	}
	w := request("POST", path, body, "secret", "")
	if w.Code != 201 {
		t.Fatalf("preview %d: %s", w.Code, w.Body.String())
	}
	var p contract.HandoffPreview
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.SnapshotID == "" || p.OperationID == "" || strings.Contains(w.Body.String(), "conversationId") {
		t.Fatalf("preview %s", w.Body.String())
	}
	payload, _ := json.Marshal(contract.HandoffSendRequest{OperationID: p.OperationID})
	for range 10 {
		w = request("POST", "/api/handoff-previews/"+p.ID+"/send", string(payload), "secret", "")
		if w.Code != 202 {
			t.Fatalf("send %d: %s", w.Code, w.Body.String())
		}
	}
	deadline := time.Now().Add(time.Second)
	var op contract.HandoffOperation
	for time.Now().Before(deadline) {
		w = request("GET", "/api/handoff-operations/"+p.OperationID, "", "secret", "")
		if err := json.Unmarshal(w.Body.Bytes(), &op); err != nil {
			t.Fatal(err)
		}
		if op.FinishedAt != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if op.State != "succeeded" || runtime.calls.Load() != 1 {
		t.Fatalf("operation %+v calls=%d", op, runtime.calls.Load())
	}
	w = request("POST", "/api/native-history", `{"action":"read","snapshotId":"`+p.SnapshotID+`"}`, "secret", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "原文") {
		t.Fatalf("original %d: %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/conversations/native", "/api/conversations/old/activities", "/api/handoffs/old/delivery"} {
		w = request("POST", path, `{}`, "secret", "")
		if w.Code != 410 {
			t.Fatalf("retired %s: %d", path, w.Code)
		}
	}
}
func TestRetiredDatabaseFilesRemainUntouched(t *testing.T) {
	contents := map[string]string{}
	h := newHybridAPI(t, func(state string) {
		for _, suffix := range []string{"", "-journal", "-wal", "-shm", ".lock"} {
			name := "conversations.sqlite" + suffix
			contents[name] = "old or corrupt bytes: " + suffix
			if err := os.WriteFile(filepath.Join(state, name), []byte(contents[name]), 0600); err != nil {
				t.Fatal(err)
			}
		}
	})
	id := h.config("ordinary chat configuration", true)
	h.expect(`{"configId":"`+id+`","view":"terminal"}`, 201)
	for name, want := range contents {
		got, err := os.ReadFile(filepath.Join(h.state, name))
		if err != nil || string(got) != want {
			t.Fatalf("retired file %s: %q %v", name, got, err)
		}
	}
}
