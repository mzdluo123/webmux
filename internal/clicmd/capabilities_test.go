//go:build linux || darwin

package clicmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/builtin"
	"github.com/rainchan/webmux/internal/contract"
)

func testCapabilities(enabled ...string) contract.Capabilities {
	if enabled == nil {
		enabled = []string{}
	}
	snapshot := contract.Capabilities{Version: 1, Plugins: []contract.PluginStatus{}, Capabilities: []contract.Capability{{ID: "terminal.pty", Available: true}}}
	owners := map[string]string{"launch": "launch.configs", "workspace": "workspace.manage", "chat": "chat.acp", "files": "files.upload", "code": "code.browse", "notifications": "notifications.push", "quick-commands": "quick-commands.send", "voice": "voice.recognize", "forward": "forward.ports", "cloudflare": "cloudflare.manage", "tailscale": "tailscale.manage", "p2p": "p2p.direct", "mcp": "mcp.control", "immersive": "immersive.xr"}
	for _, p := range builtin.Catalog() {
		on := builtin.Has(enabled, p.ID)
		status, reason := contract.PluginStatusDisabled, contract.CapabilityReasonPluginDisabled
		if on {
			status, reason = contract.PluginStatusReady, ""
		}
		snapshot.Plugins = append(snapshot.Plugins, contract.PluginStatus{ID: p.ID, Enabled: on, Status: status, Reason: reason})
		snapshot.Capabilities = append(snapshot.Capabilities, contract.Capability{ID: owners[p.ID], Available: on, Reason: reason})
	}
	return snapshot
}

