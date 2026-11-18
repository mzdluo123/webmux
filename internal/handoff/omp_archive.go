package handoff

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/hybrid"
)

type ompArchiveRecord struct {
	Role     string `json:"role"`
	Kind     string `json:"kind"`
	Text     string `json:"text"`
	ToolName string `json:"toolName,omitempty"`
	CallID   string `json:"callId,omitempty"`
	Status   string `json:"status,omitempty"`
}

type ompArchiveRequest struct {
	Model             string             `json:"model"`
	MaxFrames         int                `json:"maxFrames"`
	MaxFrameDataBytes int64              `json:"maxFrameDataBytes"`
	HistoryTokens     int64              `json:"historyTokens"`
	Records           []ompArchiveRecord `json:"records"`
}

type ompArchiveBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

type ompArchiveResult struct {
	Protocol                                               int
	Shape, Summary, History                                string
	RetainedChars, OmittedChars                            int
	ImageTokens                                            int64
	Unrenderable, ImageBudgetExceeded, ImageTokensVerified bool
	Blocks                                                 []ompArchiveBlock
}

type ompArchiveOutput struct {
	bytes.Buffer
	limit int
}

func (b *ompArchiveOutput) Write(data []byte) (int, error) {
	if len(data) > b.limit-b.Len() {
		return 0, ErrLimit
	}
	return b.Buffer.Write(data)
}

func runOMPArchive(ctx context.Context, argv []string, request ompArchiveRequest) (ompArchiveResult, error) {
	var result ompArchiveResult
	data, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	if len(data) > 8<<20 {
		return result, ErrLimit
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// #nosec G204 -- the server administrator configures argv.
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if cmd.Err != nil {
		return result, fmt.Errorf("find OMP SnapCompact: %w", cmd.Err)
	}
	cmd.Path, err = filepath.Abs(cmd.Path)
	if err != nil {
		return result, err
	}
	cmd.Dir = filepath.Dir(cmd.Path)
	cmd.Env = []string{}
	cmd.Stdin = bytes.NewReader(data)
	output := ompArchiveOutput{limit: int(request.MaxFrameDataBytes) + len(data) + (64 << 10)}
	cmd.Stdout = &output
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return result, fmt.Errorf("run OMP SnapCompact: %w", err)
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		return ompArchiveResult{}, errors.New("decode OMP SnapCompact output")
	}
	if result.Protocol != 1 || !result.Unrenderable && (result.Shape == "" || result.Summary == "" || result.RetainedChars < 0 ||
		result.OmittedChars < 0 || result.ImageTokens < 0 ||
		result.RetainedChars > historysource.MaxBytes || result.OmittedChars > historysource.MaxBytes ||
		result.ImageTokens > 250000) {
		return ompArchiveResult{}, errors.New("invalid OMP SnapCompact result")
	}
	return result, nil
}

