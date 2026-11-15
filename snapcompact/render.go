package snapcompact

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// The unmodified Noto font and its OFL notice are shipped together.
//
//go:embed fonts/NotoSansCJKsc-Regular.otf
var renderFont []byte

var (
	ErrRenderLimit  = errors.New("snapcompact render resource limit")
	ErrMissingGlyph = errors.New("snapcompact unsupported glyph")
	ErrRenderInput  = errors.New("snapcompact invalid render input")
)

type pngRenderer struct{ font *opentype.Font }

var _ Renderer = (*pngRenderer)(nil)

// NewRenderer loads the embedded, redistributable Noto Sans CJK SC font.
// Faces and their rasterizer buffers are private to each Render call.
func NewRenderer() (Renderer, error) {
	f, err := opentype.Parse(renderFont)
	if err != nil {
		return nil, fmt.Errorf("parse snapcompact font: %w", err)
	}
	return &pngRenderer{font: f}, nil
}

type renderLine struct {
	text       string
	source     string
	start, end int
	header     bool
}

func allRenderRanges(p Projection) []Range {
	ranges := make([]Range, 0, len(p.Segments))
	for _, s := range p.Segments {
		ranges = append(ranges, Range{SourceID: s.SourceID, Field: "projection", End: utf8.RuneCountInString(s.Text)})
	}
	return ranges
}

func validateRender(p Projection, c RenderConfig) (int64, error) {
	if c.Width < 1 || c.Height < 1 || c.Width > 8192 || c.Height > 8192 || c.MinHeight < 1 || c.MinHeight > c.Height || c.FontSize < 6 || c.FontSize > 128 || c.LineHeight < c.FontSize+4 || c.LineHeight > 512 || c.Padding < 0 || c.Padding > 1024 || c.Width <= 2*c.Padding || c.Height < 2*c.Padding+c.LineHeight || c.MaxPages < 1 || c.MaxPages > 4096 || c.MaxPixels < 1 || c.MaxPixels > 128<<20 || c.MaxMemoryBytes < 1 || c.MaxMemoryBytes > 1<<30 {
		return 0, fmt.Errorf("%w: geometry or limits", ErrRenderInput)
	}
	var n int64
	if len(p.Segments) > 100000 {
		return 0, fmt.Errorf("%w: record count", ErrRenderLimit)
	}
	ids := make(map[string]bool, len(p.Segments))
	for _, s := range p.Segments {
		if s.SourceID == "" || ids[s.SourceID] || !utf8.ValidString(s.Text) || !utf8.ValidString(s.SourceID+s.Role+s.ToolName+s.CallID+s.Status) {
			return 0, fmt.Errorf("%w: source identity or UTF-8", ErrRenderInput)
		}
		ids[s.SourceID] = true
		n += int64(len(s.Text) + len(s.SourceID) + len(s.Role) + len(s.ToolName) + len(s.CallID) + len(s.Status))
	}
	if n > 8<<20 {
		return 0, fmt.Errorf("%w: input bytes", ErrRenderLimit)
	}
	// Font tables, layout/cluster bookkeeping, and rasterizer have conservative reserves.
	reserve := int64(len(renderFont))*3 + n*128 + 1<<20
	if reserve > c.MaxMemoryBytes {
		return 0, fmt.Errorf("%w: layout memory", ErrRenderLimit)
	}
	return reserve, nil
}

