package handoff

import (
	"context"

	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/hybrid"
)

func (s *Service) imageCapability(ctx context.Context, target string) (string, error) {
	if target == "" {
		return "pending", nil
	}
	provider, ok := s.options.Hybrid.(interface {
		ImageCapabilities(context.Context, string) (hybrid.PromptCapabilities, error)
	})
	if !ok {
		return "unsupported", nil
	}
	caps, err := provider.ImageCapabilities(ctx, target)
	if err != nil {
		return "unsupported", err
	}
	if caps.Image {
		return "supported", nil
	}
	return "unsupported", nil
}

func (s *Service) preparePreview(ctx context.Context, in Request, doc historysource.Document, header string, budget Budget) (*Preview, error) {
	p := &Preview{Format: in.Format, Capability: "pending", UserText: in.UserText, Budget: budget, Pages: []ImagePage{}, ImageBudget: ImageBudget{MaxPages: in.MaxPages, MaxBytes: int(hybrid.MaxPromptAttachmentBytes), MaxRequestBytes: int(min(s.options.MaxPromptWireBytes, hybrid.MaxPromptWireBytes))}}
	if err := s.prepareArchive(ctx, in, doc, header, p); err != nil {
		return nil, err
	}
	if err := hybrid.ValidatePromptText(p.Text, len(p.Pages)); err != nil || int64(len(p.Text)) > s.options.MaxPromptBytes {
		p.BlockedReason = "交接文本超过提示长度限制，请缩短任务后重新预览"
	}
	p.Budget.OverBudget = p.Budget.OverBudget || p.Budget.Used > p.Budget.History
	if p.Budget.OverBudget {
		p.BlockedReason = "交接内容超过目标上下文预算，请调整后重新预览"
	}
	return p, nil
}

func previewImages(p *Preview) []hybrid.PromptImage {
	images := make([]hybrid.PromptImage, len(p.Pages))
	for i, page := range p.Pages {
		images[i] = hybrid.PromptImage{MimeType: page.MimeType, Data: page.Data}
	}
	return images
}
