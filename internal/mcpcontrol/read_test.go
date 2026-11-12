package mcpcontrol

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

type testDialer struct{ address string }

func (d testDialer) DialContext(ctx context.Context) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "tcp", d.address)
}
func websocketClient(t *testing.T, handler http.HandlerFunc) Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	api := localclient.New(testDialer{strings.TrimPrefix(server.URL, "http://")})
	t.Cleanup(api.Close)
	return Client{API: api}
}
func TestTerminalReadPagesRawBytesWithoutInput(t *testing.T) {
	c := websocketClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") != "8" {
			t.Error("cursor not forwarded")
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"webmux"}})
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = conn.CloseNow() }()
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		pos := contract.StreamPosition{Generation: "0123456789abcdef", Mode: "reset", Start: 10, End: 16, Truncated: true}
		data, err := json.Marshal(pos)
		if err != nil {
			t.Error(err)
			return
		}
		for _, frame := range [][]byte{contract.Frame(contract.FramePosition, data), contract.Frame(contract.FrameSnapshot, []byte{'a', 0xff, 'b', 'c', 'd', 'e'})} {
			if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
				t.Error(err)
				return
			}
		}
		_, _, err = conn.Read(ctx)
		if err == nil {
			t.Error("read-only client sent terminal input")
		}
	})
	result, err := c.read(context.Background(), json.RawMessage(`{"sessionId":"s","view":"terminal","generation":"0123456789abcdef","offset":"8","maxBytes":2}`))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"data":"Yf8="`, `"reset":true`, `"nextOffset":"12"`, `"limited":true`, `"historyTruncated":true`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %s: %s", want, data)
		}
	}
}
func TestActivityReadFiltersPrivateItemsAndPreservesCursor(t *testing.T) {
	cid := strings.Repeat("a", 32)
	c := testClient(t, func(r *http.Request) string {
		if r.URL.Path == "/api/sessions/s" {
			return `{"conversationId":"` + cid + `"}`
		}
		if r.URL.Path != "/api/conversations/"+cid+"/activities" {
			t.Fatalf("wrong path %s", r.URL.Path)
		}
		if r.URL.Query().Get("afterPosition") != "12" {
			t.Error("lost durable cursor")
		}
		return `{"items":[{"exportable":false,"text":"private"},{"exportable":true,"text":"visible","position":"14"}],"nextPosition":"14","gaps":[],"historyOrigin":"bounded_import","reconcileRequired":true}`
	})
	result, err := c.read(context.Background(), json.RawMessage(`{"sessionId":"s","afterPosition":"12","limit":2}`))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private") || !strings.Contains(string(data), `"nextPosition":"14"`) || !strings.Contains(string(data), `"done":false`) {
		t.Fatalf("page: %s", data)
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
func TestTerminalPageResume(t *testing.T) {
	a := readArguments{Generation: "0123456789abcdef", MaxBytes: 2}
	p := contract.StreamPosition{Generation: a.Generation, Mode: "append", Start: 12, End: 16}
	result, err := terminalPage(a, 12, p, []byte("cdef"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"data":"Y2Q="`) || !strings.Contains(string(data), `"reset":false`) {
		t.Fatalf("resume: %s", data)
	}
}