func withTestCapabilities(handler http.Handler, snapshots ...contract.Capabilities) http.Handler {
	all, _ := builtin.Resolve(nil)
	snapshot := testCapabilities(all...)
	if len(snapshots) > 0 {
		snapshot = snapshots[0]
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == contract.PathCapabilities {
			_ = json.NewEncoder(w).Encode(snapshot)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func TestDisabledPluginCommandStopsBeforeBusinessEndpoint(t *testing.T) {
	calls := 0
	e, _, stderr := serveUnix(t, make(chan struct{}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; http.NotFound(w, r) }), testCapabilities())
	if code := e.Run(context.Background(), launchConfigListCommand, nil); code != contract.ExitError || !strings.Contains(stderr.String(), "plugin_disabled") || strings.Contains(stderr.String(), "too old") || calls != 0 {
		t.Fatalf("code=%d stderr=%q business calls=%d", code, stderr, calls)
	}
}

func TestCoreRunDoesNotDiscoverOptionalResources(t *testing.T) {
	var paths []string
	e, stdout, stderr := serveUnix(t, make(chan struct{}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Method != http.MethodPost || r.URL.Path != "/api/sessions" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var req contract.CreateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.Workspace != nil || req.ConfigID != nil || len(req.Cmd) != 1 || req.Cmd[0] != "bash" {
			t.Errorf("raw request: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(contract.CreateSessionResult{Session: contract.SessionInfo{ID: "core"}})
	}), testCapabilities())
	e.ResolveCwd = func(context.Context, string) (string, error) { return "/work", nil }
	if code := Run(context.Background(), e, []string{"--detach", "--", "bash"}); code != 0 || stdout.String() != "core\n" || len(paths) != 1 {
		t.Fatalf("code=%d stdout=%q stderr=%q paths=%v", code, stdout, stderr, paths)
	}
}

func TestCoreListAndOpenDoNotReadPluginResources(t *testing.T) {
	for _, command := range []string{"ls", "open"} {
		t.Run(command, func(t *testing.T) {
			var paths []string
			e, stdout, stderr := serveUnix(t, make(chan struct{}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				switch r.URL.Path {
				case "/api/sessions":
					_ = json.NewEncoder(w).Encode([]contract.SessionInfo{{ID: "abcdefgh"}})
				case "/api/sessions/abcdefgh":
					_ = json.NewEncoder(w).Encode(contract.SessionInfo{ID: "abcdefgh"})
				case contract.PathStatus:
					_ = json.NewEncoder(w).Encode(contract.StatusV1{V: 1, URLs: contract.URLs{Local: "http://localhost"}})
				default:
					t.Errorf("unexpected optional request: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}), testCapabilities())
			var code int
			if command == "ls" {
				code = List(context.Background(), e, []string{"--json"})
			} else {
				code = Open(context.Background(), e, []string{"abcdefgh"})
			}
			if code != 0 || stdout.Len() == 0 {
				t.Fatalf("code=%d stderr=%q paths=%v", code, stderr, paths)
			}
		})
	}
}

func unavailableTestCapability(snapshot contract.Capabilities, plugin, capability, reason string) contract.Capabilities {
	for i := range snapshot.Plugins {
		if snapshot.Plugins[i].ID == plugin && reason != contract.CapabilityReasonConfigurationOff {
			snapshot.Plugins[i].Status = contract.PluginStatusUnavailable
			snapshot.Plugins[i].Reason = reason
		}
	}
	for i := range snapshot.Capabilities {
		if snapshot.Capabilities[i].ID == capability {
			snapshot.Capabilities[i].Available = false
			snapshot.Capabilities[i].Reason = reason
		}
	}
	return snapshot
}

func TestNotifyUsesCoreAttentionWithoutPush(t *testing.T) {
	for _, reason := range []string{contract.CapabilityReasonPluginDisabled, contract.CapabilityReasonConfigurationOff, contract.CapabilityReasonStorageUnavailable} {
		for _, clear := range []bool{false, true} {
			t.Run(reason+fmt.Sprint(clear), func(t *testing.T) {
				snapshot := testCapabilities()
				if reason != contract.CapabilityReasonPluginDisabled {
					snapshot = unavailableTestCapability(testCapabilities("notifications"), "notifications", "notifications.push", reason)
				}
				calls := 0
				e, stdout, stderr := serveUnix(t, make(chan struct{}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					method := http.MethodPost
					if clear {
						method = http.MethodDelete
					}
					if r.Method != method || r.URL.Path != "/api/sessions/core/attention" {
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
						http.NotFound(w, r)
						return
					}
					if clear {
						w.WriteHeader(http.StatusNoContent)
						return
					}
					var request contract.AttentionRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Kind != "done" || request.Message != "finished" {
						t.Errorf("attention request=%+v err=%v", request, err)
					}
					_ = json.NewEncoder(w).Encode(contract.SessionInfo{ID: "core"})
				}), snapshot)
				args := []string{"--session", "core", "--kind", "done", "--json", "finished"}
				if clear {
					args = []string{"--session", "core", "--clear"}
				}
				if code := e.Run(context.Background(), notifyCommand, args); code != contract.ExitOK || calls != 1 {
					t.Fatalf("code=%d calls=%d stderr=%q", code, calls, stderr)
				}
				if !clear && !strings.Contains(stdout.String(), `"id": "core"`) {
					t.Fatalf("missing attention result: %s", stdout)
				}
			})
		}
	}
}

func TestP2PDiagnosticsRequireSelectionNotConnectivity(t *testing.T) {
	for _, reason := range []string{contract.CapabilityReasonPluginDisabled, contract.CapabilityReasonConfigurationOff, contract.CapabilityReasonUnsupported, contract.CapabilityReasonStorageUnavailable} {
		for _, command := range []*Command{p2pStatusCommand, p2pPeersCommand, p2pProbeCommand} {
			t.Run(reason+"/"+command.Name, func(t *testing.T) {
				snapshot := testCapabilities()
				if reason != contract.CapabilityReasonPluginDisabled {
					snapshot = unavailableTestCapability(testCapabilities("p2p"), "p2p", "p2p.direct", reason)
				}
				calls := 0
				e, stdout, stderr := serveUnix(t, make(chan struct{}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != http.MethodGet || r.URL.Path != contract.PathP2PDiagnostics {
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
						http.NotFound(w, r)
						return
					}
					_ = json.NewEncoder(w).Encode(contract.P2PStatus{
						Supported: reason != contract.CapabilityReasonUnsupported,
						Enabled:   false, StorageAvailable: reason != contract.CapabilityReasonStorageUnavailable,
						StorageError: "settings unavailable", STUNServers: []string{"stuns:localhost:5349"},
						Peers: []contract.P2PPeerInfo{{ID: "diagnostic-peer"}},
					})
				}), snapshot)
				code := e.Run(context.Background(), command, []string{"--json"})
				if reason == contract.CapabilityReasonPluginDisabled {
					if code != contract.ExitError || calls != 0 || !strings.Contains(stderr.String(), "plugin_disabled") {
						t.Fatalf("disabled code=%d calls=%d stderr=%q", code, calls, stderr)
					}
					return
				}
				wantCode, output := contract.ExitOK, `"storageAvailable"`
				if command == p2pPeersCommand {
					output = "diagnostic-peer"
				}
				if command == p2pProbeCommand {
					// A configured but unsupported TLS target is reported immediately;
					// this exercises config lookup without external network traffic.
					wantCode, output = contract.ExitError, "TLS/TCP"
				}
				if code != wantCode || calls != 1 || !strings.Contains(stdout.String(), output) {
					t.Fatalf("code=%d calls=%d stdout=%q stderr=%q", code, calls, stdout, stderr)
				}
			})
		}
	}
}

func TestForwardRemoveCompletionRequiresCapability(t *testing.T) {
	for _, reason := range []string{"", contract.CapabilityReasonPluginDisabled, contract.CapabilityReasonStorageUnavailable} {
		t.Run(reason, func(t *testing.T) {
			snapshot := testCapabilities("forward")
			if reason == contract.CapabilityReasonPluginDisabled {
				snapshot = testCapabilities()
			} else if reason != "" {
				snapshot = unavailableTestCapability(snapshot, "forward", "forward.ports", reason)
			}
			calls := 0
			e, stdout, stderr := serveUnix(t, make(chan struct{}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/api/forwards" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode([]contract.Forward{{Port: 8080, Label: "app"}})
			}), snapshot)
			e.Commands = Commands()
			if code := e.Run(context.Background(), completeCommand, []string{"forward", "rm", ""}); code != contract.ExitOK || stderr.Len() != 0 {
				t.Fatalf("completion code=%d stderr=%q", code, stderr)
			}
			if reason == "" {
				if calls != 1 || !strings.Contains(stdout.String(), "8080\tapp") {
					t.Fatalf("available calls=%d candidates=%q", calls, stdout)
				}
			} else if calls != 0 || stdout.Len() != 0 {
				t.Fatalf("unavailable calls=%d candidates=%q", calls, stdout)
			}
		})
	}
}

func TestP2PDiagnosticsRejectInvalidDiscoveryBeforeBusinessEndpoint(t *testing.T) {
	for _, command := range []*Command{p2pStatusCommand, p2pPeersCommand, p2pProbeCommand} {
		t.Run(command.Name, func(t *testing.T) {
			snapshot := testCapabilities("p2p")
			snapshot.Version = 99
			calls := 0
			e, _, stderr := serveUnix(t, make(chan struct{}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				http.NotFound(w, r)
			}), snapshot)
			if code := e.Run(context.Background(), command, nil); code != contract.ExitError || calls != 0 || !strings.Contains(stderr.String(), "capabilities response") {
				t.Fatalf("code=%d calls=%d stderr=%q", code, calls, stderr)
			}
		})
	}
}
