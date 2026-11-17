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
	hs "github.com/rainchan/webmux/internal/historysource"
)

func TestRuntimeOMPDiscoveryIsolationAndFreeze(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	data, err := os.ReadFile("../agent/testdata/native-history/omp/session-18.22.0.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.jsonl")
	put := func(path string, body []byte) {
		t.Helper()
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	put(target, data)
	// Neither an unsupported role nor malformed body in another session is read.
	put(filepath.Join(root, "neighbor.jsonl"), []byte("{\"type\":\"session\",\"version\":99,\"id\":\"neighbor\"}\n{\"type\":\"message\",\"message\":{\"role\":\"futureRole\"}}\ninvalid body\n"))
	id := "00000003-0000-4000-8000-000000000001"
	svc := historyimport.New(historyimport.Options{
		Readers:     map[string]hs.ReadFunc{"omp": omp.Read},
		Discoverers: map[string]historyimport.DiscoverFunc{"omp": omp.Discover},
		Resolve: func(context.Context, string) (historyimport.Identity, error) {
			return historyimport.Identity{Agent: "omp", NativeID: id, Root: root}, nil
		},
	})
	source, err := svc.RuntimeSource(ctx, "runtime")
	if err != nil || source.Path != target || source.Branch == "" {
		t.Fatalf("runtime source: %+v %v", source, err)
	}
	scope, err := svc.FreezeRuntime(ctx, "runtime")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := svc.Document(ctx, scope.ID)
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for _, r := range doc.Records {
		text += r.Text
	}
	if !strings.Contains(text, "WMX-omp-capture") || !strings.Contains(text, "蓝鹭") || !strings.Contains(text, "48273") {
		t.Fatalf("lost synthetic facts: %q", text)
	}
	appended := append(append([]byte(nil), data...), []byte("{\"type\":\"message\",\"id\":\"late\",\"parentId\":\"entry-9\",\"message\":{\"role\":\"user\",\"content\":\"late fact\"}}\n")...)
	put(target, appended)
	frozen, err := svc.Document(ctx, scope.ID)
	if err != nil || frozen.Cursor != doc.Cursor || len(frozen.Records) != len(doc.Records) {
		t.Fatalf("frozen append: %v", err)
	}
	put(target, []byte(strings.Replace(string(data), "48273", "48274", 1)))
	if _, err := svc.Document(ctx, scope.ID); !errors.Is(err, hs.ErrChanged) {
		t.Fatalf("content mutation: %v", err)
	}
	put(target, data)
	put(filepath.Join(root, "duplicate.jsonl"), []byte("{\"type\":\"session\",\"version\":99,\"id\":\""+id+"\"}\n"))
	if _, err := svc.RuntimeSource(ctx, "runtime"); !errors.Is(err, hs.ErrIdentity) {
		t.Fatalf("unsupported duplicate identity: %v", err)
	}
	id = "neighbor"
	if _, err := svc.RuntimeSource(ctx, "runtime"); !errors.Is(err, hs.ErrFormat) {
		t.Fatalf("unsupported target must preserve format error: %v", err)
	}
	if _, err := svc.FreezeRuntime(ctx, "runtime"); !errors.Is(err, hs.ErrFormat) {
		t.Fatalf("unsupported freeze must preserve format error: %v", err)
	}
}
