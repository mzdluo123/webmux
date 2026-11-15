package threadread

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

func TestReaderRequiresFrozenHandoffForEveryBody(t *testing.T) {
	conversationID, handoffID, itemID := strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 64)
	source, position, returnedID := conversationID, "2", itemID
	bodyCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/api/handoffs/"+handoffID+"/selection" {
			// The record is intentionally absent from budget-selection metadata.
			_ = json.NewEncoder(w).Encode(contract.HandoffSelection{SourceConversationID: source, SourcePosition: "2", Items: []contract.HandoffSelectionItem{}})
			return
		}
		if req.URL.Query().Get("handoffId") != handoffID {
			t.Error("body read omitted trusted handoff scope")
			w.WriteHeader(400)
			return
		}
		bodyCalls++
		item := contract.ConversationActivity{Exportable: true, ActivityID: returnedID, ConversationID: source, Position: position, Text: "原文\n  {\"raw\":true}", NextTextOffset: "24"}
		if req.URL.Path == "/api/conversations/"+conversationID+"/activities/"+itemID {
			_ = json.NewEncoder(w).Encode(item)
			return
		}
		_ = json.NewEncoder(w).Encode(contract.ConversationActivities{Items: []contract.ConversationActivity{item}, NextPosition: position})
	}))
	defer server.Close()
	client, err := localclient.NewTCP(server.URL, "owner")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	reader := Reader{Client: client, ConversationID: conversationID, HandoffID: handoffID}
	raw := json.RawMessage(`{"view":"activity","itemId":"` + itemID + `"}`)
	result, err := reader.Read(context.Background(), raw)
	if err != nil || len(result.Items) != 1 || result.Items[0].Selected || result.Items[0].Text != "原文\n  {\"raw\":true}" {
		t.Fatalf("unselected frozen record: %+v %v", result, err)
	}
	if _, err := reader.Read(context.Background(), json.RawMessage(`{"view":"activity"}`)); err != nil {
		t.Fatal(err)
	}
	before := bodyCalls
	source = strings.Repeat("d", 32)
	if _, err := reader.Read(context.Background(), raw); err == nil || bodyCalls != before {
		t.Fatal("cross-conversation handoff reached body")
	}
	source, position = conversationID, "3"
	if _, err := reader.Read(context.Background(), raw); err == nil {
		t.Fatal("accepted record after frozen source point")
	}
	if _, err := reader.Read(context.Background(), json.RawMessage(`{"view":"activity"}`)); err == nil {
		t.Fatal("silently clipped unfrozen page")
	}
	position, returnedID = "2", strings.Repeat("e", 64)
	if _, err := reader.Read(context.Background(), raw); err == nil {
		t.Fatal("accepted different item from detail endpoint")
	}
	reader.HandoffID = ""
	if _, err := reader.Read(context.Background(), raw); err == nil {
		t.Fatal("accepted missing handoff")
	}
}

func TestReaderRejectsUntrustedPathIDs(t *testing.T) {
	for _, id := range []string{"../escape", strings.Repeat("a", 63), strings.Repeat("a", 65), strings.Repeat("A", 64)} {
		if _, err := parseArguments(json.RawMessage(`{"view":"activity","itemId":"` + id + `"}`)); err == nil {
			t.Errorf("accepted %q", id)
		}
	}
	for _, id := range []string{strings.Repeat("a", 32), strings.Repeat("a", 64)} {
		if _, err := parseArguments(json.RawMessage(`{"view":"activity","itemId":"` + id + `"}`)); err != nil {
			t.Errorf("rejected compatible ID: %v", err)
		}
	}
}
