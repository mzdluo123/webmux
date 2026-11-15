package conversation

import (
	"context"
	"database/sql"
	"time"
)

// recoverRuns records interrupted foreground control without recovering or
// inventing a transcript. Native history remains the only live body source.
func (s *Service) recoverRuns(ctx context.Context) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO history_gap(id,conversation_id,provider_thread_id,reason,created_at) SELECT lower(hex(randomblob(16))),conversation_id,provider_thread_id,'crash',? FROM run WHERE status='pending'`, time.Now().UTC().UnixNano())
		if err != nil {
			return storageError(ctx, err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE conversation SET reconcile_required=1 WHERE id IN (SELECT conversation_id FROM history_gap WHERE resolved=0)"); err != nil {
			return storageError(ctx, err)
		}
		_, err = tx.ExecContext(ctx, "UPDATE run SET status='interrupted' WHERE status='pending'")
		return storageError(ctx, err)
	})
}

// AddGap records uncertainty without deriving invented chat content from PTY
// bytes. Reconciliation must positively resolve a gap before delta is complete.
func (s *Service) AddGap(ctx context.Context, threadID, reason string) error {
	if reason != "tui" && reason != "replay_ambiguous" && reason != "crash" && reason != "storage_failure" && reason != "unconfirmed_prompt" && reason != "capture_disabled" {
		return ErrInvalid
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		return s.addGap(ctx, tx, threadID, reason)
	})
}

func (s *Service) addGap(ctx context.Context, tx *sql.Tx, threadID, reason string) error {
	id, err := s.allocateID(ctx, tx, "history_gap")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO history_gap(id,conversation_id,provider_thread_id,reason,created_at) SELECT ?,conversation_id,id,?,? FROM provider_thread WHERE id=?`, id, reason, time.Now().UTC().UnixNano(), threadID)
	if err != nil {
		return storageError(ctx, err)
	}
	_, err = tx.ExecContext(ctx, "UPDATE conversation SET reconcile_required=1 WHERE id=(SELECT conversation_id FROM provider_thread WHERE id=?)", threadID)
	return storageError(ctx, err)
}

func (s *Service) markCaptureDisabled(ctx context.Context, tx *sql.Tx, thread string) error {
	if !s.captureDisabled {
		return nil
	}
	return s.addGap(ctx, tx, thread, "capture_disabled")
}
