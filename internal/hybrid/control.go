package hybrid

import (
	"context"
	"fmt"

	"github.com/rainchan/webmux/internal/acp"
)

// controlRecorder owns identity, foreground runs and replay receipts only.
// ACP bodies remain in the bounded UI log and the agent's native history.
type controlRecorder struct {
	thread, run, completedRun string
	replay                    bool
	replayReceipts            map[string]bool
}

func (a *actor) controlFailure() {
	a.svc.log.Warn("conversation control persistence failed", "id", a.id)
	if a.svc.conversations != nil && a.control.thread != "" {
		if err := a.svc.conversations.AddGap(context.Background(), a.control.thread, "storage_failure"); err != nil {
			a.svc.log.Warn("conversation gap could not be persisted", "id", a.id)
		}
	}
}

func (a *actor) bindControlThread() error {
	if a.svc.conversations == nil {
		return nil
	}
	binding, err := a.svc.conversations.Binding(context.Background(), a.id)
	if err != nil {
		return fmt.Errorf("%w: durable identity unavailable", ErrUnavailable)
	}
	a.control.thread = binding.ProviderThreadID
	a.conversationID = binding.ConversationID
	return nil
}

func (a *actor) beginControlRun(promptID string) error {
	if a.svc.conversations == nil {
		return nil
	}
	if err := a.bindControlThread(); err != nil {
		return err
	}
	run, err := a.svc.conversations.StartRun(context.Background(), a.control.thread, promptID)
	if err != nil {
		return fmt.Errorf("%w: durable run unavailable", ErrUnavailable)
	}
	a.control.run = run.ID
	a.control.replay = false
	return nil
}

func (a *actor) observeControlUpdate(update acp.Update) {
	if a.svc.conversations == nil {
		return
	}
	switch update := update.(type) {
	case *acp.UsageUpdate:
		if err := a.bindControlThread(); err != nil {
			a.controlFailure()
			return
		}
		if err := a.svc.conversations.ObserveUsage(context.Background(), a.control.thread, a.agentSessionID, update.Used, update.Size); err != nil {
			a.svc.log.Warn("conversation context usage unavailable", "id", a.id)
		}
	case *acp.AgentMessageChunk:
		if !a.control.replay {
			a.acknowledgeCurrentRun()
			if a.turn != nil && a.turn.executionID != "" {
				a.finishContextExecution(a.turn, "inline", "")
			}
		}
	}
}

func (a *actor) beginControlReplay() {
	if a.svc.conversations == nil {
		return
	}
	if err := a.bindControlThread(); err != nil {
		a.controlFailure()
		return
	}
	a.control.replay = true
	a.control.replayReceipts = make(map[string]bool)
	a.startNativeReplay()
}

func (a *actor) finishControlReplay() {
	a.flushReplayUser()
	if a.svc.conversations == nil || !a.control.replay {
		return
	}
	if a.replayNative.current.DeliveryID != "" {
		a.nativeReplayGap()
	}
	a.control.replay = false
}

func (a *actor) finishControlRun(stop string) {
	a.control.completedRun = a.control.run
	if a.svc.conversations == nil || a.control.run == "" || a.control.replay {
		return
	}
	if err := a.svc.conversations.FinishRun(context.Background(), a.control.run, stop); err != nil {
		a.controlFailure()
	}
	a.control.run = ""
}
