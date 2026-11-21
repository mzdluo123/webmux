package featurehub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/hybrid"
)

func TestChatBackfillTailQuery(t *testing.T) {
	if !ChatBackfillTail(url.Values{"backfill": {"tail"}}) {
		t.Fatal("expected backfill=tail to opt in")
	}
	if ChatBackfillTail(url.Values{}) || ChatBackfillTail(url.Values{"backfill": {"head"}}) {
		t.Fatal("unexpected opt-in")
	}
}

func TestSplitBacklogTailFirst(t *testing.T) {
	now := time.Now().UTC()
	var events []hybrid.Event
	seq := uint64(1)
	for turn := range 20 {
		events = append(events, hybrid.Event{
			Seq:       seq,
			Kind:      hybrid.EventTurnStart,
			At:        now,
			TurnStart: &hybrid.TurnStart{TurnID: fmt.Sprintf("t%d", turn), PromptID: fmt.Sprintf("p%d", turn)},
		})
		seq++
		events = append(events, hybrid.Event{
			Seq:  seq,
			Kind: hybrid.EventMessage,
			At:   now,
			Message: &hybrid.MessageChunk{
				MessageID: fmt.Sprintf("m%d", turn),
				Role:      hybrid.RoleAgent,
				Text:      strings.Repeat("x", 600),
			},
		})
		seq++
		events = append(events, hybrid.Event{
			Seq:     seq,
			Kind:    hybrid.EventTurnEnd,
			At:      now,
			TurnEnd: &hybrid.TurnEnd{TurnID: fmt.Sprintf("t%d", turn), StopReason: "end_turn"},
		})
		seq++
	}

	// Below minEvents or under budget does not split.
	if tail, backfill := splitBacklogTailFirst(events[:10], 1024, 32); len(backfill) != 0 || len(tail) != 10 {
		t.Fatalf("small backlog split unexpectedly: tail=%d backfill=%d", len(tail), len(backfill))
	}
	if tail, backfill := splitBacklogTailFirst(events, 1<<20, 32); len(backfill) != 0 || len(tail) != len(events) {
		t.Fatalf("under-budget backlog split unexpectedly: tail=%d backfill=%d", len(tail), len(backfill))
	}

	tail, backfill := splitBacklogTailFirst(events, 2500, 10)
	if len(backfill) == 0 {
		t.Fatal("expected backfill chunks for large backlog")
	}
	if tail[0].Kind != hybrid.EventTurnStart {
		t.Fatalf("tail did not snap to turn.start: got %s", tail[0].Kind)
	}
	// Concatenating oldest..newest backfill chunks + tail reconstructs events losslessly.
	var rebuilt []hybrid.Event
	for i := len(backfill) - 1; i >= 0; i-- {
		if backfill[i][0].Kind != hybrid.EventTurnStart {
			t.Fatalf("chunk %d did not snap to turn.start: got %s", i, backfill[i][0].Kind)
		}
		rebuilt = append(rebuilt, backfill[i]...)
	}
	rebuilt = append(rebuilt, tail...)
	if len(rebuilt) != len(events) {
		t.Fatalf("rebuilt %d events, want %d", len(rebuilt), len(events))
	}
	for i := range events {
		if rebuilt[i].Seq != events[i].Seq {
			t.Fatalf("seq mismatch at %d: got %d want %d", i, rebuilt[i].Seq, events[i].Seq)
		}
	}
}

func TestChatWithOptionsTailBackfillAndLivePriority(t *testing.T) {
	now := time.Now().UTC()
	var events []hybrid.Event
	for i := range 60 {
		events = append(events, hybrid.Event{
			Seq:  uint64(i + 1),
			Kind: hybrid.EventMessage,
			At:   now,
			Message: &hybrid.MessageChunk{
				MessageID: fmt.Sprintf("m%d", i),
				Role:      hybrid.RoleAgent,
				Text:      strings.Repeat("a", 5000),
			},
		})
	}
	liveEvent := hybrid.Event{
		Seq:  61,
		Kind: hybrid.EventMessage,
		At:   now,
		Message: &hybrid.MessageChunk{
			MessageID: "live",
			Role:      hybrid.RoleAgent,
			Text:      "live update",
		},
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	out := make(chan hybrid.Frame, 1)
	out <- hybrid.Frame{Generation: "0123456789abcdef", Events: []hybrid.Event{liveEvent}}
	done := make(chan struct{})
	var frames []contract.ChatFrame
	streamConn := &callbackConn{
		testConn: newTestConn(),
		onWrite: func(data []byte) {
			var f contract.ChatFrame
			if err := json.Unmarshal(data, &f); err != nil {
				t.Errorf("unmarshal frame: %v", err)
				return
			}
			frames = append(frames, f)
			if f.Type == contract.ChatFrameBackfill && !f.More {
				cancel()
			}
		},
	}
	writeChatStream(
		ctx,
		hybrid.Snapshot{Generation: "0123456789abcdef", From: 1, Next: 61},
		events,
		out,
		done,
		func() error { return nil },
		streamConn,
		ChatOptions{TailBackfill: true},
	)

	if len(frames) < 4 {
		t.Fatalf("frames = %d, want at least snapshot + tail + live + backfill", len(frames))
	}
	if frames[0].Type != contract.ChatFrameSnapshot {
		t.Fatalf("frame[0] = %s, want snapshot", frames[0].Type)
	}
	if frames[1].Type != contract.ChatFrameEvents || !frames[1].More || len(frames[1].Items) == 0 {
		t.Fatalf("frame[1] = %+v, want tail events with more=true", frames[1])
	}
	// Queued live event must preempt pending backfill chunks.
	if frames[2].Type != contract.ChatFrameEvents || frames[2].More || len(frames[2].Items) != 1 || frames[2].Items[0].Seq != 61 {
		t.Fatalf("frame[2] = %+v, want prioritized live event seq=61", frames[2])
	}
	// Remaining frames are backfill chunks ending with more=false; reconstructing gives 1..60.
	var seqs []uint64
	for i := len(frames) - 1; i >= 3; i-- {
		f := frames[i]
		if f.Type != contract.ChatFrameBackfill {
			t.Fatalf("frame[%d] = %s, want backfill", i, f.Type)
		}
		wantMore := i < len(frames)-1
		if f.More != wantMore {
			t.Fatalf("frame[%d].More = %v, want %v", i, f.More, wantMore)
		}
		for _, item := range f.Items {
			seqs = append(seqs, item.Seq)
		}
	}
	for _, item := range frames[1].Items {
		seqs = append(seqs, item.Seq)
	}
	wantSeqs := make([]uint64, 60)
	for i := range wantSeqs {
		wantSeqs[i] = uint64(i + 1)
	}
	if !slices.Equal(seqs, wantSeqs) {
		t.Fatalf("reconstructed seqs len=%d, want 60", len(seqs))
	}
}

type callbackConn struct {
	*testConn
	onWrite func([]byte)
}

func (c *callbackConn) Write(ctx context.Context, kind hub.MessageType, data []byte) error {
	if err := c.testConn.Write(ctx, kind, data); err != nil {
		return err
	}
	c.onWrite(data)
	return nil
}
