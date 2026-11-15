package snapcompact

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// The reference fixture executes only the pinned upstream truncation helper;
// it is not evidence of running its complete normalization or PNG pipeline.
func TestPinnedUpstreamTruncationReference(t *testing.T) {
	var fixture struct {
		Commit string
		Cases  []struct {
			Text               string
			Limit, HeadPercent int
			Upstream           string
		}
	}
	data, err := os.ReadFile("testdata/upstream-reference.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Commit != "7c49c11aa5bbaf3f418406583096168ad5b8c6dc" {
		t.Fatal("unexpected reference commit")
	}
	for _, c := range fixture.Cases {
		cfg := DefaultPreprocessConfig()
		cfg.OutputRunes, cfg.HeadPercent = c.Limit, c.HeadPercent
		p, err := Project(context.Background(), Input{History: []Record{{ID: "reference", Role: "tool", ToolName: "shell", Kind: "structured", Text: c.Text}}}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		chars := []rune(c.Text)
		head := c.Limit * c.HeadPercent / 100
		tail := len(chars) - (c.Limit - head)
		if len(p.Omissions) != 1 || p.Omissions[0].Range != (Range{SourceID: "reference", Field: "text", Start: head, End: tail}) {
			t.Fatalf("incorrect reference coverage: %+v", p.Omissions)
		}
		if !utf8.ValidString(p.Segments[0].Text) || !strings.Contains(p.Segments[0].Text, string(chars[:head])) || !strings.Contains(p.Segments[0].Text, string(chars[tail:])) {
			t.Fatal("Go head/tail coverage differs")
		}
		// Upstream rounds the head count; Go floors it. Both preserve the same
		// total budget. BMP input avoids conflating this with UTF-16 differences.
		upstreamHead := (c.Limit*c.HeadPercent + 50) / 100
		upstreamTail := len(chars) - (c.Limit - upstreamHead)
		if !strings.HasPrefix(c.Upstream, string(chars[:upstreamHead])) || !strings.HasSuffix(c.Upstream, string(chars[upstreamTail:])) {
			t.Fatal("invalid pinned upstream reference")
		}
	}
}

func TestSyntheticConversationFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/conversation.json")
	if err != nil {
		t.Fatal(err)
	}
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	p, err := Project(context.Background(), input, DefaultPreprocessConfig())
	if err != nil {
		t.Fatal(err)
	}
	if p.CurrentTask != input.CurrentTask || len(p.Segments) != len(input.History) {
		t.Fatal("fixture boundaries changed")
	}
	for i, r := range input.History {
		s := p.Segments[i]
		if s.SourceID != r.ID || s.Role != r.Role || s.CallID != r.CallID || s.Status != r.Status {
			t.Fatalf("source mapping lost at %d", i)
		}
	}
}
