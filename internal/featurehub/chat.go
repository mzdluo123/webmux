package featurehub

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"regexp"
	"strconv"

	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/featurewire"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/hybrid"
)

var generationPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// ChatResume reads the chat socket's resume point: ?generation= of the log
// the client holds and seq= of the last event it applied (0 for none).
// Malformed values are dropped, never rejected: the client then gets the
// whole cached log.
func ChatResume(q url.Values) *hybrid.ResumePoint {
	generation := q.Get("generation")
	seq, err := strconv.ParseUint(q.Get("seq"), 10, 64)
	if err != nil || seq == math.MaxUint64 || !generationPattern.MatchString(generation) {
		return nil
	}
	return &hybrid.ResumePoint{Generation: generation, Seq: seq + 1}
}

// Chat is the only writer of a chat connection: the snapshot, the events
// from the resume point, then live events and state. A subscription the
// session dropped closes the connection (4408 when it fell behind); ending
// the connection only releases the subscription.
func Chat(ctx context.Context, sub *hybrid.Subscription, c hub.MessageConn) {
	ChatWithOptions(ctx, sub, c, ChatOptions{})
}

// ChatWithOptions behaves like Chat and, when opts.TailBackfill is set and
// the backlog exceeds the tail budget, sends the newest tail first followed
// by earlier chunks as backfill frames interleaved with live updates.
func ChatWithOptions(ctx context.Context, sub *hybrid.Subscription, c hub.MessageConn, opts ChatOptions) {
	defer sub.Close()
	defer func() { _ = c.CloseNow() }()
	writeChatStream(ctx, sub.Snapshot, sub.Events, sub.Out, sub.Done, sub.Err, c, opts)
}

func writeChatStream(
	ctx context.Context,
	snapshot hybrid.Snapshot,
	backlog []hybrid.Event,
	out <-chan hybrid.Frame,
	done <-chan struct{},
	subErr func() error,
	c hub.MessageConn,
	opts ChatOptions,
) {
	ctx = c.CloseRead(ctx)
	if !writeChat(ctx, c, wire.ChatSnapshot(snapshot)) {
		return
	}
	var backfill [][]hybrid.Event
	if opts.TailBackfill {
		var tail []hybrid.Event
		tail, backfill = splitBacklogTailFirst(backlog, tailBackfillBudget, tailBackfillMinEvents)
		if !writeChat(ctx, c, wire.ChatTailEvents(snapshot.Generation, tail, len(backfill) > 0)) {
			return
		}
	} else {
		if !writeChat(ctx, c, wire.ChatEvents(snapshot.Generation, backlog)) {
			return
		}
	}
	for {
		// A dropped subscription ends the connection before its queued
		// frames: the client resumes from what it has.
		select {
		case <-done:
			closeChat(c, subErr())
			return
		default:
		}
		if len(backfill) > 0 {
			// Prioritize live updates so streaming never waits behind history.
			select {
			case <-ctx.Done():
				return
			case <-done:
				closeChat(c, subErr())
				return
			case f := <-out:
				if !writeChat(ctx, c, wire.ChatFrame(f)) {
					return
				}
			default:
				chunk := backfill[0]
				backfill = backfill[1:]
				if !writeChat(ctx, c, wire.ChatBackfill(snapshot.Generation, chunk, len(backfill) > 0)) {
					return
				}
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-done:
			closeChat(c, subErr())
			return
		case f := <-out:
			if !writeChat(ctx, c, wire.ChatFrame(f)) {
				return
			}
		}
	}
}

func writeChat(ctx context.Context, c hub.MessageConn, f contract.ChatFrame) bool {
	data, err := json.Marshal(f)
	if err != nil {
		_ = c.Close(hub.StatusInternalError, "invalid chat frame")
		return false
	}
	if hub.Write(ctx, c, hub.MessageText, data) != nil {
		_ = c.Close(hub.StatusSlowClient, "write timeout")
		return false
	}
	return true
}

// closeChat closes a connection whose subscription the session ended.
func closeChat(c hub.MessageConn, reason error) {
	switch {
	case errors.Is(reason, hybrid.ErrSlow):
		_ = c.Close(hub.StatusSlowClient, "slow client")
	case errors.Is(reason, hybrid.ErrReset):
		_ = c.Close(hub.StatusTryAgainLater, "chat log reset; reconnect")
	default:
		_ = c.Close(hub.StatusSessionExited, "session ended")
	}
}
