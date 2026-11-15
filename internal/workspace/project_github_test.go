package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func githubTestService(t *testing.T, mode string) (*Service, string) {
	t.Helper()
	s, root, _ := cloneTestService(t, "success")
	t.Setenv("WEBMUX_GITHUB_MODE", mode)
	t.Setenv("WEBMUX_GITHUB_ARGS", filepath.Join(root, "github-args"))
	script := `#!/bin/sh
printf '%s\n' "$@" >> "$WEBMUX_GITHUB_ARGS"
if [ "$1" = repo ]; then
 test "$2" = clone || exit 1
 if [ "$3" = --help ]; then
  if [ "$WEBMUX_GITHUB_MODE" = old ]; then printf 'old clone help'; else printf 'clone --no-upstream'; fi
  exit 0
 fi
 test "$3" = --no-upstream || exit 1
 test "$("$WEBMUX_CLONE_TEST_GIT" config --get protocol.allow)" = never || exit 81
 test "$("$WEBMUX_CLONE_TEST_GIT" config --get protocol.https.allow)" = always || exit 82
 "$WEBMUX_CLONE_TEST_GIT" init --initial-branch=main "$5"
 exit $?
fi
case "$WEBMUX_GITHUB_MODE" in
 auth) printf 'SECRET_TOKEN=redact' >&2; exit 1 ;;
 block) sleep 60; exit 1 ;;
esac
if [ "$6" = user ]; then printf '{"login":"demo"}'; exit 0; fi
case "$WEBMUX_GITHUB_MODE" in
 permission) printf 'SECRET_TOKEN=permission' >&2; exit 1 ;;
 malformed) printf '{broken'; exit 0 ;;
 unsafe) printf '[{"full_name":"demo/repo","clone_url":"https://attacker.invalid/demo/repo.git"}]'; exit 0 ;;
esac
cat "$WEBMUX_GITHUB_DATA"
`
	wrapper := filepath.Join(root, "gh-wrapper")
	if err := os.WriteFile(wrapper, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	s.githubPath = wrapper
	data := filepath.Join(root, "github-data")
	t.Setenv("WEBMUX_GITHUB_DATA", data)
	return s, root
}

func TestGitHubProjectsIdentityPagingAndSearch(t *testing.T) {
	s, root := githubTestService(t, "success")
	repo := map[string]any{"full_name": "demo/private-repo", "description": "private project", "private": true, "clone_url": "https://github.com/demo/private-repo.git"}
	repos := make([]map[string]any, githubPageSize)
	for i := range repos {
		repos[i] = repo
	}
	data, _ := json.Marshal(repos)
	if err := os.WriteFile(filepath.Join(root, "github-data"), data, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := s.GitHubProjects(context.Background(), "", 2)
	if err != nil || !result.Available || result.Login != "demo" || result.Page != 2 || !result.HasMore || !result.Repositories[0].Private {
		t.Fatalf("listing: %+v %v", result, err)
	}
	data, _ = json.Marshal(map[string]any{"items": repos[:1], "incomplete_results": false})
	if err := os.WriteFile(filepath.Join(root, "github-data"), data, 0600); err != nil {
		t.Fatal(err)
	}
	result, err = s.GitHubProjects(context.Background(), "demo/private", 1)
	if err != nil || !result.Available || result.HasMore || len(result.Repositories) != 1 {
		t.Fatalf("search: %+v %v", result, err)
	}
	args, err := os.ReadFile(filepath.Join(root, "github-args"))
	if err != nil || !strings.Contains(string(args), "q=demo/private in:name\n") || !strings.Contains(string(args), "page=2\n") {
		t.Fatalf("argv: %s %v", args, err)
	}
}

func TestGitHubProjectsDegradationAndTimeout(t *testing.T) {
	for _, mode := range []string{"missing", "auth", "permission", "malformed", "unsafe", "block", "old"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := githubTestService(t, mode)
			if mode == "missing" {
				s.githubPath = ""
			}
			s.githubTimeout = 40 * time.Millisecond
			started := time.Now()
			result, err := s.GitHubProjects(context.Background(), "", 1)
			if err != nil || result.Available || result.Reason == "" || result.Repositories == nil || strings.Contains(result.Reason, "SECRET_TOKEN") {
				t.Fatalf("degradation: %+v %v", result, err)
			}
			if time.Since(started) > 3*time.Second {
				t.Fatal("timeout did not join process group")
			}
			if _, _, err := s.RegisterDirectoryProject(t.TempDir()); err != nil {
				t.Fatalf("GitHub failure disabled local projects: %v", err)
			}
		})
	}
}

