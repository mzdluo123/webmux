package conversation

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// freezeNativeGaps copies facts once. Later resolution or new gaps in the
// source conversation cannot change the snapshot's completeness envelope.
func freezeNativeGaps(ctx context.Context, tx *sql.Tx, snapshot NativeSnapshot) error {
	var thread string
	if err := tx.QueryRowContext(ctx, "SELECT provider_thread_id FROM native_source WHERE id=?", snapshot.SourceID).Scan(&thread); err != nil {
		return storageError(ctx, err)
	}
	for position, reason := range snapshot.Gaps {
		id := digest([]byte(snapshot.ID + ":" + reason))
		created := time.Now().UTC().UnixNano()
		err := tx.QueryRowContext(ctx, "SELECT id,created_at FROM history_gap WHERE provider_thread_id=? AND reason=? AND resolved=0 ORDER BY created_at LIMIT 1", thread, reason).Scan(&id, &created)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return storageError(ctx, err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO history_snapshot_gap VALUES(?,?,?,?,?,?)", snapshot.ID, position, id, thread, reason, created); err != nil {
			return storageError(ctx, err)
		}
	}
	return nil
}

func readNativeGaps(ctx context.Context, tx *sql.Tx, id string) ([]HistoryGap, error) {
	out := []HistoryGap{}
	rows, err := tx.QueryContext(ctx, "SELECT id,provider_thread_id,reason,created_at FROM history_snapshot_gap WHERE snapshot_id=? ORDER BY position", id)
	if err != nil {
		return out, storageError(ctx, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var gap HistoryGap
		var created int64
		if err = rows.Scan(&gap.ID, &gap.ProviderThreadID, &gap.Reason, &created); err != nil {
			return out, storageError(ctx, err)
		}
		gap.CreatedAt = time.Unix(0, created).UTC()
		out = append(out, gap)
	}
	return out, storageError(ctx, rows.Err())
}
