package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/session"
)

func TestProjectRoutesRequireAuthAndOrigin(t *testing.T) {
	t.Parallel()
	manager := session.NewManager(session.Options{})
	defer manager.Shutdown(context.Background())
	handler := New(testDeps(t, testServices{Config: config.Config{Token: "secret"}, Manager: manager, Workspace: testWorkspace(t, manager, t.TempDir())}))
	for _, path := range []string{"/api/projects", "/api/projects/create", "/api/projects/clone", "/api/projects/operations/missing/register"} {
		for _, row := range []struct {
			auth, origin string
			want         int
		}{{"", "", 401}, {"Bearer secret", "https://evil.example", 403}} {
			request := httptest.NewRequest("POST", "http://webmux.example"+path, strings.NewReader(`{"path":"/tmp","parent":"/tmp","name":"never-created","initializeGit":false}`))
			request.Header.Set("Authorization", row.auth)
			request.Header.Set("Origin", row.origin)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != row.want {
				t.Fatalf("%s: %d want %d", path, recorder.Code, row.want)
			}
		}
	}
}

func wireProjects(f *wireFixture) {
	f.call("projects-empty", "GET", "/api/projects", nil, 200)
	project := f.call("project-create", "POST", "/api/projects/create", map[string]any{"parent": f.root, "name": "wire project", "initializeGit": false}, 201)
	f.call("project-register-again", "POST", "/api/projects", map[string]string{"path": str(project, "path")}, 200)
	f.call("projects", "GET", "/api/projects", nil, 200)
	f.call("error-project-existing", "POST", "/api/projects/create", map[string]any{"parent": f.root, "name": "wire project", "initializeGit": false}, 409)
	f.call("error-project-name", "POST", "/api/projects/create", map[string]any{"parent": f.root, "name": "../escape", "initializeGit": false}, 400)
}
