// Package historyprojection converts frozen provider-neutral exports into
// mechanical text/image projections. It never reads native files or live parents.
package historyprojection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	out.Gaps = doc.Gaps
	input := snapcompact.Input{CurrentTask: task, History: make([]snapcompact.Record, 0, len(doc.Records))}
	for _, record := range doc.Records {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		// Unknown structure stays conservative; only an explicit prose message
		// is eligible for prose whitespace folding. Tool bodies retain lines.
		kind := "structured"
		if record.Kind == "message" {
			kind = "prose"
		}
		item := snapcompact.Record{ID: record.ID, Role: record.Role, ToolName: record.ToolName, CallID: record.CallID, Status: record.Status, Kind: kind, Text: record.Text}
		if record.Kind == "tool_arguments" {
			item.CallBody, item.Text = record.Text, ""
		}
		input.History = append(input.History, item)
	}
	out.Projection, err = snapcompact.Project(ctx, input, cfg)
	if err != nil {
		return out, err
	}
	identity := fmt.Sprintf("%s:%s:%s:%+v", id, doc.ParserVersion, out.Projection.CacheKey, cfg)
	if render {
		s.once.Do(func() { s.renderer, s.renderError = snapcompact.NewRenderer() })
		if s.renderError != nil {
			return out, s.renderError
		}
		out.Render, err = s.renderer.Render(ctx, out.Projection, s.render)
		identity += fmt.Sprintf(":%+v:%s", s.render, out.Render.CacheKey)
	}
	sum := sha256.Sum256([]byte(identity))
	out.ID = hex.EncodeToString(sum[:])
	return out, err
}
