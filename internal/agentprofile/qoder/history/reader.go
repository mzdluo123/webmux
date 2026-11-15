// Package history provisionally reads the QoderCN 1.1.84 layout for Qoder.
// International-edition compatibility is an explicit assumption, not a tested
// runtime claim; the real fixtures were captured from the Chinese edition only.
package history

import (
	"context"
	cn "github.com/rainchan/webmux/internal/agentprofile/qodercn/history"
	"github.com/rainchan/webmux/internal/historysource"
)

const ParserVersion = "qoder-provisional-jsonl/1"

func Read(ctx context.Context, req historysource.Request) (historysource.Document, error) {
	doc, err := cn.ReadAs(ctx, req, "qoder")
	if err == nil {
		doc.ParserVersion = ParserVersion
	}
	return doc, err
}

// Discover expects the explicitly supplied projects data root; its location
// follows the provisional Chinese-edition layout rather than runtime evidence.
func Discover(ctx context.Context, root string) ([]historysource.Source, error) {
	return cn.DiscoverAs(ctx, root, "qoder")
}
