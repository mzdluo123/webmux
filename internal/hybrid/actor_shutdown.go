package hybrid

import (
	"context"
	"sync"
	"time"
)

func (a *actor) lifetime() context.Context {
	if a.lifeCtx != nil {
		return a.lifeCtx
	}
	return context.Background()
}

func (a *actor) closeWait() time.Duration {
	if a.svc != nil && a.svc.timing.close > 0 {
		return a.svc.timing.close
	}
	return closeTimeout
}

func (a *actor) subtask(fn func()) {
	a.subtasks.Add(1)
	go func() {
		defer a.subtasks.Done()
		fn()
	}()
}

// shellGone ends the actor after its shell was removed or retired by
// someone else; the record is left for recovery.
func (a *actor) shellGone() {
	if a.stopped {
		return
	}
	if a.lifeCancel != nil {
		a.lifeCancel()
	}
	a.withdrawAll()
	if a.turn != nil {
		a.turn.cancel()
	}
	if link := a.link; link != nil {
		a.link = nil
		a.subtask(func() { _ = link.stop() })
	}
	a.end()
}

// shutdown stops the ACP process for a server shutdown, killing it when
// ctx ends first. Active sessions that a user already asked to close are
// persisted as StateClosed immediately while retaining unconfirmed process
// identity for restart cleanup; other active sessions record their current
// activity and lost queue count for restart recovery.
func (a *actor) shutdown(ctx context.Context) {
	if a.stopped {
		return
	}
	if a.state == StateActive {
		if a.closing || a.closeAfterStart {
			a.state = StateClosed
			a.saveShellInfo(a.shell.Info())
			a.record(true, func(r *Record) {
				r.State = StateClosed
				r.Activity, r.InterruptedAt, r.QueuedCount, r.ResumeOnOpen = "", nil, 0, false
			})
		} else {
			activity := "idle"
			switch {
			case a.switching != nil:
				activity = "switching"
			case a.turn != nil, a.side == SideTUI && tuiBusy(a.shell.Info()):
				activity = "turn"
			}
			lost := len(a.chat.Queue.Items)
			a.saveShellInfo(a.shell.Info())
			a.record(false, func(r *Record) {
				if r.State == StateActive {
					r.Activity, r.QueuedCount = activity, lost
				}
			})
		}
	}
	if a.lifeCancel != nil {
		a.lifeCancel()
	}
	a.stopTimers()
	a.withdrawAll()
	if a.turn != nil {
		a.turn.cancel()
	}
	if link := a.link; link != nil {
		a.link = nil
		a.subtask(func() { _ = link.stopWithin(ctx) })
	}
	a.end()
}

func waitGroupWithin(ctx context.Context, wg *sync.WaitGroup) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		select {
		case <-done:
			return true
		default:
			return false
		}
	}
}
