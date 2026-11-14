package api

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/push"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/transport"
)

func browserSubscription(t *testing.T, endpoint string) string {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	// Same shape as PushSubscription.toJSON() in browsers, including the
	// always-present expirationTime.
	body, _ := json.Marshal(map[string]any{"endpoint": endpoint, "expirationTime": nil, "keys": map[string]string{
		"p256dh": base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		"auth":   base64.RawURLEncoding.EncodeToString(auth),
	}})
	return string(body)
}

func TestPushAPI(t *testing.T) {
	pushService := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/gone") {
			w.WriteHeader(http.StatusGone)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer pushService.Close()
	m := session.NewManager(session.Options{})
	defer m.Shutdown(context.Background())
	cfg := config.Config{Listen: "127.0.0.1:7681", Token: "secret", Push: config.PushConfig{Enabled: true, Subject: config.DefaultPushSubject, Kinds: config.DefaultPushKinds()}}
	pushes := push.New(cfg.Push, t.TempDir(), m, slog.New(slog.NewTextHandler(io.Discard, nil)), pushService.Client().Transport)
	defer pushes.Close()
	public := &transport.PublicOrigins{}
	const (
		loopback        = "http://127.0.0.1:7681"
		quickOrigin     = "https://abc.trycloudflare.com"
		tailscaleOrigin = "https://node.tailnet.ts.net:8443"
	)
	publicHosts := map[string]string{
		quickOrigin:     "abc.trycloudflare.com",
		tailscaleOrigin: "node.tailnet.ts.net:8443",
	}
	if err := public.Set("cloudflare", quickOrigin); err != nil {
		t.Fatal(err)
	}
	if err := public.Set("tailscale", tailscaleOrigin); err != nil {
		t.Fatal(err)
	}
	handler := New(testDeps(t, testServices{Config: cfg, Manager: m, Workspace: testWorkspace(t, m, t.TempDir()), Public: public, Push: pushes}))
	var bodies strings.Builder
	// A public origin must also match the Host the browser sends. Unknown
	// origins deliberately keep the loopback Host so the Origin check, not the
	// Host check, is what rejects them.
	call := func(method, path, origin, token, body string) (int, map[string]any) {
		t.Helper()
		host := "127.0.0.1:7681"
		if publicHost, ok := publicHosts[origin]; ok {
			host = publicHost
		}
		r := httptest.NewRequest(method, "http://"+host+path, strings.NewReader(body))
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		bodies.WriteString(w.Body.String())
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	if code, _ := call("GET", "/api/push", loopback, "", ""); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status: %d", code)
	}
	code, status := call("GET", "/api/push", "", "secret", "")
	if code != 200 || status["enabled"] != true || status["available"] != true || status["subscriptions"] != float64(0) || len(status["publicKey"].(string)) != 87 || status["reason"] != nil {
		t.Fatalf("loopback status %d %v", code, status)
	}
	if code, status := call("GET", "/api/push", quickOrigin, "secret", ""); code != 200 || status["available"] != false || status["reason"] != "quick_tunnel" || status["publicKey"] != nil {
		t.Fatalf("quick tunnel status %d %v", code, status)
	}
	if code, status := call("GET", "/api/push", tailscaleOrigin, "secret", ""); code != 200 || status["available"] != true || status["publicKey"] == nil {
		t.Fatalf("tailscale status %d %v", code, status)
	}
	tailscaleDevice := browserSubscription(t, pushService.URL+"/device/TAILSCALE-PATH")
	code, tailscale := call("POST", "/api/push/subscriptions", tailscaleOrigin, "secret", tailscaleDevice)
	if code != http.StatusOK || tailscale["id"] == nil {
		t.Fatalf("tailscale registration: %d %v", code, tailscale)
	}
	if code, _ := call("DELETE", "/api/push/subscriptions/"+tailscale["id"].(string), tailscaleOrigin, "secret", ""); code != http.StatusNoContent {
		t.Fatalf("tailscale unsubscribe: %d", code)
	}

	device := browserSubscription(t, pushService.URL+"/device/SECRET-PATH")
	if code, _ := call("POST", "/api/push/subscriptions", "https://evil.example", "secret", device); code != http.StatusForbidden {
		t.Fatalf("foreign origin: %d", code)
	}
	if code, out := call("POST", "/api/push/subscriptions", quickOrigin, "secret", device); code != http.StatusConflict || out["reason"] != "quick_tunnel" || out["error"] == nil || out["error"] == "" {
		t.Fatalf("quick tunnel registration: %d %v", code, out)
	}
	if code, out := call("POST", "/api/push/subscriptions", quickOrigin, "secret", `{}`); code != http.StatusBadRequest || out["reason"] != nil {
		t.Fatalf("malformed subscription must precede origin conflict: %d %v", code, out)
	}
	if code, _ := call("POST", "/api/push/subscriptions", loopback, "secret", browserSubscription(t, "http://push.example/x")); code != http.StatusBadRequest {
		t.Fatalf("http endpoint: %d", code)
	}
	_, first := call("POST", "/api/push/subscriptions", loopback, "secret", device)
	code, second := call("POST", "/api/push/subscriptions", loopback, "secret", device)
	if code != 200 || first["id"] == nil || first["id"] != second["id"] {
		t.Fatalf("idempotent registration: %v %v", first, second)
	}
	_, gone := call("POST", "/api/push/subscriptions", loopback, "secret", browserSubscription(t, pushService.URL+"/gone/SECRET-PATH"))
	if _, status := call("GET", "/api/push", "", "secret", ""); status["subscriptions"] != float64(2) {
		t.Fatalf("subscriptions %v", status)
	}

	// The test reaches every device: one accepts, one is expired and deleted.
	code, report := call("POST", "/api/push/test", loopback, "secret", "")
	removed, _ := report["removed"].([]any)
	if code != http.StatusOK || report["devices"] != float64(2) || report["accepted"] != float64(1) || len(removed) != 1 || removed[0] != gone["id"] || len(report["errors"].([]any)) != 0 {
		t.Fatalf("test notification: %d %v", code, report)
	}
	if _, status := call("GET", "/api/push", "", "secret", ""); status["subscriptions"] != float64(1) {
		t.Fatalf("gone subscription kept: %v", status)
	}
	for range 2 {
		if code, _ := call("DELETE", "/api/push/subscriptions/"+first["id"].(string), loopback, "secret", ""); code != http.StatusNoContent {
			t.Fatalf("delete: %d", code)
		}
	}
	if _, status := call("GET", "/api/push", "", "secret", ""); status["subscriptions"] != float64(0) {
		t.Fatalf("after delete: %v", status)
	}
	call("GET", "/api/config", "", "secret", "")
	var keys map[string]map[string]string
	_ = json.Unmarshal([]byte(device), &keys)
	for _, secret := range []string{"SECRET-PATH", keys["keys"]["p256dh"], keys["keys"]["auth"]} {
		if strings.Contains(bodies.String(), secret) {
			t.Fatalf("response leaked %q", secret)
		}
	}
}

