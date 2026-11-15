package conversation

import (
	"context"

	"github.com/rainchan/webmux/internal/historysource"
)

// SourceAllowed prevents re-enabling capture from silently exposing a native
// session that contains a previously opted-out interval. Existing frozen ranges
// remain independently scoped; this gate applies before any new source capture.
func (s *Service) SourceAllowed(ctx context.Context, source historysource.Source) error {
	if !s.HistoryEnabled() {
		return ErrUnavailable
	}
	var blocked bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_thread p JOIN history_gap g ON g.provider_thread_id=p.id WHERE p.agent_name=? AND p.agent_session_id=? AND g.reason='capture_disabled')`, source.Agent, source.NativeID).Scan(&blocked)
	if err != nil {
		return storageError(ctx, err)
	}
	if blocked {
		return ErrUnavailable
	}
	return nil
}
