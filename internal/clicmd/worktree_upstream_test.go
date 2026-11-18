package clicmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

type upstreamRoundTrip func(*http.Request) (*http.Response, error)

func (f upstreamRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWorktreeUpstreamConsumers(t *testing.T) {
	for _, command := range []string{"worktree", "run", "chat"} {
		for _, tc := range []struct {
			name, policy, answer                         string
			interactive, diverged, useUpstream, succeeds bool
		}{
			{name: "requires explicit script choice"},
			{name: "script latest", policy: "latest", useUpstream: true, succeeds: true},
			{name: "script local", policy: "local", succeeds: true},
			{name: "terminal latest", interactive: true, answer: "latest\nlatest\n", useUpstream: true, succeeds: true},
			{name: "terminal local", interactive: true, answer: "local\nlocal\n", succeeds: true},
			{name: "terminal cancel", interactive: true, answer: "cancel\n"},
			{name: "terminal empty cancels", interactive: true, answer: "\n"},
			{name: "diverged latest refused", diverged: true, policy: "latest"},
			{name: "diverged local", diverged: true, policy: "local", succeeds: true},
			{name: "diverged terminal keep", diverged: true, interactive: true, answer: "local\nlocal\n", succeeds: true},
		} {
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				var requests []contract.CreateSessionRequest
				var choices []*contract.UpstreamChoice
				var bases []string
				handler := withTestCapabilities(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/repositories":
						_, _ = io.WriteString(w, `[{"id":"repo0001","anchorPath":"/repo","worktrees":[]}]`)
					case "/api/launch-configs":
						_, _ = io.WriteString(w, `[{"id":"cfg00001","name":"agent","chat":true}]`)
					case "/api/repositories/repo0001/worktrees", "/api/sessions":
						var choice *contract.UpstreamChoice
						if command == "worktree" {
							var req contract.CreateWorktreeRequest
							if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
								t.Fatal(err)
							}
							choice = req.UpstreamChoice
							bases = append(bases, req.Base)
						} else {
							var req contract.CreateSessionRequest
							if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
								t.Fatal(err)
							}
							requests = append(requests, req)
							if req.NewWorktree == nil {
								t.Fatal("missing new worktree")
							}
							choice = req.NewWorktree.UpstreamChoice
							bases = append(bases, req.NewWorktree.Base)
						}
						choices = append(choices, choice)
						if len(choices) <= 2 {
							id := "first"
							if len(choices) == 2 {
								id = "changed-refs"
							}
							w.WriteHeader(http.StatusConflict)
							_ = json.NewEncoder(w).Encode(contract.WorktreeUpstreamFailure{Error: "choose upstream", Reason: "worktree_upstream_confirmation", WorktreeUpstream: &contract.WorktreeUpstream{ConfirmationID: id, Branch: "refs/heads/main", LocalHead: "old", Upstream: "origin/main", UpstreamHead: "new", CanUseUpstream: !tc.diverged, Warning: "upstream differs"}})
							return
						}
						w.WriteHeader(http.StatusCreated)
						if command == "worktree" {
							_, _ = io.WriteString(w, `{"id":"tree0001","path":"/repo.worktrees/task"}`)
						} else {
							_, _ = io.WriteString(w, `{"session":{"id":"session1"},"worktree":{"id":"tree0001","path":"/repo.worktrees/task"}}`)
						}
					default:
						t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
					}
				}))
				var stdout, stderr bytes.Buffer
				e := Env{
					Connect: func() (*localclient.Client, error) {
						return &localclient.Client{HTTP: &http.Client{Transport: upstreamRoundTrip(func(r *http.Request) (*http.Response, error) {
							w := httptest.NewRecorder()
							handler.ServeHTTP(w, r)
							return w.Result(), nil
						})}}, nil
					},
					Stdin: strings.NewReader(tc.answer), Stdout: &stdout, Stderr: &stderr,
					Interactive: func() bool { return tc.interactive },
					ResolveCwd:  func(context.Context, string) (string, error) { return "/repo", nil },
				}
				flags := []string{}
				if tc.policy != "" {
					flags = append(flags, "--upstream="+tc.policy)
				}
				var code int
				if command == "worktree" {
					args := append([]string{"repo0001", "--no-pin"}, flags...)
					if tc.diverged {
						args = append(args, "--existing=main")
					} else {
						args = append(args, "--branch=task", "--base=refs/heads/main")
					}
					code = e.Run(context.Background(), worktreeCreateCommand, args)
				} else {
					args := append([]string{"--new-worktree", "--repo=repo0001", "--branch=task", "--no-pin", "--detach"}, flags...)
					if tc.diverged {
						args = append(args, "--existing-branch")
					} else {
						args = append(args, "--base=refs/heads/main")
					}
					if command == "chat" {
						args = append(args, "--launch-config=agent", "--view=chat", "--prompt=task")
					} else {
						args = append(args, "--", "true")
					}
					code = Run(context.Background(), e, args)
				}
				if (code == 0) != tc.succeeds {
					t.Fatalf("exit %d: %s", code, &stderr)
				}
				wantCalls := 1
				if tc.succeeds {
					wantCalls = 3
				}
				if len(choices) != wantCalls || choices[0] != nil {
					t.Fatalf("choices %#v", choices)
				}
				if tc.succeeds {
					for i, id := range []string{"first", "changed-refs"} {
						if got := choices[i+1]; got == nil || got.ConfirmationID != id || got.UseUpstream != tc.useUpstream {
							t.Fatalf("choice %d: %+v", i, got)
						}
					}
				}
				for _, base := range bases {
					if (!tc.diverged && base != "refs/heads/main") || (tc.diverged && base != "") {
						t.Fatalf("base identity changed: %q", base)
					}
				}
				for _, req := range requests {
					if req.ClientRequestID == "" || req.ClientRequestID != requests[0].ClientRequestID {
						t.Fatal("confirmation changed request identity")
					}
					if command == "chat" && (req.Prompt != "task" || req.View != contract.ViewChat) {
						t.Fatal("confirmation lost chat input")
					}
				}
				if !tc.interactive && tc.policy == "" && !strings.Contains(stderr.String(), "--upstream=") {
					t.Fatalf("missing script guidance: %s", &stderr)
				}
				if tc.diverged && tc.interactive && strings.Contains(stderr.String(), "[latest/local/cancel]") {
					t.Fatalf("offered unsafe upstream: %s", &stderr)
				}
			})
		}
	}
}

