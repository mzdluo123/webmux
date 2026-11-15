package snapcompact

// Adapted test cases from oh-my-pi packages/snapcompact/test/snapcompact.test.ts
// at 7c49c11aa5bbaf3f418406583096168ad5b8c6dc.
// Copyright (c) 2025 Mario Zechner
// Copyright (c) 2025-2026 Can Bölük
// Copyright (c) 2026 Stencil Labs, Inc.
// MIT license and unmodified test source: testdata/upstream/.
// These test common mechanical invariants through the neutral Go DTO. They do
// not assert identical serialization, image geometry, fonts, or provider rates.

import (
	"bytes"
	"context"
	"image/png"
	"strings"
	"testing"
)

func TestUpstreamCompatibleToolResultBudgets(t *testing.T) {
	// Official serializeConversation: oversized result; configured cap; cap off.
	cases := []struct {
		name, text                 string
		limit, percent, start, end int
	}{
		{"default_oversized", "HEAD-" + strings.Repeat("x", 5000) + "-TAIL", 2000, 60, 1200, 4210},
		{"configured", strings.Repeat("a", 100), 10, 50, 5, 95},
		{"odd_rounding", "abcdef", 3, 60, 2, 5},
		{"half_rounding", "abcdef", 1, 50, 1, 6},
		{"disabled_by_sufficient_budget", strings.Repeat("a", 100), 100, 60, 100, 100},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := DefaultPreprocessConfig()
			c.OutputRunes = tt.limit
			c.HeadPercent = tt.percent
			p, err := Project(context.Background(), Input{History: []Record{{ID: "result", Role: "tool", ToolName: "tool", Text: tt.text}}}, c)
			if err != nil {
				t.Fatal(err)
			}
			if tt.start == tt.end {
				if len(p.Omissions) != 0 {
					t.Fatal(p.Omissions)
				}
				return
			}
			if len(p.Omissions) != 1 || p.Omissions[0].Range != (Range{"result", "text", tt.start, tt.end}) {
				t.Fatal(p.Omissions)
			}
			if !strings.Contains(p.Segments[0].Text, tt.text[:tt.start]) || !strings.HasSuffix(p.Segments[0].Text, tt.text[tt.end:]+"\n") {
				t.Fatal("lost retained edge")
			}
		})
	}
}

func TestUpstreamCompatibleNormalizationCases(t *testing.T) {
	// Official normalize cases whose expected text also matches conservative mode.
	cases := []struct{ name, input, want string }{
		{"horizontal", "a \t b   c", "a b c"},
		{"latin1", "café größe", "café größe"},
		{"ansi", "\x1b[31mred\x1b[0m plain", "red plain"},
		{"kana", "こんにちは", "こんにちは"},
		{"katakana", "カタカナ", "カタカナ"},
		{"hangul", "안녕하세요", "안녕하세요"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Project(context.Background(), Input{History: []Record{{ID: "normalize", Role: "user", Kind: "prose", Text: tt.input}}}, DefaultPreprocessConfig())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(p.Segments[0].Text, "text:\n"+tt.want+"\n") {
				t.Fatalf("unexpected normalization: %q", p.Segments[0].Text)
			}
		})
	}
}

func TestUpstreamCompatibleOrphanResultAndOrder(t *testing.T) {
	// Official orphan result and before/call/after order cases. Unlike upstream,
	// source records remain separate, including the paired result at its position.
	history := []Record{
		{ID: "before", Role: "assistant", Text: "before"},
		{ID: "call", Role: "assistant", ToolName: "read", CallID: "c1", Arguments: map[string]any{"path": "a.ts"}},
		{ID: "after", Role: "assistant", Text: "after"},
		{ID: "result", Role: "tool", CallID: "c1", Text: "file body"},
		{ID: "orphan", Role: "tool", CallID: "outside-window", Text: "ok"},
	}
	p, err := Project(context.Background(), Input{History: history}, DefaultPreprocessConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments) != len(history) {
		t.Fatal("source boundary changed")
	}
	for i, r := range history {
		if p.Segments[i].SourceID != r.ID || p.Segments[i].CallID != r.CallID {
			t.Fatal("order or association changed")
		}
	}
	if !strings.HasSuffix(p.Segments[4].Text, "ok\n") {
		t.Fatal("orphan result lost")
	}
}

