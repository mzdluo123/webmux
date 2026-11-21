package hybrid

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/acp"
	"github.com/rainchan/webmux/internal/session"
)

func (a *actor) sendPrompt(text, wireText string, attachments []Attachment, queueItem, titleSource string, check func(string) (string, error), refresh ...bool) (PromptResult, error) {

	backgroundRefresh := len(refresh) > 0 && refresh[0]
	blocks, notes, err := promptBlocks(wireText, attachments, a.chat.PromptCapabilities, check)
	if err != nil {
		return PromptResult{}, err
	}
	return a.sendBlocks(text, attachments, queueItem, titleSource, blocks, notes, backgroundRefresh)
}

func (a *actor) sendBlocks(text string, attachments []Attachment, queueItem, titleSource string, blocks []acp.ContentBlock, notes []string, backgroundRefresh bool) (PromptResult, error) {

	a.nextTurn++
	if a.promptTitle {
		a.promptTitle = false
		if titleSource == "" {
			titleSource = text
		}
		if title := session.AutomaticTitle(titleSource); title != "" && a.title == "" {
			a.title, a.titleSession = title, a.agentSessionID
		}
	}
	n := strconv.Itoa(a.nextTurn)
	result := PromptResult{TurnID: "t" + n, PromptID: "p" + n}
	ctx, cancel := context.WithCancel(context.Background())
	t := &turn{acknowledgement: &promptAcknowledgement{done: make(chan struct{})}, id: result.TurnID, cancel: cancel, done: make(chan struct{}), backgroundRefresh: backgroundRefresh}
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

// Acknowledgement belongs to the dispatched turn, outside the bounded command.
// Closing done publishes one immutable outcome to every deduplicated observer.
type promptAcknowledgement struct {
	done chan struct{}
	once sync.Once
	err  error
}

func (t *turn) acknowledge(err error) {
	if receipt := t.acknowledgement; receipt != nil {
		receipt.once.Do(func() {
			receipt.err = err
			close(receipt.done)
		})
	}
}
