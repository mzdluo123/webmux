package hybrid

import (
	"context"
	"github.com/rainchan/webmux/internal/acp"
	"strconv"
	"time"
)

func (a *actor) sendPrompt(text, wireText string, attachments []Attachment, queueItem, titleSource string, check func(string) (string, error), refresh ...bool) (PromptResult, error) {

	backgroundRefresh := len(refresh) > 0 && refresh[0]
	blocks, notes, err := promptBlocks(wireText, attachments, a.chat.PromptCapabilities, check)
	if err != nil {
		return PromptResult{}, err
	}

	a.nextTurn++
	if a.promptTitle {
		a.promptTitle = false
		if titleSource == "" {
			titleSource = text
		}
		if title := PromptTitle(titleSource); title != "" && a.title == "" {
			a.title, a.titleSession = title, a.agentSessionID
		}
	}
	n := strconv.Itoa(a.nextTurn)
	result := PromptResult{TurnID: "t" + n, PromptID: "p" + n}
	ctx, cancel := context.WithCancel(context.Background())
	t := &turn{acknowledgement: make(chan error, 1), id: result.TurnID, cancel: cancel, done: make(chan struct{}), backgroundRefresh: backgroundRefresh}
	a.turn = t
	a.snapshotProcesses()
	a.work.endTurn()
	startedAt := time.Now().UTC()
	a.chat.Turn = Turn{Running: true, TurnID: t.id, StartedAt: &startedAt}
	a.dirty = true
	a.clearAttention()
	a.emit(Event{Kind: EventTurnStart, TurnStart: &TurnStart{TurnID: result.TurnID, PromptID: result.PromptID, QueueItemID: queueItem}}, a.tr.userMessage(text, attachments))
	a.tr.breakMessage()
	for _, note := range notes {
		a.emit(Event{Kind: EventNotice, Notice: &Notice{Level: acp.NoticeWarning, Text: note}})
	}
	conn, sid := a.link.conn, a.link.sessionID
	go func() {
		defer close(t.done)
		resp, err := conn.Prompt(ctx, acp.PromptRequest{SessionID: sid, Prompt: blocks})
		t.acknowledge(err)
		a.post(func() { a.turnEnded(t, resp, err) })
	}()
	return result, nil
}

func (t *turn) acknowledge(err error) {
	t.acknowledgementOnce.Do(func() {
		if t.acknowledgement != nil {
			t.acknowledgement <- err
		}
	})
}
