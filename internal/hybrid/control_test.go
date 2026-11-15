package hybrid

import (
	"context"
	"testing"

	"github.com/rainchan/webmux/internal/acp"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historysource"
)

func historyHarness(t *testing.T) (*harness, *conversation.Service) {
	t.Helper()
	h := newHarness(t, 1024)
	db := conversation.Open(conversation.Options{StateDir: h.state})
	if err := db.Unavailable(); err != nil {
		t.Fatal(err)
	}
	h.svc.conversations = db
	t.Cleanup(func() {
		h.svc.Close()
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return h, db
}

func TestNoIDReplayKeepsLaterLegitimateCopyOfHistoricalPacket(t *testing.T) {
	ctx := context.Background()
	h, db := historyHarness(t)
	id := h.create(h.input("omp", SideACP)).Info().ID
	binding, err := db.Binding(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	thread, err := db.ProviderThread(ctx, binding.ProviderThreadID)
	if err != nil {
		t.Fatal(err)
	}
	// This scenario has no historical bodies. Only an explicit immutable scope
	// and its dispatch receipt may authorize normalization of the first packet.
	snapshot, err := db.SaveFrozen(ctx, historysource.SnapshotData{Snapshot: historysource.Snapshot{ID: conversation.TextHash(t.Name()), ContentHash: conversation.TextHash(""), Source: historysource.Source{Agent: "metadata-test"}, Records: []historysource.RecordRef{}}}, binding.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := db.ReservePreview(ctx, conversation.PreviewReservation{SourceConversationID: binding.ConversationID, ClientRequestID: "packet-preview", RequestHash: "preview", Request: conversation.PreviewRequest{ConfigID: thread.ConfigID, AgentName: thread.AgentName, TargetProviderThreadID: thread.ID, UserText: "original"}, Snapshots: []historysource.Snapshot{snapshot}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err = db.CompletePreview(ctx, preview.ID, conversation.PreviewResult{Budget: conversation.PreviewBudget{History: 16000}})
	if err != nil {
		t.Fatal(err)
	}
	execution, err := db.BeginExecution(ctx, conversation.ExecutionInput{SourceConversationID: binding.ConversationID, HandoffID: preview.ID, ClientRequestID: "packet-command", RequestHash: "command", Revision: preview.Revision})
	if err != nil {
		t.Fatal(err)
	}
	packet := "random-scoped historical envelope\nUser message:\noriginal"
	execution.Phase = "sending"
	execution.NativeThreadID = thread.AgentSessionID
	execution.PromptHash = conversation.TextHash(packet)
	if err := db.CheckpointExecution(ctx, execution); err != nil {
		t.Fatal(err)
	}
	if err := db.ClaimExecutionText(ctx, execution.ID, execution.PromptHash, execution.PromptHash); err != nil {
		t.Fatal(err)
	}
	actor, err := h.svc.actor(id)
	if err != nil {
		t.Fatal(err)
	}
	if !actor.sync(func() {
		actor.beginControlReplay()
		for _, update := range []acp.Update{&acp.UserMessageChunk{Content: acp.TextBlock(packet)}, &acp.AgentMessageChunk{Content: acp.TextBlock("original reply")}, &acp.UserMessageChunk{Content: acp.TextBlock(packet)}, &acp.AgentMessageChunk{Content: acp.TextBlock("quoted packet reply")}} {
			actor.onUpdate(actor.epoch, acp.SessionNotification{Update: update})
		}
		actor.finishControlReplay()
	}) {
		t.Fatal("actor stopped")
	}
	users := h.replayedUsers(id)
	if len(users) != 2 || users[0] != "original" || users[1] != packet {
		t.Fatal("legitimate no-ID duplicate was stripped", users)
	}
}
