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

func TestGitDivergenceAndConflictResolverAPI(t *testing.T) {
	env := newGitAPITestEnv(t)
	wtURL := "/api/repositories/" + env.repo.ID + "/worktrees/" + env.mainWT.ID + "/git"

	// 1. Auth (401) and Origin (403) boundaries on every new V2 endpoint.
	for _, call := range [][2]string{
		{"GET", wtURL + "/operation-state"},
		{"GET", wtURL + "/conflict?path=a.txt"},
		{"POST", wtURL + "/conflict/save"},
		{"POST", wtURL + "/conflict/resolve"},
		{"POST", wtURL + "/merge-preview"},
		{"POST", wtURL + "/merge"},
		{"POST", wtURL + "/rebase-preview"},
		{"POST", wtURL + "/rebase"},
		{"POST", wtURL + "/cherry-pick-preview"},
		{"POST", wtURL + "/cherry-pick"},
		{"POST", wtURL + "/revert-preview"},
		{"POST", wtURL + "/revert"},
		{"POST", wtURL + "/operation-continue"},
		{"POST", wtURL + "/operation-abort-preview"},
		{"POST", wtURL + "/operation-abort"},
	} {
		r401 := env.doRaw(call[0], call[1], "{}", nil, "")
		r401.Body.Close()
		if r401.StatusCode != 401 {
			t.Fatalf("%s %s without token: got %d want 401", call[0], call[1], r401.StatusCode)
		}
		for _, badOrigin := range []string{"https://evil.example", "null"} {
			r403 := env.doRaw(call[0], call[1], "{}", map[string]string{
				"Authorization": "Bearer secret",
				"Origin":        badOrigin,
			}, "")
			r403.Body.Close()
			if r403.StatusCode != 403 {
				t.Fatalf("%s %s origin %q: got %d want 403", call[0], call[1], badOrigin, r403.StatusCode)
			}
		}
	}

	// 2. Prepare clean base + diverging branch on disk.
	mainBranch := strings.TrimSpace(env.gitOut(env.root, "symbolic-ref", "--short", "HEAD"))

	conflictFile := filepath.Join(env.root, "service.go")
	if err := os.WriteFile(conflictFile, []byte("package svc\n\nconst Version = \"1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "service.go")
	env.gitOut(env.root, "commit", "-m", "feat: add service.go")
	baseOID := strings.TrimSpace(env.gitOut(env.root, "rev-parse", "HEAD"))

	env.gitOut(env.root, "checkout", "-b", "feat/v2", baseOID)
	if err := os.WriteFile(conflictFile, []byte("package svc\n\nconst Version = \"2.0-feat\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "service.go")
	env.gitOut(env.root, "commit", "-m", "feat: bump to 2.0-feat")

	env.gitOut(env.root, "checkout", mainBranch)
	if err := os.WriteFile(conflictFile, []byte("package svc\n\nconst Version = \"1.1-main\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "service.go")
	env.gitOut(env.root, "commit", "-m", "feat: bump to 1.1-main")

	// 3. Rebase main onto feat/v2 -> conflict -> re-entry operation-state -> Save -> Resolve -> Continue!
	var rbPrev contract.GitDivergencePreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/rebase-preview", contract.GitBranchDivergencePreviewRequest{
		RefKind: "local",
		RefName: "feat/v2",
	}, 200), &rbPrev); err != nil {
		t.Fatal(err)
	}
	if rbPrev.ConfirmationID == "" || len(rbPrev.Blockers) != 0 {
		t.Fatalf("expected unblocked rebase preview, got %+v", rbPrev)
	}

	var rbRes contract.GitDivergenceResult
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/rebase", contract.GitDivergenceExecuteRequest{
		ConfirmationID:  rbPrev.ConfirmationID,
		ClientRequestID: "api-rebase-1",
	}, 200), &rbRes); err != nil {
		t.Fatal(err)
	}
	if rbRes.Outcome != "conflicted" || !rbRes.InProgress.Active || rbRes.InProgress.Kind != "rebase" {
		t.Fatalf("expected rebase conflict, got %+v", rbRes)
	}

	// Continue before resolution -> 409 reason="conflicted"
	var contConflict contract.GitConflict
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/operation-continue", contract.GitContinueOperationRequest{
		Kind: "rebase",
	}, 409), &contConflict); err != nil || contConflict.Reason != "conflicted" {
		t.Fatalf("expected 409 conflicted on premature continue, got %+v (%v)", contConflict, err)
	}

	// Inspect conflict -> Left must be main (stage 3), Right must be feat/v2 (stage 2)
	var detail contract.GitConflictFileDetail
	if err := json.Unmarshal(env.doJSON("GET", wtURL+"/conflict?path="+url.QueryEscape("service.go"), nil, 200), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Left.Stage != 3 || detail.Right.Stage != 2 || !strings.Contains(detail.Left.Text, "1.1-main") || !strings.Contains(detail.Right.Text, "2.0-feat") {
		t.Fatalf("rebase side inversion detected: left=%+v right=%+v", detail.Left, detail.Right)
	}

	resolvedBody := "package svc\n\nconst Version = \"2.1-merged\"\n"
	var saved contract.GitSaveConflictResponse
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/conflict/save", contract.GitSaveConflictRequest{
		Identity:   detail.Identity,
		ResultText: resolvedBody,
	}, 200), &saved); err != nil {
		t.Fatal(err)
	}

	var resolved contract.GitResolveConflictResponse
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/conflict/resolve", contract.GitResolveConflictRequest{
		Identity:   saved.Identity,
		Resolution: "text",
		ResultText: resolvedBody,
	}, 200), &resolved); err != nil {
		t.Fatal(err)
	}
	if !resolved.InProgress.CanContinue || resolved.InProgress.ConflictedCount != 0 {
		t.Fatalf("expected CanContinue=true after resolving sole conflict, got %+v", resolved.InProgress)
	}

	var doneRes contract.GitDivergenceResult
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/operation-continue", contract.GitContinueOperationRequest{
		Kind:            "rebase",
		ClientRequestID: "api-rebase-continue-1",
	}, 200), &doneRes); err != nil {
		t.Fatal(err)
	}
	if doneRes.Outcome != "completed" || doneRes.InProgress.Active {
		t.Fatalf("expected completed rebase after continue, got %+v", doneRes)
	}
}
