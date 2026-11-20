package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/perf"
	"github.com/rainchan/webmux/internal/session"
)

func TestPerfConfigTimingAndEventIngestion(t *testing.T) {
	dir := t.TempDir()
	rec, err := perf.New(perf.Options{
		Dir:           dir,
		SlowMS:        1,
		SampleSeconds: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	mgr := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer mgr.Shutdown(t.Context())

	handler := api.New(api.Deps{
		Config:  config.Config{Listen: "127.0.0.1:7681", Token: "tok"},
		Manager: mgr,
		Perf:    rec,
	})

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7681/api/config", nil)
	req.Header.Set("Authorization", "Bearer tok")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("GET /api/config status = %d", res.Code)
	}
	if st := res.Header().Get("Server-Timing"); !strings.Contains(st, "app;dur=") {
		t.Fatalf("missing Server-Timing header: %q", st)
	}
	var cfg contract.ServerConfig
	if err := json.Unmarshal(res.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Perf == nil || !cfg.Perf.Enabled || cfg.Perf.SlowMS != 1 {
		t.Fatalf("unexpected config.perf: %+v", cfg.Perf)
	}

	// Reject unauthenticated and cross-origin POST /api/perf/events.
	unauth := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7681/api/perf/events", strings.NewReader(`{"events":[]}`))
	unauthRes := httptest.NewRecorder()
	handler.ServeHTTP(unauthRes, unauth)
	if unauthRes.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", unauthRes.Code)
	}

	badOrigin := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7681/api/perf/events", strings.NewReader(`{"events":[]}`))
	badOrigin.Header.Set("Authorization", "Bearer tok")
	badOrigin.Header.Set("Origin", "https://evil.example")
	badOriginRes := httptest.NewRecorder()
	handler.ServeHTTP(badOriginRes, badOrigin)
	if badOriginRes.Code != http.StatusForbidden {
		t.Fatalf("bad origin status = %d, want 403", badOriginRes.Code)
	}

	// Accept valid authenticated batch.
	body, _ := json.Marshal(contract.PerfEventBatch{
		Events: []contract.PerfEventInput{
			{Kind: "loaf", Op: "ChatView.svelte:flush", DurationMS: 88.5, Route: "/s/:id", Detail: "blocking=30ms"},
		},
	})
	postReq := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7681/api/perf/events", bytes.NewReader(body))
	postReq.Header.Set("Authorization", "Bearer tok")
	postReq.Header.Set("Origin", "http://127.0.0.1:7681")
	postRes := httptest.NewRecorder()
	handler.ServeHTTP(postRes, postReq)
	if postRes.Code != http.StatusNoContent {
		t.Fatalf("POST /api/perf/events status = %d, body=%s", postRes.Code, postRes.Body.String())
	}

	rec.Close()
	rep, err := perf.Summarize(dir, time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.FrontendHotOps) != 1 || rep.FrontendHotOps[0].Op != "ChatView.svelte:flush" {
		t.Fatalf("frontend event not persisted: %+v", rep.FrontendHotOps)
	}
}

func TestPerfDisabledReturns404AndOmitsConfigHint(t *testing.T) {
	mgr := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer mgr.Shutdown(t.Context())
	handler := api.New(api.Deps{
		Config:  config.Config{Listen: "127.0.0.1:7681", Token: "tok"},
		Manager: mgr,
	})

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7681/api/perf/events", strings.NewReader(`{"events":[]}`))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Origin", "http://127.0.0.1:7681")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("disabled perf status = %d, want 404", res.Code)
	}
}
