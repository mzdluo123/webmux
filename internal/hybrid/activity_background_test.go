package hybrid

import (
	"testing"

	"github.com/rainchan/webmux/internal/agent/agy"
	"github.com/rainchan/webmux/internal/attentionkind"
)

func TestBackgroundRefreshCommitsControlState(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()
	shell := h.create(h.input(agy.Name, SideACP))
	id := shell.Info().ID
	prompt(t, h, id, "hi /background")
	waitFor(t, "background status response committed", func() bool {
		attention := shell.Info().Attention
		return attention != nil && attention.Kind == attentionkind.Done
	})

}
