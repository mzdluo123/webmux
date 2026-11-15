package history

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

// The input is deliberately named QoderCN: this is not an international capture.
func TestProvisionalChineseEditionFixture(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "native-history", "qodercn")
	sources, err := Discover(context.Background(), root)
	if err != nil || len(sources) != 2 {
		t.Fatal(sources, err)
	}
	for _, s := range sources {
		if s.Agent != "qoder" {
			t.Fatal(s)
		}
		doc, err := Read(context.Background(), historysource.Request{Source: s})
		if err != nil {
			t.Fatal(err)
		}
		if doc.Source.Agent != "qoder" || doc.ParserVersion != ParserVersion {
			t.Fatal(doc.Source, doc.ParserVersion)
		}
	}
}
