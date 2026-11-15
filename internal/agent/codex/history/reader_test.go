package history

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

func TestRealNativeFixtures(t *testing.T) {
	tests := []struct {
		file, id, version        string
		messages, calls, results int
	}{
		{"session.jsonl", "00000001-0000-4000-8000-000000000003", "0.162.1", 3, 3, 3},
		{"compaction.jsonl", "00000002-0000-4000-8000-000000000003", "0.157.1", 8, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			source := historysource.Source{Agent: "codex", Path: filepath.Join("..", "..", "testdata", "native-history", "codex", tt.file), NativeID: tt.id}
			doc, err := Read(context.Background(), historysource.Request{Source: source})
			if err != nil {
				t.Fatal(err)
			}
			var messages, calls, results int
			callIDs := map[string]bool{}
			for _, item := range doc.Records {
				if strings.Contains(item.Text, "OPAQUE") || strings.Contains(item.Text, "encrypted_content") {
					t.Fatal("reasoning escaped source boundary")
				}
				switch item.Kind {
				case "message":
					messages++
				case "tool", "tool_arguments":
					calls++
					callIDs[item.CallID] = true
				case "tool_result":
					results++
					if !callIDs[item.CallID] {
						t.Fatalf("result lost call identity: %+v", item)
					}
				}
			}
			if messages != tt.messages || calls != tt.calls || results != tt.results || doc.FormatVersion != tt.version || doc.Source.Branch != tt.id {
				t.Fatalf("native format mismatch: messages=%d calls=%d results=%d version=%s branch=%s", messages, calls, results, doc.FormatVersion, doc.Source.Branch)
			}
		})
	}
}

func TestExistingRolloutParserAndFrozenAppend(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "codexrollout", "testdata", "basic.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	source := historysource.Source{Agent: "codex", Path: path, NativeID: "00000000-0000-4000-8000-000000000001"}
	before, err := Read(context.Background(), historysource.Request{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("{\"type\":\"future\",\"payload\":{}}\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := Read(context.Background(), historysource.Request{Source: before.Source, Cursor: before.Cursor})
	if err != nil || before.Fingerprint != after.Fingerprint || len(before.Records) != len(after.Records) {
		t.Fatalf("frozen prefix changed after append: %v", err)
	}
	source.NativeID = "other-session"
	if _, err := Read(context.Background(), historysource.Request{Source: source}); !errors.Is(err, historysource.ErrIdentity) {
		t.Fatalf("wrong native identity accepted: %v", err)
	}
}
