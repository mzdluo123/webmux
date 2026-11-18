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
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
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
	var records []historysource.Record
	for _, record := range input.History {
		records = append(records, historysource.Record{ID: record.ID, Role: record.Role, Kind: record.Kind,
			Text: record.Text, CallID: record.CallID, ToolName: record.ToolName, Status: record.Status})
	}
	options := snapcompact.OMPOptions{
		Target: snapcompact.OMPTargetForModel(model), MaxFrames: 8, HistoryTokens: historyCap,
		MaxImageBytes: 20 << 20, RejectUnsupported: true,
	}
	start := time.Now()
	archive, err := historyprojection.CompactOMPDocument(ctx, historysource.Document{Records: records}, options)
	if err != nil {
		return err
	}
	renderMS := float64(time.Since(start).Microseconds()) / 1000
	if archive.Truncated > 0 {
		return fmt.Errorf("benchmark requires full coverage: retained=%d omitted=%d pages=%d image tokens=%d",
			archive.Characters, archive.Truncated, len(archive.Frames), archive.ImageTokens)
	}
	raw := rawHistory(input.History)
	history := archive.Summary + "\n" + archive.HistoryText()
	var candidate strings.Builder
	candidate.WriteString(archive.Summary + "\n")
	for _, block := range archive.Blocks() {
		candidate.WriteString(block.Text)
	}
	texts := map[string]string{
		"raw.txt": raw, "projected.txt": history, "archive.txt": candidate.String(),
		"archive-head.txt": archive.Head, "archive-tail.txt": archive.Tail,
		"archive-retained.txt": history, "task.txt": input.CurrentTask,
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
	pages := make([]map[string]any, 0, len(archive.Frames))
	pngBytes := 0
	for i, page := range archive.Frames {
		name := fmt.Sprintf("archive-%02d.png", i)
		if err := write(name, page.PNG); err != nil {
			return err
		}
		sum := sha256.Sum256(page.PNG)
		pages = append(pages, map[string]any{
			"file": name, "index": i, "width": page.Width, "height": page.Height,
			"png_bytes": len(page.PNG), "detail": page.Detail, "sha256": hex.EncodeToString(sum[:]),
		})
		pngBytes += len(page.PNG)
	}
	sourceRunes, maxRecordRunes := 0, 0
	for _, record := range input.History {
		n := utf8.RuneCountInString(record.Text)
		sourceRunes += n
		maxRecordRunes = max(maxRecordRunes, n)
	}
	cacheKey := sha256.Sum256([]byte(archive.Text))
	manifest := map[string]any{
		"schema": 2, "synthetic_only": true, "scenario": scenario, "seed": seed,
		"module": "github.com/mzdluo123/snapcompact-go", "omp_version": snapcompact.OMPVersion,
		"model": model, "input": input, "archive_config": options, "history_cap": historyCap,
		"expected": expected, "field_sources": fieldSources, "representations": representations,
		"coverage": map[string]any{
			"source_body_runes": sourceRunes, "archive_source_runes": archive.Characters,
			"archive_retained_runes": archive.Characters, "projection_omissions": nil, "archive_omissions": nil,
			"record_count": len(input.History), "max_record_body_runes": maxRecordRunes,
			"same_projected_coverage": true,
			"note":                    "The text baseline uses the same OMP-normalized transcript. Normalization folds whitespace, including code indentation.",
		},
		"archive": map[string]any{
			"pages": pages, "png_bytes": pngBytes, "candidate_bytes": candidate.Len() + pngBytes,
			"render_ms": renderMS, "cache_key": hex.EncodeToString(cacheKey[:]), "shape": archive.Shape,
			"estimated_image_tokens": archive.ImageTokens, "token_estimate_known": archive.ImageTokensVerified,
			"token_basis": archive.TokenBasis,
		},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return write("manifest.json", data)
}
