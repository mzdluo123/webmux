// Renderer test cases adapted from oh-my-pi, crates/pi-natives/src/snapcompact.rs
// at 7c49c11aa5bbaf3f418406583096168ad5b8c6dc (tests module).
// Copyright (c) 2025 Mario Zechner
// Copyright (c) 2025-2026 Can Bölük
// Copyright (c) 2026 Stencil Labs, Inc.
// SPDX-License-Identifier: MIT
// The complete upstream MIT license is in testdata/upstream/LICENSE.
//
// These tests preserve the named upstream assertions that apply to this API.
// Font metrics, source headers, grayscale antialiasing, and explicit uncovered
// overflow differ deliberately. They are semantic ports, not pixel parity tests.
package snapcompact

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"
)

// Upstream: fonts_parse_ascii_coverage and xorg_fonts_parse_and_render.
// Noto replaces the bitmap fonts, but every printable ASCII glyph is required.
func TestUpstreamRenderPrintableASCIICoverage(t *testing.T) {
	renderer := testRenderer(t).(*pngRenderer)
	var buffer sfnt.Buffer
	for cp := rune(0x20); cp < 0x7f; cp++ {
		glyph, err := renderer.font.GlyphIndex(&buffer, cp)
		if err != nil {
			t.Fatal(err)
		}
		if glyph == 0 {
			t.Fatalf("missing glyph for U+%04X", cp)
		}
	}
	var printable strings.Builder
	for cp := rune(0x20); cp < 0x7f; cp++ {
		printable.WriteRune(cp)
	}
	got, err := renderer.Render(context.Background(), renderProjection(printable.String()), DefaultRenderConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pages) == 0 {
		t.Fatal("ASCII must produce raster output")
	}
}

// Upstream: silver_font_covers_cjk_scripts, with its exact three test scalars.
// This checks Noto rather than Silver; it makes no claim about either's full cmap.
func TestUpstreamRenderCJKScriptCoverage(t *testing.T) {
	renderer := testRenderer(t).(*pngRenderer)
	var buffer sfnt.Buffer
	for _, ch := range []rune{'こ', '你', '안'} {
		glyph, err := renderer.font.GlyphIndex(&buffer, ch)
		if err != nil {
			t.Fatal(err)
		}
		if glyph == 0 {
			t.Fatalf("font must cover %q", ch)
		}
	}
	got, err := renderer.Render(context.Background(), renderProjection("こんにちは 你好 안녕"), DefaultRenderConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pages) != 1 {
		t.Fatal("CJK fixture should fit one page")
	}
}

// Upstream: bw_variant_prints_black_only, using the exact "Hi. Ok." fixture.
// Outline rasterization adds gray edge pixels. R=G=B proves that sentence hues
// are absent while allowing antialiasing; both nonwhite ink and black are required.
func TestUpstreamRenderBWInk(t *testing.T) {
	got, err := testRenderer(t).Render(context.Background(), renderProjection("Hi. Ok."), DefaultRenderConfig())
	if err != nil {
		t.Fatal(err)
	}
	image, err := png.Decode(bytes.NewReader(got.Pages[0].PNG))
	if err != nil {
		t.Fatal(err)
	}
	black, ink := false, false
	for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
		for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
			pixel := color.NRGBAModel.Convert(image.At(x, y)).(color.NRGBA)
			if pixel.R != pixel.G || pixel.G != pixel.B {
				t.Fatal("bw must use achromatic ink")
			}
			ink = ink || pixel.R != 255
			black = black || pixel.R == 0
		}
	}
	if !ink || !black {
		t.Fatal("raster must contain black glyph ink")
	}
}

// Upstream: rejects_bad_shapes (size 0). String font/variant selectors are not
// part of this typed API; its width/height must both be positive instead.
func TestUpstreamRenderRejectsBadShapes(t *testing.T) {
	renderer := testRenderer(t)
	for _, dimension := range []string{"width", "height"} {
		t.Run(dimension, func(t *testing.T) {
			config := DefaultRenderConfig()
			if dimension == "width" {
				config.Width = 0
			} else {
				config.Height = 0
			}
			_, err := renderer.Render(context.Background(), renderProjection("x"), config)
			if !errors.Is(err, ErrRenderInput) {
				t.Fatalf("zero %s: %v", dimension, err)
			}
		})
	}
}

// Upstream: frame_height_hugs_used_rows_above_a_64px_floor. The original
// "0123456789" fixture still fits below the floor using a 6px outline font;
// source headers count as an additional line. Explicit newlines replace the
// upstream 32-column cell grid when testing the above-floor line count.
func TestUpstreamRenderHeightHugsRowsWith64Floor(t *testing.T) {
	renderer := testRenderer(t)
	config := DefaultRenderConfig()
	config.Width = 256
	config.Height = 256
	config.FontSize = 6
	config.LineHeight = 10
	config.Padding = 0
	config.MinHeight = 64
	tests := []struct {
		name, text string
		height     int
	}{
		{"floor", "0123456789", 64},
		{"nine_rows", strings.TrimSuffix(strings.Repeat("x\n", 9), "\n"), 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderer.Render(context.Background(), renderProjection(tt.text), config)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Pages) != 1 {
				t.Fatal("fixture unexpectedly paginated")
			}
			image, err := png.DecodeConfig(bytes.NewReader(got.Pages[0].PNG))
			if err != nil {
				t.Fatal(err)
			}
			if image.Width != 256 || image.Height != tt.height {
				t.Fatalf("dimensions = %dx%d; want 256x%d", image.Width, image.Height, tt.height)
			}
		})
	}
}

// Upstream: bitmap_inks_sentences_and_caps_capacity, overflow assertion. Go
// reports a bounded prefix plus uncovered suffix instead of silently dropping
// excess cells. The exact upstream overflow fixture is 100 ASCII x characters.
func TestUpstreamRenderOverflowStaysBoundedAndReported(t *testing.T) {
	renderer := testRenderer(t)
	config := DefaultRenderConfig()
	config.Width = 64
	config.Height = 64
	config.Padding = 0
	config.MaxPages = 1
	got, err := renderer.Render(context.Background(), renderProjection(strings.Repeat("x", 100)), config)
	if !errors.Is(err, ErrRenderLimit) {
		t.Fatalf("overflow must be explicit: %v", err)
	}
	if len(got.Pages) != 1 || len(got.Uncovered) == 0 {
		t.Fatal("overflow lacks prefix page or uncovered suffix")
	}
	page := got.Pages[0]
	image, err := png.DecodeConfig(bytes.NewReader(page.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if image.Width != 64 || image.Height != 64 {
		t.Fatalf("out-of-bounds raster: %dx%d", image.Width, image.Height)
	}
	if got.Uncovered[len(got.Uncovered)-1].End != 100 {
		t.Fatal("overflow lost the end of the source range")
	}
}
