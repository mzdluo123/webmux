package handoff

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

const ompFixtureMode = "omp-fixture"

func TestOMPArchiveRunnerProcess(t *testing.T) {
	if os.Args[len(os.Args)-1] != ompFixtureMode {
		return
	}
	var request ompArchiveRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		os.Exit(1)
	}
	if len(request.Records) != 1 || request.Records[0].Text != "visible history" ||
		request.Model != "openai-codex/gpt-6-astra" || request.MaxFrames != 3 || request.HistoryTokens <= 0 {
		os.Exit(2)
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewGray(image.Rect(0, 0, 64, 64))); err != nil {
		os.Exit(3)
	}
	result := ompArchiveResult{Protocol: 1, Shape: "8x22/1568px (8x13)", Summary: "summary", History: "retained", RetainedChars: 16, OmittedChars: 10, ImageTokens: 5, ImageTokensVerified: true,
		Blocks: []ompArchiveBlock{
			{Type: "text", Text: "old"},
			{Type: "image", Data: base64.StdEncoding.EncodeToString(imageData.Bytes()), MimeType: "image/png"},
			{Type: "text", Text: "recent"},
		}}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

func TestOMPArchivePreviewFreezesOrderedBlocks(t *testing.T) {
	s, b := newTestService(t)
	b.history = []historysource.Record{
		{ID: "public", Role: "user", Kind: "prose", Text: "visible history"},
		{ID: "private", Role: "assistant", Kind: "reasoning", Text: "private history"},
	}
	target := b.records["target"]
	target.Config = map[string]string{"model": "openai-codex/gpt-6-astra"}
	b.records["target"] = target
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s.options.OMPRunner = []string{self, "-test.run=^TestOMPArchiveRunnerProcess$", "--", ompFixtureMode}
	const task = "  current task\n"
	p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target", MaxPages: 3, UserText: task})
	if err != nil {
		t.Fatal(err)
	}
	if p.BlockedReason != "" || p.OmittedCount != 1 || len(p.Pages) != 1 || p.ImageBudget.RetainedRunes != 16 || p.ImageBudget.ProjectedRunes != 26 {
		t.Fatalf("unexpected preview: %s, budget %+v, %d pages", p.BlockedReason, p.ImageBudget, len(p.Pages))
	}
	if !strings.HasSuffix(p.Text[:p.imageTextOffset], "summary\nold") ||
		!strings.HasPrefix(p.Text[p.imageTextOffset:], "recent\n") ||
		!strings.HasSuffix(p.Text, task) || !strings.HasSuffix(p.HistoryText, "retained") {
		t.Fatalf("unexpected OMP block order: %q", p.Text)
	}
	if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
		t.Fatal(err)
	}
	if op := awaitOperation(t, s, p.OperationID); op.State != "succeeded" {
		t.Fatalf("operation %+v", op)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.images) != 1 || b.images[0].Data != p.Pages[0].Data || b.text != p.Text {
		t.Fatal("delivery did not use the frozen OMP blocks")
	}
}

func TestUnavailableOMPArchiveRunnerBlocksImagePreview(t *testing.T) {
	s, _ := newTestService(t)
	s.options.OMPRunner = []string{"missing-webmux-omp-snapcompact"}
	p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target"})
	if err != nil || !strings.Contains(p.BlockedReason, "OMP SnapCompact") || len(p.Pages) != 0 {
		t.Fatalf("unexpected image preview reason %q, err %v", p.BlockedReason, err)
	}
	previewTarget(t, s)
}
