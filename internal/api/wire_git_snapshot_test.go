package api

import (
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"

	"github.com/rainchan/webmux/internal/contract"
)

// wireGitWorkbench pins the repository/worktree Git read-only, staging,
// commit-preview, commit execution and structured 409 conflict JSON contracts
// with zero active terminal or hybrid sessions.
func wireGitWorkbench(f *wireFixture) {
	repository := f.call("repository-register-git", "POST", "/api/repositories", map[string]string{"path": f.root}, 201)
	repoID := str(repository, "id")
	worktrees := field(repository, "worktrees").([]any)
	wtID := str(worktrees[0], "id")

	firstOID := f.gitOutput("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(f.root, "sub", "b.txt"), []byte("bee\nbutterfly\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	f.git("add", "sub/b.txt")
	f.git("commit", "-m", "feat: expand b.txt\n\ncommit body line")
	secondOID := f.gitOutput("rev-parse", "HEAD")

	readBase := "/api/repositories/" + repoID + "/git"
	writeBase := "/api/repositories/" + repoID + "/worktrees/" + wtID + "/git"

	// 1. Repository-only context (readonly for worktree browse, allowed for compare & commit:{oid}).
	repoCtx := f.call("git-context-repo", "GET", readBase+"/context", nil, 200)
	repoCtxID := str(repoCtx, "contextId")
	f.call("error-409-git-readonly", "GET", readBase+"/tree?contextId="+repoCtxID, nil, 409)
	nodeB := base64.RawURLEncoding.EncodeToString([]byte("sub/b.txt"))
	f.call("git-file-commit", "GET", readBase+"/file?contextId="+repoCtxID+"&nodeId="+nodeB+"&source="+url.QueryEscape("commit:"+firstOID), nil, 200)
	compared := f.call("git-compare", "GET", readBase+"/compare?"+url.Values{
		"contextId": {repoCtxID},
		"baseOid":   {firstOID},
		"targetOid": {secondOID},
		"path":      {"sub/b.txt"},
	}.Encode(), nil, 200)
	compareDiff := field(compared, "diff")
	f.call("git-snapshot-file", "GET", readBase+"/snapshots/"+str(compareDiff, "snapshotId")+"/file?side=after", nil, 200)

	// 2. Worktree context, tree, search, file, changes, diff, context_mismatch & stale.
	wtCtx := f.call("git-context-worktree", "GET", readBase+"/context?worktreeId="+wtID, nil, 200)
	wtCtxID := str(wtCtx, "contextId")
	f.call("git-tree", "GET", readBase+"/tree?contextId="+wtCtxID, nil, 200)
	f.call("git-files", "GET", readBase+"/files?contextId="+wtCtxID+"&q=b.txt", nil, 200)
	nodeA := base64.RawURLEncoding.EncodeToString([]byte("a.txt"))
	f.call("git-file", "GET", readBase+"/file?contextId="+wtCtxID+"&nodeId="+nodeA+"&source=worktree", nil, 200)
	changes := f.call("git-changes", "GET", readBase+"/changes?contextId="+wtCtxID, nil, 200)
	var changeID string
	for _, entry := range field(changes, "entries").([]any) {
		if str(entry, "newPath") == "a.txt" {
			changeID = str(entry, "changeId")
		}
	}
	f.call("git-diff", "GET", readBase+"/diff?contextId="+wtCtxID+"&changeId="+changeID, nil, 200)
	f.call("error-409-git-context-mismatch", "GET", readBase+"/tree?worktreeId="+wtID+"&contextId="+repoCtxID, nil, 409)
	f.call("error-409-git-stale", "GET", readBase+"/tree?contextId=expired00", nil, 409)

	// 3. History graph & commit detail.
	f.call("git-history", "GET", readBase+"/history?worktreeId="+wtID+"&scope=all", nil, 200)
	f.call("git-commit-detail", "GET", readBase+"/commits/"+secondOID, nil, 200)

	// 4. Worktree staging, hunk diff, busy lock, stage/unstage, commit-preview & commit.
	f.call("git-commit-preview-blocked", "POST", writeBase+"/commit-preview", contract.GitCommitPreviewRequest{}, 200)
	staging := f.call("git-staging", "GET", writeBase+"/staging", nil, 200)
	stagingDiff := f.call("git-staging-diff", "GET", writeBase+"/staging/diff?area=unstaged&path=a.txt", nil, 200)
	hunks := field(stagingDiff, "hunks").([]any)
	hunkID := str(hunks[0], "id")

	lockPath := filepath.Join(f.root, ".git", "index.lock")
	if err := os.WriteFile(lockPath, []byte("busy"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	f.call("error-409-git-busy", "POST", writeBase+"/stage", map[string]any{
		"identity": field(stagingDiff, "identity"),
		"hunkIds":  []string{hunkID},
	}, 409)
	if err := os.Remove(lockPath); err != nil {
		f.t.Fatal(err)
	}

	_ = staging
	f.call("git-stage", "POST", writeBase+"/stage", map[string]any{
		"identity": field(stagingDiff, "identity"),
		"hunkIds":  []string{hunkID},
	}, 204)
	stagedDiff := f.call("git-staging-diff-staged", "GET", writeBase+"/staging/diff?area=staged&path=a.txt", nil, 200)
	f.call("git-unstage", "POST", writeBase+"/unstage", map[string]any{
		"identity": field(stagedDiff, "identity"),
	}, 204)
	f.call("git-stage-file", "POST", writeBase+"/stage", map[string]any{
		"identity": field(stagingDiff, "identity"),
	}, 204)

	preview := f.call("git-commit-preview", "POST", writeBase+"/commit-preview", contract.GitCommitPreviewRequest{}, 200)
	commitRes := f.call("git-commit", "POST", writeBase+"/commit", contract.GitCommitRequest{
		ConfirmationID:  str(preview, "confirmationId"),
		ClientRequestID: "wire-commit-1",
		Message:         "feat: commit staged a.txt",
	}, 200)
	opID := str(commitRes, "operationId")
	f.call("git-operations", "GET", readBase+"/operations?worktreeId="+wtID+"&limit=20", nil, 200)
	f.call("git-operation", "GET", readBase+"/operations/"+opID, nil, 200)
	f.call("error-409-git-idempotency", "POST", writeBase+"/commit", contract.GitCommitRequest{
		ConfirmationID:  str(preview, "confirmationId"),
		ClientRequestID: "wire-commit-1",
		Message:         "feat: different message with reused clientRequestId",
	}, 409)
}
