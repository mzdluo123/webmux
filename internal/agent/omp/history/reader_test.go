package history

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hs "github.com/rainchan/webmux/internal/historysource"
)

func fixture(name string) string {
	return filepath.Join("..", "..", "testdata", "native-history", "omp", name+".jsonl")
}
func source(path, id string) hs.Source { return hs.Source{Agent: "omp", Path: path, NativeID: id} }
func TestNativeFixtures(t *testing.T) {
	for _, tc := range []struct {
		name, id       string
		calls, results int
	}{{"session", "00000001-0000-4000-8000-000000000001", 2, 2}, {"compaction", "00000002-0000-4000-8000-000000000001", 8, 8}} {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Read(context.Background(), hs.Request{Source: source(fixture(tc.name), tc.id)})
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			names := map[string]string{}
			failed := false
			for _, r := range doc.Records {
				counts[r.Kind]++
				if r.ID == "" || r.End <= r.Offset {
					t.Fatalf("lost native identity or offsets: %+v", r)
				}
				if r.Kind == "tool_call" {
					names[r.CallID] = r.ToolName
				}
				if r.Kind == "tool_result" {
					if r.CallID == "" || names[r.CallID] != r.ToolName {
						t.Fatalf("tool association: %+v", r)
					}
					failed = failed || r.Status == "error"
				}
				if strings.Contains(r.Text, "OPAQUE") || strings.Contains(r.Text, "credentialId") {
					t.Fatal("private payload escaped")
				}
			}
			if counts["tool_call"] != tc.calls || counts["tool_result"] != tc.results {
				t.Fatalf("tool counts: %v", counts)
			}
			if tc.name == "session" && !failed {
				t.Fatal("lost tool error")
			}
			if tc.name == "compaction" && counts["compaction"] != 1 {
				t.Fatal("lost compaction")
			}
			if doc.FormatVersion != FormatVersion || doc.ParserVersion != ParserVersion || doc.Cursor == "" || doc.Fingerprint == "" {
				t.Fatal("missing version/boundary")
			}
			replay, err := Read(context.Background(), hs.Request{Source: doc.Source, Cursor: doc.Cursor})
			if err != nil || replay.Fingerprint != doc.Fingerprint {
				t.Fatalf("frozen replay: %v", err)
			}
		})
	}
}

func TestFrozenAppendIdentityCancellation(t *testing.T) {
	data, err := os.ReadFile(fixture("session"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	req := hs.Request{Source: source(path, "00000001-0000-4000-8000-000000000001")}
	doc, err := Read(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	// Synthetic partial append: never reinterpret an unfinished record as a message.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(`{"unfinished":"秘密`)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	latest, err := Read(context.Background(), req)
	if err != nil || !latest.Pending {
		t.Fatalf("partial suffix: %v", err)
	}
	frozen, err := Read(context.Background(), hs.Request{Source: doc.Source, Cursor: doc.Cursor})
	if err != nil || frozen.Fingerprint != doc.Fingerprint || len(frozen.Records) != len(doc.Records) {
		t.Fatalf("frozen append: %v", err)
	}
	data[10] ^= 1
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Read(context.Background(), hs.Request{Source: doc.Source, Cursor: doc.Cursor}); err == nil {
		t.Fatal("accepted rewritten frozen prefix")
	}
	req.Source.NativeID = "wrong"
	if _, err = Read(context.Background(), hs.Request{Source: source(fixture("session"), "wrong")}); !errors.Is(err, hs.ErrIdentity) {
		t.Fatalf("identity: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Read(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if _, err = Discover(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("discover cancel: %v", err)
	}
}

func TestDiscoverNativeIdentity(t *testing.T) {
	sources, err := Discover(context.Background(), filepath.Dir(fixture("session")))
	if err != nil || len(sources) != 2 {
		t.Fatalf("discovery: %v %v", sources, err)
	}
	for _, s := range sources {
		if s.Agent != "omp" || s.NativeID == "" || s.Branch == "" {
			t.Fatalf("incomplete identity: %+v", s)
		}
	}
}
