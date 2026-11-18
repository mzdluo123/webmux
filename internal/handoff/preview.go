package handoff

import (
	"context"
	"fmt"
	"strings"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/hybrid"
)

func segmentText(segment snapcompact.Segment) string {
	return fmt.Sprintf("[%s id=%s tool=%s call=%s status=%s]\n%s\n", segment.Role, segment.SourceID, segment.ToolName, segment.CallID, segment.Status, segment.Text)
}

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
	if in.Format == "image" && len(s.options.OMPRunner) > 0 {
		if err := s.prepareOMPArchive(ctx, in, doc, header, p); err != nil {
			return nil, err
		}
		return s.validatePreview(p)
	}
	projected, err := s.options.HistoryProjection.ProjectDocument(ctx, doc, in.UserText, s.options.HistoryProjection.Config(), false)
	if err != nil {
		return nil, err
	}
	projection := projected.Projection
	segments := projection.Segments
	p.OmittedCount = len(projection.Omissions)
	if in.Format == "text" {
		available := max(int64(0), budget.History-jsonBytes(header)-512)
		selected := make([]snapcompact.Segment, 0, len(segments))
		for i := len(segments) - 1; i >= 0; i-- {
			cost := jsonBytes(segmentText(segments[i]))
			if cost > available {
				p.OmittedCount++
				continue
			}
			available -= cost
			selected = append(selected, segments[i])
		}
		for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
			selected[i], selected[j] = selected[j], selected[i]
		}
		segments = selected
	} else {
		if err := s.prepareArchive(ctx, in, projection, header, p); err != nil {
			return nil, err
		}
		return s.validatePreview(p)
	}
	p.SelectedCount = len(segments)
	var history strings.Builder
	history.WriteString(header)
	fmt.Fprintf(&history, "包含片段：%d；省略：%d。\n", p.SelectedCount, p.OmittedCount)
	for _, segment := range segments {
		history.WriteString(segmentText(segment))
	}
	p.HistoryText = history.String()
	p.Text = p.HistoryText + "\n当前用户任务：\n" + in.UserText
	p.Budget.Used = jsonBytes(p.HistoryText)
	return s.validatePreview(p)
}

func (s *Service) validatePreview(p *Preview) (*Preview, error) {
	if err := hybrid.ValidatePromptText(p.Text, len(p.Pages)); err != nil || int64(len(p.Text)) > s.options.MaxPromptBytes {
		p.BlockedReason = "交接文本超过提示长度限制，请缩短任务后重新预览"
		if p.Format == "text" {
			if err != nil {
				return nil, err
			}
			return nil, ErrBudget
		}
	}
	p.Budget.OverBudget = p.Budget.OverBudget || p.Budget.Used > p.Budget.History
	if p.Budget.OverBudget {
		if p.Format == "text" {
			return nil, ErrBudget
		}
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
