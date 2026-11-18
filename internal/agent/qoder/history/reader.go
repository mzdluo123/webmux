// Package history provisionally reuses the QoderCN JSONL reader for Qoder.
// The native fixtures come from the Chinese edition.
package history

import (
	"context"
	cn "github.com/rainchan/webmux/internal/agent/qodercn/history"
	"github.com/rainchan/webmux/internal/historysource"
)

const ParserVersion = "qoder-provisional-jsonl/2"

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