func TestGitHubProjectsRejectInvalidInput(t *testing.T) {
	s, _ := githubTestService(t, "success")
	for _, query := range []string{"bad\nquery", strings.Repeat("x", 201)} {
		if _, err := s.GitHubProjects(context.Background(), query, 1); !errors.Is(err, ErrInvalid) {
			t.Fatalf("query accepted: %v", err)
		}
	}
	for _, page := range []int{0, 51} {
		if _, err := s.GitHubProjects(context.Background(), "", page); !errors.Is(err, ErrInvalid) {
			t.Fatalf("page accepted: %v", err)
		}
	}
}

func TestGitHubCloneUsesAuthenticatedCLIAndSourceIdentity(t *testing.T) {
	s, root := githubTestService(t, "success")
	req := ProjectCloneRequest{RequestID: "github-clone", URL: "https://github.com/demo/private-repo.git", Path: filepath.Join(root, "target"), Source: "github"}
	op, err := s.StartProjectClone(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if done := awaitProjectClone(t, s, op.ID); done.State != "succeeded" {
		t.Fatalf("clone: %+v", done)
	}
	args, err := os.ReadFile(filepath.Join(root, "github-args"))
	if err != nil || !strings.Contains(string(args), "repo\nclone\n--no-upstream\nhttps://github.com/demo/private-repo.git\n") {
		t.Fatalf("gh not used: %s %v", args, err)
	}
	req.Source = ""
	if _, err := s.StartProjectClone(context.Background(), req); !errors.Is(err, ErrConflict) {
		t.Fatalf("source not part of identity: %v", err)
	}
	req.Source = "github"
	for _, raw := range []string{"https://attacker.invalid/demo/repo.git", "ssh://git@github.com/demo/repo.git", "https://github.com:443/demo/repo.git", "https://github.com/demo/repo/extra", "https://github.com/demo/repo?token=secret"} {
		req.URL = raw
		if _, err := s.StartProjectClone(context.Background(), req); !errors.Is(err, ErrInvalid) {
			t.Fatalf("unsafe GitHub URL accepted: %s %v", raw, err)
		}
	}
}

func TestGitHubCloneProtocolConfigurationIsTransient(t *testing.T) {
	s, root := githubTestService(t, "success")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "user.name")
	t.Setenv("GIT_CONFIG_VALUE_0", "Deployment User")
	op, err := s.StartProjectClone(context.Background(), ProjectCloneRequest{RequestID: "transient-config", URL: "https://github.com/demo/repo.git", Path: filepath.Join(root, "target"), Source: "github"})
	if err != nil {
		t.Fatal(err)
	}
	if done := awaitProjectClone(t, s, op.ID); done.State != "succeeded" {
		t.Fatalf("clone: %+v", done)
	}
	config, err := os.ReadFile(filepath.Join(root, "target", ".git", "config"))
	if err != nil || strings.Contains(string(config), "[protocol") {
		t.Fatalf("persisted clone restrictions: %s %v", config, err)
	}
	env, err := projectCloneProtocolEnv()
	if err != nil || env[0] != "GIT_CONFIG_COUNT=4" || !strings.Contains(strings.Join(env, "\n"), "GIT_CONFIG_KEY_1=protocol.allow") {
		t.Fatalf("overwrote inherited configuration: %v %v", env, err)
	}
}

func TestGitHubCloneRejectsUnsupportedCLIWithoutAdmission(t *testing.T) {
	s, root := githubTestService(t, "old")
	op, err := s.StartProjectClone(context.Background(), ProjectCloneRequest{RequestID: "old-cli", URL: "https://github.com/demo/repo.git", Path: filepath.Join(root, "target"), Source: "github"})
	if !errors.Is(err, ErrUnavailable) || op.ID != "" || !strings.Contains(err.Error(), "2.88") || len(s.clones) != 0 {
		t.Fatalf("unsupported gh admitted: %+v %v", op, err)
	}
}
