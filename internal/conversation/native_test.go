package conversation

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

func TestNativeSourceChangesDuringRead(t *testing.T) {
	for _, mode := range []string{"rewrite", "replace", "delete"} {
		t.Run(mode, func(t *testing.T) {
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
			s.beforeNativeVerify = func() {
				switch mode {
				case "rewrite":
					data[0] = ' '
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
				case "replace":
					if err := os.Rename(path, path+".old"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
				case "delete":
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err = s.ImportCodex(ctx, path); !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
			var count int
			if err = s.db.QueryRow("SELECT count(*) FROM native_source").Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
		})
	}
}

func TestNativeDoesNotGuessACPIdentity(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	live, err := s.EnsureRuntime(ctx, RuntimeInput{RuntimeID: "live", Cwd: "/example/project", AgentName: "codex", AgentSessionID: "00000000-0000-4000-8000-000000000001", Status: "archived"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	data, err := os.ReadFile("../codexrollout/testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	imported, err := s.ImportCodex(ctx, path)
	if err != nil || imported.Binding == live {
		t.Fatal(imported, err)
	}
	frozen, err := s.FreezeNative(ctx, imported.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(frozen.Gaps, "native_acp_unreconciled") {
		t.Fatal(frozen.Gaps)
	}
}

func TestNativePartialMetadataDoesNotCreateIdentity(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"session_meta"`), 0600); err != nil {
		t.Fatal(err)
	}
	pending, err := s.ImportCodex(ctx, path)
	if err != nil || !pending.Pending || pending.SourceID != "" {
		t.Fatal(pending, err)
	}
	var count int
	if err = s.db.QueryRow("SELECT count(*) FROM conversation").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestNativeConcurrentAppendDoesNotCreateGeneration(t *testing.T) {
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
	first, err := s.ImportCodex(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	s.beforeNativeVerify = func() {
		s.beforeNativeVerify = nil
		next := append(append([]byte{}, data...), []byte("{\"type\":\"event_msg\",\"payload\":{\"type\":\"turn_aborted\"}}\n")...)
		if err := os.WriteFile(path, next, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ImportCodex(ctx, path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.ImportCodex(ctx, path); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	latest, err := s.ImportCodex(ctx, path)
	if err != nil || latest.SourceID != first.SourceID || latest.Generation != 1 {
		t.Fatal(latest, err)
	}
}

func TestSnapshotRenderReportsSourceCoverageAndMissingGlyphs(t *testing.T) {
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
	input, err := s.SnapshotInput(ctx, frozen.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := snapcompact.Project(ctx, input, snapcompact.DefaultPreprocessConfig())
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, r := range input.History {
		ids[r.ID] = true
	}
	renderer, err := snapcompact.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	cfg := snapcompact.DefaultRenderConfig()
	cfg.Width = 256
	cfg.Height = 128
	cfg.MaxPages = 1
	partial, err := renderer.Render(ctx, p, cfg)
	if err == nil || len(partial.Uncovered) == 0 {
		t.Fatal("expected explicit resource coverage error")
	}
	for _, r := range partial.Uncovered {
		if !ids[r.SourceID] || r.Field != "projection" {
			t.Fatal(r)
		}
	}
	p.Segments[0].Text += "\U0010FFFF"
	missing, err := renderer.Render(ctx, p, snapcompact.DefaultRenderConfig())
	if !errors.Is(err, snapcompact.ErrMissingGlyph) || len(missing.Missing) == 0 || len(missing.Uncovered) == 0 {
		t.Fatal(missing, err)
	}
	if missing.Missing[0].SourceID != input.History[0].ID {
		t.Fatal("missing glyph lost source mapping")
	}
}

func TestNativeRenameKeepsEvidenceBackedIdentity(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	root := t.TempDir()
	path := filepath.Join(root, "rollout.jsonl")
	data, err := os.ReadFile("../codexrollout/testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	first, err := s.ImportCodex(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(root, "archived.jsonl")
	if err = os.Rename(path, archived); err != nil {
		t.Fatal(err)
	}
	moved, err := s.ImportCodex(ctx, archived)
	if err != nil || moved.SourceID != first.SourceID || moved.Binding != first.Binding {
		t.Fatal(moved, err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	replacement, err := s.ImportCodex(ctx, path)
	if err != nil || replacement.SourceID == first.SourceID {
		t.Fatal(replacement, err)
	}
	moved, err = s.ImportCodex(ctx, archived)
	if err != nil || moved.SourceID != first.SourceID {
		t.Fatal(moved, err)
	}
}

func TestSnapshotPagesUseFrozenGapsAndMatchingProvenanceLimits(t *testing.T) {
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
	page, err := s.SnapshotActivities(ctx, frozen.ID, 0, 2, 4000)
	if err != nil || !page.ReconcileRequired || len(page.Gaps) != len(frozen.Gaps) {
		t.Fatal(page, err)
	}
	refs, err := s.SnapshotProvenance(ctx, frozen.ID, 0, 2)
	if err != nil || len(refs) != len(page.Items) {
		t.Fatal(refs, err)
	}
	for i, p := range refs {
		if p.ActivityID != page.Items[i].Activity.ID || p.Position != page.Items[i].Activity.Position {
			t.Fatal(p)
		}
	}
	if err = s.AddGap(ctx, source.Binding.ProviderThreadID, "tui"); err != nil {
		t.Fatal(err)
	}
	again, err := s.SnapshotActivities(ctx, frozen.ID, 0, 2, 4000)
	if err != nil || !reflect.DeepEqual(again.Gaps, page.Gaps) {
		t.Fatal(again.Gaps, page.Gaps, err)
	}
}

func TestNativeDiagnosticResponseIsBoundedWithoutDroppingRecords(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	data, err := os.ReadFile("../codexrollout/testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte(strings.Repeat("{\"timestamp\":\"2025-12-01T00:00:10Z\",\"type\":\"future_record\",\"payload\":{}}\n", 1000))...)
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := s.ImportCodex(ctx, path)
	if err != nil || len(out.Diagnostics) > 257 || out.Consumed != int64(len(data)) {
		t.Fatal(out, err)
	}
	seen := map[string]bool{}
	for _, d := range out.Diagnostics {
		if seen[d.Category] || d.Category == "diagnostics_limited" {
			t.Fatal(out.Diagnostics)
		}
		seen[d.Category] = true
	}
	if !seen["unknown_record"] {
		t.Fatal(out.Diagnostics)
	}
	var count int
	if err = s.db.QueryRow("SELECT count(*) FROM native_record WHERE source_id=?", out.SourceID).Scan(&count); err != nil || count < 1000 {
		t.Fatal(count, err)
	}
	frozen, err := s.FreezeNative(ctx, out.SourceID)
	if err != nil || slices.Contains(frozen.Gaps, "diagnostics_limited") {
		t.Fatal(frozen, err)
	}
}

func TestNativeLargeToolHistoryProjectsWithinSourceBudget(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	data, err := os.ReadFile("../codexrollout/testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	source.Write(data)
	for range 3 {
		source.WriteString(`{"timestamp":"2025-12-01T00:00:10Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":"`)
		source.WriteString(strings.Repeat("x", 3<<20))
		source.WriteString("\"}}\n")
	}
	if err = os.WriteFile(path, source.Bytes(), 0600); err != nil {
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
	preview, err := s.PreviewNative(ctx, frozen.ID, "task", s.NativePreprocessConfig(), true)
	if err != nil || len(preview.Projection.Omissions) < 3 || len(preview.Render.Pages) == 0 || len(preview.Render.Uncovered) != 0 {
		t.Fatal("large raw history failed bounded projection", err)
	}
}

func TestNativeItemlessRecordIdentityIsEnforcedInStorage(t *testing.T) {
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
	if _, err = s.db.Exec(`INSERT INTO native_record SELECT * FROM native_record WHERE source_id=? AND offset=0 AND activity_id IS NULL`, source.SourceID); err == nil {
		t.Fatal("duplicate itemless native record was accepted")
	}
}

func TestReparsedDiagnosticAccountingDoesNotRewriteFrozenGaps(t *testing.T) {
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
	if err = s.transact(ctx, func(tx *sql.Tx) error {
		return s.addGap(ctx, tx, source.Binding.ProviderThreadID, "diagnostics_limited")
	}); err != nil {
		t.Fatal(err)
	}
	old, err := s.FreezeNative(ctx, source.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportCodex(ctx, path); err != nil {
		t.Fatal(err)
	}
	fresh, err := s.FreezeNative(ctx, source.SourceID)
	if err != nil || slices.Contains(fresh.Gaps, "diagnostics_limited") {
		t.Fatal(fresh, err)
	}
	oldPage, err := s.SnapshotActivities(ctx, old.ID, 0, 20, 4000)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, gap := range oldPage.Gaps {
		found = found || gap.Reason == "diagnostics_limited"
	}
	if !found {
		t.Fatal("reconciliation changed frozen completeness")
	}
}
