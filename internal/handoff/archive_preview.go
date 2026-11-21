package handoff

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sync"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/hybrid"
)

func (s *Service) archiveModel(in Request) (string, error) {
	if in.TargetSessionID == "" {
		return in.AgentConfig["model"], nil
	}
	record, err := s.options.Hybrid.Record(in.TargetSessionID)
	if err != nil {
		return "", err
	}
	return record.Config["model"], nil
}

func (s *Service) prepareArchive(ctx context.Context, in Request, doc historysource.Document, header string, p *Preview) error {
	model, err := s.archiveModel(in)
	if err != nil {
		return err
	}
	guide := header + "Read the history images in page order as reference for the current task. A black cell marks a newline.\n"
	p.imageTextOffset = len(guide)
	p.Text = guide + "\n当前用户任务：\n" + in.UserText
	p.Budget.Used = historyprojection.ArchiveTextTokens(guide + "\n当前用户任务：\n")
	tokens := p.Budget.History - p.Budget.Used
	wire := int64(p.ImageBudget.MaxRequestBytes) - jsonBytes(p.Text) - int64(1024+in.MaxPages*80)
	if tokens <= 0 || wire < 2 {
		p.Budget.OverBudget = true
		return nil
	}
	projected, err := s.options.HistoryProjection.ProjectDocument(ctx, doc, "", s.options.HistoryProjection.Config(), false)
	if err != nil {
		return err
	}
	projection := projected.Projection
	p.OmittedCount = len(projection.Omissions)
	if len(projection.Segments) == 0 {
		return nil
	}
	// Snapcompact uses zero for the default text budget. A one-byte budget
	// leaves only this record terminator outside the rasterized pages.
	last := &projection.Segments[len(projection.Segments)-1]
	last.Text += "\n"
	renderer, err := imageArchiveRenderer()
	if err != nil {
		return err
	}
	archive, err := renderer.Compact(ctx, projection, snapcompact.ArchiveConfig{
		Model: model, MaxPages: in.MaxPages, MaxImageTokens: tokens, MaxTextBytes: 1,
		MaxImageBytes: min(hybrid.MaxPromptAttachmentBytes, wire*3/4),
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch {
	case errors.Is(err, snapcompact.ErrMissingGlyph):
		p.BlockedReason = "历史包含图片字体无法显示的字符"
		return nil
	case err != nil:
		p.BlockedReason = "历史图片渲染失败或超过资源限制"
		return nil
	case archive.TextHead != "" || archive.TextTail != "\n":
		p.BlockedReason = "历史正文未完整转为图片"
		return nil
	}
	applyArchive(archive, p)
	if len(p.Pages) == 0 {
		p.BlockedReason = "当前预算无法容纳历史图片"
		return nil
	}
	capability, capabilityErr := s.imageCapability(ctx, in.TargetSessionID)
	p.Capability = capability
	if capabilityErr != nil {
		p.BlockedReason = "目标会话图片能力暂不可确认，请重新预览"
	} else if capability == "unsupported" {
		p.BlockedReason = "目标 agent 不支持图片，请选择支持图片的目标重新预览"
	}
	if wire, wireErr := hybrid.ValidatePromptImages(p.Text, previewImages(p)); wireErr != nil || int64(wire) > s.options.MaxPromptWireBytes {
		p.BlockedReason = "历史图片超过附件或编码请求限制，请减少图片页数后重新预览"
	}
	return ctx.Err()
}

var imageArchiveRenderer = sync.OnceValues(snapcompact.NewArchiveRenderer)

func applyArchive(archive snapcompact.Archive, p *Preview) {
	selected := make(map[string]bool)
	for _, page := range archive.Render.Pages {
		for _, r := range page.Ranges {
			selected[r.SourceID] = true
		}
		sum := sha256.Sum256(page.PNG)
		p.Pages = append(p.Pages, ImagePage{
			Index: page.Index, MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(page.PNG),
			SHA256: hex.EncodeToString(sum[:]), Width: page.Width, Height: page.Height, Bytes: len(page.PNG),
		})
		p.ImageBudget.Bytes += len(page.PNG)
	}
	p.SelectedCount = len(selected)
	p.OmittedCount += len(archive.Omitted)
	p.ImageBudget.ProjectedRunes = archive.SourceRunes - 1
	p.ImageBudget.RetainedRunes = archive.RetainedRunes - 1
	p.ImageBudget.Pages = len(p.Pages)
	p.ImageBudget.EstimatedTokens = archive.ImageTokens
	p.ImageBudget.TokenEstimateKnown = archive.TokenEstimateKnown
	p.ImageBudget.TokenBasis = archive.TokenBasis
	p.ImageBudget.Shape = archive.Shape
	p.HistoryText = archive.HistoryText
	p.Budget.Used += archive.ImageTokens
}
