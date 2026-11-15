package conversation

import (
	"context"
	"database/sql"
)

// ObserveUsage accepts telemetry only for the exact native identity currently
// associated with this provider thread. Late results cannot revive old usage.
func (s *Service) ObserveUsage(ctx context.Context, thread, nativeID string, used, window int64) error {
	if used < 0 || window < 0 || used > 1<<40 || window > 1<<40 || nativeID == "" {
		return ErrInvalid
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, "UPDATE provider_thread SET context_usage=?,context_window=?,usage_measured=1 WHERE id=? AND agent_session_id=?", used, window, thread, nativeID)
		if err != nil {
			return storageError(ctx, err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return storageError(ctx, err)
		}
		if n == 0 {
			return ErrConflict
		}
		return nil
	})
}

// RuntimeIDs supplies only bindings of this Conversation for attachment scope.
func (s *Service) RuntimeIDs(ctx context.Context, id string) ([]string, error) {
	out := []string{}
	err := s.transact(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT runtime_id FROM runtime_binding WHERE conversation_id=?", id)
		if err != nil {
			return storageError(ctx, err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var runtime string
			if err := rows.Scan(&runtime); err != nil {
				return storageError(ctx, err)
			}
			out = append(out, runtime)
		}
		return storageError(ctx, rows.Err())
	})
	return out, err
}
