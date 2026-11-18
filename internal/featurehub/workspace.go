package featurehub

import (
	"context"
	"encoding/json"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/workspace"
)

// Workspaces streams coalesced repository/worktree change notifications on one
// writer while CloseRead consumes ping and close control frames.
func Workspaces(ctx context.Context, sub *workspace.Subscription, closeSub func(), c hub.MessageConn) {
	defer closeSub()
	defer func() { _ = c.CloseNow() }()
	ctx = c.CloseRead(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-sub.C:
			if !ok {
				return
			}
			data, err := json.Marshal(contract.RepositoriesEvent{Type: "changed"})
			if err != nil {
				_ = c.Close(hub.StatusInternalError, "invalid repositories frame")
				return
			}
			if hub.Write(ctx, c, hub.MessageText, data) != nil {
				_ = c.Close(hub.StatusSlowClient, "write timeout")
				return
			}
		}
	}
}
