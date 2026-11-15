package conversation

import (
	"context"
	"errors"
	"testing"
)

func TestAcknowledgementPreservesFailedAndCancelledRunFacts(t *testing.T) {
	for _, status := range []string{"failed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			s := openTest(t, t.TempDir())
			binding := bindTest(t, s, "runtime")
			rejected := runTest(t, s, binding, "rejected")
			if err := s.FinishRun(ctx, rejected.ID, status); err != nil {
				t.Fatal(err)
			}
			accepted := runTest(t, s, binding, "accepted")
			if err := s.FinishRun(ctx, accepted.ID, "end_turn"); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := s.AcknowledgeRun(ctx, accepted.ID); err != nil {
					t.Fatal(err)
				}
			}
			for _, run := range []Run{rejected, accepted} {
				var gotStatus string
				var consumed bool
				if err := s.db.QueryRow("SELECT status,consumed FROM run WHERE id=?", run.ID).Scan(&gotStatus, &consumed); err != nil || gotStatus != map[bool]string{true: "end_turn", false: status}[run.ID == accepted.ID] || consumed != (run.ID == accepted.ID) {
					t.Fatalf("acknowledgement rewrote another run: status=%s consumed=%t err=%v", gotStatus, consumed, err)
				}
			}
			thread, err := s.ProviderThread(ctx, binding.ProviderThreadID)
			if err != nil || thread.LastDeliveredRunOrdinal != accepted.Ordinal {
				t.Fatal("accepted control ordinal lost", thread, err)
			}
			if err := s.FinishRun(ctx, rejected.ID, "end_turn"); !errors.Is(err, ErrConflict) {
				t.Fatal("terminal result overwritten", err)
			}
		})
	}
}

func TestForgettingOneProviderRuntimeKeepsSharedConversation(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	a := bindTest(t, s, "runtime-a")
	p, err := s.ReservePreview(ctx, PreviewReservation{Snapshots: controlSnapshots(t, s, a.ConversationID), SourceConversationID: a.ConversationID, ClientRequestID: "preview-B", RequestHash: "B", Request: PreviewRequest{ConfigID: "B", AgentName: "omp"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BindRuntime(ctx, "runtime-b", p.TargetProviderThreadID, "B", "omp"); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetRuntime(ctx, "runtime-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Conversation(ctx, a.ConversationID); err != nil {
		t.Fatal("deleting archived provider removed live provider", err)
	}
	if err := s.ForgetRuntime(ctx, "runtime-b"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Conversation(ctx, a.ConversationID); !errors.Is(err, ErrNotFound) {
		t.Fatal("last runtime retained private history", err)
	}
}
