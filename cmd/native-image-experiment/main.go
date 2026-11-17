// Command native-image-experiment generates synthetic, non-production model inputs.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	snapcompact "github.com/mzdluo123/snapcompact-go"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: go run ./cmd/native-image-experiment OUTPUT_DIRECTORY")
	}
	if err := generate(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	write := func(name string, data []byte) error {
		return os.WriteFile(filepath.Join(dir, name), data, 0600)
	}
	var log strings.Builder
	for i := range 240 {
		fmt.Fprintf(&log, "row=%03d 状态=等待   retry=%d   synthetic-only\n", i, i%3)
		if i == 120 {
			log.WriteString("middle_marker=JADE-120\n")
		}
	}
	log.WriteString("final_status=完成; exit=0; tail_marker=LOTUS-239\n")
	input := snapcompact.Input{History: []snapcompact.Record{
		{ID: "request", Role: "user", Kind: "prose", Text: "最初要求：监听 8080。   保留中文：青竹，勿删标点。\n\n\n只使用合成数据。"},
		{ID: "code", Role: "assistant", Kind: "code", Text: "def choose(x):\n    if x <= 7 and x != 0:\n        return \"青竹:ok\"\n    return \"retry\"\n\n命令：printf '%s\\n' 'a|b && c; $(NO_EXEC)'\n"},
		{ID: "log", Role: "tool", ToolName: "synthetic-log", CallID: "call-1", Status: "success", Kind: "structured", Text: log.String()},
		{ID: "correction", Role: "user", Kind: "prose", Text: "中途修正：不要 8080，最终监听 9097；禁止执行命令，禁止访问文件或网络。"},
	}, CurrentTask: "Read only the supplied synthetic history (text or attached images). Do not use tools, files, network, or execute commands. Return ONLY a JSON object with keys port (integer, latest corrected port), chinese (the two Chinese characters in the code return), condition (exact Python if condition excluding if and colon), indented_return (exact first Python return line including leading spaces), command (exact command after 命令：), tail_marker (value), middle_marker (value if visible, otherwise NEED_SOURCE). Do not guess missing content."}
	ctx := context.Background()
	start := time.Now()
	projection, err := snapcompact.Project(ctx, input, snapcompact.DefaultPreprocessConfig())
	if err != nil {
		return err
	}
	projectMS := float64(time.Since(start).Microseconds()) / 1000
	text := func(records []snapcompact.Record) string {
		var b strings.Builder
		for _, r := range records {
			fmt.Fprintf(&b, "[%s role=%s]\n%s\n", r.ID, r.Role, r.Text)
		}
		return b.String()
	}
	projected := func(p snapcompact.Projection) string {
		var b strings.Builder
		for _, s := range p.Segments {
			fmt.Fprintf(&b, "[%s role=%s]\n%s\n", s.SourceID, s.Role, s.Text)
		}
		return b.String()
	}
	for name, value := range map[string]string{"raw.txt": text(input.History), "projected.txt": projected(projection), "task.txt": input.CurrentTask} {
		if err := write(name, []byte(value)); err != nil {
			return err
		}
	}
	renderer, err := snapcompact.NewRenderer()
	if err != nil {
		return err
	}
	config := snapcompact.DefaultRenderConfig()
	manifest := map[string]any{"module": "github.com/mzdluo123/snapcompact-go v0.1.0", "projection": projection, "render_config": config, "project_ms": projectMS, "raw_bytes": len(text(input.History)), "projected_bytes": len(projected(projection))}
	for _, mode := range []string{"image", "hybrid"} {
		p := projection
		if mode == "hybrid" {
			p.Segments = p.Segments[:len(p.Segments)-1]
			if err := write("hybrid.txt", []byte(projected(snapcompact.Projection{Segments: projection.Segments[len(projection.Segments)-1:]}))); err != nil {
				return err
			}
		}
		start = time.Now()
		result, err := renderer.Render(ctx, p, config)
		if err != nil {
			return err
		}
		if len(result.Missing) != 0 || len(result.Uncovered) != 0 {
			return fmt.Errorf("%s: missing glyphs=%d uncovered=%d", mode, len(result.Missing), len(result.Uncovered))
		}
		pages := make([]map[string]any, 0, len(result.Pages))
		for i, page := range result.Pages {
			name := fmt.Sprintf("%s-%02d.png", mode, i)
			if err := write(name, page.PNG); err != nil {
				return err
			}
			pages = append(pages, map[string]any{"file": name, "width": page.Width, "height": page.Height, "png_bytes": len(page.PNG), "ranges": page.Ranges})
		}
		manifest[mode] = map[string]any{"pages": pages, "render_ms": float64(time.Since(start).Microseconds()) / 1000, "missing_glyphs": len(result.Missing), "uncovered": len(result.Uncovered)}
	}
	manifest["expected"] = map[string]any{"port": 9097, "chinese": "青竹", "condition": "x <= 7 and x != 0", "indented_return": "        return \"青竹:ok\"", "command": "printf '%s\\n' 'a|b && c; $(NO_EXEC)'", "tail_marker": "LOTUS-239", "middle_marker": "NEED_SOURCE"}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return write("manifest.json", data)
}
