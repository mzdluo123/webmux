package hybrid

import (
	"strings"

	"github.com/rainchan/webmux/internal/acp"
)

type replayUserBuffer struct {
	text                  strings.Builder
	messageID             string
	role                  string
	buffered, passthrough bool
	epoch                 int
}

// onUpdate holds at most one prompt-sized replayed user message. Verification
// happens before either durable capture or the bounded UI translator sees it.
func (a *actor) onUpdate(epoch int, n acp.SessionNotification) {
	if epoch != a.epoch || a.stopped {
		return
	}
	if a.control.replay {
		role, id, text, ok := replayTextChunk(n.Update)
		if ok && role == "user" {
			b := &a.replayUser
			if b.buffered && (b.messageID != id || b.role != role) {
				a.flushReplayUser()
			}
			if b.passthrough && b.messageID == id && b.role == role {
				a.onUpdateDirect(epoch, n)
				return
			}
			if b.messageID != id || b.role != role {
				b.passthrough = false
			}
			b.messageID, b.role, b.epoch = id, role, epoch
			if b.text.Len()+len(text) > MaxPromptBytes {
				a.flushReplayUser()
				b.passthrough = true
				b.messageID, b.role = id, role
				a.onUpdateDirect(epoch, n)
				return
			}
			b.buffered = true
			b.text.WriteString(text)
			return
		}
		// Session metadata can be interleaved between chunks of one message.
		// It is not evidence that the buffered replay message has ended.
		switch n.Update.(type) {
		case *acp.UsageUpdate, *acp.ConfigOptionUpdate, *acp.CurrentModeUpdate,
			*acp.AvailableCommandsUpdate, *acp.SessionInfoUpdate:
			a.onUpdateDirect(epoch, n)
			return
		}
	}
	a.flushReplayUser()
	if u, ok := n.Update.(*acp.UserMessageChunk); ok && a.control.replay && a.replayOwnedMessage && u.MessageID == a.replayOwnedMessageID {
		return
	}
	a.replayOwnedMessage = false
	a.replayUser.passthrough = false
	a.onUpdateDirect(epoch, n)
}

func (a *actor) flushReplayUser() {
	b := &a.replayUser
	if !b.buffered {
		return
	}
	text := b.text.String()

	var attachments []Attachment

	n := acp.SessionNotification{SessionID: a.agentSessionID, Update: &acp.UserMessageChunk{Content: acp.TextBlock(text), MessageID: b.messageID}}
	if b.role == "assistant" {
		n.Update = &acp.AgentMessageChunk{Content: acp.TextBlock(text), MessageID: b.messageID}
	}
	epoch := b.epoch
	*b = replayUserBuffer{}
	a.onUpdateDirect(epoch, n, attachments...)
}

func replayTextChunk(update acp.Update) (role, id, text string, ok bool) {
	var content acp.ContentBlock
	switch u := update.(type) {
	case *acp.UserMessageChunk:
		role, id, content = "user", u.MessageID, u.Content
	case *acp.AgentMessageChunk:
		role, id, content = "assistant", u.MessageID, u.Content
	default:
		return "", "", "", false
	}
	return role, id, content.Text, content.Type == acp.ContentText
}
