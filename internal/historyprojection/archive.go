package historyprojection

import (
	"context"
	"encoding/json"
	"sync"
	"unicode/utf8"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/tiktoken-go/tokenizer/codec"
)

var archiveTokenizer = sync.OnceValue(codec.NewCl100kBase)

// ArchiveTextTokens estimates context usage with the upstream edge-sizing
// baseline and its 15% margin. Wire and prompt byte ceilings are separate.
func ArchiveTextTokens(text string) int64 {
	var tokens int64
	for len(text) > 0 {
		// Bound the tokenizer's work on long unbroken words and data blobs.
		end := min(len(text), 1024)
		for end < len(text) && !utf8.RuneStart(text[end]) {
			end++
		}
		count, err := archiveTokenizer().Count(text[:end])
		if err != nil || count == 0 {
			count = end
		}
		tokens += int64(count)
		text = text[end:]
	}
	return (tokens*115 + 99) / 100
}

type archiveCandidate struct {
	snapcompact.Archive
	tokens int64
	fits   bool
}

// CompactProjection compares a frame of text at each edge with a more compact
// layout. The history token allowance and the encoded wire room exclude the
// caller's fixed prompt overhead.
func (s *Service) CompactProjection(ctx context.Context, projection snapcompact.Projection, cfg snapcompact.ArchiveConfig, tokens, wireBytes int64) (snapcompact.Archive, error) {
	s.archiveOnce.Do(func() { s.archive, s.archiveError = snapcompact.NewArchiveRenderer() })
	if s.archiveError != nil {
		return snapcompact.Archive{}, s.archiveError
	}

	wide, err := s.compactWithEdges(ctx, projection, cfg, tokens, wireBytes)
	if err != nil || (wide.fits && len(wide.Omitted) == 0 && len(wide.Render.Pages) == 0) {
		return wide.Archive, err
	}

	const compactEdgeBytes = 4096
	cfg.MaxTextBytes = min(cfg.MaxTextBytes, compactEdgeBytes, max(int64(1), tokens/3), max(int64(1), wireBytes/12))
	packed, err := s.compactWithEdges(ctx, projection, cfg, tokens, wireBytes)
	if ctx.Err() != nil {
		return packed.Archive, ctx.Err()
	}
	if !wide.fits || len(wide.Render.Pages) == 0 {
		return packed.Archive, err
	}
	if err == nil && packed.fits && len(packed.Render.Pages) > 0 &&
		(packed.RetainedRunes > wide.RetainedRunes ||
			packed.RetainedRunes == wide.RetainedRunes && packed.tokens < wide.tokens) {
		return packed.Archive, nil
	}
	return wide.Archive, nil
}

func (s *Service) compactWithEdges(ctx context.Context, projection snapcompact.Projection, cfg snapcompact.ArchiveConfig, tokens, wireBytes int64) (archiveCandidate, error) {
	// One image token fits no supported frame. This pass gets the core's
	// grapheme-aligned text edges without encoding any images.
	cfg.MaxImageTokens = 1
	edges, err := s.archive.Compact(ctx, projection, cfg)
	if err != nil {
		return archiveCandidate{Archive: edges}, err
	}
	edgeTokens := ArchiveTextTokens(edges.TextHead) + ArchiveTextTokens(edges.TextTail)
	remainingWire := wireBytes - archiveJSONBytes(edges.TextHead) - archiveJSONBytes(edges.TextTail)
	if edgeTokens > tokens || remainingWire < 0 {
		return archiveCandidate{}, nil
	}
	if len(edges.Omitted) == 0 || edgeTokens == tokens || remainingWire < 2 {
		return archiveCandidate{Archive: edges, tokens: edgeTokens, fits: true}, nil
	}
	cfg.MaxImageTokens = tokens - edgeTokens
	cfg.MaxImageBytes = min(cfg.MaxImageBytes, remainingWire*3/4)
	archive, err := s.archive.Compact(ctx, projection, cfg)
	return archiveCandidate{Archive: archive, tokens: edgeTokens + archive.ImageTokens, fits: err == nil}, err
}

func archiveJSONBytes(text string) int64 {
	encoded, _ := json.Marshal(text)
	return int64(len(encoded))
}
