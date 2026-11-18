package historyprojection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestNativeToolKindsPreserveArgumentsAndAssociation(t *testing.T) {
	tests := []struct {
		name       string
		callKind   string
		resultKind string
	}{
		{name: "native", callKind: "tool-call", resultKind: "tool-result"},
		{name: "normalized", callKind: "tool_arguments", resultKind: "tool_result"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const args = `{"limit":9007199254740993,"command":"fixture"}`
			original := strings.Repeat("output\n", 100)
			doc := historysource.Document{Records: []historysource.Record{
				{ID: "call", Kind: tt.callKind, Role: "assistant", Text: args, ToolName: "exec", CallID: "native-call"},
				{ID: "result", Kind: tt.resultKind, Role: "tool", Text: original, CallID: "native-call", Status: "failed"},
			}}
			svc := New(Options{Preprocess: DefaultPreprocessConfig()})
			cfg := svc.Config()
			cfg.OutputRunes = 20
			preview, err := svc.ProjectDocument(context.Background(), doc, "原样任务\n  ", cfg, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(preview.Projection.Segments) != 2 {
				t.Fatalf("native tools filtered: %+v", preview.Projection)
			}
			call, result := preview.Projection.Segments[0], preview.Projection.Segments[1]
			if call.Role != "tool_call" || !strings.Contains(call.Text, `"limit":9007199254740993`) || len(call.Sources) != 1 || call.Sources[0].Field != "arguments" {
				t.Fatalf("argument extraction: %+v", call)
			}
			if result.Role != "tool" || result.ToolName != "exec" || result.CallID != call.CallID || result.Status != "failed" {
				t.Fatalf("tool association: %+v", result)
			}
			if len(preview.Projection.Omissions) != 1 || preview.Projection.Omissions[0].Range.SourceID != "result" || preview.Projection.Omissions[0].Range.Field != "text" {
				t.Fatalf("output budget: %+v", preview.Projection.Omissions)
			}
			for _, segment := range preview.Projection.Segments {
				if strings.HasPrefix(segment.Text, `{"Source":`) || strings.Contains(segment.Text, "[role=") {
					t.Fatalf("duplicate identity wrapper: %q", segment.Text)
				}
			}
			if doc.Records[0].Text != args || doc.Records[1].Text != original || preview.Projection.CurrentTask != "原样任务\n  " {
				t.Fatal("projection modified frozen source or task")
			}
		})
	}
}

func TestNativeCallBodyUsesCallBudget(t *testing.T) {
	doc := historysource.Document{Records: []historysource.Record{
		{ID: "call", Kind: "tool-call", Role: "assistant", Text: strings.Repeat("shell command\n", 100), ToolName: "exec", CallID: "c"},
	}}
	svc := New(Options{Preprocess: DefaultPreprocessConfig()})
	cfg := svc.Config()
	cfg.CallRunes = 20
	preview, err := svc.ProjectDocument(context.Background(), doc, "", cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Projection.Omissions) != 1 || preview.Projection.Omissions[0].Range.Field != "call_body" {
		t.Fatalf("call body budget: %+v", preview.Projection)
	}
}

func TestRenderProjectionPageBudgetAndCompleteReselection(t *testing.T) {
	cfg := snapcompact.DefaultRenderConfig()
	cfg.Height, cfg.MinHeight = 80, 64
	svc := New(Options{Render: cfg})
	projection := snapcompact.Projection{Segments: []snapcompact.Segment{
		{SourceID: "a", Role: "user", Text: strings.Repeat("line\n", 50)},
		{SourceID: "b", Role: "assistant", Text: "retained"},
	}}
	partial, err := svc.RenderProjection(context.Background(), projection, 1)
	if !errors.Is(err, snapcompact.ErrRenderLimit) || len(partial.Pages) != 1 || len(partial.Uncovered) == 0 {
		t.Fatalf("page budget: pages=%d uncovered=%+v err=%v", len(partial.Pages), partial.Uncovered, err)
	}
	for _, r := range partial.Pages[0].Ranges {
		if r.Start != 0 || r.End != 0 {
			t.Fatalf("header-only page claimed body coverage: %+v", r)
		}
	}
	selected := projection
	selected.Segments = projection.Segments[1:]
	complete, err := svc.RenderProjection(context.Background(), selected)
	if err != nil || len(complete.Uncovered) != 0 || len(complete.Pages) != 2 {
		t.Fatalf("reselection: pages=%d uncovered=%+v err=%v", len(complete.Pages), complete.Uncovered, err)
	}
	end := 0
	for _, page := range complete.Pages {
		for _, r := range page.Ranges {
			if r.SourceID != "b" || r.Field != "projection" {
				t.Fatalf("selected source association: %+v", r)
			}
			end = max(end, r.End)
		}
	}
	if end != utf8.RuneCountInString(selected.Segments[0].Text) {
		t.Fatalf("coverage=%d", end)
	}
	if svc.render.MaxPages != cfg.MaxPages || len(projection.Segments) != 2 {
		t.Fatal("page budget or reselection mutated input")
	}
}

func TestRenderProjectionMissingGlyphAndInvalidConfiguredLimit(t *testing.T) {
	svc := New(Options{})
	projection := snapcompact.Projection{Segments: []snapcompact.Segment{{SourceID: "a", Role: "user", Text: "\U0010ffff"}}}
	render, err := svc.RenderProjection(context.Background(), projection)
	if !errors.Is(err, snapcompact.ErrMissingGlyph) || len(render.Pages) != 0 || len(render.Missing) != 1 || len(render.Uncovered) != 1 {
		t.Fatalf("missing glyph coverage: %+v %v", render, err)
	}
	cfg := snapcompact.DefaultRenderConfig()
	cfg.MaxPages = 0
	invalid := New(Options{Render: cfg})
	projection.Segments[0].Text = "plain text"
	if _, err := invalid.RenderProjection(context.Background(), projection, 1); !errors.Is(err, snapcompact.ErrRenderInput) {
		t.Fatalf("configured invalid limit was hidden: %v", err)
	}
}

func TestProjectionSkipsEmptyMessagesButKeepsToolAssociation(t *testing.T) {
	doc := historysource.Document{Records: []historysource.Record{
		{ID: "empty", Kind: "message", Role: "assistant", Text: " \n\t"},
		{ID: "call", Kind: "tool-call", Role: "assistant", CallID: "c1", ToolName: "check", Text: "{}"},
		{ID: "result", Kind: "tool-result", Role: "tool", CallID: "c1", Status: "completed"},
	}}
	service := New(Options{Preprocess: DefaultPreprocessConfig()})
	preview, err := service.ProjectDocument(context.Background(), doc, "task", service.Config(), false)
	if err != nil {
		t.Fatal(err)
	}
	segments := preview.Projection.Segments
	if len(segments) != 2 || segments[0].SourceID != "call" || segments[1].SourceID != "result" || segments[1].CallID != "c1" || segments[1].Status != "completed" {
		t.Fatalf("lost meaningful tool records: %+v", segments)
	}
}
