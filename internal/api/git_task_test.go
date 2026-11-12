package api

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestGitTaskBaselineAndTaskDiffAPI(t *testing.T) {
	env := newGitAPITestEnv(t)
	readBase := "/api/repositories/" + env.repo.ID + "/git"
	writeBase := "/api/repositories/" + env.repo.ID + "/worktrees/" + env.mainWT.ID + "/git"

	// 1. Auth (401), Origin (403), and Cache-Control: no-store on task endpoints.
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", writeBase + "/task-baseline"},
		{"PUT", writeBase + "/task-baseline"},
		{"DELETE", writeBase + "/task-baseline"},
		{"GET", writeBase + "/task-changes"},
		{"GET", writeBase + "/task-diff?path=README.md"},
	}
	for _, ep := range endpoints {
		r401 := env.doRaw(ep.method, ep.path, "", nil, "")
		r401.Body.Close()
		if r401.StatusCode != 401 {
			t.Fatalf("%s %s without auth: got %d want 401", ep.method, ep.path, r401.StatusCode)
		}
		r403 := env.doRaw(ep.method, ep.path, "", map[string]string{
			"Authorization": "Bearer secret",
			"Origin":        "https://evil.example",
		}, "")
		r403.Body.Close()
		if r403.StatusCode != 403 {
			t.Fatalf("%s %s bad origin: got %d want 403", ep.method, ep.path, r403.StatusCode)
		}
	}

	rawCheck := env.doRaw("GET", writeBase+"/task-baseline", "", map[string]string{
		"Authorization": "Bearer secret",
	}, "")
	rawCheck.Body.Close()
	if cc := rawCheck.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}

	// 2. 404 on unknown repo/worktree; 409 no_task_baseline on task-diff before baseline.
	env.doJSON("GET", "/api/repositories/missing/worktrees/"+env.mainWT.ID+"/git/task-baseline", nil, 404)
	env.doJSON("GET", "/api/repositories/"+env.repo.ID+"/worktrees/missing/git/task-baseline", nil, 404)

	var conflict contract.GitConflict
	if err := json.Unmarshal(env.doJSON("GET", writeBase+"/task-diff?path=README.md", nil, 409), &conflict); err != nil {
		t.Fatal(err)
	}
	if conflict.Reason != "no_task_baseline" {
		t.Fatalf("expected 409 reason=no_task_baseline, got %+v", conflict)
	}

	// 3. Record initial commit as baseline, make 2 commits + 1 additional edit on same file, verify net diff & scope=task history.
	baseOID := strings.TrimSpace(env.gitOut(env.root, "rev-parse", "HEAD"))
	var putRes contract.GitTaskBaselineResponse
	if err := json.Unmarshal(env.doJSON("PUT", writeBase+"/task-baseline", contract.GitPutTaskBaselineRequest{
		BaseOID: baseOID,
	}, 200), &putRes); err != nil {
		t.Fatal(err)
	}
	if putRes.Baseline == nil || putRes.Baseline.BaseOID != baseOID || !putRes.Baseline.CleanAtStart {
		t.Fatalf("unexpected PUT task-baseline response: %+v", putRes)
	}

	fileA := filepath.Join(env.root, "feature.go")
	if err := os.WriteFile(fileA, []byte("package main\nfunc step1() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "feature.go")
	env.gitOut(env.root, "commit", "-m", "feat: step 1")

	fileB := filepath.Join(env.root, "notes.md")
	if err := os.WriteFile(fileB, []byte("# Notes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "notes.md")
	env.gitOut(env.root, "commit", "-m", "docs: add notes")

	// Modify feature.go again in working tree (anti-double-count check).
	if err := os.WriteFile(fileA, []byte("package main\nfunc step2() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var taskChanges contract.GitTaskChanges
	if err := json.Unmarshal(env.doJSON("GET", writeBase+"/task-changes", nil, 200), &taskChanges); err != nil {
		t.Fatal(err)
	}
	if taskChanges.Status != "ready" || taskChanges.CommitCount != 2 || taskChanges.UncommittedCount != 1 || taskChanges.Changes.Total != 2 {
		t.Fatalf("unexpected task-changes: %+v", taskChanges)
	}

	var diff contract.GitDiff
	if err := json.Unmarshal(env.doJSON("GET", writeBase+"/task-diff?"+url.Values{"path": {"feature.go"}}.Encode(), nil, 200), &diff); err != nil {
		t.Fatal(err)
	}
	if diff.Old.Source != "empty" || diff.New.Text != "package main\nfunc step2() {}\n" || diff.Snapshot == "" {
		t.Fatalf("unexpected task-diff: %+v", diff)
	}

	var snapFile contract.GitFile
	if err := json.Unmarshal(env.doJSON("GET", readBase+"/snapshots/"+diff.Snapshot+"/file?side=after", nil, 200), &snapFile); err != nil {
		t.Fatal(err)
	}
	if snapFile.Document.Text != "package main\nfunc step2() {}\n" {
		t.Fatalf("unexpected snapshot file: %+v", snapFile)
	}

	var histPage contract.GitHistoryPage
	if err := json.Unmarshal(env.doJSON("GET", readBase+"/history?worktreeId="+env.mainWT.ID+"&scope=task", nil, 200), &histPage); err != nil {
		t.Fatal(err)
	}
	if histPage.Status != "ready" || histPage.Scope != "task" || len(histPage.Rows) != 2 || histPage.TaskBaseline == nil || histPage.TaskBaseline.BaseOID != baseOID {
		t.Fatalf("unexpected scope=task history page: %+v", histPage)
	}

	// 4. DELETE task-baseline clears baseline and returns scope=task to no_task_baseline.
	env.doJSON("DELETE", writeBase+"/task-baseline", nil, 204)
	var afterDel contract.GitTaskBaselineResponse
	if err := json.Unmarshal(env.doJSON("GET", writeBase+"/task-baseline", nil, 200), &afterDel); err != nil {
		t.Fatal(err)
	}
	if afterDel.Baseline != nil {
		t.Fatalf("expected nil baseline after DELETE, got %+v", afterDel)
	}
}
