package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/streamproto"
)

func coreOnlyDeps(t *testing.T) Deps {
	t.Helper()
	cwd := t.TempDir()
	manager := session.NewManager(session.Options{DefaultCwd: cwd})
	t.Cleanup(func() { manager.Shutdown(context.Background()) })
	preferences := settings.Open(t.TempDir())
	t.Cleanup(preferences.Close)
	cfg := config.Config{DefaultCwd: cwd, Token: "core-secret", Plugins: config.PluginConfig{Enabled: []string{}}}
	return Deps{Config: cfg, Manager: manager, Settings: preferences, Create: sessioncreate.New(manager, sessioncreate.Options{DefaultCwd: cwd})}
}

func TestCoreOnlyHostKeepsCreateAndDiscovery(t *testing.T) {
	deps := coreOnlyDeps(t)
	bridge := NewBridge(deps)
	request := httptest.NewRequest(http.MethodPost, "http://webmux/api/sessions", strings.NewReader(`{"cmd":["/bin/sh","-c","printf core-ready; read value"]}`))
	request.Header.Set("Authorization", "Bearer core-secret")
	response := httptest.NewRecorder()
	bridge.HTTPHandler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("raw create: %d %s", response.Code, response.Body.String())
	}
	if len(deps.Manager.List()) != 1 {
		t.Fatal("raw create did not produce a real managed PTY")
	}
	discovery := bridge.ServeRPC(context.Background(), streamproto.RPCRequest{Method: "GET", Path: contract.PathCapabilities, Headers: map[string]string{"Authorization": "Bearer core-secret"}})
	if discovery.Status != http.StatusOK {
		t.Fatalf("discovery: %d %s", discovery.Status, discovery.Body)
	}
	var snapshot contract.Capabilities
	if err := json.Unmarshal(discovery.Body, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Version != 1 || len(snapshot.Plugins) != 14 || len(snapshot.Capabilities) != 15 {
		t.Fatalf("incomplete static discovery: %+v", snapshot)
	}
	for _, plugin := range snapshot.Plugins {
		if plugin.Enabled || plugin.Status != contract.PluginStatusDisabled || plugin.Reason != contract.CapabilityReasonPluginDisabled {
			t.Fatalf("disabled plugin claimed ready: %+v", plugin)
		}
	}
}

func TestDisabledBusinessRoutesAreNotStaticFallbacks(t *testing.T) {
	deps := coreOnlyDeps(t)
	bridge := NewBridge(deps)
	local := NewLocal(deps, Local{})
	paths := []string{"/api/repositories", "/api/launch-configs", "/api/fs/dirs", "/api/uploads", "/api/push", "/api/voice/settings", "/api/p2p/ping", "/api/tunnel", "/api/tailscale", "/ws/voice", "/mcp", "/mcp/", "/forward", "/forward?next=%2Fforward%2F5173%2F", "/forward/5173/", "/forward/_auth", "/forward-auth", "/forward-auth?confirm=1&next=%2Fforward%2F5173%2F"}
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		for _, path := range paths {
			for _, handler := range []http.Handler{bridge.HTTPHandler, local} {
				request := httptest.NewRequest(method, "http://webmux"+path, nil)
				request.Header.Set("Authorization", "Bearer core-secret")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusNotFound {
					t.Fatalf("%s %s: %d %s", method, path, response.Code, response.Body.String())
				}
			}
			direct := bridge.ServeRPC(context.Background(), streamproto.RPCRequest{Method: method, Path: path, Headers: map[string]string{"Authorization": "Bearer core-secret"}})
			if direct.Status != http.StatusNotFound {
				t.Fatalf("direct %s %s: %d", method, path, direct.Status)
			}
		}
	}
}

func TestDisabledMCPStillCrossesSecurityBoundary(t *testing.T) {
	handler := New(coreOnlyDeps(t))
	for _, give := range []struct {
		token, origin string
		want          int
	}{{"", "", 401}, {"wrong", "", 401}, {"core-secret", "https://evil.example", 403}, {"core-secret", "", 404}} {
		for _, path := range []string{"/mcp", "/mcp/"} {
			request := httptest.NewRequest(http.MethodPost, "http://webmux"+path, nil)
			if give.token != "" {
				request.Header.Set("Authorization", "Bearer "+give.token)
			}
			if give.origin != "" {
				request.Header.Set("Origin", give.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != give.want {
				t.Fatalf("%s security: %d, want %d", path, response.Code, give.want)
			}
		}
	}
}

func TestInactivePluginSettingsRemainStoredButNotWritable(t *testing.T) {
	deps := coreOnlyDeps(t)
	commands := []settings.Command{{ID: "kept", Name: "Kept", Command: "pwd"}}
	stored, err := deps.Settings.Patch(0, settings.Changes{Fields: map[string]bool{"quickCommands": true}, Values: settings.Values{QuickCommands: &commands}})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(deps)
	for _, field := range []string{"quickCommands", "preview", "codeReader", "homeWorktrees", "ignoreHighPorts"} {
		request := httptest.NewRequest(http.MethodPatch, "http://webmux/api/settings", strings.NewReader(`{"baseRevision":1,"changes":{"`+field+`":null}}`))
		request.Header.Set("Authorization", "Bearer core-secret")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("inactive %s writable: %d", field, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPatch, "http://webmux/api/settings", strings.NewReader(`{"baseRevision":1,"changes":{"theme":"dark"}}`))
	request.Header.Set("Authorization", "Bearer core-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("core preferences: %d %s", response.Code, response.Body.String())
	}
	var snapshot contract.SettingsSnapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Revision != stored.Revision+1 || snapshot.Values.QuickCommands == nil || len(*snapshot.Values.QuickCommands) != 1 {
		t.Fatalf("inactive value lost: %+v", snapshot)
	}
}
