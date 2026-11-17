package historyimport_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	omp "github.com/rainchan/webmux/internal/agent/omp/history"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historyprojection"
	hs "github.com/rainchan/webmux/internal/historysource"
)

func TestOMPRuntimeHistoryIsolation(t *testing.T) {
	tests := []struct {
		name, unrelated string
	}{
		{"unsupported_role", `{"type":"message","id":"other","message":{"role":"future_role","content":"private"}}`},
		{"malformed_body", `{invalid JSON}`},
		{"duplicate_node", "{\"type\":\"message\",\"id\":\"same\"}\n{\"type\":\"message\",\"id\":\"same\"}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"a-unrelated.jsonl": "{\"type\":\"session\",\"id\":\"unrelated\",\"version\":3}\n" + tt.unrelated + "\n",
				"b-target.jsonl": `{"type":"title","title":"Synthetic title"}
{"type":"session","id":"target","version":3}
{"type":"message","id":"task","message":{"role":"user","content":"original task"}}
{"type":"message","id":"constraint","parentId":"task","message":{"role":"developer","content":[{"type":"text","text":"retain constraint"}]}}
{"type":"message","id":"context","parentId":"constraint","message":{"role":"custom","content":"extension context"}}
{"type":"message","id":"result","parentId":"context","message":{"role":"bashExecution","command":"echo done","output":"done","exitCode":0}}
`,
			}
			for name, data := range files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			svc := historyimport.New(historyimport.Options{
				Readers:     map[string]hs.ReadFunc{"omp": omp.Read},
				Discoverers: map[string]historyimport.DiscoverFunc{"omp": omp.Discover},
				Resolve: func(_ context.Context, id string) (historyimport.Identity, error) {
					return historyimport.Identity{Agent: "omp", NativeID: id, Root: root}, nil
				},
			})
			ctx := context.Background()
			scope, err := svc.FreezeRuntime(ctx, "target")
			if err != nil {
				t.Fatal(err)
			}
			doc, err := svc.ScopeDocument(ctx, scope.ID)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Source.NativeID != "target" || doc.Source.Branch != "result" || len(doc.Records) != 4 {
				t.Fatalf("wrong frozen source: %+v", doc)
			}
			projection := historyprojection.New(historyprojection.Options{Source: svc, Preprocess: historyprojection.DefaultPreprocessConfig()})
			preview, err := projection.Project(ctx, scope.ID, "next task", projection.Config(), false)
			if err != nil {
				t.Fatal(err)
			}
			var text strings.Builder
			for _, segment := range preview.Projection.Segments {
				text.WriteString(segment.Text)
			}
			for _, want := range []string{"original task", "retain constraint", "extension context", "echo done", "done"} {
				if !strings.Contains(text.String(), want) {
					t.Fatalf("handoff lost %q: %+v", want, preview.Projection)
				}
			}
			if _, err := svc.FreezeRuntime(ctx, "unrelated"); !errors.Is(err, hs.ErrFormat) {
				t.Fatalf("selected invalid history = %v", err)
			}
		})
	}
}

func TestOMPDiscoveryKeepsDuplicateIdentityAmbiguous(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.jsonl", "b.jsonl"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("{\"type\":\"session\",\"id\":\"same\",\"version\":3}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	svc := historyimport.New(historyimport.Options{
		Discoverers: map[string]historyimport.DiscoverFunc{"omp": omp.Discover},
		Resolve: func(context.Context, string) (historyimport.Identity, error) {
			return historyimport.Identity{Agent: "omp", NativeID: "same", Root: root}, nil
		},
	})
	if _, err := svc.RuntimeSource(context.Background(), "runtime"); !errors.Is(err, hs.ErrIdentity) {
		t.Fatalf("ambiguous identity = %v", err)
	}
}
