package hybrid

import "github.com/rainchan/webmux/internal/acp"

type controlRecorder struct{ replay bool }

func (a *actor) observeControlUpdate(update acp.Update) {
	if _, ok := update.(*acp.AgentMessageChunk); ok && !a.control.replay && a.turn != nil {
		a.turn.consumed = true
		a.turn.acknowledge(nil)
	}
}
func (a *actor) beginControlReplay()  { a.control.replay = true }
func (a *actor) finishControlReplay() { a.flushReplayUser(); a.control.replay = false }
