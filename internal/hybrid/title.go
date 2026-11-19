package hybrid

import (
	"path/filepath"

	"github.com/rainchan/webmux/internal/session"
)

// observeTerminalTitle records a live terminal run's OSC 0/2 title for the
// current agent session so switching to chat keeps it. Clearing an active
// terminal title clears the matching session title.
func (a *actor) observeTerminalTitle(title string, seq uint64) {
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
	title := session.AutomaticTitle(raw)
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
		} else if term := id.Title; term != "" {
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
