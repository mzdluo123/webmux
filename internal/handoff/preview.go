package handoff

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	snapcompact "github.com/mzdluo123/snapcompact-go"
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

func (s *Service) preparePreview(ctx context.Context, in Request, projection snapcompact.Projection, header string, budget Budget) (*Preview, error) {
	p := &Preview{Format: in.Format, Capability: "pending", UserText: in.UserText, Budget: budget, Pages: []ImagePage{}, ImageBudget: ImageBudget{MaxPages: in.MaxPages, MaxBytes: int(hybrid.MaxPromptAttachmentBytes), MaxRequestBytes: int(min(s.options.MaxPromptWireBytes, hybrid.MaxPromptWireBytes))}}
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
		capability, err := s.imageCapability(ctx, in.TargetSessionID)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p.Capability = capability
		if err != nil {
			p.BlockedReason = "目标会话图片能力暂不可确认，请重新预览或选择文本模式"
		} else if capability == "unsupported" {
			p.BlockedReason = "目标 agent 不支持图片，请选择文本模式重新预览"
		}
		var rendered snapcompact.RenderResult
		for {
			projection.Segments = segments
			rendered, err = s.options.HistoryProjection.RenderProjection(ctx, projection, in.MaxPages)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// 页数超限时重选完整片段并重新渲染，最终页集始终全部保留。
			overPages := len(rendered.Pages) > in.MaxPages || (errors.Is(err, snapcompact.ErrRenderLimit) && len(rendered.Pages) >= in.MaxPages)
			if overPages && (err == nil || errors.Is(err, snapcompact.ErrRenderLimit)) && len(segments) > 1 {
				remove := max(1, len(segments)/2)
				p.OmittedCount += remove
				segments = segments[remove:]
				continue
			}
			break
		}
		switch {
		case errors.Is(err, snapcompact.ErrRenderLimit) && len(rendered.Pages) >= in.MaxPages:
			p.BlockedReason = "完整历史片段超过图片页数预算，请选择文本模式重新预览"
		case len(rendered.Missing) > 0 || errors.Is(err, snapcompact.ErrMissingGlyph):
			p.BlockedReason = "历史包含字体未覆盖的字符，请选择文本模式重新预览"
		case err != nil:
			p.BlockedReason = "历史图片渲染失败，请选择文本模式重新预览"
		case len(rendered.Pages) > in.MaxPages:
			p.BlockedReason = "完整历史片段超过图片页数预算，请选择文本模式重新预览"
		case !renderCovered(segments, rendered):
			p.BlockedReason = "历史图片覆盖不完整，请选择文本模式重新预览"
		}
		// 失败的部分渲染也完整展示；BlockedReason 阻止其发送。
		for _, page := range rendered.Pages {
			sum := sha256.Sum256(page.PNG)
			p.Pages = append(p.Pages, ImagePage{Index: page.Index, MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(page.PNG), SHA256: hex.EncodeToString(sum[:]), Width: page.Width, Height: page.Height, Bytes: len(page.PNG)})
			p.ImageBudget.Bytes += len(page.PNG)
		}
		p.ImageBudget.Pages = len(p.Pages)
	}
	p.SelectedCount = len(segments)
	var history strings.Builder
	history.WriteString(header)
	fmt.Fprintf(&history, "包含片段：%d；省略：%d。\n", p.SelectedCount, p.OmittedCount)
	guide := history.String()
	for _, segment := range segments {
		history.WriteString(segmentText(segment))
	}
	p.HistoryText = history.String()
	if in.Format == "text" {
		p.Text = p.HistoryText + "\n当前用户任务：\n" + in.UserText
		p.Budget.Used = jsonBytes(p.HistoryText)
	} else {
		p.Text = guide + "图片按页序承载历史机械投影；精确内容以冻结原文为准。\n\n当前用户任务：\n" + in.UserText
		p.Budget.Used = jsonBytes(strings.TrimSuffix(p.Text, in.UserText))
		if wire, err := hybrid.ValidatePromptImages(p.Text, previewImages(p)); err != nil || int64(wire) > s.options.MaxPromptWireBytes {
			p.BlockedReason = "历史图片超过附件或编码请求限制，请选择文本模式重新预览"
		}
	}
	if err := hybrid.ValidatePromptText(p.Text, len(p.Pages)); err != nil || int64(len(p.Text)) > s.options.MaxPromptBytes {
		p.BlockedReason = "交接文本超过提示长度限制，请缩短任务后重新预览"
		if in.Format == "text" {
			if err != nil {
				return nil, err
			}
			return nil, ErrBudget
		}
	}
	p.Budget.OverBudget = p.Budget.OverBudget || p.Budget.Used > p.Budget.History
	if p.Budget.OverBudget {
		if in.Format == "text" {
			return nil, ErrBudget
		}
		p.BlockedReason = "交接文本超过目标上下文预算，请调整后重新预览"
	}
	return p, nil
}

// renderCovered 只用 projection rune 坐标校验正文；标题的零长度范围不算正文。
func renderCovered(segments []snapcompact.Segment, rendered snapcompact.RenderResult) bool {
	if len(rendered.Uncovered) > 0 || len(rendered.Missing) > 0 || len(rendered.Pages) == 0 {
		return false
	}
	ends := make(map[string]int, len(segments))
	lengths := make(map[string]int, len(segments))
	seen := make(map[string]bool, len(segments))
	for _, segment := range segments {
		lengths[segment.SourceID] = utf8.RuneCountInString(segment.Text)
	}
	for i, page := range rendered.Pages {
		if page.Index != i || page.Width < 1 || page.Height < 1 || len(page.PNG) == 0 {
			return false
		}
		for _, r := range page.Ranges {
			length, ok := lengths[r.SourceID]
			if !ok || r.Field != "projection" || r.Start < 0 || r.End < r.Start || r.End > length {
				return false
			}
			seen[r.SourceID] = true
			if r.Start == r.End {
				continue
			}
			if r.Start > ends[r.SourceID] {
				return false
			}
			ends[r.SourceID] = max(ends[r.SourceID], r.End)
		}
	}
	for id, length := range lengths {
		if !seen[id] || ends[id] != length {
			return false
		}
	}
	return true
}

func previewImages(p *Preview) []hybrid.PromptImage {
	images := make([]hybrid.PromptImage, len(p.Pages))
	for i, page := range p.Pages {
		images[i] = hybrid.PromptImage{MimeType: page.MimeType, Data: page.Data}
	}
	return images
}
