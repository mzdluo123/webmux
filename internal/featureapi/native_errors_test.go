package featureapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	codex "github.com/rainchan/webmux/internal/agent/codex/history"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
)

func TestNativeTargetVersionHTTPError(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	if err := os.Mkdir(sessions, 0700); err != nil {
		t.Fatal(err)
	}
	// Valid content identity with a writer version outside the supported set.
	data := []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"native\",\"cli_version\":\"0.999.0\",\"cwd\":\"/synthetic\",\"originator\":\"webmux\",\"source\":\"vscode\"}}\n")
	if err := os.WriteFile(filepath.Join(sessions, "source.jsonl"), data, 0600); err != nil {
		t.Fatal(err)
	}
	svc := historyimport.New(historyimport.Options{
		Readers:     map[string]historysource.ReadFunc{"codex": codex.Read},
		Discoverers: map[string]historyimport.DiscoverFunc{"codex": codex.Discover},
		Resolve: func(context.Context, string) (historyimport.Identity, error) {
			return historyimport.Identity{Agent: "codex", NativeID: "native", Root: root}, nil
		},
	})
	_, err := svc.FreezeRuntime(context.Background(), "runtime")
	if err == nil {
		t.Fatal("accepted unsupported target")
	}
	response := httptest.NewRecorder()
	historyFailure(response, err)
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || !strings.Contains(body.Error, "version") || strings.Contains(body.Error, "identity mismatch") {
		t.Fatalf("source error masked: HTTP %d %s", response.Code, body.Error)
	}
}
