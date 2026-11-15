package conversation

import (
	"context"
	"fmt"

	snapcompact "github.com/mzdluo123/snapcompact-go"
)

type SnapshotPreview struct {
	Gaps       []string
	ID         string
	Projection snapcompact.Projection
	Render     snapcompact.RenderResult
}

// PreviewNative is local derivation only. Callers receive rendering limitations;
// a PNG is not evidence of model readability, transport support or token savings.
func (s *Service) PreviewNative(ctx context.Context, id, task string, cfg snapcompact.PreprocessConfig, render bool) (SnapshotPreview, error) {
	var out SnapshotPreview
	snapshot, err := s.NativeSnapshot(ctx, id)
	if err != nil {
		return out, err
	}
	out.Gaps = snapshot.Gaps
	input, err := s.SnapshotInput(ctx, id, task)
	if err != nil {
		return out, err
	}
	out.Projection, err = snapcompact.Project(ctx, input, cfg)
	if err != nil {
		return out, err
	}
	config := fmt.Sprintf("%+v", cfg)
	out.ID = digest([]byte(id + snapcompact.Version + out.Projection.CacheKey + string(config)))
	if render {
		renderer, err := snapcompact.NewRenderer()
		if err != nil {
			return out, err
		}
		out.Render, err = renderer.Render(ctx, out.Projection, snapcompact.DefaultRenderConfig())
		out.ID = digest([]byte(out.ID + out.Render.CacheKey))
		return out, err
	}
	return out, nil
}
