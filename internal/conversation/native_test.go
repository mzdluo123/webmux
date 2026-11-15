package conversation

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	snapcompact "github.com/mzdluo123/snapcompact-go"
)

func TestNativeImportSnapshotRecovery(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	body, err := os.ReadFile("../codexrollout/testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	s := openTest(t, dir)
	first, err := s.ImportCodex(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.ImportCodex(ctx, path)
	if err != nil || first.SourceID != again.SourceID || first.Binding != again.Binding {
		t.Fatal(first, again, err)
	}
	snap, err := s.FreezeNative(ctx, first.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	input, err := s.SnapshotInput(ctx, snap.ID, "  current\n task  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(input.History) != 5 {
		t.Fatalf("history count %d", len(input.History))
	}
	projection, err := snapcompact.Project(ctx, input, snapcompact.DefaultPreprocessConfig())
	if err != nil || projection.CurrentTask != input.CurrentTask {
		t.Fatal(err)
	}
	tail := []byte("{\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"role\":\"user\",\"content\":[{\"type\":\"input_text\",\"text\":\"later\"}]}}\n")
	if err = os.WriteFile(path, append(append([]byte{}, body...), tail[:20]...), 0600); err != nil {
		t.Fatal(err)
	}
	pending, err := s.ImportCodex(ctx, path)
	if err != nil || !pending.Pending || pending.Consumed != int64(len(body)) {
		t.Fatal(pending, err)
	}
	if err = os.WriteFile(path, append(append([]byte{}, body...), tail...), 0600); err != nil {
		t.Fatal(err)
	}
	s.beforeCommit = func() error { return errors.New("injected failure") }
	if _, err = s.ImportCodex(ctx, path); err == nil {
		t.Fatal("expected rollback")
	}
	s.beforeCommit = nil
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTest(t, dir)
	updated, err := s.ImportCodex(ctx, path)
	if err != nil || updated.SourceID != first.SourceID {
		t.Fatal(updated, err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	old, err := s.SnapshotInput(ctx, snap.ID, "")
	if err != nil || len(old.History) != len(input.History) {
		t.Fatal(old, err)
	}
	for i := range old.History {
		if old.History[i].Text != input.History[i].Text || old.History[i].ID != input.History[i].ID { // Arguments is a map; compare stable body fields below.
			t.Fatal("snapshot changed")
		}
	}
}

func TestNativeCaptureDisabled(t *testing.T) {
	s := Open(Options{StateDir: t.TempDir(), DisableCapture: true})
	defer func() { _ = s.Close() }()
	if _, err := s.ImportCodex(context.Background(), "/nonexistent"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestNativeConcurrentAndGenerations(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	data, err := os.ReadFile("../codexrollout/testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	results := make(chan NativeImport, 8)
	failures := make(chan error, 8)
	for range 8 {
		go func() { r, err := s.ImportCodex(ctx, path); results <- r; failures <- err }()
	}
	var source string
	for range 8 {
		r := <-results
		if err := <-failures; err != nil {
			t.Fatal(err)
		}
		if source != "" && source != r.SourceID {
			t.Fatal("duplicate source")
		}
		source = r.SourceID
	}
	if err = os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	replaced, err := s.ImportCodex(ctx, path)
	if err != nil || replaced.SourceID == source || replaced.Generation != 2 {
		t.Fatal(replaced, err)
	}
	newline := strings.IndexByte(string(data), '\n') + 1
	if err = os.WriteFile(path, data[:newline], 0600); err != nil {
		t.Fatal(err)
	}
	truncated, err := s.ImportCodex(ctx, path)
	if err != nil || truncated.Generation != 3 {
		t.Fatal(truncated, err)
	}
}

func TestNativePNGAndExplicitDeletion(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	data, err := os.ReadFile("../codexrollout/testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	imported, err := s.ImportCodex(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := s.FreezeNative(ctx, imported.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := snapcompact.DefaultPreprocessConfig()
	cfg.OutputRunes = 4
	preview, err := s.PreviewNative(ctx, frozen.ID, "  task\n unchanged  ", cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Render.Pages) == 0 || len(preview.Render.Uncovered) != 0 || len(preview.Projection.Omissions) == 0 {
		t.Fatal("incomplete render or no truncation")
	}
	if _, err := png.Decode(bytes.NewReader(preview.Render.Pages[0].PNG)); err != nil {
		t.Fatal(err)
	}
	if preview.Projection.CurrentTask != "  task\n unchanged  " {
		t.Fatal("task altered")
	}
	raw, err := s.SnapshotInput(ctx, frozen.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if raw.History[2].Text != "hello\n  untouched" {
		t.Fatal("source was truncated")
	}
	if err = s.DeleteNativeSource(ctx, imported.SourceID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.NativeSnapshot(ctx, frozen.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal("native source removed", err)
	}
}

func TestNativeSnapshotReferenceAndParserGeneration(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	data, err := os.ReadFile("../codexrollout/testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	source, err := s.ImportCodex(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := s.FreezeNative(ctx, source.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RetainNativeSnapshot(ctx, frozen.ID, "future-fork"); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteNativeSource(ctx, source.SourceID); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	// Simulate an old importer generation after an application upgrade. Frozen
	// parser provenance must remain the original version, not follow the source.
	if _, err = s.db.Exec("UPDATE native_source SET parser='older-parser' WHERE id=?", source.SourceID); err != nil {
		t.Fatal(err)
	}
	next, err := s.ImportCodex(ctx, path)
	if err != nil || next.SourceID == source.SourceID {
		t.Fatal(next, err)
	}
	old, err := s.NativeSnapshot(ctx, frozen.ID)
	if err != nil || old.Parser != frozen.Parser {
		t.Fatal(old, err)
	}
	if err = s.ReleaseNativeSnapshot(ctx, frozen.ID, "future-fork"); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteNativeSource(ctx, source.SourceID); err != nil {
		t.Fatal(err)
	}
}
