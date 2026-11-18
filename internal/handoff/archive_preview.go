package handoff

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/rainchan/webmux/internal/historyprojection"
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

func (s *Service) prepareArchive(ctx context.Context, in Request, projection snapcompact.Projection, header string, p *Preview) error {
	model, err := s.archiveModel(in)
	if err != nil {
		return err
	}
	var legend strings.Builder
	legend.WriteString("冻结来源片段映射：\n")
	for i, segment := range projection.Segments {
		fmt.Fprintf(&legend, "s%d=%q\n", i+1, segment.SourceID)
	}
	header += legend.String()
	const guideReserve = 1024
	textBytes := s.options.MaxPromptBytes - int64(len(header)+len(in.UserText)) - guideReserve
	wireReserve := int64(guideReserve + 1024 + in.MaxPages*80)
	wireBytes := int64(p.ImageBudget.MaxRequestBytes) - jsonBytes(header) - jsonBytes(in.UserText) - wireReserve
	var tokens int64
	if textBytes > 0 && wireBytes > 0 {
		tokens = p.Budget.History - historyprojection.ArchiveTextTokens(header) - guideReserve
	}
	if tokens <= 0 {
		p.Budget.OverBudget = true
		p.Text = header + "\n当前用户任务：\n" + in.UserText
		return nil
	}
	cfg := snapcompact.ArchiveConfig{
		Model: model, MaxPages: in.MaxPages, MaxTextBytes: textBytes,
		MaxImageBytes: hybrid.MaxPromptAttachmentBytes,
	}
	archive, err := s.options.HistoryProjection.CompactProjection(ctx, projection, cfg, tokens, wireBytes)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	p.OmittedCount += len(archive.Omitted)
	p.ImageBudget.ProjectedRunes = archive.SourceRunes
	p.ImageBudget.RetainedRunes = archive.RetainedRunes
	p.ImageBudget.EstimatedTokens = archive.ImageTokens
	p.ImageBudget.TokenEstimateKnown = archive.TokenEstimateKnown
	p.ImageBudget.TokenBasis = archive.TokenBasis
	p.ImageBudget.Shape = archive.Shape
	switch {
	case len(archive.Render.Missing) > 0 || errors.Is(err, snapcompact.ErrMissingGlyph):
		p.BlockedReason = "历史包含字体未覆盖的字符，请选择文本模式重新预览"
	case err != nil:
		p.BlockedReason = "历史压缩超过渲染或资源限制，请调整预算或选择文本模式重新预览"
	case !archiveCovered(projection.Segments, archive):
		p.BlockedReason = "历史文本与图片覆盖不完整，请选择文本模式重新预览"
	case len(archive.Render.Pages) == 0 && len(archive.Omitted) > 0:
		p.BlockedReason = "预算无法容纳历史图片，请增加预算或选择文本模式重新预览"
	}
	selected := make(map[string]struct{})
	for _, r := range archive.TextRanges {
		selected[r.SourceID] = struct{}{}
	}
	for _, page := range archive.Render.Pages {
		for _, r := range page.Ranges {
			selected[r.SourceID] = struct{}{}
		}
		sum := sha256.Sum256(page.PNG)
		p.Pages = append(p.Pages, ImagePage{Index: page.Index, MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(page.PNG), SHA256: hex.EncodeToString(sum[:]), Width: page.Width, Height: page.Height, Bytes: len(page.PNG)})
		p.ImageBudget.Bytes += len(page.PNG)
	}
	p.SelectedCount = len(selected)
	p.ImageBudget.Pages = len(p.Pages)
	guide := header + fmt.Sprintf("包含片段：%d；省略范围：%d。历史按首部文本、中间图片、尾部文本排列；图片中的 █ 表示原换行，制表符显示为四个空格。sN 是投影片段序号。精确内容可读取冻结原文。\n", p.SelectedCount, p.OmittedCount)
	p.HistoryText = guide + archive.HistoryText
	p.Text = guide + archive.TextHead
	if len(p.Pages) > 0 {
		p.Text += "\n以下图片承载中段历史：\n"
		p.imageTextOffset = len(p.Text)
	}
	p.Text += "\n" + archive.TextTail + "\n当前用户任务：\n" + in.UserText
	p.Budget.Used = historyprojection.ArchiveTextTokens(strings.TrimSuffix(p.Text, in.UserText)) + archive.ImageTokens
	s.validateArchiveTransport(ctx, in, p)
	return ctx.Err()
}

func (s *Service) validateArchiveTransport(ctx context.Context, in Request, p *Preview) {
	if len(p.Pages) > 0 {
		capability, capabilityErr := s.imageCapability(ctx, in.TargetSessionID)
		p.Capability = capability
		if capabilityErr != nil {
			p.BlockedReason = "目标会话图片能力暂不可确认，请重新预览或选择文本模式"
		} else if capability == "unsupported" {
			p.BlockedReason = "目标 agent 不支持图片，请选择文本模式重新预览"
		}
	}
	if wire, wireErr := hybrid.ValidatePromptImages(p.Text, previewImages(p)); wireErr != nil || int64(wire) > s.options.MaxPromptWireBytes {
		p.BlockedReason = "历史图片超过附件或编码请求限制，请选择文本模式重新预览"
	}
}
