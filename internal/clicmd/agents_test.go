//go:build linux || darwin

package clicmd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
)

func TestAgentsCommandModesAndFiltering(t *testing.T) {
	checkedAt := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	remoteCheckedAt := time.Date(2026, 4, 15, 12, 1, 0, 0, time.UTC)

	sampleReport := func(remote bool) contract.AgentToolsReport {
		status := "unchecked"
		latest := ""
		var remoteAt *time.Time
		if remote {
			status = "update_available"
			latest = "2.2.0"
			remoteAt = &remoteCheckedAt
		}
		return contract.AgentToolsReport{
			CheckedAt:       checkedAt,
			RemoteCheckedAt: remoteAt,
			Shell:           "/bin/bash",
			Tools: []contract.AgentToolStatus{
				{
					ID:               "alpha",
					Title:            "Alpha Agent",
					Kind:             "hybrid",
					Installed:        true,
					Executable:       "alpha-cli",
					Executables:      []string{"alpha-cli"},
					Path:             "/usr/local/bin/alpha-cli",
					InstalledVersion: "2.0.5",
					LatestVersion:    latest,
					VerifiedVersion:  "2.1.0",
					BelowVerified:    true,
					UpdateStatus:     status,
					Connector: &contract.AgentConnectorStatus{
						Executable:     "alpha-acp",
						Title:          "alpha-acp",
						Installed:      false,
						UpdateStatus:   "not_installed",
						InstallCommand: "npm install -g alpha-acp",
					},
				},
				{
					ID:           "beta",
					Title:        "Beta Agent",
					Kind:         "terminal",
					Installed:    false,
					Executable:   "beta-cli",
					Executables:  []string{"beta-cli"},
					UpdateStatus: "not_installed",
				},
			},
		}
	}

	var gotRequests []string
	var lastCheckBody contract.AgentToolsCheckRequest
	e, stdout, stderr := serveUnix(t, make(chan struct{}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequests = append(gotRequests, r.Method+" "+r.URL.RequestURI())
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/agent-tools":
			_ = json.NewEncoder(w).Encode(sampleReport(false))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agent-tools/check":
			if err := json.NewDecoder(r.Body).Decode(&lastCheckBody); err != nil {
				t.Errorf("decode check body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(sampleReport(true))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}), testCapabilities("launch"))

	// 1. Default table prints installed tools + connector row and omits uninstalled tools.
	if code := e.Run(context.Background(), agentsCommand, nil); code != contract.ExitOK {
		t.Fatalf("default exit=%d stderr=%q", code, stderr)
	}
	out := stdout.String()
	for _, want := range []string{"Alpha Agent", "2.0.5", "unchecked (<2.1.0)", "/usr/local/bin/alpha-cli", "+ alpha-acp", "not_installed"} {
		if !strings.Contains(out, want) {
			t.Errorf("default output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Beta Agent") {
		t.Errorf("default output included uninstalled Beta Agent:\n%s", out)
	}

	// 2. --all --refresh sends ?refresh=1 and includes uninstalled tools.
	stdout.Reset()
	stderr.Reset()
	if code := e.Run(context.Background(), agentsCommand, []string{"--all", "--refresh"}); code != contract.ExitOK {
		t.Fatalf("--all --refresh exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout.String(), "Beta Agent") {
		t.Fatalf("--all output missing Beta Agent:\n%s", stdout)
	}

	// 3. --check --refresh --json calls POST /api/agent-tools/check and prints AgentToolsV1.
	stdout.Reset()
	stderr.Reset()
	if code := e.Run(context.Background(), agentsCommand, []string{"--check", "--refresh", "--json"}); code != contract.ExitOK {
		t.Fatalf("--check --json exit=%d stderr=%q", code, stderr)
	}
	if !lastCheckBody.RefreshLocal {
		t.Fatalf("expected RefreshLocal=true in POST /api/agent-tools/check body")
	}
	var payload contract.AgentToolsV1
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json output: %v\n%s", err, stdout)
	}
	if payload.V != 1 || len(payload.Tools) != 1 || payload.Tools[0].UpdateStatus != "update_available" || payload.Tools[0].LatestVersion != "2.2.0" {
		t.Fatalf("unexpected AgentToolsV1 payload: %+v", payload)
	}

	wantRequests := []string{
		"GET /api/agent-tools",
		"GET /api/agent-tools?refresh=1",
		"POST /api/agent-tools/check",
	}
	if strings.Join(gotRequests, "|") != strings.Join(wantRequests, "|") {
		t.Fatalf("requests=%v, want %v", gotRequests, wantRequests)
	}

	// 4. Positional arguments are rejected with usage exit code without hitting the server.
	stdout.Reset()
	stderr.Reset()
	if code := e.Run(context.Background(), agentsCommand, []string{"extra"}); code != contract.ExitUsage {
		t.Fatalf("extra arg exit=%d, want %d", code, contract.ExitUsage)
	}
}

func TestAgentsCommandFeatureDisabledAndOldServer(t *testing.T) {
	t.Run("feature disabled stops before endpoint", func(t *testing.T) {
		calls := 0
		e, _, stderr := serveUnix(t, make(chan struct{}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			http.NotFound(w, r)
		}), testCapabilities())
		if code := e.Run(context.Background(), agentsCommand, nil); code != contract.ExitError || calls != 0 || !strings.Contains(stderr.String(), "feature_disabled") {
			t.Fatalf("code=%d calls=%d stderr=%q", code, calls, stderr)
		}
	})

	t.Run("old server 404 reports version mismatch", func(t *testing.T) {
		e, _, stderr := serveUnix(t, make(chan struct{}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == contract.PathStatus {
				_ = json.NewEncoder(w).Encode(contract.StatusV1{V: 1, Version: "0.9.0"})
				return
			}
			http.NotFound(w, r)
		}), testCapabilities("launch"))
		e.Version = "1.2.0"
		if code := e.Run(context.Background(), agentsCommand, nil); code != contract.ExitError || !strings.Contains(stderr.String(), "too old") {
			t.Fatalf("code=%d stderr=%q", code, stderr)
		}
	})
}
