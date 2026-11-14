package pluginapi

import (
	"testing"
)

func TestRewriteLocation(t *testing.T) {
	for _, row := range []struct {
		in    string
		strip bool
		want  string
	}{
		{"http://localhost:5173/login", true, "/forward/5173/login"},
		{"http://127.0.0.1:5173/forward/5173/x?y=1", false, "/forward/5173/x?y=1"},
		{"/login", true, "/forward/5173/login"},
		{"/forward/5173/login", false, "/forward/5173/login"},
		{"https://example.com/", true, "https://example.com/"},
		{"http://localhost:5174/", true, "http://localhost:5174/"},
		{"//example.com/", true, "//example.com/"},
		{"next", true, "next"},
	} {
		if got := rewriteLocation(row.in, 5173, "/forward/5173", row.strip); got != row.want {
			t.Errorf("rewriteLocation(%q, strip=%v) = %q, want %q", row.in, row.strip, got, row.want)
		}
	}
}

func TestForwardPortParsing(t *testing.T) {
	for raw, want := range map[string]bool{"1": true, "65535": true, "5173": true, "0": false, "05173": false, "65536": false, "-1": false, "+1": false, "1e3": false, "": false, "123456": false} {
		if _, ok := forwardPort(raw); ok != want {
			t.Errorf("forwardPort(%q) ok=%v, want %v", raw, ok, want)
		}
	}
}
