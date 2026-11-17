package featureapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
	"github.com/rainchan/webmux/internal/messageplugin"
	"github.com/rainchan/webmux/internal/messaging"
	"github.com/rainchan/webmux/internal/transport"
)

func TestMessagePluginToolsRoundTripAndScope(t *testing.T) {
	store := messaging.Open(t.TempDir())
	defer store.Close()
	if _, err := store.CreateChannel("general", ""); err != nil {
		t.Fatal(err)
	}
	credential, err := store.Bind("codex-one", "Codex", "codex")
	if err != nil {
		t.Fatal(err)
	}
	server := Bind(Deps{Config: config.Config{Features: config.FeatureConfig{Enabled: []string{"messaging"}}}, Messaging: store})
	mux := http.NewServeMux()
	for _, contribution := range server.Contributions() {
		for _, route := range contribution.Routes {
			mux.Handle(route.Pattern, route.Handler)
		}
	}
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(transport.LocalContext(r.Context())))
	}))
	defer local.Close()
	client, err := localclient.NewTCP(local.URL, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	registry, err := (messageplugin.Client{API: client, Credential: credential}).Registry(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	session := registry.NewSession()
	dispatch := func(method string, params any, id string) *contract.MCPResponse {
		t.Helper()
		body, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		result, err := session.Dispatch(context.Background(), contract.MCPRequest{JSONRPC: "2.0", ID: json.RawMessage(id), Method: method, Params: body})
		if err != nil || result != nil && result.Error != nil {
			t.Fatalf("%s: %+v %v", method, result, err)
		}
		return result
	}
	initialized := dispatch("initialize", map[string]any{"protocolVersion": "2025-11-25"}, "1")
	if strings.Contains(string(initialized.Result), credential) {
		t.Fatal("credential in MCP instructions")
	}
	dispatch("notifications/initialized", map[string]any{}, "")
	call := func(name string, args any, fail bool) json.RawMessage {
		t.Helper()
		response := dispatch("tools/call", map[string]any{"name": name, "arguments": args}, "2")
		var result contract.MCPToolResult
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatal(err)
		}
		if result.IsError != fail {
			t.Fatalf("%s error=%v want %v: %s", name, result.IsError, fail, response.Result)
		}
		return result.StructuredContent
	}
	call("message_send", map[string]any{"channel": "general", "text": "before joining", "clientRequestId": "request-before"}, true)
	call("channel_join", map[string]any{"channel": "general"}, false)
	incoming, err := store.Send("general", "Please review", "owner-request")
	if err != nil {
		t.Fatal(err)
	}
	call("messages_read", map[string]any{"channel": "general"}, false)
	call("message_send", map[string]any{"channel": "general", "text": "Reviewed", "clientRequestId": "agent-request", "agentId": "other"}, true)
	args := map[string]any{"channel": "general", "text": "Reviewed", "clientRequestId": "agent-request"}
	call("message_send", args, false)
	call("message_send", args, false)
	call("messages_ack", map[string]any{"channel": "general", "through": incoming.ID}, false)
	call("messages_read", map[string]any{"channel": "general"}, false)
	messages, err := store.Messages("general", "", 50)
	if err != nil || len(messages) != 2 || messages[1].AgentID != "session:codex-one" || messages[1].AgentName != "Codex" {
		t.Fatalf("history: %+v %v", messages, err)
	}
	// The identical credential is rejected on a network request without local provenance.
	data, _ := json.Marshal(contract.MessagingAgentRequest{Credential: credential, Operation: "identity"})
	request := httptest.NewRequest("POST", "/api/messaging/agent", strings.NewReader(string(data)))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("network plugin route: %d", response.Code)
	}
}
