package agentcheck

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPyPIReleaseMetadataSize(t *testing.T) {
	for _, tt := range []struct {
		name      string
		bodyBytes int
		wantError bool
	}{
		{name: "full release history", bodyBytes: 176704},
		{name: "response limit", bodyBytes: maxUpstreamBodyBytes},
		{name: "oversized response", bodyBytes: maxUpstreamBodyBytes + 1, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			const prefix = `{"info":{"version":"3.0.0"},"description":"`
			const suffix = `"}`
			body := prefix + strings.Repeat("x", tt.bodyBytes-len(prefix)-len(suffix)) + suffix
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/pypi/kimi-cli/json" {
					http.NotFound(w, r)
					return
				}
				_, _ = fmt.Fprint(w, body)
			}))
			defer server.Close()
			service := New(Options{PyPIBase: server.URL})
			version, err := service.queryPyPI(context.Background(), "kimi-cli")
			if tt.wantError {
				if err == nil || !strings.Contains(err.Error(), "上游响应体积超出上限") {
					t.Fatalf("oversized response: version=%q error=%v", version, err)
				}
				return
			}
			if err != nil || version != "3.0.0" {
				t.Fatalf("release metadata: version=%q error=%v", version, err)
			}
		})
	}
}
