package history_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	codex "github.com/rainchan/webmux/internal/agent/codex/history"
	"github.com/rainchan/webmux/internal/codexrollout"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
)

func TestRuntimeDiscoveryCurrentCaptureAndFreeze(t *testing.T) {
	capture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "native-history", "codex", "current-0.169.0.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0.160.1", "0.167.2", "0.169.0", "0.169.1"} {
		t.Run(version, func(t *testing.T) {
			// Reuse the capture body with synthetic headers to exercise version admission.
			data := []byte(strings.Replace(string(capture), "0.169.0", version, 1))
			testRuntimeDiscoveryAndFreeze(t, data, version)
		})
	}
}

func testRuntimeDiscoveryAndFreeze(t *testing.T, data []byte, version string) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	dir := filepath.Join(root, "sessions")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	put := func(path string, body []byte) {
		t.Helper()
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(dir, "unrelated-filename.jsonl")
	put(target, data)
	header, _, _ := strings.Cut(string(data), "\n")
	id := "00000003-0000-4000-8000-000000000003"
	unknownHeader := strings.Replace(header, version, "0.170.0", 1)
	neighbor := strings.ReplaceAll(unknownHeader, id, "neighbor") + "\ninvalid unrelated body\n"
	put(filepath.Join(dir, "neighbor.jsonl"), []byte(neighbor))
	// A filename containing the runtime ID cannot authenticate a malformed header.
	put(filepath.Join(dir, "rollout-"+id+".jsonl"), []byte("{\"type\":\"session_meta\",\"payload\":{}}\n"))
	svc := historyimport.New(historyimport.Options{
		Readers:     map[string]historysource.ReadFunc{"codex": codex.Read},
		Discoverers: map[string]historyimport.DiscoverFunc{"codex": codex.Discover},
		Resolve: func(context.Context, string) (historyimport.Identity, error) {
			return historyimport.Identity{Agent: "codex", NativeID: id, Root: root}, nil
		},
	})
	source, err := svc.RuntimeSource(ctx, "runtime")
	if err != nil || source.Path != target || source.NativeID != id || source.Branch != id {
		t.Fatalf("runtime content identity: %+v %v", source, err)
	}
	scope, err := svc.FreezeRuntime(ctx, "runtime")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := svc.Document(ctx, scope.ID)
	if err != nil {
		t.Fatal(err)
	}
	var messages []string
	for _, record := range doc.Records {
		if record.Kind == "message" {
			messages = append(messages, record.Role+":"+record.Text)
		}
	}
	want := []string{"user:Synthetic handoff code WMX-codex-capture; project Heron; port 48273.", "agent:ACK WMX-codex-capture"}
	if doc.FormatVersion != version || !reflect.DeepEqual(messages, want) {
		t.Fatalf("consumer lost current body: %q (%s)", messages, doc.FormatVersion)
	}
	tail := `{"timestamp":"2000-01-01T00:01:00Z","type":"response_item","payload":{"type":"message","id":"late","role":"assistant","content":[{"type":"output_text","text":"late answer"}]}}` + "\n"
	put(target, append(append([]byte{}, data...), tail...))
	frozen, err := svc.Document(ctx, scope.ID)
	if err != nil || frozen.Cursor != doc.Cursor || !reflect.DeepEqual(frozen.Records, doc.Records) {
		t.Fatalf("runtime frozen append: %v", err)
	}
	put(target, []byte(strings.Replace(string(data), "48273", "48274", 1)+tail))
	if _, err := svc.Document(ctx, scope.ID); !errors.Is(err, historysource.ErrChanged) {
		t.Fatalf("runtime frozen mutation accepted: %v", err)
	}
	put(target, data)
	for _, duplicate := range []string{header, unknownHeader} {
		put(filepath.Join(dir, "duplicate.jsonl"), []byte(duplicate+"\ninvalid duplicate body\n"))
		if _, err := svc.RuntimeSource(ctx, "runtime"); !errors.Is(err, historysource.ErrIdentity) {
			t.Fatalf("duplicate metadata identity accepted: %v", err)
		}
		if _, err := svc.FreezeRuntime(ctx, "runtime"); !errors.Is(err, historysource.ErrIdentity) {
			t.Fatalf("duplicate metadata freeze accepted: %v", err)
		}
	}
	id = "neighbor"
	for _, operation := range []func() error{
		func() error { _, err := svc.RuntimeSource(ctx, "runtime"); return err },
		func() error { _, err := svc.FreezeRuntime(ctx, "runtime"); return err },
	} {
		err := operation()
		var parseError *historysource.ParseError
		if !errors.Is(err, historysource.ErrFormat) || !errors.Is(err, codexrollout.ErrVersion) || !errors.As(err, &parseError) || parseError.Offset != 0 {
			t.Fatalf("runtime unsupported target lost version cause: %v", err)
		}
	}
	id = "missing-session"
	if _, err := svc.RuntimeSource(ctx, "runtime"); !errors.Is(err, historysource.ErrIdentity) {
		t.Fatalf("missing metadata identity accepted: %v", err)
	}
}
