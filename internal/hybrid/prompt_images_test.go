package hybrid

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agent/omp"
)

func TestFrozenImageBlocksPreservePreviewData(t *testing.T) {
	data := base64.StdEncoding.EncodeToString(pngBytes())
	blocks, err := imagePromptBlocks("任务\n\tkeep  spaces", []PromptImage{{MimeType: "image/png", Data: data}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 || blocks[0].Text != "任务\n\tkeep  spaces" {
		t.Fatalf("blocks %+v", blocks)
	}
	var image storedBlock
	if err := json.Unmarshal(blocks[1].Raw, &image); err != nil {
		t.Fatal(err)
	}
	if image.Type != "image" || image.MimeType != "image/png" || image.Data != data {
		t.Fatalf("image %+v", image)
	}
}

func TestFrozenImageBlocksKeepChronologicalTextEdges(t *testing.T) {
	data := base64.StdEncoding.EncodeToString(pngBytes())
	head, tail := "早期约束\n", "最近修正\n\n当前任务：\n  exact task"
	blocks, err := imagePromptBlocks(head+tail, []PromptImage{{MimeType: "image/png", Data: data}}, len(head))
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 3 || blocks[0].Text != head || blocks[1].Type != "image" || blocks[2].Text != tail {
		t.Fatalf("chronological blocks: %+v", blocks)
	}
	for _, offset := range []int{-1, 1, len(head+tail) + 1} {
		if _, err := imagePromptBlocks(head+tail, nil, offset); !errors.Is(err, ErrInvalid) {
			t.Fatalf("offset %d accepted: %v", offset, err)
		}
	}
}

func TestFrozenImageValidationSharesAttachmentLimits(t *testing.T) {
	data := base64.StdEncoding.EncodeToString(pngBytes())
	tests := []struct {
		name   string
		images []PromptImage
		want   error
	}{
		{"count", make([]PromptImage, MaxAttachments+1), ErrTooLarge},
		{"mime", []PromptImage{{MimeType: "image/svg+xml", Data: data}}, ErrInvalid},
		{"empty", []PromptImage{{MimeType: "image/png"}}, ErrInvalid},
		{"encoding", []PromptImage{{MimeType: "image/png", Data: "invalid!"}}, ErrInvalid},
		{"line-break", []PromptImage{{MimeType: "image/png", Data: data + "\n"}}, ErrInvalid},
		{"size", []PromptImage{{MimeType: "image/png", Data: strings.Repeat("A", base64.StdEncoding.EncodedLen(int(MaxAttachmentBytes))+4)}}, ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidatePromptImages("task", tt.images); !errors.Is(err, tt.want) {
				t.Fatalf("err=%v want=%v", err, tt.want)
			}
		})
	}
	image := PromptImage{MimeType: "image/png", Data: strings.Repeat("A", base64.StdEncoding.EncodedLen(int(MaxAttachmentBytes-2)))}
	if _, err := ValidatePromptImages("task", []PromptImage{image, image, image}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("total limit: %v", err)
	}
}

func TestPromptSendsFrozenImageBlocks(t *testing.T) {
	h := newHarness(t, 0)
	s := h.create(h.input(omp.Name, SideACP, "FAKEAGENT_PROMPT_IMAGE=1"))
	id := s.Info().ID
	st := h.follow(id)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	caps, err := h.svc.ImageCapabilities(ctx, id)
	if err != nil || !caps.Image {
		t.Fatalf("capabilities=%+v err=%v", caps, err)
	}
	data := base64.StdEncoding.EncodeToString(pngBytes())
	in := PromptInput{ClientRequestID: rid(), Text: "任务\n\tkeep  spaces", Mode: ModeRefuse, Images: []PromptImage{{MimeType: "image/png", Data: data}}, AwaitAcknowledgement: true}
	if _, err := h.svc.Prompt(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	st.until("turn end", hasKind(EventTurnEnd))
	if _, err := h.svc.Prompt(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	blocks := userBlocks(t, h.store)
	if len(blocks) != 2 || blocks[0].Type != "text" || blocks[0].Text != in.Text || blocks[1].Type != "image" || blocks[1].Data != data || blocks[1].MimeType != "image/png" {
		t.Fatalf("blocks %+v", blocks)
	}
}

func TestFrozenImagesNeverDegradeOrQueue(t *testing.T) {
	h := newHarness(t, 0)
	s := h.create(h.input(omp.Name, SideACP))
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	for _, mode := range []string{ModeRefuse, ModeAuto, ModeSteer, ModeNow} {
		_, err := h.svc.Prompt(ctx, s.Info().ID, PromptInput{ClientRequestID: rid(), Text: "task", Mode: mode, Images: []PromptImage{{MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(pngBytes())}}})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("mode=%s err=%v", mode, err)
		}
	}
	if blocks := userBlocks(t, h.store); len(blocks) != 0 {
		t.Fatalf("unsupported images generated a prompt: %+v", blocks)
	}
}
