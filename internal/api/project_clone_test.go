package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
)

// Git still runs normally for fixtures and repository discovery; only remote
// cloning is replaced, so wire tests never access a network or user account.
func installWireProjectCloneGit(t *testing.T) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("requires Git")
	}
	root := t.TempDir()
	t.Setenv("WEBMUX_WIRE_CLONE_GIT", realGit)
	script := `#!/bin/sh
clone=
for argument do
 if [ "$argument" = clone ]; then clone=yes; fi
 destination=$argument
done
if [ "$clone" = yes ]; then
 exec "$WEBMUX_WIRE_CLONE_GIT" init --initial-branch=main "$destination"
fi
exec "$WEBMUX_WIRE_CLONE_GIT" "$@"
`
	if err := os.WriteFile(filepath.Join(root, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func wireProjectClones(f *wireFixture) {
	request := contract.ProjectCloneRequest{RequestID: "wire-clone", URL: "https://example.invalid/wire.git", Path: filepath.Join(f.root, "cloned project")}
	started := f.call("project-clone-start", "POST", "/api/projects/clone", request, 202)
	id := str(started, "id")
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, err := http.NewRequestWithContext(ctx, "GET", f.server.URL+"/api/projects/operations/"+id, nil)
		if err != nil {
			f.t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer secret")
		response, err := f.server.Client().Do(req)
		if err != nil {
			f.t.Fatal(err)
		}
		var operation contract.ProjectCloneOperation
		err = json.NewDecoder(response.Body).Decode(&operation)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 {
			f.t.Fatalf("poll: %d %v", response.StatusCode, err)
		}
		if operation.State == "succeeded" {
			break
		}
		if operation.State != "running" && operation.State != "registering" {
			f.t.Fatalf("clone: %+v", operation)
		}
		select {
		case <-ctx.Done():
			f.t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	f.call("project-clone-completed", "GET", "/api/projects/operations/"+id, nil, 200)
	f.call("project-clone-repeat", "POST", "/api/projects/clone", request, 202)
	f.call("project-clone-register-repeat", "POST", "/api/projects/operations/"+id+"/register", nil, 202)
	f.call("error-project-clone-missing", "GET", "/api/projects/operations/missing", nil, 404)
	request.RequestID = "different-request"
	f.call("error-project-clone-target", "POST", "/api/projects/clone", request, 409)
	request.URL = "https://secret@example.invalid/repo.git"
	f.call("error-project-clone-credentials", "POST", "/api/projects/clone", request, 400)
}
