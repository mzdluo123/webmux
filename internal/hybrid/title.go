package hybrid

import (
	"path/filepath"
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
// terminal title clears the matching session title.
func (a *actor) observeTerminalTitle(raw string, seq uint64) {
	title := sessionTitle(raw)
	if title == "" {
		prev := a.termTitle
		a.termTitle, a.termTitleSeq = "", 0
		if prev != "" && a.title == prev && a.titleSession == a.agentSessionID {
			a.title, a.titleSession = "", ""
			a.resetRecordTitle()
			a.dirty = true
		}
		return
	}
	if a.agentSessionID == "" {
		return
	}
	a.termTitleSeq = seq
	if title == a.termTitle {
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
	if sid == "" {
		return
	}
	if a.side == SideTUI && a.shell != nil {
		id := a.shell.Identity()
		if a.agentSessionID != "" {
			a.observeTerminalTitle(id.Title, id.TitleSeq)
		} else if term := sessionTitle(id.Title); term != "" {
			a.termTitle, a.termTitleSeq = term, id.TitleSeq
			a.promptTitle = false
			if a.title != term || a.titleSession != sid {
				a.title, a.titleSession = term, sid
				a.dirty = true
			}
			return
		}
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

// resetRecordTitle restores the recoverable record's title to the cwd basename
// when an unnamed session clears its conversation title.
func (a *actor) resetRecordTitle() {
	var reset string
	a.record(false, func(r *Record) {
		if r.Name == "" {
			r.Title = filepath.Base(r.Cwd)
			reset = r.Title
		}
	})
	if reset != "" {
		a.recordTitle = reset
	}
}
