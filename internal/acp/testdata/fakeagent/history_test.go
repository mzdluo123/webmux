package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	claudehistory "github.com/rainchan/webmux/internal/agentprofile/claudecode/history"
	codexhistory "github.com/rainchan/webmux/internal/agentprofile/codex/history"
	omphistory "github.com/rainchan/webmux/internal/agentprofile/omp/history"
	hs "github.com/rainchan/webmux/internal/historysource"
)

func TestNativeHistoryAppendPreservesFrozenPrefix(t *testing.T) {
	for _, profile := range []string{"omp", "codex", "claudecode"} {
		t.Run(profile, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("FAKEAGENT_HISTORY_HOME", home)
			st, err := openStore(t.TempDir(), "acp", profile)
			if err != nil {
				t.Fatal(err)
			}
			ss := st.draft("/synthetic/work")
			ss.Messages = []message{{ID: "user-native", Role: "user", Text: "constraint 中文🙂"}, {ID: "assistant-native", Role: "agent", Text: "reply"}}
			if err := st.save(ss); err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(home, ".omp", "agent", "sessions")
			discover, read := omphistory.Discover, omphistory.Read
			switch profile {
			case "codex":
				root, discover, read = filepath.Join(home, ".codex"), codexhistory.Discover, codexhistory.Read
			case "claudecode":
				root, discover, read = filepath.Join(home, ".claude", "projects"), claudehistory.Discover, claudehistory.Read
			}
			ctx := context.Background()
			sources, err := discover(ctx, root)
			if err != nil || len(sources) != 1 {
				t.Fatalf("discover: %v, %v", sources, err)
			}
			src := sources[0]
			doc, err := read(ctx, hs.Request{Source: src})
			if err != nil || len(doc.Records) != 2 {
				t.Fatalf("read: %+v, %v", doc, err)
			}
			if doc.Records[0].NativeID != "user-native" || doc.Records[1].NativeID != "assistant-native" || doc.Records[0].Text != ss.Messages[0].Text {
				t.Fatalf("native IDs/text lost: %+v", doc.Records)
			}
			if profile != "codex" && doc.Records[1].ParentID != "user-native" {
				t.Fatalf("parent lost: %+v", doc.Records)
			}
			before, err := os.Stat(src.Path)
			if err != nil {
				t.Fatal(err)
			}
			prefix, err := os.ReadFile(src.Path)
			if err != nil {
				t.Fatal(err)
			}
			// Cold TUI resume shares the same private store and native transcript.
			resumed, err := openStore(st.dir, "tui", profile)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := resumed.load(ss.ID)
			if err != nil {
				t.Fatal(err)
			}
			loaded.Messages = append(loaded.Messages, message{ID: "later-user", Role: "user", Text: "later"})
			if err := resumed.save(loaded); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(src.Path)
			if err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(src.Path)
			if err != nil || !os.SameFile(before, after) || !bytes.HasPrefix(content, prefix) || len(content) <= len(prefix) {
				t.Fatalf("append replaced native prefix/inode: %v", err)
			}
			frozen, err := read(ctx, hs.Request{Source: doc.Source, Cursor: doc.Cursor})
			if err != nil || len(frozen.Records) != 2 || frozen.Fingerprint != doc.Fingerprint {
				t.Fatalf("frozen read changed: %+v, %v", frozen, err)
			}
			loaded.Title = "metadata-only save"
			if err := resumed.save(loaded); err != nil {
				t.Fatal(err)
			}
			loaded.Messages[0].Text = "mutation"
			if err := resumed.save(loaded); err == nil {
				t.Fatal("rewriting a persisted native prefix succeeded")
			}
		})
	}
}

func TestNativeHistoryRequiresExplicitIsolatedHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FAKEAGENT_HISTORY_HOME", "")
	st, err := openStore(t.TempDir(), "acp", "omp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.create("/synthetic/work"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".omp")); !os.IsNotExist(err) {
		t.Fatalf("implicit history output: %v", err)
	}
	t.Setenv("FAKEAGENT_HISTORY_HOME", t.TempDir())
	if _, err := st.create("/synthetic/work"); err == nil {
		t.Fatal("accepted a native history root outside HOME")
	}
}
