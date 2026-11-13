package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestGitCollabDraftAndReviewsAPI(t *testing.T) {
	env := newGitAPITestEnv(t)
	writeBase := "/api/repositories/" + env.repo.ID + "/worktrees/" + env.mainWT.ID + "/git"

	// 1. Auth (401), Origin (403), and Cache-Control: no-store on collab endpoints.
	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", writeBase + "/commit-draft", ""},
		{"PUT", writeBase + "/commit-draft", `{"subject":"s","body":"","baseRevision":0}`},
		{"GET", writeBase + "/reviews?scope=uncommitted", ""},
		{"PUT", writeBase + "/reviews", `{"scope":"uncommitted","action":"clear"}`},
	}
	for _, ep := range endpoints {
		r401 := env.doRaw(ep.method, ep.path, ep.body, nil, "")
		r401.Body.Close()
		if r401.StatusCode != 401 {
			t.Fatalf("%s %s without auth: got %d want 401", ep.method, ep.path, r401.StatusCode)
		}
		r403 := env.doRaw(ep.method, ep.path, ep.body, map[string]string{
			"Authorization": "Bearer secret",
			"Origin":        "https://evil.example",
		}, "")
		r403.Body.Close()
		if r403.StatusCode != 403 {
			t.Fatalf("%s %s bad origin: got %d want 403", ep.method, ep.path, r403.StatusCode)
		}
	}

	rawCheck := env.doRaw("GET", writeBase+"/commit-draft", "", map[string]string{
		"Authorization": "Bearer secret",
	}, "")
	rawCheck.Body.Close()
	if cc := rawCheck.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}

	// 2. Commit draft GET -> PUT -> 409 draft_conflict with CurrentDraft -> resolve.
	var emptyDraft contract.GitCommitDraft
	if err := json.Unmarshal(env.doJSON("GET", writeBase+"/commit-draft", nil, 200), &emptyDraft); err != nil {
		t.Fatal(err)
	}
	if emptyDraft.Revision != 0 || emptyDraft.Subject != "" {
		t.Fatalf("expected empty draft rev 0, got %+v", emptyDraft)
	}

	var savedDraft contract.GitCommitDraft
	if err := json.Unmarshal(env.doJSON("PUT", writeBase+"/commit-draft", contract.GitPutCommitDraftRequest{
		Subject:      "feat(git): sync draft",
		Body:         "device A body",
		BaseRevision: 0,
	}, 200), &savedDraft); err != nil {
		t.Fatal(err)
	}
	if savedDraft.Revision != 1 || savedDraft.Subject != "feat(git): sync draft" {
		t.Fatalf("unexpected saved draft: %+v", savedDraft)
	}

	var conflict contract.GitConflict
	if err := json.Unmarshal(env.doJSON("PUT", writeBase+"/commit-draft", contract.GitPutCommitDraftRequest{
		Subject:      "feat(git): device B conflicting draft",
		Body:         "device B body",
		BaseRevision: 0,
	}, 409), &conflict); err != nil {
		t.Fatal(err)
	}
	if conflict.Reason != "draft_conflict" || conflict.CurrentDraft == nil || conflict.CurrentDraft.Revision != 1 || conflict.CurrentDraft.Subject != "feat(git): sync draft" {
		t.Fatalf("expected 409 draft_conflict with CurrentDraft rev 1, got %+v", conflict)
	}

	// 3. Reviews GET -> PUT mark -> external file edit invalidates -> stale PUT rejected.
	targetFile := filepath.Join(env.root, "review-me.txt")
	if err := os.WriteFile(targetFile, []byte("hello v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var initialReviews contract.GitReviewsResponse
	if err := json.Unmarshal(env.doJSON("GET", writeBase+"/reviews?scope=uncommitted", nil, 200), &initialReviews); err != nil {
		t.Fatal(err)
	}
	var item contract.GitReviewFileState
	found := false
	for _, f := range initialReviews.Files {
		if f.Path == "review-me.txt" {
			item = f
			found = true
			break
		}
	}
	if !found || item.Reviewed {
		t.Fatalf("expected unreviewed review-me.txt in %+v", initialReviews)
	}

	var markedReviews contract.GitReviewsResponse
	if err := json.Unmarshal(env.doJSON("PUT", writeBase+"/reviews", contract.GitPutReviewsRequest{
		Scope:  "uncommitted",
		Action: "mark",
		Item: &contract.GitReviewMarkIdentity{
			Scope:          item.Scope,
			Area:           item.Area,
			Path:           item.Path,
			BaseMode:       item.BaseMode,
			BaseIdentity:   item.BaseIdentity,
			TargetMode:     item.TargetMode,
			TargetIdentity: item.TargetIdentity,
		},
	}, 200), &markedReviews); err != nil {
		t.Fatal(err)
	}
	if !markedReviews.Applied || markedReviews.ReviewedFiles != 1 {
		t.Fatalf("expected mark applied, got %+v", markedReviews)
	}

	// External edit invalidates mark and causes stale offline mark to be rejected.
	if err := os.WriteFile(targetFile, []byte("hello v2 modified\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var staleRes contract.GitReviewsResponse
	if err := json.Unmarshal(env.doJSON("PUT", writeBase+"/reviews", contract.GitPutReviewsRequest{
		Scope:  "uncommitted",
		Action: "mark",
		Item: &contract.GitReviewMarkIdentity{
			Scope:          item.Scope,
			Area:           item.Area,
			Path:           item.Path,
			BaseMode:       item.BaseMode,
			BaseIdentity:   item.BaseIdentity,
			TargetMode:     item.TargetMode,
			TargetIdentity: item.TargetIdentity,
		},
	}, 200), &staleRes); err != nil {
		t.Fatal(err)
	}
	if staleRes.Applied || !staleRes.StaleRejected || staleRes.ReviewedFiles != 0 {
		t.Fatalf("expected stale mark rejected with 0 reviewed files, got %+v", staleRes)
	}
}
