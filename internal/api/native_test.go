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

	"github.com/rainchan/webmux/internal/contract"
)

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
	if r := call(`{"action":"list"}`, "", ""); r.Code != 401 {
		t.Fatal(r.Code)
	}
	if r := call(`{"action":"list"}`, "secret", "https://evil.example"); r.Code != 403 {
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
	request, err := json.Marshal(contract.NativeRequest{Action: "import", Path: path})
	if err != nil {
		t.Fatal(err)
	}
	response := call(string(request), "secret", "")
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var imported contract.NativeResult
	if err = json.Unmarshal(response.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	response = call(`{"action":"freeze","sourceId":"`+imported.SourceID+`"}`, "secret", "")
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var frozen contract.NativeResult
	if err = json.Unmarshal(response.Body.Bytes(), &frozen); err != nil {
		t.Fatal(err)
	}
	response = call(`{"action":"read","snapshotId":"`+frozen.SnapshotID+`"}`, "secret", "")
	var page contract.NativeResult
	if err = json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Activities.Items) == 0 {
		t.Fatal(err, response.Body.String())
	}
	activity := page.Activities.Items[0]
	exact := historyRequest(handler, "/api/conversations/"+activity.ConversationID+"/activities/"+activity.ActivityID+"?maxBytes=4", "secret", "")
	if exact.Code != 200 {
		t.Fatal("native activity cannot be read by original history API", exact.Code, exact.Body.String())
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
	if response.Code != 400 || !strings.Contains(response.Body.String(), "byte "+strconv.Itoa(len(body))) || !strings.Contains(response.Body.String(), "invalid rollout record") || strings.Contains(response.Body.String(), "SYNTHETIC_SECRET") {
		t.Fatal(response.Code, response.Body.String())
	}

}
