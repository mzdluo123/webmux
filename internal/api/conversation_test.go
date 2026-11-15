package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/session"
)

func historyAPI(t *testing.T) (http.Handler, *conversation.Service, conversation.Binding) {
	t.Helper()
	s := conversation.Open(conversation.Options{StateDir: t.TempDir()})
	if err := s.Unavailable(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	sources, projection := nativeFixture(s)
	binding := bindHistorySource(t, s, "archived")
	manager := session.NewManager(session.Options{})
	return New(testDeps(t, testServices{Config: config.Config{Token: "secret"}, Manager: manager, Workspace: testWorkspace(t, manager, t.TempDir()), Conversations: s, NativeHistory: sources, HistoryProjection: projection})), s, binding
}

func historyRequest(handler http.Handler, path, token, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "http://webmux.example"+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, req)
	return out
}

func bindHistorySource(t *testing.T, store *conversation.Service, runtimeID string) conversation.Binding {
	t.Helper()
	root := t.TempDir()
	var identity [16]byte
	if _, err := rand.Read(identity[:]); err != nil {
		t.Fatal(err)
	}
	nativeID := fmt.Sprintf("%x-%x-%x-%x-%x", identity[:4], identity[4:6], identity[6:8], identity[8:10], identity[10:])
	path := filepath.Join(root, "rollout.jsonl")
	meta, err := json.Marshal(map[string]any{"timestamp": "2025-12-01T00:00:00Z", "type": "session_meta", "payload": map[string]any{"id": nativeID, "cwd": root, "cli_version": "0.77.0", "timestamp": "2025-12-01T00:00:00Z"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(meta, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	binding, err := store.EnsureRuntime(context.Background(), conversation.RuntimeInput{RuntimeID: runtimeID, Cwd: root, AgentName: "codex", AgentSessionID: nativeID, HistoryRoot: root, Status: "closed", HistoryOrigin: "complete"})
	if err != nil {
		t.Fatal(err)
	}
	sources, _ := nativeFixture(store)
	if err := sources.Bind(context.Background(), binding.ProviderThreadID, historysource.Source{Agent: "codex", NativeID: nativeID, Path: path}); err != nil {
		t.Fatal(err)
	}
	return binding
}

func seedHistory(t *testing.T, store *conversation.Service, binding conversation.Binding, text string) string {
	t.Helper()
	return seedNativeMessages(t, store, binding, []string{text})[0]
}

func seedNativeMessages(t *testing.T, store *conversation.Service, binding conversation.Binding, texts []string) []string {
	t.Helper()
	bindings, err := store.NativeBindings(context.Background(), binding.ConversationID)
	if err != nil || len(bindings) != 1 {
		t.Fatalf("native binding: %v %v", bindings, err)
	}
	source := bindings[0].Source
	file, err := os.OpenFile(source.Path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, text := range texts {
		if err := encoder.Encode(map[string]any{"timestamp": "2025-12-01T00:00:01Z", "type": "response_item", "payload": map[string]any{"id": rand.Text(), "type": "message", "role": "user", "content": []map[string]string{{"type": "input_text", "text": text}}}}); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	sources, _ := nativeFixture(store)
	snapshot, err := sources.Freeze(context.Background(), source, binding.ConversationID, false)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(texts))
	for _, record := range snapshot.Records[len(snapshot.Records)-len(texts):] {
		ids = append(ids, record.ID)
	}
	return ids
}

func TestConversationHistoryAuthenticationAndCursorValidation(t *testing.T) {
	handler, s, b := historyAPI(t)
	item := seedHistory(t, s, b, "你好🙂")
	base := "/api/conversations/" + b.ConversationID + "/activities"
	tests := []struct {
		name, path, token, origin string
		want                      int
	}{
		{"unauthenticated", base, "", "", 401},
		{"malicious Origin", base, "secret", "https://evil.example", 403},
		{"valid", base, "secret", "http://webmux.example", 200},
		{"negative", base + "?afterPosition=-1", "secret", "", 400},
		{"SQL cursor", base + "?afterPosition=0%20OR%201=1", "secret", "", 400},
		{"overflow", base + "?afterPosition=9223372036854775808", "secret", "", 400},
		{"leading zero", base + "?afterPosition=01", "secret", "", 400},
		{"duplicate cursor", base + "?afterPosition=0&afterPosition=1", "secret", "", 400},
		{"empty cursor", base + "?afterPosition=", "secret", "", 400},
		{"unknown parameter", base + "?path=/etc/passwd", "secret", "", 400},
		{"oversize limit", base + "?limit=101", "secret", "", 400},
		{"zero limit", base + "?limit=0", "secret", "", 400},
		{"oversize bytes", base + "?maxBytesPerItem=65537", "secret", "", 400},
		{"aggregate limit", base + "?limit=100&maxBytesPerItem=65536", "secret", "", 200},
		{"bad rune offset", base + "/" + item + "?textOffset=1", "secret", "", 400},
		{"outside offset", base + "/" + item + "?textOffset=999", "secret", "", 400},
		{"missing conversation", "/api/conversations/" + strings.Repeat("0", 32) + "/activities", "secret", "", 404},
		{"missing activity", base + "/" + strings.Repeat("0", 32), "secret", "", 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := historyRequest(handler, tt.path, tt.token, tt.origin)
			if tt.want == 200 && r.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("history response cacheable")
			}
			if r.Code != tt.want {
				t.Fatalf("status=%d want %d: %s", r.Code, tt.want, r.Body.String())
			}
		})
	}
	other := bindHistorySource(t, s, "other")
	response := historyRequest(handler, "/api/conversations/"+other.ConversationID+"/activities/"+item, "secret", "")
	if response.Code != 404 {
		t.Fatalf("cross-conversation read: %d", response.Code)
	}
}

func TestArchivedConversationHistoryPaginationAndLiveAppend(t *testing.T) {
	handler, s, b := historyAPI(t)
	texts := []string{"first", strings.Repeat("中文🙂", 500), "third"}
	ids := make([]string, 0, len(texts))
	for _, text := range texts {
		ids = append(ids, seedHistory(t, s, b, text))
	}
	// Control-plane gaps do not manufacture missing native records.
	if err := s.AddGap(context.Background(), b.ProviderThreadID, "tui"); err != nil {
		t.Fatal(err)
	}
	base := "/api/conversations/" + b.ConversationID + "/activities"
	readPage := func(cursor string) contract.ConversationActivities {
		t.Helper()
		response := historyRequest(handler, base+"?limit=1&afterPosition="+cursor, "secret", "")
		if response.Code != 200 {
			t.Fatal(response.Body.String())
		}
		var page contract.ConversationActivities
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page
	}
	page := readPage("0")
	if len(page.Items) != 1 || page.Items[0].ActivityID != ids[0] || page.HistoryOrigin != "complete" || page.ReconcileRequired || len(page.Gaps) != 0 {
		t.Fatalf("history completeness: %+v", page)
	}
	// Native appends remain visible without a runtime or hybrid actor.
	fourth := seedHistory(t, s, b, "fourth")
	ids = append(ids, fourth)
	frozen := historyRequest(handler, base+"?snapshotIds="+strings.Join(page.SnapshotIDs, ",")+"&afterPosition=3", "secret", "")
	if frozen.Code != 200 {
		t.Fatal(frozen.Body.String())
	}
	var frozenPage contract.ConversationActivities
	if err := json.Unmarshal(frozen.Body.Bytes(), &frozenPage); err != nil || len(frozenPage.Items) != 0 {
		t.Fatalf("frozen source included later append: %+v %v", frozenPage, err)
	}
	other := bindHistorySource(t, s, "snapshot-other")
	if response := historyRequest(handler, "/api/conversations/"+other.ConversationID+"/activities?snapshotIds="+strings.Join(page.SnapshotIDs, ","), "secret", ""); response.Code != 409 {
		t.Fatalf("cross-conversation snapshot read: %d %s", response.Code, response.Body.String())
	}
	cursor := page.NextPosition
	for _, id := range ids[1:] {
		page = readPage(cursor)
		if len(page.Items) != 1 || page.Items[0].ActivityID != id {
			t.Fatalf("pagination duplicate/hole: %+v", page)
		}
		cursor = page.NextPosition
	}
	if page := readPage(cursor); len(page.Items) != 0 || page.Items == nil || page.NextPosition != cursor {
		t.Fatalf("empty page: %+v", page)
	}
	var body strings.Builder
	offset := "0"
	for {
		response := historyRequest(handler, base+"/"+ids[1]+"?maxBytes=7&textOffset="+offset, "secret", "")
		if response.Code != 200 {
			t.Fatal(response.Body.String())
		}
		var part contract.ConversationActivity
		if err := json.Unmarshal(response.Body.Bytes(), &part); err != nil {
			t.Fatal(err)
		}
		if !utf8.ValidString(part.Text) || !part.Exportable {
			t.Fatal("invalid text/exportability")
		}
		before, _ := strconv.ParseInt(offset, 10, 64)
		next, _ := strconv.ParseInt(part.NextTextOffset, 10, 64)
		if part.Truncated && next <= before {
			t.Fatal("cursor stalled")
		}
		body.WriteString(part.Text)
		offset = part.NextTextOffset
		if !part.Truncated {
			break
		}
	}
	if body.String() != texts[1] {
		t.Fatal("multibyte text lost")
	}
}
