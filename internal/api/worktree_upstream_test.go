package api

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestWorktreeAndSessionUpstreamHTTPConfirmation(t *testing.T) {
	handler, _, _, root := workspaceAPI(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	gitWorkspaceAPI(t, root, "clone", "--bare", root, remote)
	gitWorkspaceAPI(t, root, "remote", "add", "origin", remote)
	gitWorkspaceAPI(t, root, "push", "-u", "origin", "main")
	other := filepath.Join(t.TempDir(), "other")
	gitWorkspaceAPI(t, root, "clone", remote, other)
	gitWorkspaceAPI(t, other, "commit", "--allow-empty", "-m", "upstream update")
	gitWorkspaceAPI(t, other, "push", "origin", "main")
	registered := workspaceRequest(t, handler, "POST", "/api/repositories", map[string]string{"path": root}, 201)
	var repository contract.Repository
	if err := json.Unmarshal(registered.Body.Bytes(), &repository); err != nil {
		t.Fatal(err)
	}

	url := "/api/repositories/" + repository.ID + "/worktrees"
	req := contract.CreateWorktreeRequest{Mode: "new", Branch: "direct", Base: "HEAD", Path: filepath.Join(t.TempDir(), "direct")}
	response := workspaceRequest(t, handler, "POST", url, req, 409)
	var challenge contract.WorktreeUpstreamFailure
	if err := json.Unmarshal(response.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	if challenge.Reason != "worktree_upstream_confirmation" || challenge.WorktreeUpstream == nil || !challenge.WorktreeUpstream.CanUseUpstream || challenge.WorktreeUpstream.UpstreamHead == "" {
		t.Fatalf("unusable worktree challenge: %s", response.Body.String())
	}
	req.UpstreamChoice = &contract.UpstreamChoice{ConfirmationID: challenge.WorktreeUpstream.ConfirmationID, UseUpstream: true}
	workspaceRequest(t, handler, "POST", url, req, 201)

	sessionReq := contract.CreateSessionRequest{ClientRequestID: "upstream-http-retry", Cmd: []string{"/bin/sh", "-c", "exit 0"}, NewWorktree: &contract.NewWorktree{RepositoryID: repository.ID, Mode: "new", Branch: "session", Base: "HEAD", Path: filepath.Join(t.TempDir(), "session")}}
	response = workspaceRequest(t, handler, "POST", "/api/sessions", sessionReq, 409)
	var failed contract.CreateSessionFailure
	if err := json.Unmarshal(response.Body.Bytes(), &failed); err != nil {
		t.Fatal(err)
	}
	if failed.Reason != "worktree_upstream_confirmation" || failed.Step != contract.CreateStepPrecheck || failed.WorktreeUpstream == nil || failed.Worktree != nil || failed.SessionID != "" {
		t.Fatalf("wrapped challenge lost metadata: %s", response.Body.String())
	}
	sessionReq.NewWorktree.UpstreamChoice = &contract.UpstreamChoice{ConfirmationID: failed.WorktreeUpstream.ConfirmationID}
	workspaceRequest(t, handler, "POST", "/api/sessions", sessionReq, 201)
}
