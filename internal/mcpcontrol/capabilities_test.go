package mcpcontrol

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/builtin"
	"github.com/rainchan/webmux/internal/contract"
)

func testCapabilities(enabled []string) contract.Capabilities {
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

func TestCoreMCPDiscoveryWithoutLaunchOrChat(t *testing.T) {
	c := testClient(t, func(r *http.Request) string { t.Errorf("unexpected business discovery: %s", r.URL.Path); return "{}" })
	snapshot := testCapabilities([]string{"mcp"})
	c.Capabilities = func() contract.Capabilities { return snapshot }
	c.snapshot = &snapshot
	if _, err := c.Registry(); err != nil {
		t.Fatal(err)
	}
	value, err := c.capabilities(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	result := value.(contract.MCPControlCapabilitiesResult)
	if len(result.LaunchConfigs) != 0 || len(result.Views) != 1 || result.Views[0] != "terminal" {
		t.Fatalf("discovery: %+v", result)
	}
	for _, tool := range c.tools() {
		name := tool.Definition["name"].(string)
		schema := tool.Definition["inputSchema"].(map[string]any)
		properties := schema["properties"].(map[string]any)
		switch name {
		case "session_read":
			views := properties["view"].(map[string]any)["enum"].([]string)
			if len(views) != 1 || views[0] != "terminal" {
				t.Fatalf("read views: %v", views)
			}
			if _, ok := properties["afterPosition"]; ok {
				t.Fatal("chat history cursor advertised without chat")
			}
		case "session_launch":
			for _, key := range []string{"configId", "workspace", "newWorktree", "attachments", "agentConfig", "permissionMode"} {
				if _, ok := properties[key]; ok {
					t.Errorf("disabled launch property %s advertised", key)
				}
			}
		}
	}
	if _, err := c.read(context.Background(), json.RawMessage(`{"sessionId":"core","view":"activity"}`)); err == nil || !strings.Contains(err.Error(), "plugin_disabled") {
		t.Fatalf("disabled activity read: %v", err)
	}
}

func TestRegistryRejectsInvalidCapabilities(t *testing.T) {
	c := Client{Capabilities: func() contract.Capabilities { s := testCapabilities([]string{"mcp"}); s.Version = 99; return s }}
	if _, err := c.Registry(); err == nil {
		t.Fatal("accepted unsupported capability version")
	}
	c.Capabilities = func() contract.Capabilities { return testCapabilities([]string{}) }
	if _, err := c.Registry(); err == nil || !strings.Contains(err.Error(), "plugin_disabled") {
		t.Fatalf("MCP-disabled registry: %v", err)
	}
}

func TestStdioCoreMCPDiscoversOnlyCapabilityEndpoint(t *testing.T) {
	snapshot := testCapabilities([]string{"mcp"})
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	c := testClient(t, func(r *http.Request) string {
		if r.URL.Path != contract.PathCapabilities {
			t.Errorf("unexpected optional discovery %s", r.URL.Path)
			return "{}"
		}
		return string(data)
	})
	c.Capabilities, c.snapshot = nil, nil
	if _, err := c.Registry(); err != nil {
		t.Fatal(err)
	}
	value, err := c.capabilities(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	result := value.(contract.MCPControlCapabilitiesResult)
	if len(result.Views) != 1 || result.Views[0] != "terminal" || len(result.LaunchConfigs) != 0 {
		t.Fatalf("core-only stdio discovery: %+v", result)
	}
}
