package hybrid

import (
	"context"
	"testing"

	"github.com/rainchan/webmux/internal/agent/agy"
	"github.com/rainchan/webmux/internal/attentionkind"
)

func TestBackgroundRefreshCommitsControlState(t *testing.T) {
	h, db := historyHarness(t)
	h.fast()
	shell := h.create(h.input(agy.Name, SideACP))
	id := shell.Info().ID
	prompt(t, h, id, "hi /background")
	waitFor(t, "background status response committed", func() bool {
		attention := shell.Info().Attention
		return attention != nil && attention.Kind == attentionkind.Done
	})
	binding, err := db.Binding(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	state, err := db.Conversation(context.Background(), binding.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	thread, err := db.ProviderThread(context.Background(), binding.ProviderThreadID)
	if err != nil {
		t.Fatal(err)
	}
	if state.NextRunOrdinal != 3 || thread.LastDeliveredRunOrdinal != 2 {
		t.Fatalf("background refresh not committed: conversation=%+v thread=%+v", state, thread)
	}
}
