package handoff

import (
	"context"
	"errors"
	"testing"

	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/hybridresume"
)

func TestDirectoryHandoffRecoversDurableReservationWithoutWorkspace(t *testing.T) {
	h := deliveryTestWithPlugins(t, nil, nil, true)
	ctx := context.Background()
	if h.workspace != nil || h.service.options.Locations != nil {
		t.Fatal("directory chat constructed workspace")
	}
	h.service.pauseAfterPhase = func(phase string) bool { return phase == "session_created" }
	p, in := h.preview(conversation.PreviewLocation{Cwd: h.root})
	first, err := h.service.Deliver(ctx, h.source.ConversationID, in)
	if err != nil {
		t.Fatal(err)
	}
	h.service.workers.Wait()
	pending := h.waitStatus(p.ID, "pending")
	record, err := h.hybrids.Record(pending.RuntimeID)
	if err != nil || record.Cwd != h.root {
		t.Fatalf("directory reservation: %+v %v", record, err)
	}
	h.close()
	h.open()
	again, err := h.service.Deliver(ctx, h.source.ConversationID, in)
	if err != nil || again.ID != first.ID || again.RuntimeID != pending.RuntimeID {
		t.Fatalf("receipt recovery: %+v %v", again, err)
	}
	delivered := h.waitStatus(p.ID, "inline")
	waitIdle(t, h.hybrids, delivered.RuntimeID)
	if h.promptCalls() != 1 {
		t.Fatalf("recovery sent %d prompts", h.promptCalls())
	}
	h.close()
	h.open()
	restored, err := h.recovery.Resume(ctx, delivered.RuntimeID, hybridresume.Input{ClientRequestID: "directory-restart-resume", Side: "acp"})
	if err != nil || restored.Info().Cwd != h.root {
		t.Fatalf("directory resume: %v %v", restored, err)
	}
	replay, err := h.service.Deliver(ctx, h.source.ConversationID, in)
	if err != nil || replay.ID != delivered.ID || h.promptCalls() != 1 {
		t.Fatalf("resumed receipt replay: %+v %v", replay, err)
	}
}

func TestDisabledWorkspaceRejectsGitSelectionBeforeReceipt(t *testing.T) {
	h := deliveryTestWithPlugins(t, nil, nil, true)
	ctx := context.Background()
	for _, location := range []conversation.PreviewLocation{
		{RepositoryID: "repo", WorktreeID: "tree"},
		{RepositoryID: "repo", NewWorktreeBranch: "new", NewWorktreeBase: "main"},
	} {
		request := Request{ClientRequestID: "disabled-location-preview", ConfigID: h.config.ID, Fork: true, UserText: "continue", Location: location}
		if _, err := h.service.Preview(ctx, h.source.ConversationID, request); !errors.Is(err, conversation.ErrUnavailable) {
			t.Fatalf("Git preview admitted: %v", err)
		}
		if _, err := h.store.Receipt(ctx, h.source.ConversationID, request.ClientRequestID); !errors.Is(err, conversation.ErrNotFound) {
			t.Fatalf("rejected preview wrote receipt: %v", err)
		}
		message := MessageRequest{ClientRequestID: "disabled-location-delivery", ConfigID: h.config.ID, Text: "continue", Location: location}
		if _, err := h.service.Deliver(ctx, h.source.ConversationID, message); !errors.Is(err, conversation.ErrUnavailable) {
			t.Fatalf("Git delivery admitted: %v", err)
		}
		if _, err := h.store.Receipt(ctx, h.source.ConversationID, message.ClientRequestID); !errors.Is(err, conversation.ErrNotFound) {
			t.Fatalf("rejected delivery wrote receipt: %v", err)
		}
	}
}
