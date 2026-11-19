package session

import (
	"strings"
	"unicode"
)

const maxAutomaticTitleRunes = 80

// AutomaticTitle makes a single-line session title from a prompt or program title.
// The limit includes the ellipsis when the title is shortened.
func AutomaticTitle(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(text, ""))
	title := strings.Join(strings.Fields(text), " ")
	if runes := []rune(title); len(runes) > maxAutomaticTitleRunes {
		return strings.TrimRight(string(runes[:maxAutomaticTitleRunes-1]), " ") + "…"
	}
	return title
}
