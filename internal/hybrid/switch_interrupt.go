package hybrid

import (
	"context"
	"fmt"
	"time"

	"github.com/rainchan/webmux/internal/attentionkind"
	"github.com/rainchan/webmux/internal/session"
)

// tuiBusy reports whether the terminal UI is in a turn by its signals:
// running, or waiting on the user inside a turn (not a latched done or
// error).
func tuiBusy(info session.Info) bool {
	switch info.Status {
	case session.StatusRunning:
		return true
	case session.StatusWaiting:
		return info.Attention == nil || (info.Attention.Kind != attentionkind.Done && info.Attention.Kind != attentionkind.Error)
	}
	return false
}

// interruptKeyGap separates key presses so the terminal UI reads each one
// separately; the M0 baseline stopped OpenCode with Esc, 150 ms, Esc.
const interruptKeyGap = 150 * time.Millisecond

// interruptKey writes the profile's interrupt key presses to the terminal UI.
func (a *actor) interruptKey() {
	life := a.lifetime()
	for i, key := range a.cmd.Agent.InterruptKeys() {
		if i > 0 {
			timer := time.NewTimer(interruptKeyGap)
			select {
			case <-timer.C:
			case <-life.Done():
				timer.Stop()
				return
			}
		}
		ctx, cancel := context.WithTimeout(life, keyTimeout)
		err := a.shell.WriteContext(ctx, key)
		cancel()
		if err != nil {
			a.svc.log.Info("hybrid interrupt key not written", "id", a.id, "error", err)
			return
		}
	}
}

// waitIdle waits up to d for the terminal UI to leave its turn.
func (a *actor) waitIdle(d time.Duration) bool {
	life := a.lifetime()
	deadline := time.NewTimer(d)
	defer deadline.Stop()
	tick := time.NewTicker(idlePoll)
	defer tick.Stop()
	for {
		if !tuiBusy(a.shell.Info()) {
			return true
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			return !tuiBusy(a.shell.Info())
		case <-life.Done():
			return false
		case <-a.quit:
			return false
		}
	}
}

// interruptOutcome is how interrupting the terminal UI ended.
type interruptOutcome int

const (
	interruptIdle interruptOutcome = iota
	interruptCancelled
	interruptGone
)

// interruptTUI sends the interrupt key and waits for the terminal UI to go
// idle. After timing.interrupt it asks the user (D21): wait another
// period, force the switch, or cancel it. The terminal going idle while
// the question is open continues the switch.
func (a *actor) interruptTUI(op *switchOp) interruptOutcome {
	a.interruptKey()
	for {
		if a.waitIdle(a.svc.timing.interrupt) {
			return interruptIdle
		}
		choice := make(chan string, 1)
		if !a.sync(func() { op.choice = choice; a.setPhase(PhaseAwaitingChoice) }) {
			return interruptGone
		}
		c := a.awaitChoice(op, choice)
		if !a.sync(func() {
			op.choice = nil
			if c == ChoiceWait {
				a.setPhase(PhaseInterrupting)
			}
		}) {
			return interruptGone
		}
		switch c {
		case ChoiceWait:
			continue
		case ChoiceCancel:
			return interruptCancelled
		case "":
			return interruptGone
		}
		return interruptIdle
	}
}

// awaitChoice waits for the user's choice; "idle" when the terminal UI
// went idle first, "" when the actor ended. The terminal going idle wins
// only while the question is still open: deciding so closes it on the
// actor, so a choice is either acted on or refused, never lost.
func (a *actor) awaitChoice(op *switchOp, choice chan string) string {
	life := a.lifetime()
	tick := time.NewTicker(idlePoll)
	defer tick.Stop()
	for {
		select {
		case c := <-choice:
			return c
		case <-tick.C:
			if tuiBusy(a.shell.Info()) {
				continue
			}
			idle := false
			if !a.sync(func() {
				if op.choice == choice {
					op.choice, idle = nil, true
				}
			}) {
				return ""
			}
			if idle {
				return "idle"
			}
			// choose answered first and closed the question.
			return <-choice
		case <-life.Done():
			return ""
		case <-a.quit:
			return ""
		}
	}
}

// choose answers a switch waiting in PhaseAwaitingChoice.
func (a *actor) choose(choice string) (SwitchResult, error) {
	op := a.switching
	if op == nil || op.choice == nil {
		return SwitchResult{}, fmt.Errorf("%w: 没有等待选择的切换", ErrConflict)
	}
	op.choice <- choice
	op.choice = nil
	if choice == ChoiceWait {
		// The question is closed now; no device may answer it again.
		a.setPhase(PhaseInterrupting)
	}
	if choice == ChoiceCancel {
		return a.resultOn(SideTUI), nil
	}
	// The switch goes on; the result names where it is heading.
	return SwitchResult{Side: op.to, Dropped: []string{}, Unsupported: []string{}}, nil
}
