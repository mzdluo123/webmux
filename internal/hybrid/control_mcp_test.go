package hybrid

import (
	"context"
	"reflect"
	"testing"

	"github.com/rainchan/webmux/internal/acp"
)

func TestDefaultMCPDoesNotRequireHistory(t *testing.T) {
	want := []acp.MCPServer{{Name: "webmux", Command: "/fixture/webmux"}}
	a := &actor{svc: &Service{mcpServers: func(context.Context) []acp.MCPServer { return want }}}
	got, scope := a.sessionMCPServers(context.Background())
	if !reflect.DeepEqual(got, want) || scope != "" {
		t.Fatalf("servers %+v scope %q", got, scope)
	}
}

func TestDefaultMCPKeepsHandoffScopeAndCallbackSlice(t *testing.T) {
	h := newHarness(t, 0)
	a := &actor{id: "runtime", svc: h.svc}
	h.svc.historyReader = func(context.Context, string) []acp.MCPServer { return []acp.MCPServer{{Name: "webmux-history"}} }
	backing := []acp.MCPServer{{Name: "webmux"}, {Name: "sentinel"}}
	h.svc.mcpServers = func(context.Context) []acp.MCPServer { return backing[:1] }
	got, scope := a.sessionMCPServers(context.Background())
	if len(got) != 2 || got[0].Name != "webmux" || got[1].Name != "webmux-history" || scope != "" {
		t.Fatalf("servers %+v scope %q", got, scope)
	}
	if backing[1].Name != "sentinel" {
		t.Fatal("mutated descriptor callback's backing slice")
	}
}
