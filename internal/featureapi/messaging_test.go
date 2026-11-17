package featureapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/messaging"
)

func TestMessagingRoutes(t *testing.T) {
	store := messaging.Open(t.TempDir())
	defer store.Close()
	server := Bind(Deps{Config: config.Config{Features: config.FeatureConfig{Enabled: []string{"messaging"}}}, Messaging: store})
	mux := http.NewServeMux()
	for _, c := range server.Contributions() {
		for _, route := range c.Routes {
			mux.Handle(route.Pattern, route.Handler)
		}
	}
	for _, tt := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/api/messaging/channels", "", 200},
		{"POST", "/api/messaging/channels", `{"name":"general","topic":"Team"}`, 201},
		{"POST", "/api/messaging/channels", `{"name":"general","topic":"Team"}`, 409},
		{"POST", "/api/messaging/channels/general/messages", `{"text":"hello","clientRequestId":"request-0001"}`, 201},
		{"POST", "/api/messaging/channels/general/messages", `{"text":"hello","clientRequestId":"request-0001"}`, 201},
		{"POST", "/api/messaging/channels/general/messages", `{"text":"changed","clientRequestId":"request-0001"}`, 409},
		{"POST", "/api/messaging/channels/general/messages", `{"text":"hello","agentName":"spoofed","clientRequestId":"request-0002"}`, 400},
		{"GET", "/api/messaging/channels/general/messages?limit=101", "", 400},
		{"GET", "/api/messaging/channels/missing/messages", "", 404},
	} {
		r := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Errorf("%s %s: %d %s", tt.method, tt.path, w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest("GET", "/api/messaging/channels/general/messages", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	var messages []contract.MessagingMessage
	if err := json.Unmarshal(w.Body.Bytes(), &messages); err != nil || len(messages) != 1 || messages[0].AgentID != "owner" {
		t.Fatalf("history: %s, %v", w.Body.String(), err)
	}
}
