package hybrid

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// maxPromptTitleRunes bounds the first-prompt title.
	maxPromptTitleRunes = 80
)

func dropControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(s, ""))
}

// PromptTitle is the title for a first prompt: control
// characters dropped, whitespace runs folded to one space, and at most
// maxPromptTitleRunes runes, the last an ellipsis when cut.
func PromptTitle(text string) string {
	s := strings.Join(strings.Fields(dropControl(strings.Map(func(r rune) rune {
		// Line breaks and tabs separate words; strip only other controls.
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, text))), " ")
	if utf8.RuneCountInString(s) <= maxPromptTitleRunes {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:maxPromptTitleRunes-1])) + "…"
}

// observeTerminalTitle records a live terminal run's OSC 0/2 title for the
// current agent session so switching to chat keeps it. Clearing an active
// terminal title clears the matching session title; a title left over from an
// earlier agent session in the same terminal run is ignored.
func (a *actor) observeTerminalTitle(raw string) {
	title := sessionTitle(raw)
	if title == "" {
		prev := a.termTitle
		a.termTitle = ""
		if prev != "" && a.title == prev && a.titleSession == a.agentSessionID {
			a.title = ""
			a.dirty = true
		}
		return
	}
	if a.agentSessionID == "" || title == a.termTitle {
		return
	}
	a.termTitle = title
	a.promptTitle = false
	if a.title != title || a.titleSession != a.agentSessionID {
		a.title, a.titleSession = title, a.agentSessionID
		a.dirty = true
	}
}

// adoptListedTitle applies a non-empty title returned by session/list for sid
// when the live terminal run has not reported its own OSC 0/2 title.
func (a *actor) adoptListedTitle(sid, raw string) {
	title := sessionTitle(raw)
	if sid == "" || title == "" || a.termTitle != "" {
		return
	}
	a.promptTitle = false
	if a.title != title || a.titleSession != sid {
		a.title, a.titleSession = title, sid
		a.dirty = true
	}
}

// adoptProbeTitle applies any session/list title carried by ans for sid unless
// the live terminal run already reported an OSC 0/2 title.
func (a *actor) adoptProbeTitle(sid string, ans probeAnswer) {
	if a.side == SideTUI && a.shell != nil {
		a.observeTerminalTitle(a.shell.Identity().Title)
	}
	title := ans.title
	if title == "" {
		for _, c := range ans.candidates {
			if c.AgentSessionID == sid {
				title = c.Title
				break
			}
		}
	}
	a.adoptListedTitle(sid, title)
}
