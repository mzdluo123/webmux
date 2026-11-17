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

func fixture(t *testing.T, name, id string) historysource.Source {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "native-history", "qodercn", name))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return historysource.Source{Agent: "qodercn", Path: path, NativeID: id}
}

func TestNativeSessionsAndMetadata(t *testing.T) {
	for _, tt := range []struct {
		file, id string
		messages int
	}{
		{"session-1.jsonl", "cbf69028-0808-5568-829a-60d59e3b0868", 3},
		{"session-2.jsonl", "54b3d76a-0976-5482-94ae-367317ee052e", 2},
	} {
		t.Run(tt.file, func(t *testing.T) {
			s := fixture(t, tt.file, tt.id)
			doc, err := Read(context.Background(), historysource.Request{Source: s})
			if err != nil {
				t.Fatal(err)
			}
			messages, metadata := 0, 0
			for _, r := range doc.Records {
				if r.Kind == "message" {
					messages++
				}
				if r.Kind == "metadata" {
					metadata++
					if r.Text != "" {
						t.Fatal("metadata body exposed")
					}
				}
			}
			if messages != tt.messages || metadata == 0 || doc.Source.Branch == "" || doc.FormatVersion != "qoder-jsonl/unversioned" {
				t.Fatal(messages, metadata, doc.FormatVersion)
			}
			// Session two contains an actual cycle in its sanitized system ancestry.
			if tt.messages == 2 && (len(doc.Gaps) != 1 || doc.Gaps[0].Category != "cycle") {
				t.Fatal(doc.Gaps)
			}
			sources, err := Discover(context.Background(), filepath.Dir(s.Path))
			if err != nil || len(sources) != 1 || sources[0].NativeID != tt.id {
				t.Fatal(sources, err)
			}
			s.NativeID = "not-the-session"
			if _, err := Read(context.Background(), historysource.Request{Source: s}); !errors.Is(err, historysource.ErrIdentity) {
				t.Fatal(err)
			}
		})
	}
}

// Synthetic mutations supplement, not replace, real native captures. Tools and
// compaction were absent from the observed QoderCN sessions.
func TestBranchToolsCompactionAndFrozenBoundary(t *testing.T) {
	s := fixture(t, "session-2.jsonl", "54b3d76a-0976-5482-94ae-367317ee052e")
	appendText := func(text string) {
		t.Helper()
		f, err := os.OpenFile(s.Path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString(text); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	appendText(`{"type":"system","uuid":"compact","parentUuid":"outside","subtype":"compact_boundary"}` + "\n" +
		`{"type":"assistant","uuid":"call","parentUuid":"compact","message":{"role":"assistant","content":[{"type":"thinking","thinking":"SECRET"},{"type":"text","text":"你好 🌍"},{"type":"tool_use","id":"tool","name":"Read","input":{"secret":"SECRET"}}]}}` + "\n" +
		`{"type":"user","uuid":"result","parentUuid":"call","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool","content":"done","is_error":true}]}}` + "\n" +
		`{"type":"assistant","uuid":"other","parentUuid":"compact","message":{"role":"assistant","content":"WRONG_BRANCH"}}` + "\n" +
		`{"type":"active-leaf","leafUuid":"result"}` + "\n")
	doc, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil {
		t.Fatal(err)
	}
	calls, results, unicode := 0, 0, 0
	for _, r := range doc.Records {
		if strings.Contains(r.Text, "SECRET") || strings.Contains(r.Text, "WRONG_BRANCH") {
			t.Fatal(r)
		}
		if r.Kind == "tool_use" {
			calls++
		}
		if r.Kind == "tool_result" && r.CallID == "tool" && r.Status == "error" {
			results++
		}
		if r.Text == "你好 🌍" {
			unicode++
		}
	}
	if calls != 1 || results != 1 || unicode != 1 || len(doc.Gaps) != 2 {
		t.Fatal(calls, results, unicode, doc.Gaps)
	}
	appendText(`{"type":"active-leaf","leafUuid":"other"}` + "\n" + `{"partial":`)
	frozen, err := Read(context.Background(), historysource.Request{Source: s, Cursor: doc.Cursor})
	if err != nil || frozen.Source.Branch != "result" || frozen.Fingerprint != doc.Fingerprint {
		t.Fatal(err)
	}
	latest, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil || latest.Source.Branch != "other" || !latest.Pending {
		t.Fatal(err, latest.Source)
	}
	s.Branch = "call"
	branch, err := Read(context.Background(), historysource.Request{Source: s})
	if err != nil || branch.Source.Branch != "call" {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "[REDACTED]", "[MUTATED!]", 1))
	if err := os.WriteFile(s.Path, data, 0600); err != nil {
		t.Fatal(err)
	}
	s.Branch = ""
	if _, err := Read(context.Background(), historysource.Request{Source: s, Cursor: doc.Cursor}); !errors.Is(err, historysource.ErrChanged) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(ctx, historysource.Request{Source: s}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := Discover(ctx, filepath.Dir(s.Path)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestFrozenTruncationAndReplacement(t *testing.T) {
	for _, replace := range []bool{false, true} {
		s := fixture(t, "session-1.jsonl", "cbf69028-0808-5568-829a-60d59e3b0868")
		doc, err := Read(context.Background(), historysource.Request{Source: s})
		if err != nil {
			t.Fatal(err)
		}
		if replace {
			data, err := os.ReadFile(s.Path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(s.Path, s.Path+".old"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.Path, data, 0600); err != nil {
				t.Fatal(err)
			}
		} else if err := os.Truncate(s.Path, 10); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(context.Background(), historysource.Request{Source: s, Cursor: doc.Cursor}); !errors.Is(err, historysource.ErrChanged) {
			t.Fatal(replace, err)
		}
	}
}
