package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/rainchan/webmux/internal/hybridresume"
	"github.com/rainchan/webmux/internal/session"
)

// restartResumeTimeout bounds resuming one session after a restart.
const restartResumeTimeout = time.Minute

// resumeRestarted resumes the hybrid sessions ids, one at a time, on their
// recorded side. One that cannot come back (its worktree was released, its
// launch config or agent session is gone) stays archived and is logged.
func resumeRestarted(ctx context.Context, resumer interface {
	Resume(context.Context, string, hybridresume.Input) (*session.Session, error)
}, ids []string) {
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		rctx, cancel := context.WithTimeout(ctx, restartResumeTimeout)
		_, err := resumer.Resume(rctx, id, hybridresume.Input{ClientRequestID: "restart-" + id})
		cancel()
		if err != nil {
			slog.Warn("session not resumed after restart; it stays archived", "id", id, "error", err)
			continue
		}
		slog.Info("session resumed after restart", "id", id)
	}
}
