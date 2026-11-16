package threadread

import (
	"context"
	"encoding/json"
	"github.com/rainchan/webmux/internal/localclient"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReaderPreservesFrozenScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sessions/runtime-1/handoff-history" || r.URL.Query().Get("snapshotId") != "instance_scope1" || r.URL.Query().Get("afterPosition") != "2" || r.URL.Query().Get("textOffset") != "3" {
			t.Errorf("request: %s", r.URL)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"snapshotId": "instance_scope1", "records": []any{}, "more": false})
	}))
	defer server.Close()
	client, err := localclient.NewTCP(server.URL, "test-owner-token")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	reader := Reader{Client: client, SessionID: "runtime-1"}
	result, err := reader.Read(context.Background(), json.RawMessage(`{"view":"activity","snapshotId":"instance_scope1","afterPosition":"2","textOffset":3}`))
	if err != nil || !result.Done || result.SnapshotID != "instance_scope1" {
		t.Fatalf("result %+v %v", result, err)
	}
}