func TestPushAPIWhenDisabled(t *testing.T) {
	m := session.NewManager(session.Options{})
	defer m.Shutdown(context.Background())
	handler := New(testDeps(t, testServices{Config: config.Config{}, Manager: m, Workspace: testWorkspace(t, m, t.TempDir())}))
	for _, row := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/api/push", "", http.StatusOK},
		{"POST", "/api/push/subscriptions", browserSubscription(t, "https://push.example/x"), http.StatusConflict},
		{"POST", "/api/push/subscriptions", `{}`, http.StatusConflict},
		{"POST", "/api/push/test", "", http.StatusConflict},
		{"DELETE", "/api/push/subscriptions/x", "", http.StatusConflict},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(row.method, "http://127.0.0.1:7681"+row.path, strings.NewReader(row.body)))
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		if w.Code != row.want || out["reason"] != "disabled" {
			t.Errorf("%s %s: %d %v", row.method, row.path, w.Code, out)
		}
		if row.path == "/api/push" && (out["enabled"] != false || out["available"] != false) {
			t.Errorf("disabled status %v", out)
		}
	}
}

func TestPushStorageFailuresAreLoggedWithoutSubscriptionSecrets(t *testing.T) {
	// No parallelism: the API intentionally uses the process-wide slog logger.
	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(original)
	m := session.NewManager(session.Options{})
	defer m.Shutdown(context.Background())
	dir := t.TempDir()
	cfg := config.PushConfig{Enabled: true, Subject: config.DefaultPushSubject, Kinds: config.DefaultPushKinds()}
	pushes := push.New(cfg, dir, m, slog.Default(), nil)
	defer pushes.Close()
	if err := pushes.Unavailable(); err != nil {
		t.Fatal(err)
	}
	const endpoint = "https://push.example.invalid/PRIVATE-ENDPOINT"
	body := browserSubscription(t, endpoint)
	var subscription contract.PushSubscriptionRequest
	if err := json.Unmarshal([]byte(body), &subscription); err != nil {
		t.Fatal(err)
	}
	id, err := pushes.Register(endpoint, subscription.Keys.P256dh, subscription.Keys.Auth, "http://localhost")
	if err != nil {
		t.Fatal(err)
	}
	// Replacing the destination file with a directory deterministically makes
	// atomic rename fail, even when tests run with elevated filesystem access.
	path := filepath.Join(dir, "push.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	handler := New(testDeps(t, testServices{Config: config.Config{Token: "secret"}, Manager: m, Push: pushes}))
	for _, row := range []struct{ method, path, body, message, response string }{
		{"POST", "/api/push/subscriptions", body, "save push subscription", "推送订阅保存失败，请查看服务日志"},
		{"DELETE", "/api/push/subscriptions/" + id, "", "delete push subscription", "推送订阅删除失败，请查看服务日志"},
	} {
		logs.Reset()
		r := httptest.NewRequest(row.method, "http://localhost"+row.path, strings.NewReader(row.body))
		r.Header.Set("Authorization", "Bearer secret")
		r.Header.Set("Origin", "http://localhost")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), row.response) {
			t.Fatalf("response: %d %s", w.Code, w.Body.String())
		}
		if !strings.Contains(logs.String(), row.message) || !strings.Contains(logs.String(), "push.json") {
			t.Fatalf("missing diagnostic: %s", logs.String())
		}
		for _, secret := range []string{endpoint, subscription.Keys.Auth, subscription.Keys.P256dh} {
			if strings.Contains(logs.String()+w.Body.String(), secret) {
				t.Fatal("subscription secret leaked")
			}
		}
		if strings.Contains(w.Body.String(), dir) {
			t.Fatal("internal storage path leaked to response")
		}
	}
}
