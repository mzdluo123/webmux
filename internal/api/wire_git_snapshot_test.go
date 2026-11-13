package api

import (
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/rainchan/webmux/internal/contract"
)

// wireGitWorkbench pins the repository/worktree Git read-only, staging,
// commit-preview, commit execution and structured 409 conflict JSON contracts
// with zero active terminal or hybrid sessions.
func optStr(v any, key string) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

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

	// 5. Fixed TaskBaseline, scope=task history, and single-entry net task diff.
	f.call("git-task-baseline-empty", "GET", writeBase+"/task-baseline", nil, 200)
	f.call("git-history-no-baseline", "GET", readBase+"/history?worktreeId="+wtID+"&scope=task", nil, 200)
	f.call("git-task-changes-no-baseline", "GET", writeBase+"/task-changes", nil, 200)
	f.call("error-409-git-no-task-baseline", "GET", writeBase+"/task-diff?path=sub/b.txt", nil, 409)

	bPath := filepath.Join(f.root, "sub", "b.txt")
	if err := os.WriteFile(bPath, []byte("bee\nbutterfly\nmoth\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	stamp := time.Unix(1700000000, 0)
	if err := os.Chtimes(bPath, stamp, stamp); err != nil {
		f.t.Fatal(err)
	}

	f.call("git-task-baseline-put", "PUT", writeBase+"/task-baseline", contract.GitPutTaskBaselineRequest{
		BaseOID: firstOID,
	}, 200)
	f.call("git-task-baseline", "GET", writeBase+"/task-baseline", nil, 200)
	f.call("git-history-task", "GET", readBase+"/history?worktreeId="+wtID+"&scope=task", nil, 200)
	f.call("git-task-changes", "GET", writeBase+"/task-changes", nil, 200)
	f.call("git-task-diff", "GET", writeBase+"/task-diff?path=sub/b.txt", nil, 200)

	// 6. Server-side review progress sync & optimistic commit draft concurrency.
	reviews := f.call("git-reviews", "GET", writeBase+"/reviews?scope=task", nil, 200)
	reviewFiles := field(reviews, "files").([]any)
	firstReviewFile := reviewFiles[0]
	f.call("git-reviews-put", "PUT", writeBase+"/reviews", contract.GitPutReviewsRequest{
		Scope:  "task",
		Action: "mark",
		Item: &contract.GitReviewMarkIdentity{
			Scope:          str(firstReviewFile, "scope"),
			Area:           str(firstReviewFile, "area"),
			Path:           str(firstReviewFile, "path"),
			OldPath:        optStr(firstReviewFile, "oldPath"),
			BaseMode:       optStr(firstReviewFile, "baseMode"),
			BaseIdentity:   optStr(firstReviewFile, "baseIdentity"),
			TargetMode:     optStr(firstReviewFile, "targetMode"),
			TargetIdentity: optStr(firstReviewFile, "targetIdentity"),
		},
	}, 200)
	f.call("git-commit-draft-empty", "GET", writeBase+"/commit-draft", nil, 200)
	f.call("git-commit-draft-put", "PUT", writeBase+"/commit-draft", contract.GitPutCommitDraftRequest{
		Subject:      "feat: wire commit draft",
		Body:         "draft body line",
		BaseRevision: 0,
	}, 200)
	f.call("error-409-git-draft-conflict", "PUT", writeBase+"/commit-draft", contract.GitPutCommitDraftRequest{
		Subject:      "feat: stale client draft",
		Body:         "",
		BaseRevision: 0,
	}, 409)

	f.call("git-task-baseline-delete", "DELETE", writeBase+"/task-baseline", nil, 204)

	// 7. V2 Divergence (Merge / Rebase / Cherry-pick / Revert), 3-way conflict resolver, Continue & Abort.
	// Clean working tree on main first so divergence previews are unblocked.
	f.git("checkout", "--", "sub/b.txt")
	f.git("clean", "-fd")
	f.call("git-operation-state-idle", "GET", writeBase+"/operation-state", nil, 200)

	// Create a diverging local branch `feat-conflict` from firstOID that edits sub/b.txt differently.
	f.git("checkout", "-b", "feat-conflict", firstOID)
	if err := os.WriteFile(filepath.Join(f.root, "sub", "b.txt"), []byte("bee\ndragonfly\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	f.git("add", "sub/b.txt")
	f.git("commit", "-m", "feat: dragonfly in b.txt")
	conflictBranchOID := f.gitOutput("rev-parse", "HEAD")
	f.git("checkout", "main")

	// Preview rebase, cherry-pick, revert, and merge.
	f.call("git-rebase-preview", "POST", writeBase+"/rebase-preview", contract.GitBranchDivergencePreviewRequest{
		RefKind: "local",
		RefName: "feat-conflict",
	}, 200)
	f.call("git-cherry-pick-preview", "POST", writeBase+"/cherry-pick-preview", contract.GitCommitDivergencePreviewRequest{
		CommitOID: conflictBranchOID,
	}, 200)
	f.call("git-revert-preview", "POST", writeBase+"/revert-preview", contract.GitCommitDivergencePreviewRequest{
		CommitOID: secondOID,
	}, 200)
	mergePreview := f.call("git-merge-preview", "POST", writeBase+"/merge-preview", contract.GitBranchDivergencePreviewRequest{
		RefKind: "local",
		RefName: "feat-conflict",
	}, 200)

	// Execute merge -> hits text conflict in sub/b.txt (HTTP 200 outcome: "conflicted").
	f.call("git-merge-conflicted", "POST", writeBase+"/merge", contract.GitDivergenceExecuteRequest{
		ConfirmationID:  str(mergePreview, "confirmationId"),
		ClientRequestID: "wire-merge-1",
	}, 200)
	f.call("git-operation-state", "GET", writeBase+"/operation-state", nil, 200)

	// Attempting Continue before resolving must return 409 reason="conflicted".
	f.call("error-409-git-conflicted", "POST", writeBase+"/operation-continue", contract.GitContinueOperationRequest{
		Kind: "merge",
	}, 409)

	// Inspect 3-way conflict -> Save draft to worktree -> Mark as Resolved -> Continue!
	conflictDetail := f.call("git-conflict-detail", "GET", writeBase+"/conflict?path="+url.QueryEscape("sub/b.txt"), nil, 200)
	savedRes := f.call("git-conflict-save", "POST", writeBase+"/conflict/save", map[string]any{
		"identity":   field(conflictDetail, "identity"),
		"resultText": "bee\nbutterfly\ndragonfly\n",
	}, 200)
	f.call("git-conflict-resolve", "POST", writeBase+"/conflict/resolve", map[string]any{
		"identity":   field(savedRes, "identity"),
		"resolution": "text",
		"resultText": "bee\nbutterfly\ndragonfly\n",
	}, 200)
	f.call("git-operation-continue", "POST", writeBase+"/operation-continue", contract.GitContinueOperationRequest{
		Kind:            "merge",
		ClientRequestID: "wire-continue-1",
	}, 200)

	// Trigger cherry-pick conflict and exercise PreviewAbortOperation + AbortOperation!
	// Create another conflicting commit on feat-conflict so cherry-picking it conflicts cleanly.
	f.git("checkout", "feat-conflict")
	if err := os.WriteFile(filepath.Join(f.root, "sub", "b.txt"), []byte("bee\nfirefly\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	f.git("add", "sub/b.txt")
	f.git("commit", "-m", "feat: firefly conflict")
	fireflyOID := f.gitOutput("rev-parse", "HEAD")
	f.git("checkout", "main")

	cpPreview := f.call("git-cherry-pick-preview-2", "POST", writeBase+"/cherry-pick-preview", contract.GitCommitDivergencePreviewRequest{
		CommitOID: fireflyOID,
	}, 200)
	f.call("git-cherry-pick-conflicted", "POST", writeBase+"/cherry-pick", contract.GitDivergenceExecuteRequest{
		ConfirmationID: str(cpPreview, "confirmationId"),
	}, 200)
	abortPreview := f.call("git-abort-preview", "POST", writeBase+"/operation-abort-preview", contract.GitAbortOperationPreviewRequest{}, 200)
	f.call("git-operation-abort", "POST", writeBase+"/operation-abort", contract.GitAbortOperationRequest{
		ConfirmationID:  str(abortPreview, "confirmationId"),
		ClientRequestID: "wire-abort-1",
	}, 200)

	// 8. V2 Stash, Amend/Author/Sign-off, Branch Rename/Upstream, Tags, Remotes, File History & Blame (#550).
	repoBase := "/api/repositories/" + repoID
	if err := os.WriteFile(filepath.Join(f.root, "sub", "b.txt"), []byte("bee\nbutterfly\ndragonfly\nstash-line\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "stash-note.txt"), []byte("untracked note\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	stashPrev := f.call("git-stash-create-preview", "POST", writeBase+"/stash-preview", contract.GitStashCreatePreviewRequest{
		Message:          "WIP: stash snapshot",
		IncludeUntracked: true,
	}, 200)
	stashCreate := f.call("git-stash-create", "POST", writeBase+"/stash", contract.GitStashExecuteRequest{
		ConfirmationID:  str(stashPrev, "confirmationId"),
		ClientRequestID: "wire-stash-create-1",
	}, 200)
	stashEntry := field(stashCreate, "entry")
	stashOID := str(stashEntry, "oid")
	stashRef := str(stashEntry, "stashRef")
	f.call("git-stashes", "GET", readBase+"/stashes", nil, 200)
	f.call("git-stash-detail", "GET", readBase+"/stashes/"+stashOID, nil, 200)
	f.call("git-stash-diff", "GET", readBase+"/stashes/"+stashOID+"/diff?"+url.Values{
		"contextId": {wtCtxID},
		"path":      {"stash-note.txt"},
		"untracked": {"true"},
	}.Encode(), nil, 200)

	applyPrev := f.call("git-stash-apply-preview", "POST", writeBase+"/stash-apply-preview", contract.GitStashApplyPreviewRequest{
		Mode:     "apply",
		StashRef: stashRef,
		StashOID: stashOID,
	}, 200)
	f.call("git-stash-apply", "POST", writeBase+"/stash-apply", contract.GitStashExecuteRequest{
		ConfirmationID:  str(applyPrev, "confirmationId"),
		ClientRequestID: "wire-stash-apply-1",
	}, 200)
	f.git("checkout", "--", "sub/b.txt")
	f.git("clean", "-fd")

	dropPrev := f.call("git-stash-drop-preview", "POST", readBase+"/stashes/drop-preview", contract.GitStashDropPreviewRequest{
		StashRef: stashRef,
		StashOID: stashOID,
	}, 200)
	f.call("git-stash-drop", "POST", readBase+"/stashes/drop", contract.GitStashDropRequest{
		ConfirmationID:  str(dropPrev, "confirmationId"),
		ClientRequestID: "wire-stash-drop-1",
	}, 200)

	amendPrev := f.call("git-commit-amend-preview", "POST", writeBase+"/commit-preview", contract.GitCommitPreviewRequest{
		Amend:          true,
		AuthorOverride: "Ada Lovelace <ada@example.com>",
		Signoff:        true,
	}, 200)
	amendRes := f.call("git-commit-amend", "POST", writeBase+"/commit", contract.GitCommitRequest{
		ConfirmationID:  str(amendPrev, "confirmationId"),
		ClientRequestID: "wire-commit-amend-1",
		Message:         "feat: amend merge commit with author override",
	}, 200)
	amendedOID := str(amendRes, "commitOid")

	rnPrev := f.call("git-branch-rename-preview", "POST", repoBase+"/branches/rename-preview", contract.GitBranchRenamePreviewRequest{
		OldName: "feat-conflict",
		NewName: "feat-renamed",
	}, 200)
	f.call("git-branch-rename", "POST", repoBase+"/branches/rename", contract.GitBranchRenameRequest{
		ConfirmationID:  str(rnPrev, "confirmationId"),
		ClientRequestID: "wire-branch-rename-1",
	}, 200)
	f.call("git-branch-upstream", "POST", repoBase+"/branches/upstream", contract.GitBranchUpstreamRequest{
		Branch:          "feat-renamed",
		Upstream:        "",
		ClientRequestID: "wire-branch-upstream-1",
	}, 200)

	f.call("git-tag-create", "POST", repoBase+"/tags", contract.GitTagCreateRequest{
		Name:            "v1.0.0-wire",
		TargetOID:       amendedOID,
		Annotated:       false,
		ClientRequestID: "wire-tag-create-1",
	}, 201)
	f.call("git-tag-delete", "POST", repoBase+"/tags/delete", contract.GitTagDeleteRequest{
		Name:              "v1.0.0-wire",
		ExpectedTargetOID: amendedOID,
		ClientRequestID:   "wire-tag-delete-1",
	}, 200)

	f.call("git-remote-add", "POST", repoBase+"/remotes", contract.GitRemoteAddRequest{
		Name:            "mirror",
		FetchURL:        "https://alice:secret123@example.com/org/repo.git",
		ClientRequestID: "wire-remote-add-1",
	}, 201)
	newFetch := "https://example.com/org/public.git"
	f.call("git-remote-update", "PATCH", repoBase+"/remotes/mirror", contract.GitRemoteUpdateRequest{
		FetchURL:        &newFetch,
		ClientRequestID: "wire-remote-update-1",
	}, 200)
	f.call("git-remote-remove", "DELETE", repoBase+"/remotes/mirror", contract.GitRemoteRemoveRequest{
		ClientRequestID: "wire-remote-remove-1",
	}, 200)

	f.call("git-file-history", "GET", readBase+"/file-history?worktreeId="+wtID+"&path="+url.QueryEscape("sub/b.txt"), nil, 200)
	f.call("git-blame", "GET", readBase+"/blame?worktreeId="+wtID+"&rev="+amendedOID+"&path="+url.QueryEscape("sub/b.txt"), nil, 200)
	if err := os.WriteFile(filepath.Join(f.root, "sub", "b.txt"), []byte("bee\nworktree-compare\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	bComparePath := filepath.Join(f.root, "sub", "b.txt")
	if err := os.Chtimes(bComparePath, stamp, stamp); err != nil {
		f.t.Fatal(err)
	}
	f.call("git-revision-diff", "GET", readBase+"/revision-diff?"+url.Values{
		"worktreeId": {wtID},
		"contextId":  {wtCtxID},
		"baseOid":    {firstOID},
		"path":       {"sub/b.txt"},
	}.Encode(), nil, 200)
}
