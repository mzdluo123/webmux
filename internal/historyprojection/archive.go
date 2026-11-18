package historyprojection

import (
	"sync"
	"unicode/utf8"

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
