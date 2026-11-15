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

func TestGitStashAmendRefsRemotesAndBlameAPI(t *testing.T) {
	env := newGitAPITestEnv(t)
	repoURL := "/api/repositories/" + env.repo.ID
	gitURL := repoURL + "/git"
	wtURL := repoURL + "/worktrees/" + env.mainWT.ID + "/git"
	dummyOID := "0123456789abcdef0123456789abcdef01234567"

	// 1. Auth (401) and Origin (403) boundaries on every new V2 (#550) endpoint.
	for _, call := range [][2]string{
		{"GET", gitURL + "/stashes"},
		{"GET", gitURL + "/stashes/" + dummyOID},
		{"GET", gitURL + "/stashes/" + dummyOID + "/diff?path=a.txt"},
		{"POST", gitURL + "/stashes/drop-preview"},
		{"POST", gitURL + "/stashes/drop"},
		{"POST", wtURL + "/stash-preview"},
		{"POST", wtURL + "/stash"},
		{"POST", wtURL + "/stash-apply-preview"},
		{"POST", wtURL + "/stash-apply"},
		{"POST", wtURL + "/stash-pop"},
		{"GET", gitURL + "/file-history?path=a.txt"},
		{"GET", gitURL + "/blame?path=a.txt"},
		{"GET", gitURL + "/revision-diff?worktreeId=" + env.mainWT.ID + "&baseOid=" + dummyOID + "&path=a.txt"},
		{"POST", repoURL + "/branches/rename-preview"},
		{"POST", repoURL + "/branches/rename"},
		{"POST", repoURL + "/branches/upstream"},
		{"POST", repoURL + "/tags"},
		{"POST", repoURL + "/tags/delete"},
		{"POST", repoURL + "/remotes"},
		{"PATCH", repoURL + "/remotes/origin"},
		{"DELETE", repoURL + "/remotes/origin"},
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

	// 2. Stash create (with untracked), inspect, diff, conflicting Pop (preserving stash!), and resolve!
	if err := os.WriteFile(filepath.Join(env.root, "a.txt"), []byte("baseline-v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "a.txt")
	env.gitOut(env.root, "commit", "-m", "feat: baseline a.txt")

	if err := os.WriteFile(filepath.Join(env.root, "a.txt"), []byte("stashed-edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "untracked.txt"), []byte("untracked-stash\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stashPrev contract.GitStashCreatePreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/stash-preview", contract.GitStashCreatePreviewRequest{
		Message:          "WIP: api stash",
		IncludeUntracked: true,
	}, 200), &stashPrev); err != nil || stashPrev.ConfirmationID == "" {
		t.Fatalf("stash-preview failed: %+v (%v)", stashPrev, err)
	}

	var stashCreated contract.GitStashCreateResult
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/stash", contract.GitStashExecuteRequest{
		ConfirmationID:  stashPrev.ConfirmationID,
		ClientRequestID: "api-stash-1",
	}, 200), &stashCreated); err != nil || !stashCreated.Entry.HasUntracked {
		t.Fatalf("stash create failed: %+v (%v)", stashCreated, err)
	}

	var detail contract.GitStashDetail
	if err := json.Unmarshal(env.doJSON("GET", gitURL+"/stashes/"+stashCreated.Entry.OID, nil, 200), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.UntrackedCount != 1 {
		t.Fatalf("expected 1 untracked file in stash detail, got %+v", detail)
	}

	var repoGitCtx contract.GitContext
	if err := json.Unmarshal(env.doJSON("GET", gitURL+"/context", nil, 200), &repoGitCtx); err != nil {
		t.Fatal(err)
	}
	var wtGitCtx contract.GitContext
	if err := json.Unmarshal(env.doJSON("GET", gitURL+"/context?worktreeId="+env.mainWT.ID, nil, 200), &wtGitCtx); err != nil {
		t.Fatal(err)
	}

	var uDiff contract.GitDiff
	if err := json.Unmarshal(env.doJSON("GET", gitURL+"/stashes/"+stashCreated.Entry.OID+"/diff?contextId="+url.QueryEscape(repoGitCtx.ID)+"&path=untracked.txt&untracked=true", nil, 200), &uDiff); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(uDiff.New.Text, "untracked-stash") {
		t.Fatalf("unexpected untracked stash diff: %+v", uDiff)
	}

	// Commit conflicting change on a.txt, then Pop -> must return outcome="conflicted" and preserve stash!
	if err := os.WriteFile(filepath.Join(env.root, "a.txt"), []byte("main-conflicting-edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "a.txt")
	env.gitOut(env.root, "commit", "-m", "feat: main edit on a.txt")

	var popPrev contract.GitStashApplyPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/stash-apply-preview", contract.GitStashApplyPreviewRequest{
		Mode:     "pop",
		StashRef: stashCreated.Entry.StashRef,
		StashOID: stashCreated.Entry.OID,
	}, 200), &popPrev); err != nil || popPrev.ConfirmationID == "" {
		t.Fatalf("stash-apply-preview failed: %+v (%v)", popPrev, err)
	}

	var popRes contract.GitStashApplyResult
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/stash-pop", contract.GitStashExecuteRequest{
		ConfirmationID: popPrev.ConfirmationID,
	}, 200), &popRes); err != nil {
		t.Fatal(err)
	}
	if popRes.Outcome != "conflicted" || popRes.StashDropped || !popRes.StashPreserved || popRes.InProgress.Kind != "unstash" {
		t.Fatalf("conflicting Pop failed safety invariant: %+v", popRes)
	}

	// Clean up conflict and drop stash via API.
	env.gitOut(env.root, "reset", "--hard", "HEAD")
	env.gitOut(env.root, "clean", "-fd")

	var dropPrev contract.GitStashDropPreview
	if err := json.Unmarshal(env.doJSON("POST", gitURL+"/stashes/drop-preview", contract.GitStashDropPreviewRequest{
		StashRef: stashCreated.Entry.StashRef,
		StashOID: stashCreated.Entry.OID,
	}, 200), &dropPrev); err != nil || dropPrev.ConfirmationID == "" {
		t.Fatal(err)
	}
	var dropRes contract.GitStashDropResult
	if err := json.Unmarshal(env.doJSON("POST", gitURL+"/stashes/drop", contract.GitStashDropRequest{
		ConfirmationID: dropPrev.ConfirmationID,
	}, 200), &dropRes); err != nil || len(dropRes.Stashes) != 0 {
		t.Fatalf("stash drop failed: %+v (%v)", dropRes, err)
	}

	// 3. File history, Blame, and Revision diff!
	headOID := strings.TrimSpace(env.gitOut(env.root, "rev-parse", "HEAD"))
	var fh contract.GitFileHistoryPage
	if err := json.Unmarshal(env.doJSON("GET", gitURL+"/file-history?worktreeId="+env.mainWT.ID+"&path="+url.QueryEscape("a.txt"), nil, 200), &fh); err != nil || len(fh.Commits) == 0 {
		t.Fatalf("file-history failed: %+v (%v)", fh, err)
	}

	var blame contract.GitBlameResult
	if err := json.Unmarshal(env.doJSON("GET", gitURL+"/blame?worktreeId="+env.mainWT.ID+"&rev="+headOID+"&path="+url.QueryEscape("a.txt"), nil, 200), &blame); err != nil || blame.Kind != "text" || len(blame.Lines) == 0 {
		t.Fatalf("blame failed: %+v (%v)", blame, err)
	}

	if err := os.WriteFile(filepath.Join(env.root, "a.txt"), []byte("live-worktree-diff\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var revDiff contract.GitDiff
	if err := json.Unmarshal(env.doJSON("GET", gitURL+"/revision-diff?worktreeId="+env.mainWT.ID+"&contextId="+url.QueryEscape(wtGitCtx.ID)+"&baseOid="+headOID+"&path="+url.QueryEscape("a.txt"), nil, 200), &revDiff); err != nil || !strings.Contains(revDiff.New.Text, "live-worktree-diff") {
		t.Fatalf("revision-diff failed: %+v (%v)", revDiff, err)
	}

	// 4. Remotes credential redaction via HTTP!
	var remRes contract.GitRemoteMutationResult
	if err := json.Unmarshal(env.doJSON("POST", repoURL+"/remotes", contract.GitRemoteAddRequest{
		Name:     "upstream",
		FetchURL: "https://user:topsecret999@example.com/org/repo.git",
	}, 201), &remRes); err != nil {
		t.Fatal(err)
	}
	if len(remRes.Remotes) == 0 || strings.Contains(remRes.Remotes[0].FetchURL, "topsecret999") {
		t.Fatalf("remote secret leaked over HTTP! %+v", remRes)
	}
}
