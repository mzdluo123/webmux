package api

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/session"
)

func TestTerminalThemeSecurityValidationAndBroadcast(t *testing.T) {
	dir := t.TempDir()
	m := session.NewManager(session.Options{DefaultCwd: dir})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	handler := New(testDeps(t, testServices{Config: config.Config{Token: "login-secret", DefaultCwd: dir}, Manager: m}))

	for _, row := range []struct {
		name, token, origin, body string
		want                      int
	}{
		{name: "unauthenticated", token: "", body: `{"theme":"dark"}`, want: http.StatusUnauthorized},
		{name: "foreign origin", token: "login-secret", origin: "https://evil.example", body: `{"theme":"dark"}`, want: http.StatusForbidden},
		{name: "invalid theme", token: "login-secret", body: `{"theme":"blue"}`, want: http.StatusBadRequest},
		{name: "unknown field", token: "login-secret", body: `{"theme":"dark","extra":true}`, want: http.StatusBadRequest},
	} {
		t.Run(row.name, func(t *testing.T) {
			w := tunnelAPIRequest(handler, http.MethodPost, "/api/terminal/theme", row.body, row.token, row.origin)
			if w.Code != row.want {
				t.Fatalf("status %d, want %d: %s", w.Code, row.want, w.Body.String())
			}
		})
	}

	enabledPath := filepath.Join(dir, "enabled-input")
	disabledPath := filepath.Join(dir, "disabled-input")
	create := func(enable bool, path string) *session.Session {
		t.Helper()
		mode := ""
		if enable {
			mode = `printf '\033[?2031h'; `
		}
		item, err := m.Create(session.CreateInput{Cmd: []string{"/bin/bash", "-c", `stty raw -echo; ` + mode + `exec cat > "$0"`, path}})
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	enabled := create(true, enabledPath)
	create(false, disabledPath)
	deadline := time.Now().Add(5 * time.Second)
	for !bytes.Contains(enabled.Buffer(), []byte("\x1b[?2031h")) {
		if time.Now().After(deadline) {
			t.Fatal("program did not enable color scheme reports")
		}
		time.Sleep(10 * time.Millisecond)
	}

	post := func(theme string, want int) {
		t.Helper()
		w := tunnelAPIRequest(handler, http.MethodPost, "/api/terminal/theme", `{"theme":"`+theme+`"}`, "login-secret", "")
		if w.Code != want {
			t.Fatalf("theme %s status %d, want %d: %s", theme, w.Code, want, w.Body.String())
		}
	}
	post("dark", http.StatusNoContent)
	post("dark", http.StatusNoContent)
	post("light", http.StatusNoContent)

	wantEnabled := "\x1b[?997;1n\x1b[?997;1n\x1b[?997;2n"
	deadline = time.Now().Add(5 * time.Second)
	for {
		enabledInput, _ := os.ReadFile(enabledPath)
		disabledInput, _ := os.ReadFile(disabledPath)
		if string(enabledInput) == wantEnabled {
			if len(disabledInput) != 0 {
				t.Fatalf("session without reports received %q", disabledInput)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("enabled input %q, want %q; disabled input %q", enabledInput, wantEnabled, disabledInput)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
