package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestGitOperationsEndpointsAndIdempotencyAPI(t *testing.T) {
	env := newGitAPITestEnv(t)
	readBase := "/api/repositories/" + env.repo.ID + "/git"
	writeBase := "/api/repositories/" + env.repo.ID + "/worktrees/" + env.mainWT.ID + "/git"
	repoBase := "/api/repositories/" + env.repo.ID

	// 1. Auth (401) and Origin (403) checks on operations routes.
	for _, p := range []string{readBase + "/operations", readBase + "/operations/missing"} {
		r401 := env.doRaw("GET", p, "", nil, "")
		r401.Body.Close()
		if r401.StatusCode != 401 {
			t.Fatalf("GET %s without auth: got %d want 401", p, r401.StatusCode)
		}
		r403 := env.doRaw("GET", p, "", map[string]string{
			"Authorization": "Bearer secret",
			"Origin":        "https://evil.example",
		}, "")
		r403.Body.Close()
		if r403.StatusCode != 403 {
			t.Fatalf("GET %s bad origin: got %d want 403", p, r403.StatusCode)
		}
	}

	// 2. 404 on unknown repository or unknown operationId; invalid limit -> 400.
	env.doJSON("GET", "/api/repositories/missing/git/operations", nil, 404)
	env.doJSON("GET", readBase+"/operations/missing", nil, 404)
	env.doJSON("GET", readBase+"/operations?limit=abc", nil, 400)

	// 3. Commit with clientRequestId returns operationId, replays idempotently, and rejects conflict with 409 idempotency_conflict.
	if err := os.WriteFile(filepath.Join(env.root, "op.txt"), []byte("op\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "op.txt")

	var preview contract.GitCommitPreview
	if err := json.Unmarshal(env.doJSON("POST", writeBase+"/commit-preview", contract.GitCommitPreviewRequest{}, 200), &preview); err != nil {
		t.Fatal(err)
	}

	commitReq := contract.GitCommitRequest{
		ConfirmationID:  preview.ConfirmationID,
		ClientRequestID: "api-commit-req-1",
		Message:         "feat: api durable commit",
	}
	var commitRes1, commitRes2 contract.GitCommitResult
	if err := json.Unmarshal(env.doJSON("POST", writeBase+"/commit", commitReq, 200), &commitRes1); err != nil {
		t.Fatal(err)
	}
	if commitRes1.OperationID == "" || commitRes1.Outcome != "matched" {
		t.Fatalf("unexpected commit result 1: %+v", commitRes1)
	}
	// Duplicate request replays exact same operationId & commitOid!
	if err := json.Unmarshal(env.doJSON("POST", writeBase+"/commit", commitReq, 200), &commitRes2); err != nil {
		t.Fatal(err)
	}
	if commitRes2.OperationID != commitRes1.OperationID || commitRes2.CommitOID != commitRes1.CommitOID {
		t.Fatalf("idempotent commit replay mismatch: %+v vs %+v", commitRes1, commitRes2)
	}
	if count := strings.TrimSpace(env.gitOut(env.root, "rev-list", "--count", "HEAD")); count != "2" {
		t.Fatalf("expected strictly 1 new commit (total 2), got %s", count)
	}

	// Reusing clientRequestId with different message -> 409 idempotency_conflict.
	var conflict contract.GitConflict
	if err := json.Unmarshal(env.doJSON("POST", writeBase+"/commit", contract.GitCommitRequest{
		ConfirmationID:  preview.ConfirmationID,
		ClientRequestID: "api-commit-req-1",
		Message:         "feat: modified message",
	}, 409), &conflict); err != nil {
		t.Fatal(err)
	}
	if conflict.Reason != "idempotency_conflict" {
		t.Fatalf("expected reason=idempotency_conflict, got %+v", conflict)
	}

	// 4. Switch & Fetch return operationId and appear in GET /operations list & detail.
	env.gitOut(env.root, "branch", "switch-target")
	var swPrev contract.SwitchPreview
	if err := json.Unmarshal(env.doJSON("POST", repoBase+"/worktrees/"+env.mainWT.ID+"/switch-preview", contract.SwitchPreviewRequest{
		Target: contract.SwitchTarget{Kind: "local", Branch: "switch-target"},
	}, 200), &swPrev); err != nil {
		t.Fatal(err)
	}
	var swRes contract.SwitchResult
	if err := json.Unmarshal(env.doJSON("POST", repoBase+"/worktrees/"+env.mainWT.ID+"/switch", contract.SwitchRequest{
		ConfirmationID:  swPrev.ConfirmationID,
		ClientRequestID: "api-switch-req-1",
	}, 200), &swRes); err != nil {
		t.Fatal(err)
	}
	if swRes.OperationID == "" || swRes.Branch == nil || *swRes.Branch != "switch-target" {
		t.Fatalf("unexpected switch result: %+v", swRes)
	}

	// Reusing switch clientRequestId on fetch -> 409 idempotency_conflict via workspaceFailure!
	var wsConflict contract.GitConflict
	if err := json.Unmarshal(env.doJSON("POST", repoBase+"/fetch", contract.FetchRemotesRequest{
		ClientRequestID: "api-switch-req-1",
	}, 409), &wsConflict); err != nil {
		t.Fatal(err)
	}
	if wsConflict.Reason != "idempotency_conflict" {
		t.Fatalf("expected workspaceFailure 409 idempotency_conflict, got %+v", wsConflict)
	}

	// Verify GET /git/operations and GET /git/operations/{operationId}.
	var list contract.GitOperations
	if err := json.Unmarshal(env.doJSON("GET", readBase+"/operations?worktreeId="+env.mainWT.ID+"&limit=20", nil, 200), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Operations) != 2 || list.Operations[0].ID != swRes.OperationID || list.Operations[1].ID != commitRes1.OperationID {
		t.Fatalf("unexpected operations list: %+v", list)
	}

	var single contract.GitOperation
	if err := json.Unmarshal(env.doJSON("GET", readBase+"/operations/"+commitRes1.OperationID, nil, 200), &single); err != nil {
		t.Fatal(err)
	}
	if single.ID != commitRes1.OperationID || single.Status != "succeeded" || single.ActualOID != commitRes1.CommitOID || single.ClientRequestID != "api-commit-req-1" {
		t.Fatalf("unexpected single operation: %+v", single)
	}
}
