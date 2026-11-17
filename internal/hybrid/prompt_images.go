package hybrid

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/rainchan/webmux/internal/acp"
)

// PromptImage 保存服务端冻结的图片；Data 与预览使用同一份 base64 数据。
type PromptImage struct {
	MimeType string
	Data     string
}

// ValidatePromptImages 复用附件数量、原始字节与 ACP 编码请求限制。
// 返回含 JSON 结构预留的请求字节上界，不估算图片 token。
func ValidatePromptImages(text string, images []PromptImage) (int, error) {
	if len(images) > MaxAttachments {
		return 0, fmt.Errorf("%w: 每条消息最多 %d 张图片", ErrTooLarge, MaxAttachments)
	}
	encoded, _ := json.Marshal(text)
	wire := len(encoded) + 1024
	var total int64
	for _, image := range images {
		if !imageMimes[image.MimeType] || len(image.Data) == 0 || strings.ContainsAny(image.Data, "\r\n") {
			return 0, fmt.Errorf("%w: 图片类型或内容无效", ErrInvalid)
		}
		if len(image.Data) > base64.StdEncoding.EncodedLen(int(MaxAttachmentBytes)) {
			return 0, fmt.Errorf("%w: 单张图片超过附件限制", ErrTooLarge)
		}
		n, err := io.Copy(io.Discard, base64.NewDecoder(base64.StdEncoding.Strict(), strings.NewReader(image.Data)))
		if err != nil || n == 0 {
			return 0, fmt.Errorf("%w: 图片编码无效", ErrInvalid)
		}
		if n > MaxAttachmentBytes {
			return 0, fmt.Errorf("%w: 单张图片超过附件限制", ErrTooLarge)
		}
		total += n
		wire += len(image.Data) + len(image.MimeType) + 64
		if total > MaxPromptAttachmentBytes || wire > MaxPromptWireBytes {
			return 0, fmt.Errorf("%w: 图片或编码请求超过附件限制", ErrTooLarge)
		}
	}
	if wire > MaxPromptWireBytes {
		return 0, fmt.Errorf("%w: 编码请求超过附件限制", ErrTooLarge)
	}
	return wire, nil
}

func imagePromptBlocks(text string, images []PromptImage) ([]acp.ContentBlock, error) {
	if _, err := ValidatePromptImages(text, images); err != nil {
		return nil, err
	}
	blocks := make([]acp.ContentBlock, 0, 1+len(images))
	blocks = append(blocks, acp.TextBlock(text))
	for _, image := range images {
		blocks = append(blocks, acp.Base64ImageBlock(image.Data, image.MimeType, ""))
	}
	return blocks, nil
}
