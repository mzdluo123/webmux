// Command native-image-experiment generates deterministic, synthetic model inputs.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	snapcompact "github.com/mzdluo123/snapcompact-go"
)

func main() {
	scenario := flag.String("scenario", "long", "synthetic history: long prose, tools, or small")
	model := flag.String("model", "", "required model ID for archive geometry")
	seed := flag.Int64("seed", 20261118, "deterministic synthetic seed")
	historyCap := flag.Int64("history-cap", 16000, "history context budget including text and estimated images")
	flag.Parse()
	if flag.NArg() != 1 || *model == "" || *historyCap < 1024 || *historyCap > 64000 || (*scenario != "long" && *scenario != "tools" && *scenario != "small") {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/native-image-experiment -model MODEL [-scenario long|tools|small] [-history-cap 1024..64000] [-seed N] OUTPUT_DIRECTORY")
		os.Exit(2)
	}
	if err := generate(flag.Arg(0), *scenario, *model, *seed, *historyCap); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(dir, scenario, model string, seed, historyCap int64) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	write := func(name string, data []byte) error {
		return os.WriteFile(filepath.Join(dir, name), data, 0600)
	}
	input, expected, fieldSources := syntheticHistory(scenario, seed)
	ctx := context.Background()
	preprocess := snapcompact.DefaultPreprocessConfig()
	start := time.Now()
	projection, err := snapcompact.Project(ctx, input, preprocess)
	if err != nil {
		return err
	}
	projectMS := float64(time.Since(start).Microseconds()) / 1000
	var legend strings.Builder
	for i := range projection.Segments {
		_, body, ok := strings.Cut(projection.Segments[i].Text, "\n")
		if ok {
			projection.Segments[i].Text = strings.TrimPrefix(body, "text:\n")
		}
		fmt.Fprintf(&legend, "s%d=%q\n", i+1, projection.Segments[i].SourceID)
	}
	legendJSON, err := json.Marshal(legend.String())
	if err != nil {
		return err
	}
	available := historyCap - int64(len(legendJSON)) - 1024
	if available <= 0 {
		return fmt.Errorf("history budget cannot hold source mapping")
	}
	edgeBytes := min(available/3, 8192)
	renderer, err := snapcompact.NewArchiveRenderer()
	if err != nil {
		return err
	}
	config := snapcompact.ArchiveConfig{Model: model, MaxPages: 8,
		MaxTextBytes: max(int64(1), edgeBytes/2), MaxImageTokens: available - edgeBytes,
		MaxImageBytes: 20 << 20}
	start = time.Now()
	archive, err := renderer.Compact(ctx, projection, config)
	if err != nil {
		return err
	}
	renderMS := float64(time.Since(start).Microseconds()) / 1000
	// Never accept a cheaper candidate whose apparent savings came from lost history.
	if len(projection.Omissions) != 0 || len(archive.Omitted) != 0 ||
		len(archive.Render.Uncovered) != 0 || len(archive.Render.Missing) != 0 ||
		archive.SourceRunes != archive.RetainedRunes {
		return fmt.Errorf("benchmark requires full coverage: projection omissions=%d archive omissions=%d uncovered=%d missing=%d source=%d retained=%d",
			len(projection.Omissions), len(archive.Omitted), len(archive.Render.Uncovered),
			len(archive.Render.Missing), archive.SourceRunes, archive.RetainedRunes)
	}
	if scenario == "long" && len(archive.Render.Pages) < 3 {
		return fmt.Errorf("long scenario needs at least three middle frames; got %d", len(archive.Render.Pages))
	}
	raw := rawHistory(input.History)
	candidate := legend.String() + archive.TextHead
	if len(archive.Render.Pages) > 0 {
		candidate += "\n\n[Chronological middle history: attached PNG pages in listed order.]\n\n"
	}
	candidate += archive.TextTail
	texts := map[string]string{
		"raw.txt": raw, "projected.txt": legend.String() + archive.HistoryText, "archive.txt": candidate,
		"archive-head.txt": archive.TextHead, "archive-tail.txt": archive.TextTail,
		"archive-retained.txt": legend.String() + archive.HistoryText, "task.txt": input.CurrentTask,
	}
	representations := make(map[string]any, len(texts))
	for name, value := range texts {
		data := []byte(value)
		if err := write(name, data); err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		representations[name] = map[string]any{"bytes": len(data), "runes": utf8.RuneCountInString(value), "sha256": hex.EncodeToString(sum[:])}
	}
	pages := make([]map[string]any, 0, len(archive.Render.Pages))
	pngBytes := 0
	for i, page := range archive.Render.Pages {
		name := fmt.Sprintf("archive-%02d.png", i)
		if err := write(name, page.PNG); err != nil {
			return err
		}
		sum := sha256.Sum256(page.PNG)
		pages = append(pages, map[string]any{
			"file": name, "index": page.Index, "width": page.Width, "height": page.Height,
			"png_bytes": len(page.PNG), "ranges": page.Ranges, "sha256": hex.EncodeToString(sum[:]),
		})
		pngBytes += len(page.PNG)
	}
	sourceRunes, projectionRunes, maxRecordRunes := 0, 0, 0
	for _, record := range input.History {
		n := utf8.RuneCountInString(record.Text)
		sourceRunes += n
		maxRecordRunes = max(maxRecordRunes, n)
	}
	for _, segment := range projection.Segments {
		projectionRunes += utf8.RuneCountInString(segment.Text)
	}
	manifest := map[string]any{
		"schema": 2, "synthetic_only": true, "scenario": scenario, "seed": seed,
		"module": "github.com/mzdluo123/snapcompact-go", "projection_version": projection.Version,
		"model": model, "input": input, "projection": projection, "preprocess_config": preprocess,
		"archive_config": config, "history_cap": historyCap, "expected": expected, "field_sources": fieldSources,
		"representations": representations, "project_ms": projectMS,
		"coverage": map[string]any{
			"source_body_runes": sourceRunes, "projection_segment_runes": projectionRunes,
			"archive_source_runes": archive.SourceRunes, "archive_retained_runes": archive.RetainedRunes,
			"projection_omissions": projection.Omissions, "archive_omissions": archive.Omitted,
			"record_count": len(input.History), "max_record_body_runes": maxRecordRunes,
			"same_projected_coverage": true,
			"note":                    "Projection excludes generated identity headers, matching WebMux. projected.txt is the complete archive serialization and identical source legend, without budget omissions.",
		},
		"archive": map[string]any{
			"pages": pages, "png_bytes": pngBytes, "candidate_bytes": len(candidate) + pngBytes,
			"render_ms": renderMS, "text_ranges": archive.TextRanges,
			"missing_glyphs": archive.Render.Missing, "uncovered": archive.Render.Uncovered,
			"cache_key": archive.Render.CacheKey, "shape": archive.Shape,
			"estimated_image_tokens": archive.ImageTokens, "token_estimate_known": archive.TokenEstimateKnown,
			"token_basis": archive.TokenBasis,
		},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return write("manifest.json", data)
}
