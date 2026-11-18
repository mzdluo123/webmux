package hybrid

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/acp"
	"github.com/rainchan/webmux/internal/agent/omp"
)

func TestPromptAcknowledgementOutlivesDispatchDeadline(t *testing.T) {
	h := newHarness(t, 0)
	delay := commandTimeout + 250*time.Millisecond
	s := h.create(h.input(omp.Name, SideACP, "FAKEAGENT_DELAY=session/prompt="+delay.String(), "FAKEAGENT_TURN_DELAY=300ms"))
	id := s.Info().ID
	st := h.follow(id)
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout+testTimeout)
	defer cancel()
	in := PromptInput{ClientRequestID: rid(), Text: "slow receipt", Mode: ModeRefuse, AwaitAcknowledgement: true}
	type outcome struct {
		result PromptResult
		err    error
	}
	out := make(chan outcome, 2)
	for range 2 {
		go func() { result, err := h.svc.Prompt(ctx, id, in); out <- outcome{result, err} }()
	}
	first, second := <-out, <-out
	if first.err != nil || second.err != nil || first.result.TurnID == "" || first.result != second.result {
		t.Fatalf("deduplicated receipt: %+v %+v", first, second)
	}
	sub, err := h.svc.Subscribe(id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sub.Snapshot.State.Turn.Running {
		t.Error("receipt waited for turn completion")
	}
	sub.Close()
	st.until("turn end", hasKind(EventTurnEnd))
	blocks := userBlocks(t, h.store)
	if len(blocks) != 1 || blocks[0].Text != in.Text {
		t.Fatalf("duplicate dispatch: %+v", blocks)
	}
}

func TestCancelledReceiptObserverCanRejoinWithoutResending(t *testing.T) {
	h := newHarness(t, 0)
	s := h.create(h.input(omp.Name, SideACP, "FAKEAGENT_DELAY=session/prompt=500ms"))
	id := s.Info().ID
	in := PromptInput{ClientRequestID: rid(), Text: "one dispatch", Mode: ModeRefuse, AwaitAcknowledgement: true}
	// Complete dispatch first, then cancel only its receipt observer.
	dispatch := in
	dispatch.AwaitAcknowledgement = false
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	result, err := h.svc.Prompt(ctx, id, dispatch)
	if err != nil {
		t.Fatal(err)
	}
	stopped, stop := context.WithCancel(ctx)
	stop()
	if _, err := h.svc.Prompt(stopped, id, in); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled observation: %v", err)
	}
	confirmed, err := h.svc.Prompt(ctx, id, in)
	if err != nil || confirmed != result {
		t.Fatalf("rejoin: %+v %v", confirmed, err)
	}
	h.follow(id).until("turn end", hasKind(EventTurnEnd))
	if blocks := userBlocks(t, h.store); len(blocks) != 1 || blocks[0].Text != in.Text {
		t.Fatalf("replayed prompt: %+v", blocks)
	}
}

func TestDisconnectBeforeReceiptRemainsUnconfirmed(t *testing.T) {
	h := newHarness(t, 0)
	s := h.create(h.input(omp.Name, SideACP, "FAKEAGENT_CRASH=prompt-saved"))
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	_, err := h.svc.Prompt(ctx, s.Info().ID, PromptInput{ClientRequestID: rid(), Text: "uncertain delivery", Mode: ModeRefuse, AwaitAcknowledgement: true})
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lost connection receipt: %v", err)
	}
	if blocks := userBlocks(t, h.store); len(blocks) != 1 || blocks[0].Text != "uncertain delivery" {
		t.Fatalf("dispatch evidence: %+v", blocks)
	}
}

func TestOnlyLiveTurnActivityAcknowledgesPrompt(t *testing.T) {
	for _, update := range []acp.Update{&acp.AgentThoughtChunk{}, &acp.ToolCall{}, &acp.ToolCallUpdate{}, &acp.Plan{}, &acp.AgentMessageChunk{}} {
		t.Run(fmt.Sprintf("%T", update), func(t *testing.T) {
			receipt := &promptAcknowledgement{done: make(chan struct{})}
			a := &actor{turn: &turn{acknowledgement: receipt}}
			a.observeControlUpdate(&acp.ConfigOptionUpdate{})
			a.beginControlReplay()
			a.observeControlUpdate(update)
			select {
			case <-receipt.done:
				t.Fatal("metadata or replay confirmed delivery")
			default:
			}
			a.control.replay = false
			a.observeControlUpdate(update)
			select {
			case <-receipt.done:
				if receipt.err != nil {
					t.Fatal(receipt.err)
				}
			default:
				t.Fatal("live turn activity did not confirm delivery")
			}
		})
	}
}