func TestUpstreamCompatibleShortFinalPage(t *testing.T) {
	// Official render short-final-page invariant, adapted to the Go geometry.
	r := testRenderer(t)
	c := DefaultRenderConfig()
	c.Width = 320
	c.Height = 320
	got, err := r.Render(context.Background(), renderProjection("hello world"), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pages) != 1 {
		t.Fatal("short input did not produce one page")
	}
	page := got.Pages[0]
	decoded, err := png.Decode(bytes.NewReader(page.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if page.Height < c.MinHeight || page.Height > c.Height || decoded.Bounds().Dx() != c.Width || decoded.Bounds().Dy() != page.Height {
		t.Fatal("incorrect final page geometry")
	}
}

func TestUpstreamArgumentCasesWithDocumentedWholeObjectDifference(t *testing.T) {
	// Official names: "caps oversized tool-call argument values without touching
	// small ones" and "caps the whole serialized argument list per call".
	// Same arguments; Go deliberately caps the entire canonical JSON object at
	// ArgumentRunes, while upstream caps EACH encoded value at 500 and the
	// rendered name=value list at 2000. These representations are not equivalent.
	cases := []struct {
		name string
		args map[string]any
	}{
		{"large_value", map[string]any{"path": "a.ts", "content": strings.Repeat("y", 3000)}},
		{"whole_call", func() map[string]any {
			m := map[string]any{}
			for i := 0; i < 10; i++ {
				m["arg"+string(rune('0'+i))] = strings.Repeat("z", 400)
			}
			return m
		}()},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := DefaultPreprocessConfig()
			p, err := Project(context.Background(), Input{History: []Record{{ID: "call", Role: "assistant", ToolName: "write", CallID: "c1", Arguments: tt.args}}}, c)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Omissions) != 1 || p.Omissions[0].Range.Field != "arguments" {
				t.Fatal(p.Omissions)
			}
			o := p.Omissions[0]
			if o.OriginalRunes-(o.Range.End-o.Range.Start) != c.ArgumentRunes {
				t.Fatal("canonical object cap differs")
			}
			if tt.name == "large_value" && !strings.Contains(p.Segments[0].Text, "a.ts") {
				t.Fatal("small path in retained tail lost")
			}
		})
	}
}

func TestUpstreamNormalizationDeliberateDifferences(t *testing.T) {
	// Exact official inputs. Expected Go behavior retains Unicode/line structure;
	// upstream outputs are included to make the incompatibility reviewable.
	cases := []struct{ name, input, want, upstream string }{
		{"symbols", "x → y ✓ “quoted” — em…", "x → y ✓ “quoted” — em…", `x -> y v "quoted" - em...`},
		{"newlines", "a\n\n\tb   c\r\nd", "a\n\nb c\nd", "a█b c█d"},
		{"semantic_emoji", "✅ pass ⚠️ warn ❌ fail 😄", "✅ pass ⚠️ warn ❌ fail 😄", "[OK] pass [WARN] warn [FAIL] fail"},
		{"fullwidth", "ＨＥＬＬＯ", "ＨＥＬＬＯ", "HELLO"},
		{"ligature", "ﬁle", "ﬁle", "file"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Project(context.Background(), Input{History: []Record{{ID: "difference", Role: "user", Kind: "prose", Text: tt.input}}}, DefaultPreprocessConfig())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(p.Segments[0].Text, "text:\n"+tt.want+"\n") {
				t.Fatalf("Go behavior: %q; upstream: %q", p.Segments[0].Text, tt.upstream)
			}
		})
	}
}
