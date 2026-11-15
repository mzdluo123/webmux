package snapcompact

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// This is the previous full-line measuring implementation, retained only as a
// compatibility oracle and benchmark baseline for the linear-time wrapper.
func referenceWrappedRenderLines(face font.Face, width fixed.Int26_6, text string) ([]renderLine, error) {
	var lines []renderLine
	gs := uniseg.NewGraphemes(text)
	offset, start := 0, 0
	var line string
	for gs.Next() {
		cluster := gs.Str()
		size := utf8.RuneCountInString(cluster)
		if cluster == "\n" || cluster == "\r\n" {
			lines = append(lines, renderLine{line, "source", start, offset + size, false})
			offset += size
			start = offset
			line = ""
			continue
		}
		if font.MeasureString(face, cluster) > width {
			return lines, ErrRenderInput
		}
		if line != "" && font.MeasureString(face, line+cluster) > width {
			lines = append(lines, renderLine{line, "source", start, offset, false})
			start = offset
			line = ""
		}
		line += cluster
		offset += size
	}
	if line != "" || offset == 0 {
		lines = append(lines, renderLine{line, "source", start, offset, false})
	}
	return lines, nil
}

type boundaryKernFace struct{ font.Face }

func (f boundaryKernFace) Kern(left, right rune) fixed.Int26_6 {
	if (left == 'A' && right == 'V') || (left == 'T' && right == 'o') || (left == '你' && right == 'e') {
		return -fixed.I(2)
	}
	return f.Face.Kern(left, right)
}

func TestRenderWrapMatchesFullLineMeasurements(t *testing.T) {
	renderer := testRenderer(t).(*pngRenderer)
	face, err := opentype.NewFace(renderer.font, &opentype.FaceOptions{Size: 18, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := face.Close(); err != nil {
			t.Error(err)
		}
	})
	tests := []struct{ name, text string }{
		{"kerning_pairs", "AVAV ToTo AV To"},
		{"non_ascii_combining", "你e\u0301好中文 e\u0301 AV To"},
		{"newlines", "AV To\r\n中文e\u0301\n\nTo\n"},
		{"empty", ""},
	}
	for _, faceTest := range []struct {
		name string
		face font.Face
	}{{"noto", face}, {"nonzero_boundary_kern", boundaryKernFace{Face: face}}} {
		t.Run(faceTest.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					for _, pixels := range []int{24, 35, 64, 127, 1024} {
						width := fixed.I(pixels)
						want, err := referenceWrappedRenderLines(faceTest.face, width, tt.text)
						if err != nil {
							t.Fatal(err)
						}
						got, err := appendWrappedRenderLines(context.Background(), nil, faceTest.face, width, tt.text, "source", false)
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("width %d: got %#v; want %#v", pixels, got, want)
						}
						for _, line := range got {
							measured := font.MeasureString(faceTest.face, line.text)
							if measured > width {
								t.Fatalf("line overflows: %q", line.text)
							}
							drawer := font.Drawer{Dst: image.NewRGBA(image.Rect(0, 0, 1, 1)), Src: image.NewUniform(color.Black), Face: faceTest.face}
							drawer.DrawString(line.text)
							if drawer.Dot.X != measured {
								t.Fatalf("DrawString advance %v differs from measure %v", drawer.Dot.X, measured)
							}
						}
					}
				})
			}
		})
	}
}

// The old wrapper repeatedly copied and remeasured the growing line. Compare
// both paths on a very wide line; this isolates layout from font parse/PNG work.
func BenchmarkRenderWrap(b *testing.B) {
	renderer, err := NewRenderer()
	if err != nil {
		b.Fatal(err)
	}
	face, err := opentype.NewFace(renderer.(*pngRenderer).font, &opentype.FaceOptions{Size: 6, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := face.Close(); err != nil {
			b.Error(err)
		}
	})
	for _, size := range []int{256, 2048} {
		text := strings.Repeat("AVTo", size/4)
		for _, mode := range []string{"full_line_baseline", "incremental"} {
			b.Run(fmt.Sprintf("%s/%d", mode, size), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(text)))
				for b.Loop() {
					if mode == "full_line_baseline" {
						_, err = referenceWrappedRenderLines(face, fixed.I(8192), text)
					} else {
						_, err = appendWrappedRenderLines(context.Background(), nil, face, fixed.I(8192), text, "source", false)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
