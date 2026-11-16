package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/agentcatalog"
	"github.com/rainchan/webmux/internal/agentcheck"
	"github.com/rainchan/webmux/internal/agentprofile/agents"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
)

func TestAgentToolsAuthOriginFeatureGateAndCheck(t *testing.T) {
	cwd := t.TempDir()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte("#!/bin/sh\necho 'codex-cli 0.160.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/@openai/codex/latest" {
			_, _ = w.Write([]byte(`{"version":"0.165.0"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer upstream.Close()

	m := session.NewManager(session.Options{DefaultCwd: cwd})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})

	checker := agentcheck.New(agentcheck.Options{
		Entries:       agentcatalog.Build(agents.Default()),
		ShellPath:     "/bin/sh",
		Env:           []string{"PATH=" + binDir},
		NPMBases:      []string{upstream.URL},
		PyPIBase:      upstream.URL,
		GitHubAPIBase: upstream.URL,
	})
	handler := New(testDeps(t, testServices{
		Config:     config.Config{Token: "secret", DefaultCwd: cwd},
		Manager:    m,
		Launch:     openLaunchStore(t),
		AgentTools: checker,
	}))

	// Unauthenticated requests are rejected with 401.
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/agent-tools"},
		{"POST", "/api/agent-tools/check"},
	} {
		req := httptest.NewRequest(tc.method, "http://webmux.example"+tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: got %d, want 401", tc.method, tc.path, rec.Code)
		}
	}

	// Cross-origin POST is rejected with 403.
	crossReq := httptest.NewRequest("POST", "http://webmux.example/api/agent-tools/check", strings.NewReader(`{}`))
	crossReq.Header.Set("Authorization", "Bearer secret")
	crossReq.Header.Set("Origin", "https://evil.example")
	crossRec := httptest.NewRecorder()
	handler.ServeHTTP(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Errorf("cross-origin POST /api/agent-tools/check: got %d, want 403", crossRec.Code)
	}

	// Local inspect succeeds without upstream check.
	getReq := httptest.NewRequest("GET", "http://webmux.example/api/agent-tools?refresh=1", nil)
	getReq.Header.Set("Authorization", "Bearer secret")
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /api/agent-tools: got %d: %s", getRec.Code, getRec.Body.String())
	}
	var report contract.AgentToolsReport
	if err := json.Unmarshal(getRec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode inspect report: %v", err)
	}
	if report.RemoteCheckedAt != nil {
		t.Errorf("RemoteCheckedAt = %v, want nil before check", report.RemoteCheckedAt)
	}

	// Explicit check with empty body succeeds and populates update_available.
	postReq := httptest.NewRequest("POST", "http://webmux.example/api/agent-tools/check", nil)
	postReq.Header.Set("Authorization", "Bearer secret")
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("POST /api/agent-tools/check: got %d: %s", postRec.Code, postRec.Body.String())
	}
	if err := json.Unmarshal(postRec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode check report: %v", err)
	}
	if report.RemoteCheckedAt == nil {
		t.Error("RemoteCheckedAt = nil, want non-nil after check")
	}
	foundCodex := false
	for _, tool := range report.Tools {
		if tool.ID == "codex" {
			foundCodex = true
			if !tool.Installed || tool.InstalledVersion != "0.160.0" || tool.LatestVersion != "0.165.0" || tool.UpdateStatus != "update_available" {
				t.Errorf("codex tool = %+v, want installed 0.160.0 -> 0.165.0 update_available", tool)
			}
			if tool.Connector == nil || tool.Connector.Installed {
				t.Errorf("codex connector = %+v, want missing codex-acp", tool.Connector)
			}
		}
	}
	if !foundCodex {
		t.Error("codex entry missing from report")
	}

	// Unknown or duplicate query parameters on GET /api/agent-tools deterministically return 400.
	for range 40 {
		for _, badQuery := range []string{
			"?refresh=1&bogus=1",
			"?bogus=1&refresh=0",
			"?refresh=1&refresh=0",
			"?refresh=invalid",
		} {
			badGet := httptest.NewRequest("GET", "http://webmux.example/api/agent-tools"+badQuery, nil)
			badGet.Header.Set("Authorization", "Bearer secret")
			badRec := httptest.NewRecorder()
			handler.ServeHTTP(badRec, badGet)
			if badRec.Code != http.StatusBadRequest {
				t.Fatalf("GET /api/agent-tools%s: got %d, want 400", badQuery, badRec.Code)
			}
		}
	}

	// Unknown JSON field on POST /api/agent-tools/check is rejected with 400.
	badBodyReq := httptest.NewRequest("POST", "http://webmux.example/api/agent-tools/check", strings.NewReader(`{"unknown":true}`))
	badBodyReq.Header.Set("Authorization", "Bearer secret")
	badBodyRec := httptest.NewRecorder()
	handler.ServeHTTP(badBodyRec, badBodyReq)
	if badBodyRec.Code != http.StatusBadRequest {
		t.Errorf("unknown field POST: got %d, want 400", badBodyRec.Code)
	}

	// Feature disabled (features.enabled = []) returns 404 even when authenticated.
	disabledCfg := config.Config{
		Token:      "secret",
		DefaultCwd: cwd,
		Features:   config.FeatureConfig{Enabled: []string{}},
	}
	disabledHandler := New(testDeps(t, testServices{
		Config:     disabledCfg,
		Manager:    m,
		AgentTools: checker,
	}))
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/agent-tools"},
		{"POST", "/api/agent-tools/check"},
	} {
		req := httptest.NewRequest(tc.method, "http://webmux.example"+tc.path, nil)
		req.Header.Set("Authorization", "Bearer secret")
		rec := httptest.NewRecorder()
		disabledHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("disabled launch %s %s: got %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
}
