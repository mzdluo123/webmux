package snapcompact

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"
)

func testRenderer(t *testing.T) Renderer {
	t.Helper()
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func renderProjection(text string) Projection {
	return Projection{Version: Version, CacheKey: "fixture", Segments: []Segment{{SourceID: "source-1", Role: "user", Text: text}}}
}

func TestRenderPNGUnicodeAndDeterminism(t *testing.T) {
	r := testRenderer(t)
	p := renderProjection("Hello, 中文！ e\u0301\n  code := 42")
	c := DefaultRenderConfig()
	c.LineHeight = 28
	first, err := r.Render(context.Background(), p, c)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Render(context.Background(), p, c)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("render is not deterministic")
	}
	if len(first.Pages) != 1 || len(first.Uncovered) != 0 {
		t.Fatalf("unexpected pages: %+v", first)
	}
	page := first.Pages[0]
	img, err := png.Decode(bytes.NewReader(page.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != page.Width || img.Bounds().Dy() != page.Height || page.Height < c.MinHeight {
		t.Fatal("wrong dimensions")
	}
	ink := 0
	for y := 0; y < page.Height; y++ {
		for x := 0; x < page.Width; x++ {
			if color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y < 200 {
				ink++
			}
		}
	}
	if ink < 100 {
		t.Fatalf("glyph raster is empty: %d", ink)
	}
	c.FontSize++
	c.LineHeight += 2
	changed, err := r.Render(context.Background(), p, c)
	if err != nil {
		t.Fatal(err)
	}
	if changed.CacheKey == first.CacheKey {
		t.Fatal("config absent from cache key")
	}
	p.Segments[0].Text += "!"
	changed, err = r.Render(context.Background(), p, c)
	if err != nil {
		t.Fatal(err)
	}
	if changed.CacheKey == first.CacheKey {
		t.Fatal("text absent from cache key")
	}
}

func TestRenderPaginationAndRanges(t *testing.T) {
	r := testRenderer(t)
	p := renderProjection(strings.Repeat("中e\u0301", 80) + "\nlast")
	c := DefaultRenderConfig()
	c.LineHeight = 28
	c.Width = 128
	c.Height = 128
	c.MinHeight = 64
	c.Padding = 4
	got, err := r.Render(context.Background(), p, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pages) < 2 {
		t.Fatal("did not paginate")
	}
	at := 0
	for i, page := range got.Pages {
		if page.Index != i || page.Height < c.MinHeight || page.Height > c.Height {
			t.Fatal("page geometry")
		}
		for _, span := range page.Ranges {
			if span.SourceID != "source-1" || span.Field != "projection" || span.Start != at {
				t.Fatalf("range gap %d %+v", at, span)
			}
			at = span.End
			runes := []rune(p.Segments[0].Text)
			if at < len(runes) && runes[at] == '\u0301' {
				t.Fatal("split grapheme")
			}
		}
	}
	if at != len([]rune(p.Segments[0].Text)) {
		t.Fatal("incomplete coverage")
	}
	c.MaxPages = 1
	partial, err := r.Render(context.Background(), p, c)
	if !errors.Is(err, ErrRenderLimit) || len(partial.Pages) != 1 || len(partial.Uncovered) == 0 {
		t.Fatalf("limit didn't report coverage: %v %+v", err, partial)
	}
	last := 0
	for _, span := range partial.Pages[0].Ranges {
		last = span.End
	}
	if partial.Uncovered[0].Start != last || partial.Uncovered[len(partial.Uncovered)-1].End != at {
		t.Fatal("incorrect omitted suffix")
	}
}

func TestRenderMissingAndCancellation(t *testing.T) {
	r := testRenderer(t)
	got, err := r.Render(context.Background(), renderProjection("hello \U0010FFFF"), func() RenderConfig { c := DefaultRenderConfig(); c.LineHeight = 28; return c }())
	if !errors.Is(err, ErrMissingGlyph) || len(got.Missing) != 1 || len(got.Pages) != 0 || len(got.Uncovered) != 1 {
		t.Fatalf("missing glyph: %v %+v", err, got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err = r.Render(ctx, renderProjection("hello"), DefaultRenderConfig())
	if !errors.Is(err, context.Canceled) || len(got.Uncovered) != 1 {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestRenderLimitsAndEmpty(t *testing.T) {
	r := testRenderer(t)
	c := DefaultRenderConfig()
	c.LineHeight = 28
	got, err := r.Render(context.Background(), Projection{}, c)
	if err != nil || len(got.Pages) != 0 {
		t.Fatalf("empty input: %v", err)
	}
	tests := []struct {
		name   string
		change func(*RenderConfig)
		want   error
	}{
		{"geometry", func(c *RenderConfig) { c.Width = 0 }, ErrRenderInput},
		{"memory", func(c *RenderConfig) { c.MaxMemoryBytes = 1 }, ErrRenderLimit},
		{"pixels", func(c *RenderConfig) { c.MaxPixels = 1 }, ErrRenderLimit},
		{"font", func(c *RenderConfig) { c.FontSize = 1 }, ErrRenderInput},
		{"minimum", func(c *RenderConfig) { c.MinHeight = c.Height + 1 }, ErrRenderInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := c
			tt.change(&cfg)
			got, err := r.Render(context.Background(), renderProjection("hello"), cfg)
			if !errors.Is(err, tt.want) || len(got.Uncovered) != 1 {
				t.Fatalf("%v %+v", err, got)
			}
		})
	}
	got, err = r.Render(context.Background(), renderProjection(strings.Repeat("x", 8<<20+1)), c)
	if !errors.Is(err, ErrRenderLimit) {
		t.Fatalf("input size: %v %+v", err, got)
	}
}

func TestRenderLastPageMinimumAndNoClipping(t *testing.T) {
	r := testRenderer(t)
	c := DefaultRenderConfig()
	c.LineHeight = 28
	c.MinHeight = 512
	got, err := r.Render(context.Background(), renderProjection("中文 glyph gyp e\u0301"), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pages) != 1 || got.Pages[0].Height != 512 {
		t.Fatal("last page below configured minimum")
	}
	image, err := png.Decode(bytes.NewReader(got.Pages[0].PNG))
	if err != nil {
		t.Fatal(err)
	}
	// There is a full white padding strip above and below the rasterized lines.
	for x := 0; x < image.Bounds().Dx(); x++ {
		if color.GrayModel.Convert(image.At(x, 0)).(color.Gray).Y != 255 || color.GrayModel.Convert(image.At(x, 511)).(color.Gray).Y != 255 {
			t.Fatal("glyph clipped at page border")
		}
	}
	c.LineHeight = 22
	_, err = r.Render(context.Background(), renderProjection("中文 gyp"), c)
	if !errors.Is(err, ErrRenderInput) {
		t.Fatal("accepted line spacing that clips font metrics")
	}
}

func TestRenderHeaderOnlyPagesRetainSource(t *testing.T) {
	c := DefaultRenderConfig()
	c.Width, c.Height, c.MinHeight = 160, 60, 60
	p := renderProjection("body")
	got, err := testRenderer(t).Render(context.Background(), p, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pages) < 2 {
		t.Fatal("expected header pagination")
	}
	for _, page := range got.Pages {
		if len(page.Ranges) == 0 || page.Ranges[0].SourceID != "source-1" {
			t.Fatalf("page %d lost source", page.Index)
		}
	}
	if !completePageCoverage(p, got.Pages) {
		t.Fatal("header references changed body coverage")
	}
}
