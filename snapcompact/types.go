// Package snapcompact provides deterministic, provider-neutral history projections.
package snapcompact

import "context"

const Version = "webmux-snapcompact-v1"

// Record is immutable input; order in the slice is authoritative. IDs must be unique.
// Kind is prose, code, patch, or structured. Unknown kinds preserve structure.
type Record struct {
	ID, Role, ToolName, CallID, Status, Kind, Text, CallBody string
	Arguments                                                map[string]any
}

type Input struct {
	History     []Record
	CurrentTask string
}

// Range uses half-open Unicode scalar offsets in the named original field.
// Render ranges instead use Field=projection and offsets in Segment.Text.
type Range struct {
	SourceID, Field string
	Start, End      int
}
type Omission struct {
	Range         Range
	OriginalRunes int
	Reason        string
}
type Segment struct {
	SourceID, Role, ToolName, CallID, Status, Text string
	Sources                                        []Range
}
type Projection struct {
	Version, CacheKey, CurrentTask string
	Segments                       []Segment
	Omissions                      []Omission
}
type PreprocessConfig struct {
	OutputRunes, ArgumentRunes, CallRunes int
	HeadPercent                           int
	MaxInputBytes                         int
}

func DefaultPreprocessConfig() PreprocessConfig {
	return PreprocessConfig{2000, 2000, 2000, 60, 8 << 20}
}

// Page ranges refer to projection segments, never to raw source offsets.
type Page struct {
	PNG                  []byte
	Width, Height, Index int
	Ranges               []Range
}
type RenderConfig struct {
	Width, Height, MinHeight, FontSize, LineHeight, Padding, MaxPages int
	MaxPixels, MaxMemoryBytes                                         int64
}

func DefaultRenderConfig() RenderConfig {
	return RenderConfig{1024, 1024, 64, 18, 28, 16, 32, 32 << 20, 256 << 20}
}

type MissingGlyph struct {
	SourceID string
	Rune     rune
}
type RenderResult struct {
	Pages     []Page
	Uncovered []Range
	Missing   []MissingGlyph
	CacheKey  string
}

// Renderer implementations must return uncovered ranges on bounded partial results.
type Renderer interface {
	Render(context.Context, Projection, RenderConfig) (RenderResult, error)
}
