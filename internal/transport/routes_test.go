package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func testCoreRoutes(mux Registrar) {
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /api/sessions/{id}/input", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/", http.NotFound)
}

func TestContributionsRejectConflictsAndReservedOverlaps(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	tests := []struct {
		name   string
		routes []Route
	}{
		{"exact duplicate", []Route{{Pattern: "GET /api/plugin", Handler: handler, Exposure: All}, {Pattern: "GET /api/plugin", Handler: handler, Exposure: All}}},
		{"wildcard equivalent", []Route{{Pattern: "GET /api/plugin/{id}", Handler: handler, Exposure: TCP}, {Pattern: "GET /api/plugin/{name}", Handler: handler, Exposure: Local}}},
		{"ambiguous overlap", []Route{{Pattern: "GET /api/plugin/{id}/x", Handler: handler, Exposure: All}, {Pattern: "GET /api/plugin/fixed/{name}", Handler: handler, Exposure: All}}},
		{"reserved exact", []Route{{Pattern: "GET /api/health", Handler: handler, Exposure: All}}},
		{"reserved wildcard", []Route{{Pattern: "GET /api/{rest...}", Handler: handler, Exposure: All}}},
		{"reserved operation wildcard", []Route{{Pattern: "POST /api/sessions/{id}/{operation}", Handler: handler, Exposure: All}}},
		{"reserved local", []Route{{Pattern: "GET /local/v1/token", Handler: handler, Exposure: Local}}},
	}
	for _, give := range tests {
		t.Run(give.name, func(t *testing.T) {
			if _, err := Mount(testCoreRoutes, []Contribution{{Owner: "code", Routes: give.routes}}, TCP, http.NotFoundHandler()); err == nil {
				t.Fatal("invalid contribution accepted before readiness")
			}
		})
	}
}

func TestContributionExposureUsesSameBoundHandler(t *testing.T) {
	calls := 0
	bound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusNoContent) })
	contributions := []Contribution{{Owner: "code", Routes: []Route{{Pattern: "GET /api/plugin", Handler: bound, Exposure: All}, {Pattern: "POST /api/plugin/tcp", Handler: bound, Exposure: TCP}}}}
	for _, exposure := range []Exposure{TCP, Local, P2P} {
		mux, err := Mount(testCoreRoutes, contributions, exposure, http.NotFoundHandler())
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/plugin", nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("shared route: %d", response.Code)
		}
		response = httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/plugin/tcp", nil))
		want := http.StatusNotFound
		if exposure == TCP {
			want = http.StatusNoContent
		}
		if response.Code != want {
			t.Fatalf("listener %d: %d want %d", exposure, response.Code, want)
		}
	}
	if calls != 4 {
		t.Fatalf("bound handler invoked %d times", calls)
	}
}

func TestShellContributionGuardsHostContinuationAndExposure(t *testing.T) {
	shell := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("host shell"))
	})
	contribution := Contribution{Owner: "forward", Routes: []Route{{
		Pattern: "GET /forward-auth", Exposure: TCP,
		Shell: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Confirmed") != "yes" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
			})
		},
	}}}
	for _, exposure := range []Exposure{TCP, Local, P2P} {
		mux, err := Mount(testCoreRoutes, []Contribution{contribution}, exposure, shell)
		if err != nil {
			t.Fatal(err)
		}
		for _, confirmed := range []bool{false, true} {
			request := httptest.NewRequest(http.MethodGet, "/forward-auth?next=%2Fforward%2F5173%2F", nil)
			if confirmed {
				request.Header.Set("X-Confirmed", "yes")
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			want := http.StatusNotFound
			if exposure == TCP {
				want = http.StatusForbidden
				if confirmed {
					want = http.StatusOK
				}
			}
			if response.Code != want {
				t.Fatalf("listener %d confirmed=%t: %d, want %d", exposure, confirmed, response.Code, want)
			}
			if gotShell := response.Body.String() == "host shell"; gotShell != (want == http.StatusOK) {
				t.Fatalf("listener %d confirmed=%t shell continuation: %q", exposure, confirmed, response.Body.String())
			}
		}
	}
}
