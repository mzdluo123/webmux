package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	codexhistory "github.com/rainchan/webmux/internal/agent/codex/history"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
)

func nativeFixture(store *conversation.Service) (*historyimport.Service, *historyprojection.Service) {
	sources := historyimport.New(historyimport.Options{Store: store,
		Readers:     map[string]historysource.ReadFunc{"codex": codexhistory.Read},
		Discoverers: map[string]historyimport.DiscoverFunc{"codex": codexhistory.Discover},
	})
	projection := historyprojection.New(historyprojection.Options{Source: sources, Preprocess: historyprojection.DefaultPreprocessConfig(), Render: snapcompact.DefaultRenderConfig()})
	return sources, projection
}

func TestNativeAuthenticatedWorkflow(t *testing.T) {
	handler, _, _ := historyAPI(t)
	call := func(body, token, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://webmux.example/api/conversations/native", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	if r := call(`{"action":"list","agent":"codex"}`, "", ""); r.Code != 401 {
		t.Fatal(r.Code)
	}
	if r := call(`{"action":"list","agent":"codex"}`, "secret", "https://evil.example"); r.Code != 403 {
		t.Fatal(r.Code)
	}
	if response := call(`{"action":"render","snapshotId":"missing"}`, "secret", ""); response.Code != 404 {
		t.Fatal(response.Code, response.Body.String())
	}
	if response := call(`{"action":"project","snapshotId":"missing","outputRunes":-1}`, "secret", ""); response.Code != 400 {
		t.Fatal(response.Code, response.Body.String())
	}
	body, err := os.ReadFile("../codexrollout/testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err = os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	freeze := contract.NativeRequest{Action: "freeze", Agent: "codex", NativeID: "00000000-0000-4000-8000-000000000001", Path: path, Owner: "explicit-reader"}
	request, err := json.Marshal(freeze)
	if err != nil {
		t.Fatal(err)
	}
	response := call(string(request), "secret", "")
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var frozen contract.NativeResult
	if err = json.Unmarshal(response.Body.Bytes(), &frozen); err != nil || frozen.Retained || frozen.RecordCount != 6 {
		t.Fatal(err, response.Body.String())
	}
	freeze.Retain, freeze.Owner = true, "independent-fork"
	retainRequest, err := json.Marshal(freeze)
	if err != nil {
		t.Fatal(err)
	}
	response = call(string(retainRequest), "secret", "")
	var retained contract.NativeResult
	if err = json.Unmarshal(response.Body.Bytes(), &retained); err != nil || !retained.Retained || response.Code != 200 {
		t.Fatal(err, response.Code, response.Body.String())
	}
	response = call(`{"action":"read","snapshotId":"`+frozen.SnapshotID+`"}`, "secret", "")
	var page contract.NativeResult
	if err = json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Records) != 6 || page.Records[0].Hash == "" {
		t.Fatal(err, response.Body.String())
	}
	response = call(`{"action":"project","snapshotId":"`+frozen.SnapshotID+`","currentTask":"任务"}`, "secret", "")
	var preview contract.NativePreview
	if err = json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.CurrentTask != "任务" || len(preview.Segments) != 5 || preview.Problem != "" {
		t.Fatal(preview)
	}
	corrupt := append(append([]byte{}, body...), []byte("SYNTHETIC_SECRET_NOT_JSON\n")...)
	if err = os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	response = call(string(request), "secret", "")
	if response.Code != 400 || !strings.Contains(response.Body.String(), "byte "+strconv.Itoa(len(body))) || strings.Contains(response.Body.String(), "SYNTHETIC_SECRET") {
		t.Fatal(response.Code, response.Body.String())
	}
	if response := call(`{"action":"read","snapshotId":"`+frozen.SnapshotID+`"}`, "secret", ""); response.Code != 200 {
		t.Fatal("unfrozen corrupt suffix entered the reference", response.Code, response.Body.String())
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if response := call(`{"action":"read","snapshotId":"`+frozen.SnapshotID+`"}`, "secret", ""); response.Code != 503 {
		t.Fatal("reference snapshot disguised a missing source", response.Code)
	}
	if response := call(`{"action":"read","snapshotId":"`+retained.SnapshotID+`"}`, "secret", ""); response.Code != 200 {
		t.Fatal("retained export lost its independent bytes", response.Code, response.Body.String())
	}
}
