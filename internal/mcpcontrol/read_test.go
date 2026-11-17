package mcpcontrol

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestTerminalReadUsesBoundedRawEndpoint(t *testing.T) {
	c := testClient(t, func(r *http.Request) string {
		if r.Method != "GET" || r.URL.Path != "/api/sessions/s/output" || r.URL.Query().Get("maxBytes") != "2" || r.URL.Query().Get("offset") != "8" {
			t.Fatalf("unexpected raw read: %s %s", r.Method, r.URL)
		}
		return `{"generation":"0123456789abcdef","start":"10","end":"16","nextOffset":"12","encoding":"base64","data":"Yf8=","reset":true,"historyTruncated":true,"limited":true}`
	})
	result, err := c.read(context.Background(), json.RawMessage(`{"sessionId":"s","view":"terminal","generation":"0123456789abcdef","offset":"8","maxBytes":2}`))
	if err != nil {
		t.Fatal(err)
	}
	page := result.(contract.MCPTerminalPageResult)
	if page.Data != "Yf8=" || !page.Reset || page.NextOffset != "12" || !page.Limited || !page.HistoryTruncated {
		t.Fatalf("terminal page: %+v", page)
	}
}
func TestActivityReadFiltersPrivateItemsAndPreservesCursor(t *testing.T) {
	c := testClient(t, func(r *http.Request) string {
		if r.URL.Path != "/api/sessions/s/native-history" || r.URL.Query().Get("snapshotId") != "frozen-scope" || r.URL.Query().Get("afterPosition") != "12" {
			t.Fatalf("wrong native scope: %s", r.URL)
		}
		return `{"snapshotId":"frozen-scope","records":[{"text":"visible","position":"14"}],"nextPosition":"14","more":true}`
	})
	result, err := c.read(context.Background(), json.RawMessage(`{"sessionId":"s","snapshotId":"frozen-scope","afterPosition":"12","limit":2}`))
	if err != nil {
		t.Fatal(err)
	}
	page := result.(contract.MCPActivityPageResult)
	if len(page.Records) != 1 || page.Records[0].Text != "visible" || page.NextPosition != "14" || page.Done {
		t.Fatalf("page %+v", page)
	}
}

func TestReadRejectsInvalidCursorsBeforeAPI(t *testing.T) {
	c := testClient(t, func(*http.Request) string { t.Fatal("invalid cursor reached API"); return "" })
	for _, raw := range []string{`{"sessionId":"s","afterPosition":"01"}`, `{"sessionId":"s","afterPosition":"9223372036854775808"}`, `{"sessionId":"s","limit":100,"maxBytesPerItem":65536}`, `{"sessionId":"s","view":"terminal","generation":"bad","offset":"0"}`, `{"sessionId":"s","view":"terminal","offset":"0"}`} {
		if _, err := c.read(context.Background(), json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
