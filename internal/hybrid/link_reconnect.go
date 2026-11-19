package hybrid

import (
	"fmt"

	"github.com/rainchan/webmux/internal/acp"
)

// checkLink handles a connection that died before the actor installed it.
func (a *actor) checkLink() {
	if link := a.link; link != nil {
		select {
		case <-link.conn.Done():
			a.linkLost(link)
		default:
		}
	}
}

// linkLost handles the end of the current ACP connection outside a
// switch or close: the agent exited or broke the protocol. The running
// turn ends cancelled and the session reconnects once (D24): with
// session/resume when possible, else session/load in a new generation. A
// second loss before a turn completes normally, or a failed reconnect,
// interrupts the session.
func (a *actor) linkLost(link *acpLink) {
	if a.link != link || a.stopped {
		return
	}
	a.stopBackgroundWatch()
	a.link = nil
	a.chat.PromptCapabilities = PromptCapabilities{}
	a.dirty = true
	a.svc.log.Warn("hybrid acp connection ended", "id", a.id, "error", link.conn.Err())
	a.withdrawAll()
	if a.turn != nil {
		a.turnEnded(a.turn, acp.PromptResponse{}, acp.ErrDisconnected)
	}
	if a.state != StateActive || a.closing {
		a.subtask(func() { _ = link.stop() })
		return
	}
	if a.reconnects > 0 {
		a.emit(Event{Kind: EventNotice, Notice: &Notice{Level: acp.NoticeError, Text: "与 agent 的连接再次断开"}})
		a.interrupt("turn", false)
		a.stopLost(link)
		return
	}
	a.reconnects++
	a.emit(Event{Kind: EventNotice, Notice: &Notice{Level: acp.NoticeWarning, Text: "与 agent 的连接已断开，正在重新连接"}})
	a.reopen(link)
}

// reopen opens the agent session again on a new process after old (if
// any) is gone, reconnecting after a crash. Prompts queue meanwhile.
func (a *actor) reopen(old *acpLink) {
	a.opening = true
	a.epoch++
	epoch, sid, cwd := a.epoch, a.agentSessionID, a.agentCwd
	a.dirty = true
	a.subtask(func() {
		var res openResult
		oldGone := true
		if old != nil && old.stop() != nil {
			oldGone = false
			res.fail = &failure{err: fmt.Errorf("%w: 原 agent 进程未能结束", ErrAgent), public: "原 agent 进程未能结束"}
		} else if a.lifetime().Err() != nil {
			res.fail = &failure{err: errStopped, public: errStopped.Error()}
		} else {
			res = a.openACP(epoch, sid, cwd, true, func() bool { return a.sync(a.restartLog) })
		}
		if !a.sync(func() { a.reopened(res, oldGone) }) && res.link != nil {
			_ = res.link.stop()
		}
	})
}

// reopened applies a finished reconnect. A failed reconnect interrupts
// the session, keeping the old process's identity unless it was confirmed
// gone (oldGone).
func (a *actor) reopened(res openResult, oldGone bool) {
	a.opening = false
	a.dirty = true
	if res.fail != nil {
		a.emit(Event{Kind: EventNotice, Notice: &Notice{Level: acp.NoticeError, Text: res.fail.public}})
		a.noteConnector(res.fail.err)
		a.interrupt("idle", oldGone)
		a.afterTransition()
		return
	}
	a.installLink(res.link, res.init, res.config, res.notes)
	a.emit(Event{Kind: EventMarker, Marker: &Marker{Kind: MarkerReconnected, Side: SideACP}})
	pid := res.link.sidecar.PID()
	process := processIdentity(pid)
	a.record(true, func(r *Record) { r.PID, r.Process = pid, process })
	a.svc.log.Info("hybrid acp reopened", "id", a.id, "resumed", res.resumed)
	a.checkLink()
	a.drain()
	a.afterTransition()
}

// afterTransition runs a close that waited for a start or reconnect to
// finish.
func (a *actor) afterTransition() {
	if a.closeAfterStart && !a.starting && !a.opening {
		a.closeAfterStart = false
		a.beginClose()
	}
}
