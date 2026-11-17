package history

import (
	"encoding/json"
	"strings"

	"github.com/rainchan/webmux/internal/historysource"
)

// contentText 提取正文和 ACP 工具内容；未支持的非空块通过 gap 显式报告。
func contentText(raw json.RawMessage) (string, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false, nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, false, nil
	}
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) == nil {
		var b strings.Builder
		gap := false
		for _, block := range blocks {
			text, missing, err := contentText(block)
			if err != nil {
				return "", false, err
			}
			gap = gap || missing
			b.WriteString(text)
		}
		return b.String(), gap, nil
	}
	var block struct {
		Type     string          `json:"type"`
		Text     string          `json:"text"`
		Content  json.RawMessage `json:"content"`
		Resource json.RawMessage `json:"resource"`
	}
	if json.Unmarshal(raw, &block) != nil {
		if json.Valid(raw) {
			return "", true, nil
		}
		return "", false, historysource.ErrFormat
	}
	switch block.Type {
	case "text":
		return block.Text, false, nil
	case "content":
		return contentText(block.Content)
	case "resource":
		var resource struct {
			Text *string `json:"text"`
		}
		if json.Unmarshal(block.Resource, &resource) != nil {
			return "", false, historysource.ErrFormat
		}
		if resource.Text != nil {
			return *resource.Text, false, nil
		}
	case "diff", "resource_link":
		// 保留路径、修改前后正文或资源引用；不读取引用指向的文件。
		return string(raw), false, nil
	}
	return "", true, nil
}

// valueText 保留参数与原始结果的 JSON 值；字符串使用其正文。
func valueText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	if !json.Valid(raw) {
		return "", historysource.ErrFormat
	}
	return string(raw), nil
}
