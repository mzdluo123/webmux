package featureapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/localclient"
	"github.com/rainchan/webmux/internal/threadread"
)

func frozenConsumerReader(ctx context.Context, req historysource.Request) (historysource.Document, error) {
	var records []historysource.Record
	doc, err := historysource.ReadJSONL(ctx, req, func(line []byte, offset, end int64) error {
		var text string
		if json.Unmarshal(line, &text) != nil {
			return historysource.ErrFormat
		}
		records = append(records, historysource.Record{ID: strconv.FormatInt(offset, 10), Kind: "tool_arguments", Role: "agent", Text: text, Offset: offset, End: end, CallID: "call", ParentID: "parent", ToolName: "shell"})
		return nil
	})
	doc.FormatVersion, doc.ParserVersion, doc.Records = "consumer/v1", "consumer/v1", records
	return doc, err
}

func TestFrozenConversationHTTPConsumers(t *testing.T) {
	ctx := context.Background()
	store := conversation.Open(conversation.Options{StateDir: t.TempDir()})
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Unavailable(); err != nil {
		t.Fatal(err)
	}
	svc := historyimport.New(historyimport.Options{Store: store, Readers: map[string]historysource.ReadFunc{"consumer": frozenConsumerReader}})
	server := &Server{Conversations: store, NativeHistory: svc}
	mux := http.NewServeMux()
	server.conversationRoutes(mux)
	server.handoffRoutes(mux)
	text := strings.Repeat("甲🙂\n  {\"raw\": true}\n", 300)
	body, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	initial := append(body, '\n')
	path := filepath.Join(t.TempDir(), "native.jsonl")
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatal(err)
	}
	binding, err := store.EnsureRuntime(ctx, conversation.RuntimeInput{RuntimeID: "source", Cwd: t.TempDir(), AgentName: "consumer", AgentSessionID: "native", HistoryOrigin: "complete", Status: "closed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Bind(ctx, binding.ProviderThreadID, historysource.Source{Agent: "consumer", Path: path, NativeID: "native"}); err != nil {
		t.Fatal(err)
	}
	base := "/api/conversations/" + binding.ConversationID + "/activities"
	get := func(path string, want int, out any) {
		t.Helper()
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != want {
			t.Fatalf("GET %s: %d %s", path, response.Code, response.Body.String())
		}
		if out != nil {
			if err := json.Unmarshal(response.Body.Bytes(), out); err != nil {
				t.Fatal(err)
			}
		}
	}
	var first contract.ConversationActivities
	get(base+"?maxBytesPerItem=4", 200, &first)
	if len(first.Items) != 1 || len(first.SnapshotIDs) != 1 {
		t.Fatalf("first freeze: %+v", first)
	}
	item := first.Items[0]
	if len(item.ActivityID) != 64 || item.SourceHash == "" || item.ParentID == "" || item.CallID == "" || item.ProviderThreadID != "" || item.RunID != "" {
		t.Fatalf("native provenance: %+v", item)
	}
	snapshot, err := svc.Snapshot(ctx, first.SnapshotIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	preview, err := store.ReservePreview(ctx, conversation.PreviewReservation{SourceConversationID: binding.ConversationID, ClientRequestID: "first-preview", RequestHash: "first", Request: conversation.PreviewRequest{ConfigID: "target", AgentName: "consumer"}, Snapshots: []historysource.Snapshot{snapshot}})
	if err != nil {
		t.Fatal(err)
	}
	get(base+"?handoffId="+preview.ID, 409, nil)
	preview, err = store.CompletePreview(ctx, preview.ID, conversation.PreviewResult{OmittedItemIDs: []string{item.ActivityID}, Budget: conversation.PreviewBudget{History: 16000, Source: "estimated"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(append([]byte{}, initial...), []byte("\"later\"\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	scope := "snapshotIds=" + url.QueryEscape(strings.Join(first.SnapshotIDs, ","))
	var frozen contract.ConversationActivities
	get(base+"?"+scope, 200, &frozen)
	if len(frozen.Items) != 1 || frozen.Items[0].ActivityID != item.ActivityID {
		t.Fatalf("frozen range expanded: %+v", frozen)
	}
	var rebuilt strings.Builder
	offset := "0"
	for {
		var part contract.ConversationActivity
		get(base+"/"+item.ActivityID+"?"+scope+"&maxBytes=31&textOffset="+offset, 200, &part)
		if !utf8.ValidString(part.Text) || len(part.Text) > 31 || part.ConversationID != binding.ConversationID || part.Position != "1" {
			t.Fatalf("invalid byte page: %+v", part)
		}
		rebuilt.WriteString(part.Text)
		if !part.Truncated {
			break
		}
		if part.NextTextOffset == offset {
			t.Fatal("stalled byte cursor")
		}
		offset = part.NextTextOffset
	}
	if rebuilt.String() != text {
		t.Fatal("raw Unicode/tool arguments/newlines changed")
	}
	get(base+"/"+item.ActivityID+"?"+scope+"&textOffset=1", 400, nil)
	other, err := store.EnsureRuntime(ctx, conversation.RuntimeInput{RuntimeID: "other", Cwd: t.TempDir(), AgentName: "consumer", AgentSessionID: "other-native", HistoryOrigin: "complete", Status: "closed"})
	if err != nil {
		t.Fatal(err)
	}
	get("/api/conversations/"+other.ConversationID+"/activities?"+scope, 409, nil)
	var live contract.ConversationActivities
	get(base, 200, &live)
	if len(live.Items) != 2 {
		t.Fatalf("new freeze missed growth: %+v", live)
	}
	get(base+"/"+live.Items[1].ActivityID+"?"+scope, 404, nil)
	var handoffPage contract.ConversationActivities
	get(base+"?handoffId="+preview.ID, 200, &handoffPage)
	if len(handoffPage.Items) != 1 {
		t.Fatal("handoff range expanded with native source")
	}
	get(base+"/"+live.Items[1].ActivityID+"?handoffId="+preview.ID, 404, nil)
	get("/api/conversations/"+other.ConversationID+"/activities?handoffId="+preview.ID, 409, nil)
	get(base+"?handoffId="+preview.ID+"&"+scope, 400, nil)
	laterSnapshots, err := svc.FreezeConversation(ctx, binding.ConversationID, binding.ConversationID, false)
	if err != nil {
		t.Fatal(err)
	}
	laterPreview, err := store.ReservePreview(ctx, conversation.PreviewReservation{SourceConversationID: binding.ConversationID, ClientRequestID: "later-preview", RequestHash: "later", Request: conversation.PreviewRequest{ConfigID: "target", AgentName: "consumer"}, Snapshots: laterSnapshots})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CompletePreview(ctx, laterPreview.ID, conversation.PreviewResult{OmittedItemIDs: []string{live.Items[0].ActivityID, live.Items[1].ActivityID}, Budget: conversation.PreviewBudget{History: 16000, Source: "estimated"}})
	if err != nil {
		t.Fatal(err)
	}
	get(base+"/"+live.Items[1].ActivityID+"?handoffId="+laterPreview.ID, 200, nil)
	get(base+"/"+live.Items[1].ActivityID+"?handoffId="+preview.ID, 404, nil)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()
	client, err := localclient.NewTCP(httpServer.URL, "owner")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	reader := threadread.Reader{Client: client, ConversationID: binding.ConversationID, HandoffID: preview.ID}
	result, err := reader.Read(ctx, json.RawMessage(`{"view":"activity","maxBytesPerItem":4}`))
	if err != nil || len(result.Items) != 1 || result.SourcePosition != "1" || !result.Items[0].Omitted || result.Items[0].ActivityID != item.ActivityID {
		t.Fatalf("MCP frozen omitted source: %+v %v", result, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	get(base+"?"+scope, 503, nil)
}
