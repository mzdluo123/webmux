package history

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hs "github.com/rainchan/webmux/internal/historysource"
)

// Synthetic graph exercises branches absent from the real excerpt, not runtime evidence.
func TestSyntheticGraphCompactionUnicodeAttachments(t *testing.T) {
	data := `{"type":"session","id":"native","version":3}
{"type":"message","id":"old","parentId":null,"message":{"role":"user","content":"old discarded"}}
{"type":"message","id":"keep","parentId":"old","message":{"role":"user","content":[{"type":"text","text":"你好🌳"},{"type":"image","mimeType":"image/png","data":"SECRET"}]}}
{"type":"message","id":"other","parentId":"old","message":{"role":"assistant","content":[{"type":"text","text":"wrong branch"}]}}
{"type":"compaction","id":"compact","parentId":"keep","firstKeptEntryId":"keep","providerReplayThroughEntryId":"keep","summary":"summary","preserveData":{"secret":"SECRET"}}
`
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), hs.Request{Source: source(path, "native")})
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Records) != 2 || doc.Records[0].ID != "keep" || doc.Records[0].Text != "你好🌳" || len(doc.Records[0].Attachments) != 1 {
		t.Fatalf("compaction graph: %+v", doc)
	}
	for _, r := range doc.Records {
		if strings.Contains(r.Text, "SECRET") {
			t.Fatal("attachment body escaped")
		}
	}
	req := hs.Request{Source: source(path, "native")}
	req.Source.Branch = "other"
	branch, err := Read(context.Background(), req)
	if err != nil || len(branch.Records) != 2 || branch.Records[1].ID != "other" {
		t.Fatalf("explicit branch: %+v %v", branch, err)
	}
	req.Source.Branch = "absent"
	if _, err := Read(context.Background(), req); !errors.Is(err, hs.ErrIdentity) {
		t.Fatalf("missing branch: %v", err)
	}
}
