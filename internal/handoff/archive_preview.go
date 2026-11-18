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
	p.Text = header + "\n当前用户任务：\n" + in.UserText
	tokens := p.Budget.History - historyprojection.ArchiveTextTokens(header) - 32
	wire := int64(p.ImageBudget.MaxRequestBytes) - jsonBytes(p.Text) - int64(1024+in.MaxPages*80)
	if tokens <= 0 || wire < 2 {
		p.Budget.OverBudget = true
		return nil
	}
	archive, err := historyprojection.CompactOMPDocument(ctx, doc, snapcompact.OMPOptions{
		Target: snapcompact.OMPTargetForModel(model), MaxFrames: in.MaxPages, HistoryTokens: tokens,
		MaxImageBytes: min(hybrid.MaxPromptAttachmentBytes, wire*3/4), RejectUnsupported: true,
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch {
	case errors.Is(err, snapcompact.ErrMissingGlyph):
		p.BlockedReason = "历史字体覆盖率不足，请选择文本模式重新预览"
		return nil
	case err != nil:
		p.BlockedReason = "历史压缩超过渲染或资源限制，请调整预算或选择文本模式重新预览"
		return nil
	}
	applyArchive(in, archive, header, p)
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
	return ctx.Err()
}

func applyArchive(in Request, archive snapcompact.OMPArchive, header string, p *Preview) {
	var prompt strings.Builder
	prompt.WriteString(header)
	prompt.WriteString(archive.Summary)
	prompt.WriteByte('\n')
	for _, block := range archive.Blocks() {
		frame := block.Frame
		if frame == nil {
			prompt.WriteString(block.Text)
			continue
		}
		if len(p.Pages) == 0 {
			p.imageTextOffset = prompt.Len()
		}
		sum := sha256.Sum256(frame.PNG)
		p.Pages = append(p.Pages, ImagePage{
			Index: len(p.Pages), MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(frame.PNG),
			SHA256: hex.EncodeToString(sum[:]), Width: frame.Width, Height: frame.Height, Bytes: len(frame.PNG),
		})
		p.ImageBudget.Bytes += len(frame.PNG)
	}
	if archive.Characters > 0 {
		p.SelectedCount = 1
	}
	if archive.Truncated > 0 {
		p.OmittedCount = 1
	}
	p.ImageBudget.ProjectedRunes = archive.Characters + archive.Truncated
	p.ImageBudget.RetainedRunes = archive.Characters
	p.ImageBudget.Pages = len(p.Pages)
	p.ImageBudget.EstimatedTokens = archive.ImageTokens
	p.ImageBudget.TokenEstimateKnown = archive.ImageTokensVerified
	p.ImageBudget.TokenBasis = archive.TokenBasis
	p.ImageBudget.Shape = fmt.Sprintf("%s/%dpx (%s)", archive.Shape.Name, archive.Shape.Edge, archive.Shape.Font)
	p.HistoryText = header + archive.HistoryText()
	prompt.WriteString("\n当前用户任务：\n")
	p.Budget.Used = historyprojection.ArchiveTextTokens(prompt.String()) + archive.ImageTokens
	prompt.WriteString(in.UserText)
	p.Text = prompt.String()
}
