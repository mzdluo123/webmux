package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestAuthCLICommandsOfflineAndLive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	cfgPath := filepath.Join(dir, "config.toml")
	body := "listen = '127.0.0.1:0'\nstate_dir = '" + state + "'\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	runAuth := func(args ...string) (string, string, int) {
		t.Helper()
		var out, stderr bytes.Buffer
		full := append([]string{"auth"}, args...)
		code := dispatch(full, &out, &stderr)
		return out.String(), stderr.String(), code
	}

	// Offline auth status reports needs_onboarding.
	stdout, stderr, code := runAuth("status", "--config", cfgPath, "--json")
	if code != contract.ExitOK {
		t.Fatalf("offline auth status: exit %d stderr=%q", code, stderr)
	}
	var summary contract.AuthSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode auth status: %v (%s)", err, stdout)
	}
	if summary.State != "needs_onboarding" || summary.PasskeyCount != 0 {
		t.Fatalf("unexpected offline summary: %+v", summary)
	}

	// auth reset requires --force.
	_, stderr, code = runAuth("reset", "--config", cfgPath)
	if code != contract.ExitUsage || !strings.Contains(stderr, "--force") {
		t.Fatalf("reset without --force: code=%d stderr=%q", code, stderr)
	}

	// Offline auth reset --force succeeds cleanly.
	stdout, stderr, code = runAuth("reset", "--config", cfgPath, "--force", "--json")
	if code != contract.ExitOK || !strings.Contains(stdout, `"ok": true`) {
		t.Fatalf("offline reset: code=%d out=%q err=%q", code, stdout, stderr)
	}

	// Start a live Passkey-onboarding instance and test auth setup + auth reset over local socket.
	f := newFixture(t, "")
	// Replace fixture config with empty token so Passkey onboarding is active.
	if err := os.WriteFile(f.config, []byte("listen = '127.0.0.1:0'\nstate_dir = '"+f.state+"'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := f.start(t, nil, true, "--config", f.config)
	defer func() {
		_, _, _ = f.run(t, "stop")
		srv.waitExit(t)
	}()

	eventually(t, "setup link printed", func() bool {
		return strings.Contains(srv.stderr.String(), "/onboarding#setup=")
	})

	// Live auth setup --json returns current setup code and localhost URL.
	stdout, stderr, code = f.run(t, "auth", "setup", "--json")
	if code != contract.ExitOK {
		t.Fatalf("live auth setup: code=%d stderr=%q", code, stderr)
	}
	var setup contract.LocalAuthSetupResponse
	if err := json.Unmarshal([]byte(stdout), &setup); err != nil {
		t.Fatalf("decode setup: %v (%s)", err, stdout)
	}
	if setup.Code == "" || !strings.Contains(setup.SetupURL, "http://localhost:") || !strings.Contains(setup.SetupURL, "/onboarding#setup=") {
		t.Fatalf("unexpected setup info: %+v", setup)
	}

	// Refresh rotates setup code.
	stdout, stderr, code = f.run(t, "auth", "setup", "--refresh", "--json")
	if code != contract.ExitOK {
		t.Fatalf("live auth setup --refresh: code=%d stderr=%q", code, stderr)
	}
	var refreshed contract.LocalAuthSetupResponse
	if err := json.Unmarshal([]byte(stdout), &refreshed); err != nil {
		t.Fatalf("decode refreshed setup: %v (%s)", err, stdout)
	}
	if refreshed.Code == "" || refreshed.Code == setup.Code {
		t.Fatalf("refresh did not rotate setup code: %q -> %q", setup.Code, refreshed.Code)
	}

	// Live auth reset --force returns fresh setup info.
	stdout, stderr, code = f.run(t, "auth", "reset", "--force", "--json")
	if code != contract.ExitOK || !strings.Contains(stdout, "/onboarding#setup=") {
		t.Fatalf("live auth reset: code=%d out=%q err=%q", code, stdout, stderr)
	}
}
