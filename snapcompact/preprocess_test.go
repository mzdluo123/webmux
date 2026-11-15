package snapcompact

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestProjectTruncation(t *testing.T) {
	cfg := DefaultPreprocessConfig()
	cfg.OutputRunes = 4
	cfg.ArgumentRunes = 8
	cfg.CallRunes = 4
	input := Input{History: []Record{{ID: "r1", Role: "tool", ToolName: "shell", CallID: "call1", Status: "failed", Text: "甲乙丙丁戊己庚辛壬癸", CallBody: "abcdefghij", Arguments: map[string]any{"z": "longvalue", "a": 1}}}, CurrentTask: "\x1b[31m  task\r\n\t   "}
	p, err := Project(context.Background(), input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p.CurrentTask != input.CurrentTask || input.History[0].Text != "甲乙丙丁戊己庚辛壬癸" {
		t.Fatal("mutated raw input")
	}
	if len(p.Omissions) != 3 {
		t.Fatalf("omissions: %+v", p.Omissions)
	}
	var found bool
	for _, o := range p.Omissions {
		if o.Range.Field == "text" {
			found = true
			if o.Range.Start != 2 || o.Range.End != 8 || o.OriginalRunes != 10 {
				t.Fatalf("bad omission %+v", o)
			}
		}
	}
	if !found {
		t.Fatal("missing text omission")
	}
	if !strings.Contains(p.Segments[0].Text, "甲乙") || !strings.Contains(p.Segments[0].Text, "壬癸") || !utf8.ValidString(p.Segments[0].Text) {
		t.Fatal(p.Segments[0].Text)
	}
	if p.Segments[0].CallID != "call1" || p.Segments[0].Status != "failed" {
		t.Fatal("lost metadata")
	}
}

func TestNormalizeStructure(t *testing.T) {
	tests := []struct {
		name, give string
		structured bool
		want       string
	}{
		{"prose", "\x1b[31mHello\x1b[0m\t world\r\n\rnext\n\n\nend", false, "Hello world\n\nnext\n\nend"},
		{"code", "\tif x {  \r\n\t\tcall()\r}\n\n\n", true, "    if x {\n        call()\n}"},
		{"fence", "a   b\n```go\n\tvar x  = 1  \n```\n c  d", false, "a b\n```go\n    var x  = 1\n```\nc d"},
		{"osc", "before\x1b]0;secret\aafter\x1b]8;;https://example.test\x1b\\link\x1b]8;;\x1b\\", false, "beforeafterlink"},
		{"empty", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalize(context.Background(), tt.give, tt.structured)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestProjectDeterministicAndCache(t *testing.T) {
	cfg := DefaultPreprocessConfig()
	in := Input{History: []Record{{ID: "one", Role: "assistant", Arguments: map[string]any{"z": map[string]any{"b": 2, "a": 1}, "a": "first"}, Text: "text"}}}
	a, err := Project(context.Background(), in, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		b, err := Project(context.Background(), in, cfg)
		if err != nil {
			t.Fatal(err)
		}
		x, _ := json.Marshal(a)
		y, _ := json.Marshal(b)
		if string(x) != string(y) {
			t.Fatal("nondeterministic projection")
		}
	}
	if !strings.Contains(a.Segments[0].Text, `{"a":"first","z":{"a":1,"b":2}}`) {
		t.Fatal(a.Segments[0].Text)
	}
	cfg.HeadPercent--
	b, err := Project(context.Background(), in, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if a.CacheKey == b.CacheKey {
		t.Fatal("config absent from cache")
	}
	in.History[0].Text = "different"
	c, err := Project(context.Background(), in, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if b.CacheKey == c.CacheKey {
		t.Fatal("source absent from cache")
	}
}

func TestProjectInvalidAndBounded(t *testing.T) {
	cfg := DefaultPreprocessConfig()
	tests := []struct {
		name string
		give Input
	}{
		{"empty id", Input{History: []Record{{Text: "x"}}}},
		{"duplicate id", Input{History: []Record{{ID: "x"}, {ID: "x"}}}},
		{"invalid text", Input{History: []Record{{ID: "x", Text: string([]byte{255})}}}},
		{"invalid arg", Input{History: []Record{{ID: "x", Arguments: map[string]any{"x": string([]byte{255})}}}}},
		{"invalid key", Input{History: []Record{{ID: "x", Arguments: map[string]any{string([]byte{255}): 1}}}}},
		{"nan", Input{History: []Record{{ID: "x", Arguments: map[string]any{"x": math.NaN()}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Project(context.Background(), tt.give, cfg); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	cfg.MaxInputBytes = 5
	if _, err := Project(context.Background(), Input{CurrentTask: "too long"}, cfg); err == nil {
		t.Fatal("expected size error")
	}
	cfg = DefaultPreprocessConfig()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Project(ctx, Input{}, cfg); err != context.Canceled {
		t.Fatalf("cancel: %v", err)
	}
	cfg.HeadPercent = 101
	if _, err := Project(context.Background(), Input{}, cfg); err == nil {
		t.Fatal("invalid config")
	}
	if _, err := Project(context.Background(), Input{}, DefaultPreprocessConfig()); err != nil {
		t.Fatal(err)
	}
	cycle := map[string]any{}
	cycle["x"] = cycle
	if _, err := Project(context.Background(), Input{History: []Record{{ID: "x", Arguments: cycle}}}, DefaultPreprocessConfig()); err == nil {
		t.Fatal("cycle accepted")
	}
}

func TestProjectClusterBoundaries(t *testing.T) {
	cfg := DefaultPreprocessConfig()
	cfg.OutputRunes = 2
	tests := []string{"e\u0301abcdefe\u0301", "👩\u200d💻abcdef👩\u200d💻", "🇨🇳abcdef🇨🇳"}
	for _, text := range tests {
		p, err := Project(context.Background(), Input{History: []Record{{ID: "x", Role: "tool", Text: text}}}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		r := []rune(text)
		for _, o := range p.Omissions {
			if !clusterBoundary(r, o.Range.Start) || !clusterBoundary(r, o.Range.End) {
				t.Fatalf("split cluster %+v", o)
			}
		}
	}
	cfg.OutputRunes = 0
	p, err := Project(context.Background(), Input{History: []Record{{ID: "x", Role: "tool", Text: "abc"}}}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Omissions) != 1 || p.Omissions[0].Range.Start != 0 || p.Omissions[0].Range.End != 3 {
		t.Fatal(p)
	}
}

func TestProjectEscapeCuts(t *testing.T) {
	tests := []struct {
		name, text string
		limit      int
	}{
		{"CSI", "a\x1b[38;5;200mb", 8},
		{"OSC title", "a\x1b]0;hidden-title\ab", 10},
		{"OSC8 URL", "a\x1b]8;;https://private.example/hidden\x1b\\b", 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultPreprocessConfig()
			cfg.OutputRunes = tt.limit
			p, err := Project(context.Background(), Input{History: []Record{{ID: "r", Role: "tool", Text: tt.text}}}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Omissions) != 1 {
				t.Fatal(p)
			}
			o := p.Omissions[0]
			if o.Range.Start != 1 || o.Range.End != utf8.RuneCountInString(tt.text)-1 {
				t.Fatalf("escape split: %+v", o)
			}
			body := p.Segments[0].Text
			if strings.Contains(body, "private") || strings.Contains(body, "200m") || strings.Contains(body, "hidden") {
				t.Fatal(body)
			}
		})
	}
}

func TestProjectHardLimits(t *testing.T) {
	cfg := DefaultPreprocessConfig()
	cfg.MaxInputBytes = 65 << 20
	if _, err := Project(context.Background(), Input{}, cfg); err == nil {
		t.Fatal("oversized config accepted")
	}
	if _, err := Project(context.Background(), Input{History: make([]Record, 100001)}, DefaultPreprocessConfig()); err == nil {
		t.Fatal("record count accepted")
	}
}

func TestProjectRoundedHeadRatio(t *testing.T) {
	cfg := DefaultPreprocessConfig()
	cfg.OutputRunes = 3
	p, err := Project(context.Background(), Input{History: []Record{{ID: "r", Role: "tool", Text: "abcdefghi"}}}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Omissions) != 1 || p.Omissions[0].Range.Start != 2 || p.Omissions[0].Range.End != 8 {
		t.Fatalf("expected upstream Math.round(3*.6)=2 head split: %+v", p.Omissions)
	}
	cfg.HeadPercent = 50
	cfg.OutputRunes = 1
	p, err = Project(context.Background(), Input{History: []Record{{ID: "r", Role: "tool", Text: "abc"}}}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p.Omissions[0].Range.Start != 1 || p.Omissions[0].Range.End != 3 {
		t.Fatal(p.Omissions)
	}
}
