package api

import (
	"os"
	"path/filepath"
	"testing"
)

func installWireProjectGitHub(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = repo ]; then printf 'clone --no-upstream'; exit 0; fi
if [ "$WEBMUX_WIRE_GITHUB_FAIL" = yes ]; then printf SECRET_TOKEN >&2; exit 1; fi
if [ "$6" = user ]; then printf '{"login":"wire-user"}'; exit 0; fi
printf '[{"full_name":"wire-user/private-project","description":"Private demo project","private":true,"clone_url":"https://github.com/wire-user/private-project.git"}]'
`
	if err := os.WriteFile(filepath.Join(root, "gh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func wireProjectGitHub(f *wireFixture) {
	f.call("projects-github", "GET", "/api/projects/github", nil, 200)
	f.call("error-projects-github-page", "GET", "/api/projects/github?page=invalid", nil, 400)
	f.call("error-projects-github-query", "GET", "/api/projects/github?query=bad%0Aquery", nil, 400)
	f.t.Setenv("WEBMUX_WIRE_GITHUB_FAIL", "yes")
	f.call("projects-github-unavailable", "GET", "/api/projects/github", nil, 200)
	f.call("error-projects-github-auth", "GET", "/api/projects/github", nil, 401, withToken(""))
}