func TestRunUpstreamDryRunDoesNotFetchOrConfirm(t *testing.T) {
	var stdout, stderr bytes.Buffer
	e := Env{
		Stdout: &stdout, Stderr: &stderr,
		Connect: func() (*localclient.Client, error) {
			return &localclient.Client{HTTP: &http.Client{Transport: upstreamRoundTrip(func(r *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				withTestCapabilities(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/api/repositories" {
						t.Fatalf("dry run requested %s", r.URL.Path)
					}
					_, _ = io.WriteString(w, `[{"id":"repo0001","anchorPath":"/repo","worktrees":[]}]`)
				})).ServeHTTP(w, r)
				return w.Result(), nil
			})}}, nil
		},
		ResolveCwd:  func(context.Context, string) (string, error) { return "/repo", nil },
		Interactive: func() bool { t.Fatal("dry run asked for confirmation"); return false },
	}
	if code := Run(context.Background(), e, []string{"--new-worktree", "--repo=repo0001", "--dry-run", "--upstream=latest", "--", "true"}); code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	var out struct {
		Request contract.CreateSessionRequest `json:"request"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Request.NewWorktree == nil || out.Request.NewWorktree.Base != "" || out.Request.NewWorktree.UpstreamChoice != nil {
		t.Fatalf("unexpected dry request: %+v", out.Request.NewWorktree)
	}
}