func (s *Service) prepareOMPArchive(ctx context.Context, in Request, doc historysource.Document, header string, p *Preview) error {
	model, err := s.archiveModel(in)
	if err != nil {
		return err
	}
	p.Text = header + "\n当前用户任务：\n" + in.UserText
	tokens := p.Budget.History - historyprojection.ArchiveTextTokens(header) - 32
	wireBytes := int64(p.ImageBudget.MaxRequestBytes) - jsonBytes(p.Text) - 1024
	if tokens <= 0 || wireBytes <= 0 {
		p.Budget.OverBudget = true
		return nil
	}
	request := ompArchiveRequest{
		Model: model, MaxFrames: in.MaxPages, MaxFrameDataBytes: wireBytes, HistoryTokens: tokens,
		Records: make([]ompArchiveRecord, 0, len(doc.Records)),
	}
	var inputBytes int
	for _, record := range doc.Records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !record.Exportable() {
			continue
		}
		inputBytes += len(record.Text) + len(record.ToolName) + len(record.CallID) + len(record.Role) + len(record.Status) + len(record.Kind)
		if inputBytes > 8<<20 {
			p.BlockedReason = "冻结历史超过 OMP SnapCompact 输入上限，请选择文本模式"
			return nil
		}
		request.Records = append(request.Records, ompArchiveRecord{
			Role: record.Role, Kind: record.Kind, Text: record.Text, ToolName: record.ToolName,
			CallID: record.CallID, Status: record.Status,
		})
	}
	result, err := runOMPArchive(ctx, s.options.OMPRunner, request)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch {
	case err != nil:
		p.BlockedReason = "OMP SnapCompact 运行失败，请检查服务端运行器或选择文本模式"
	case result.Unrenderable:
		p.BlockedReason = "OMP SnapCompact 字体覆盖率不足，请选择文本模式"
	case result.ImageBudgetExceeded:
		p.BlockedReason = "OMP SnapCompact 图片超过请求预算，请减少页数或选择文本模式"
	default:
		if err := applyOMPArchive(in, header, result, p); err != nil {
			p.BlockedReason = "OMP SnapCompact 返回的归档无效，请选择文本模式"
			return nil
		}
		s.validateArchiveTransport(ctx, in, p)
	}
	return ctx.Err()
}

func applyOMPArchive(in Request, header string, result ompArchiveResult, p *Preview) error {
	var prompt strings.Builder
	prompt.WriteString(header)
	prompt.WriteString(result.Summary)
	prompt.WriteByte('\n')
	var pages []ImagePage
	var imageBytes, offset int
	for _, block := range result.Blocks {
		switch block.Type {
		case "text":
			prompt.WriteString(block.Text)
		case "image":
			if block.MimeType != "image/png" || len(pages) >= in.MaxPages ||
				(len(pages) > 0 && offset != prompt.Len()) ||
				len(block.Data) > base64.StdEncoding.EncodedLen(int(hybrid.MaxAttachmentBytes)) {
				return ErrInvalid
			}
			if len(pages) == 0 {
				offset = prompt.Len()
			}
			data, err := base64.StdEncoding.Strict().DecodeString(block.Data)
			if err != nil {
				return ErrInvalid
			}
			size, err := png.DecodeConfig(bytes.NewReader(data))
			if err != nil || size.Width <= 0 || size.Height <= 0 || size.Width > 4096 || size.Height > 4096 {
				return ErrInvalid
			}
			sum := sha256.Sum256(data)
			imageBytes += len(data)
			pages = append(pages, ImagePage{Index: len(pages), MimeType: block.MimeType, Data: block.Data,
				SHA256: hex.EncodeToString(sum[:]), Width: size.Width, Height: size.Height, Bytes: len(data)})
		default:
			return ErrInvalid
		}
	}
	if (len(pages) == 0) != (result.ImageTokens == 0) ||
		(result.RetainedChars == 0 && len(pages) > 0) {
		return ErrInvalid
	}
	if len(pages) > 0 {
		p.Pages = pages
		p.imageTextOffset = offset
	}
	if result.RetainedChars > 0 {
		p.SelectedCount = 1
	}
	if result.OmittedChars > 0 {
		p.OmittedCount = 1
	}
	p.ImageBudget.ProjectedRunes = result.RetainedChars + result.OmittedChars
	p.ImageBudget.RetainedRunes = result.RetainedChars
	p.ImageBudget.Bytes, p.ImageBudget.Pages = imageBytes, len(pages)
	p.ImageBudget.EstimatedTokens = result.ImageTokens
	p.ImageBudget.TokenEstimateKnown = result.ImageTokensVerified
	p.ImageBudget.TokenBasis = "OMP SnapCompact 18.22.2 model catalog estimate"
	p.ImageBudget.Shape = result.Shape
	p.HistoryText = header + result.History
	prompt.WriteString("\n当前用户任务：\n")
	p.Budget.Used = historyprojection.ArchiveTextTokens(prompt.String()) + result.ImageTokens
	prompt.WriteString(in.UserText)
	p.Text = prompt.String()
	return nil
}
