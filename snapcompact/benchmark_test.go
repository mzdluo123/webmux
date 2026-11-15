package snapcompact

import (
	"context"
	"strings"
	"testing"
)

func benchmarkHistory() Input {
	return Input{CurrentTask: "继续检查分页边界", History: []Record{
		{ID: "request", Role: "user", Kind: "prose", Text: "请检查日志并保留原始引用。"},
		{ID: "call", Role: "assistant", ToolName: "read", CallID: "call-1", Arguments: map[string]any{"path": "sanitized.log", "offset": 0}},
		{ID: "result", Role: "tool", ToolName: "read", CallID: "call-1", Status: "ok", Kind: "structured", Text: strings.Repeat("2026-01-01 INFO 中文日志: status=ok\n", 300)},
		{ID: "answer", Role: "assistant", Kind: "code", Text: "func main() {\n\tprintln(\"ready\")\n}"},
	}}
}
func BenchmarkProject(b *testing.B) {
	input := benchmarkHistory()
	cfg := DefaultPreprocessConfig()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Project(context.Background(), input, cfg); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkRender(b *testing.B) {
	p, err := Project(context.Background(), benchmarkHistory(), DefaultPreprocessConfig())
	if err != nil {
		b.Fatal(err)
	}
	renderer, err := NewRenderer()
	if err != nil {
		b.Fatal(err)
	}
	cfg := DefaultRenderConfig()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result, err := renderer.Render(context.Background(), p, cfg)
		if err != nil {
			b.Fatal(err)
		}
		size := 0
		for _, page := range result.Pages {
			size += len(page.PNG)
		}
		b.ReportMetric(float64(size), "png-bytes/op")
		b.ReportMetric(float64(len(result.Pages)), "pages/op")
	}
}
