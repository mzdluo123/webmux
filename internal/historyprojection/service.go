// Package historyprojection converts frozen provider-neutral exports into
// mechanical text/image projections. It never reads native files or live parents.
package historyprojection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/rainchan/webmux/internal/historysource"
)

type Source interface {
	Document(context.Context, string) (historysource.Document, error)
}

type Options struct {
	Source     Source
	Preprocess snapcompact.PreprocessConfig
	Render     snapcompact.RenderConfig
}

type Service struct {
	source      Source
	preprocess  snapcompact.PreprocessConfig
	render      snapcompact.RenderConfig
	once        sync.Once
	renderer    snapcompact.Renderer
	renderError error
}

type Preview struct {
	ID         string
	Gaps       []historysource.Gap
	Projection snapcompact.Projection
	Render     snapcompact.RenderResult
}

func New(o Options) *Service {
	if o.Render == (snapcompact.RenderConfig{}) {
		o.Render = snapcompact.DefaultRenderConfig()
	}
	return &Service{source: o.Source, preprocess: o.Preprocess, render: o.Render}
}

func DefaultPreprocessConfig() snapcompact.PreprocessConfig {
	cfg := snapcompact.DefaultPreprocessConfig()
	cfg.MaxInputBytes = historysource.MaxBytes
	return cfg
}

func (s *Service) Config() snapcompact.PreprocessConfig { return s.preprocess }

func (s *Service) Project(ctx context.Context, id, task string, cfg snapcompact.PreprocessConfig, render bool) (Preview, error) {
	var out Preview
	if s == nil || s.source == nil {
		return out, historysource.ErrUnavailable
	}
	doc, err := s.source.Document(ctx, id)
	if err != nil {
		return out, err
	}
	return s.ProjectDocument(ctx, doc, task, cfg, render)
}

// ProjectDocument derives a view from an already-frozen transient composition.
// It never reopens a live source or changes the current task.
func (s *Service) ProjectDocument(ctx context.Context, doc historysource.Document, task string, cfg snapcompact.PreprocessConfig, render bool) (Preview, error) {
	var out Preview
	if s == nil {
		return out, historysource.ErrUnavailable
	}
	var err error
	out.Gaps = doc.Gaps
	input := snapcompact.Input{CurrentTask: task, History: make([]snapcompact.Record, 0, len(doc.Records))}
	var calls map[string]string
	for _, record := range doc.Records {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if !record.Exportable() {
			continue
		}
		if strings.TrimSpace(record.Text) == "" && record.ToolName == "" && record.CallID == "" && record.Status == "" {
			continue
		}
		// Unknown structure stays conservative; only an explicit prose message
		// is eligible for prose whitespace folding. Tool bodies retain lines.
		kind := "structured"
		if record.Kind == "prose" {
			kind = "prose"
		}
		item := snapcompact.Record{ID: record.ID, Role: record.Role, ToolName: record.ToolName, CallID: record.CallID, Status: record.Status, Kind: kind, Text: record.Text}
		if item.Role == "agent" {
			item.Role = "assistant"
		}
		switch record.Kind {
		case "tool_arguments", "tool-call":
			item.Role = "tool_call"
			decoder := json.NewDecoder(strings.NewReader(record.Text))
			decoder.UseNumber()
			var args map[string]any
			if decoder.Decode(&args) == nil && args != nil && decoder.Decode(new(any)) == io.EOF {
				item.Arguments, item.Text = args, ""
			} else {
				item.CallBody, item.Text = record.Text, ""
			}
		case "tool":
			item.Role = "tool_call"
			item.CallBody, item.Text = record.Text, ""
		case "tool_result", "tool-result":
			item.Role = "tool"
			if item.ToolName == "" {
				item.ToolName = calls[item.CallID]
			}
		}
		if item.Role == "tool_call" && item.CallID != "" && item.ToolName != "" {
			if calls == nil {
				calls = make(map[string]string)
			}
			calls[item.CallID] = item.ToolName
		}
		input.History = append(input.History, item)
	}
	out.Projection, err = snapcompact.Project(ctx, input, cfg)
	if err != nil {
		return out, err
	}
	// v0.1.0 在正文首行写入身份 JSON；图片 renderer 自带身份标题。
	// 正文保留字段标签与原始 Sources，文本交接由调用方添加一次标题。
	for i := range out.Projection.Segments {
		_, body, ok := strings.Cut(out.Projection.Segments[i].Text, "\n")
		if ok {
			record := input.History[i]
			if record.Arguments == nil && record.CallBody == "" {
				body = strings.TrimPrefix(body, "text:\n")
			} else if record.Text == "" {
				body = strings.TrimSuffix(body, "text:\n\n")
			}
			out.Projection.Segments[i].Text = body
		}
	}
	identity := fmt.Sprintf("compact-body-1:%s:%s:%s:%+v", doc.Cursor, doc.ParserVersion, out.Projection.CacheKey, cfg)
	if render {
		out.Render, err = s.RenderProjection(ctx, out.Projection)
		identity += fmt.Sprintf(":%+v:%s", s.render, out.Render.CacheKey)
	}
	sum := sha256.Sum256([]byte(identity))
	out.ID = hex.EncodeToString(sum[:])
	return out, err
}

// RenderProjection renders the selected complete segments without changing
// their source association or reopening native history.
func (s *Service) RenderProjection(ctx context.Context, projection snapcompact.Projection, maxPages ...int) (snapcompact.RenderResult, error) {
	s.once.Do(func() { s.renderer, s.renderError = snapcompact.NewRenderer() })
	if s.renderError != nil {
		return snapcompact.RenderResult{}, s.renderError
	}
	cfg := s.render
	if len(maxPages) > 0 && maxPages[0] > 0 {
		cfg.MaxPages = min(cfg.MaxPages, maxPages[0])
	}
	return s.renderer.Render(ctx, projection, cfg)
}
