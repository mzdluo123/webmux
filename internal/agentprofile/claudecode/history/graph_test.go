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

// Synthetic branching/attachment cases complement, but do not replace, #653.
func TestSyntheticSidechainCompactionUnicodeAttachments(t *testing.T) {
	data := `{"type":"user","uuid":"root","sessionId":"native","message":{"role":"user","content":"你好🌳"}}
{"type":"assistant","uuid":"side","parentUuid":"root","sessionId":"native","isSidechain":true,"agentId":"worker","message":{"role":"assistant","content":[{"type":"text","text":"wrong sidechain"}]}}
{"type":"assistant","uuid":"main","parentUuid":"root","sessionId":"native","message":{"role":"assistant","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"SECRET"}},{"type":"thinking","thinking":"SECRET"},{"type":"text","text":"visible"}]}}
{"type":"system","subtype":"compact_boundary","uuid":"compact","sessionId":"native","parentUuid":null,"logicalParentUuid":"main","content":"summary","compactMetadata":{"preservedMessages":{"uuids":["root","main"],"anchorUuid":"external"}}}
`
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), hs.Request{Source: source(path, "native")})
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Records) != 3 || doc.Records[0].Text != "你好🌳" || len(doc.Records[1].Attachments) != 1 {
		t.Fatalf("preserved graph: %+v", doc)
	}
	for _, r := range doc.Records {
		if r.ID == "side" || strings.Contains(r.Text, "SECRET") {
			t.Fatal("private branch/body escaped")
		}
	}
	req := hs.Request{Source: source(path, "native")}
	req.Source.Branch = "side"
	branch, err := Read(context.Background(), req)
	if err != nil || len(branch.Records) != 1 || branch.Records[0].ID != "side" || len(branch.Gaps) != 1 || branch.Gaps[0].Category != "external_branch" {
		t.Fatalf("sidechain isolation: %+v %v", branch, err)
	}
	req.Source.Branch = "absent"
	if _, err := Read(context.Background(), req); !errors.Is(err, hs.ErrIdentity) {
		t.Fatalf("missing branch: %v", err)
	}
}
