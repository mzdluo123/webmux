package historyprojection

import (
	"context"
	"strings"
	"testing"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/rainchan/webmux/internal/historysource"
)

type fixedSource struct{ doc historysource.Document }

func (s fixedSource) Document(context.Context, string) (historysource.Document, error) {
	return s.doc, nil
}

func TestFrozenProjectionKeepsTaskAndOriginalSeparate(t *testing.T) {
	original := strings.Repeat("工具输出🙂\n  code  \n", 500)
	doc := historysource.Document{ParserVersion: "synthetic/v1", Records: []historysource.Record{{ID: "call-result", Kind: "tool_result", Role: "tool", Text: original, CallID: "call", ToolName: "exec", Status: "failed"}}}
	svc := New(Options{Source: fixedSource{doc}, Preprocess: DefaultPreprocessConfig(), Render: snapcompact.DefaultRenderConfig()})
	cfg := svc.Config()
	cfg.OutputRunes = 120
	task := "当前任务🙂\n\n  保留缩进与尾部  \n"
	first, err := svc.Project(context.Background(), "frozen-id", task, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Project(context.Background(), "frozen-id", task, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.Projection.CurrentTask != task || doc.Records[0].Text != original {
		t.Fatal("projection changed immutable input or task identity")
	}
	if len(first.Projection.Omissions) == 0 || first.Projection.Segments[0].CallID != "call" || first.Projection.Segments[0].Status != "failed" {
		t.Fatalf("projection lost omissions or tool association: %+v", first.Projection)
	}
	cfg.OutputRunes++
	changed, err := svc.Project(context.Background(), "frozen-id", task, cfg, false)
	if err != nil || changed.ID == first.ID {
		t.Fatal("projection parameters did not affect derivation identity", err)
	}
}
