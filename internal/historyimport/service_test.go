package historyimport_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historysource"
)

// This synthetic second format exercises orchestration without any Codex type,
// constant or branch. Real agent format coverage lives with each leaf reader.
func syntheticRead(ctx context.Context, req historysource.Request) (historysource.Document, error) {
	var records []historysource.Record
	out, err := historysource.ReadJSONL(ctx, req, func(line []byte, offset, end int64) error {
		var text string
		if json.Unmarshal(line, &text) != nil {
			return historysource.ErrFormat
		}
		records = append(records, historysource.Record{ID: strconv.FormatInt(offset, 10), Kind: "message", Role: "user", Text: text, Offset: offset, End: end})
		return nil
	})
	out.FormatVersion, out.ParserVersion = "synthetic/v1", "synthetic-reader/v1"
	out.Records = records
	return out, err
}

func TestFrozenSourceAndRetainedLifetime(t *testing.T) {
	ctx := context.Background()
	state := t.TempDir()
	store := conversation.Open(conversation.Options{StateDir: state})
	if err := store.Unavailable(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "native.jsonl")
	if err := os.WriteFile(path, []byte("\"甲🙂乙\\n  code\"\n\"tail\"\npartial"), 0600); err != nil {
		t.Fatal(err)
	}
	source := historysource.Source{Agent: "synthetic", Path: path, NativeID: "session"}
	opts := historyimport.Options{Store: store, Readers: map[string]historysource.ReadFunc{"synthetic": syntheticRead}}
	svc := historyimport.New(opts)
	reference, err := svc.Freeze(ctx, source, "parent", false)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := svc.Freeze(ctx, source, "child-a", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Retain(ctx, retained.ID, "child-b"); err != nil {
		t.Fatal(err)
	}
	before, err := svc.Document(ctx, reference.ID)
	if err != nil || len(before.Records) != 2 {
		t.Fatalf("initial frozen range: %+v %v", before, err)
	}
	// A suffix can complete and grow without entering a frozen reference.
	if err := os.WriteFile(path, []byte("\"甲🙂乙\\n  code\"\n\"tail\"\n\"new\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Document(ctx, reference.ID)
	if err != nil || len(after.Records) != 2 || after.Records[0].Text != before.Records[0].Text {
		t.Fatalf("append changed frozen history: %+v %v", after, err)
	}
	var rebuilt strings.Builder
	req := historysource.PageRequest{Limit: 1, MaxBytes: 4}
	for {
		page, err := svc.Page(ctx, reference.ID, req)
		if err != nil {
			t.Fatal(err)
		}
		for _, record := range page.Records {
			if req.After == 0 {
				rebuilt.WriteString(record.Text)
			}
		}
		if page.Next != 0 || !page.More {
			break
		}
		req.After, req.TextOffset = page.Next, page.TextOffset
	}
	if rebuilt.String() != before.Records[0].Text {
		t.Fatalf("UTF-8 continuation lost text: %q", rebuilt.String())
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Document(ctx, reference.ID); !errors.Is(err, historysource.ErrUnavailable) {
		t.Fatalf("missing reference source: %v", err)
	}
	if err := svc.Release(ctx, retained.ID, "child-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = conversation.Open(conversation.Options{StateDir: state})
	defer func() { _ = store.Close() }()
	opts.Store = store
	svc = historyimport.New(opts)
	kept, err := svc.Document(ctx, retained.ID)
	if err != nil || len(kept.Records) != 2 || kept.Records[0].Text != before.Records[0].Text {
		t.Fatalf("retained history after restart/source removal: %+v %v", kept, err)
	}
	if err := svc.Release(ctx, retained.ID, "child-b"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Document(ctx, retained.ID); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatalf("last reference did not collect export: %v", err)
	}
}

func TestFrozenPrefixMutationAndCancellation(t *testing.T) {
	ctx := context.Background()
	store := conversation.Open(conversation.Options{StateDir: t.TempDir()})
	defer func() { _ = store.Close() }()
	path := filepath.Join(t.TempDir(), "native.jsonl")
	if err := os.WriteFile(path, []byte("\"before\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := historyimport.New(historyimport.Options{Store: store, Readers: map[string]historysource.ReadFunc{"synthetic": syntheticRead}})
	source := historysource.Source{Agent: "synthetic", Path: path, NativeID: "session"}
	snapshot, err := svc.Freeze(ctx, source, "owner", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("\"change\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Document(ctx, snapshot.ID); !errors.Is(err, historysource.ErrChanged) {
		t.Fatalf("mutated frozen source accepted: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := svc.Freeze(cancelled, source, "owner", false); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled source read: %v", err)
	}
}

func TestFrozenRangesDoNotExpandPastHistoricalOptOut(t *testing.T) {
	ctx := context.Background()
	store := conversation.Open(conversation.Options{StateDir: t.TempDir()})
	defer func() { _ = store.Close() }()
	binding, err := store.EnsureRuntime(ctx, conversation.RuntimeInput{RuntimeID: "runtime", Cwd: t.TempDir(), AgentName: "synthetic", AgentSessionID: "session", HistoryOrigin: "complete", Status: "closed"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "native.jsonl")
	if err := os.WriteFile(path, []byte("\"public prefix\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := historyimport.New(historyimport.Options{Store: store, Readers: map[string]historysource.ReadFunc{"synthetic": syntheticRead}})
	source := historysource.Source{Agent: "synthetic", NativeID: "session", Path: path}
	frozen, err := svc.Freeze(ctx, source, "public-reader", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddGap(ctx, binding.ProviderThreadID, "capture_disabled"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("\"public prefix\"\n\"private interval\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Freeze(ctx, source, "new-reader", false); !errors.Is(err, conversation.ErrUnavailable) {
		t.Fatalf("new freeze exposed an opted-out native session: %v", err)
	}
	old, err := svc.Document(ctx, frozen.ID)
	if err != nil || len(old.Records) != 1 || old.Records[0].Text != "public prefix" {
		t.Fatalf("old authorized range expanded or disappeared: %+v %v", old, err)
	}
}