func (r *pngRenderer) Render(ctx context.Context, p Projection, c RenderConfig) (result RenderResult, err error) {
	// Validate fixed input bounds before allocating any source or layout index.
	if len(p.Segments) > 100000 {
		return result, fmt.Errorf("%w: record count", ErrRenderLimit)
	}
	var inputBytes int64
	for _, segment := range p.Segments {
		inputBytes += int64(len(segment.Text)) + int64(len(segment.SourceID)) + int64(len(segment.Role)) + int64(len(segment.ToolName)) + int64(len(segment.CallID)) + int64(len(segment.Status))
		if inputBytes > 8<<20 {
			return result, fmt.Errorf("%w: input bytes", ErrRenderLimit)
		}
	}
	result.Uncovered = allRenderRanges(p)
	if err = ctx.Err(); err != nil {
		return result, err
	}
	reserve, err := validateRender(p, c)
	if err != nil {
		return result, err
	}
	keyData, err := json.Marshal(struct {
		Version, Projection string
		Font                [32]byte
		Config              RenderConfig
		Segments            []Segment
	}{Version, p.CacheKey, sha256.Sum256(renderFont), c, p.Segments})
	if err != nil {
		return result, fmt.Errorf("render key: %w", err)
	}
	key := sha256.Sum256(keyData)
	result.CacheKey = hex.EncodeToString(key[:])
	face, err := opentype.NewFace(r.font, &opentype.FaceOptions{Size: float64(c.FontSize), DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return result, fmt.Errorf("create snapcompact face: %w", err)
	}
	defer func() {
		if closeErr := face.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close snapcompact face: %w", closeErr)
		}
	}()
	if c.LineHeight < face.Metrics().Ascent.Ceil()+face.Metrics().Descent.Ceil() {
		return result, fmt.Errorf("%w: line height clips font metrics", ErrRenderInput)
	}
	var buf sfnt.Buffer
	missing := make(map[MissingGlyph]bool)
	// Check every visible scalar before drawing: a tofu box is never reported as coverage.
	for _, s := range p.Segments {
		for _, ch := range s.Text + s.SourceID + s.Role + s.ToolName + s.CallID + s.Status {
			if ch == '\n' {
				continue
			}
			if err = ctx.Err(); err != nil {
				return result, err
			}
			idx, glyphErr := r.font.GlyphIndex(&buf, ch)
			if glyphErr != nil {
				return result, fmt.Errorf("lookup snapcompact glyph: %w", glyphErr)
			}
			if idx == 0 {
				m := MissingGlyph{SourceID: s.SourceID, Rune: ch}
				if !missing[m] {
					missing[m] = true
					result.Missing = append(result.Missing, m)
				}
			}
		}
	}
	if len(result.Missing) > 0 {
		return result, ErrMissingGlyph
	}
	width := fixed.I(c.Width - 2*c.Padding)
	lines := make([]renderLine, 0)
	for _, s := range p.Segments {
		header := fmt.Sprintf("[%q | %q | %q | %q | %q | history]", s.SourceID, s.Role, s.ToolName, s.CallID, s.Status)
		for _, part := range []struct {
			text   string
			header bool
		}{{header, true}, {s.Text, false}} {
			gs := uniseg.NewGraphemes(part.text)
			offset, start := 0, 0
			var line string
			for gs.Next() {
				if err = ctx.Err(); err != nil {
					return result, err
				}
				cluster := gs.Str()
				size := utf8.RuneCountInString(cluster)
				if cluster == "\n" || cluster == "\r\n" {
					lines = append(lines, renderLine{line, s.SourceID, start, offset + size, part.header})
					offset += size
					start = offset
					line = ""
					continue
				}
				if font.MeasureString(face, cluster) > width {
					return result, fmt.Errorf("%w: grapheme wider than page", ErrRenderInput)
				}
				if line != "" && font.MeasureString(face, line+cluster) > width {
					lines = append(lines, renderLine{line, s.SourceID, start, offset, part.header})
					start = offset
					line = ""
				}
				line += cluster
				offset += size
			}
			// A trailing newline is already covered by the preceding line.
			if line != "" || offset == 0 {
				lines = append(lines, renderLine{line, s.SourceID, start, offset, part.header})
			}
		}
	}
	result.Uncovered = nil
	capacity := (c.Height - 2*c.Padding) / c.LineHeight
	var pixels, encodedBytes int64
	for at := 0; at < len(lines); {
		if err = ctx.Err(); err != nil {
			result.Uncovered = uncoveredRenderLines(lines[at:])
			return result, err
		}
		end := at + capacity
		if end > len(lines) {
			end = len(lines)
		}
		height := 2*c.Padding + (end-at)*c.LineHeight
		if height < c.MinHeight {
			height = c.MinHeight
		}
		pagePixels := int64(c.Width) * int64(height)
		// RGBA backing and worst-case PNG + encoder scratch + retained pages.
		if len(result.Pages) >= c.MaxPages || pixels+pagePixels > c.MaxPixels || reserve+encodedBytes+pagePixels*16 > c.MaxMemoryBytes {
			result.Uncovered = uncoveredRenderLines(lines[at:])
			return result, ErrRenderLimit
		}
		canvas := image.NewRGBA(image.Rect(0, 0, c.Width, height))
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		drawer := font.Drawer{Dst: canvas, Src: image.NewUniform(color.Black), Face: face}
		page := Page{Width: c.Width, Height: height, Index: len(result.Pages)}
		for i, line := range lines[at:end] {
			if err = ctx.Err(); err != nil {
				result.Uncovered = uncoveredRenderLines(lines[at:])
				return result, err
			}
			drawer.Dot = fixed.P(c.Padding, c.Padding+i*c.LineHeight+(c.LineHeight-face.Metrics().Descent.Ceil()))
			drawer.DrawString(line.text)
			if !line.header {
				page.Ranges = appendRenderRange(page.Ranges, Range{SourceID: line.source, Field: "projection", Start: line.start, End: line.end})
			}
		}
		var out bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestSpeed}
		if err = encoder.Encode(contextPNGWriter{ctx: ctx, out: &out}, canvas); err != nil {
			result.Uncovered = uncoveredRenderLines(lines[at:])
			return result, fmt.Errorf("encode snapcompact PNG: %w", err)
		}
		if err = ctx.Err(); err != nil {
			result.Uncovered = uncoveredRenderLines(lines[at:])
			return result, err
		}
		page.PNG = out.Bytes()
		encodedBytes += int64(len(page.PNG))
		pixels += pagePixels
		result.Pages = append(result.Pages, page)
		at = end
	}
	return result, nil
}

func appendRenderRange(ranges []Range, next Range) []Range {
	if len(ranges) > 0 {
		last := &ranges[len(ranges)-1]
		if last.SourceID == next.SourceID && last.End == next.Start {
			last.End = next.End
			return ranges
		}
	}
	return append(ranges, next)
}
func uncoveredRenderLines(lines []renderLine) []Range {
	var ranges []Range
	for _, line := range lines {
		if !line.header {
			ranges = appendRenderRange(ranges, Range{SourceID: line.source, Field: "projection", Start: line.start, End: line.end})
		}
	}
	return ranges
}

// png.Encoder writes IDAT chunks through this cancellation boundary.
type contextPNGWriter struct {
	ctx context.Context
	out *bytes.Buffer
}

func (w contextPNGWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.out.Write(p)
}
