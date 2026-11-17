package hybrid

import "github.com/rainchan/webmux/internal/acp"

type controlRecorder struct{ replay bool }

func (a *actor) observeControlUpdate(update acp.Update) {
	if a.control.replay || a.turn == nil {
		return
	}
	switch update.(type) {
	case *acp.AgentMessageChunk, *acp.AgentThoughtChunk, *acp.ToolCall, *acp.ToolCallUpdate, *acp.Plan:
		a.turn.acknowledge(nil)
	}
}
func (a *actor) beginControlReplay()  { a.control.replay = true }
func (a *actor) finishControlReplay() { a.flushReplayUser(); a.control.replay = false }
