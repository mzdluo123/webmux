package conversation

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/rainchan/webmux/internal/historysource"
)

// HandoffSnapshots returns only the immutable sources owned by this handoff.
// The caller separately verifies its source Conversation before exposing bodies.
func (s *Service) HandoffSnapshots(ctx context.Context, id string) ([]string, error) {
	out := make([]string, 0)
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var source string
		if err := tx.QueryRowContext(ctx, "SELECT source_conversation_id FROM context_handoff WHERE id=?", id).Scan(&source); err != nil {
			return storageError(ctx, err)
		}
		rows, err := tx.QueryContext(ctx, "SELECT snapshot_id FROM handoff_source_snapshot WHERE handoff_id=? ORDER BY ordinal LIMIT 1001", id)
		if err != nil {
			return storageError(ctx, err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var snapshot string
			if err := rows.Scan(&snapshot); err != nil {
				return storageError(ctx, err)
			}
			out = append(out, snapshot)
		}
		if len(out) > 1000 {
			return historysource.ErrLimit
		}
		return storageError(ctx, rows.Err())
	})
	return out, err
}

func bindPreviewSources(ctx context.Context, tx *sql.Tx, id, child string, request *PreviewRequest, snapshots []historysource.Snapshot) (int64, error) {
	if len(snapshots) > 1000 {
		return 0, historysource.ErrLimit
	}
	request.SnapshotIDs = make([]string, 0, len(snapshots))
	seen := make(map[string]string)
	var position int64
	for ordinal, supplied := range snapshots {
		var manifest []byte
		if err := tx.QueryRowContext(ctx, "SELECT manifest FROM frozen_snapshot WHERE id=?", supplied.ID).Scan(&manifest); err != nil {
			return 0, storageError(ctx, err)
		}
		var snapshot historysource.Snapshot
		if len(manifest) > historysource.MaxBytes || json.Unmarshal(manifest, &snapshot) != nil || snapshot.ID != supplied.ID || snapshot.RecordCount != len(snapshot.Records) {
			return 0, ErrUnavailable
		}
		if request.Fork && !snapshot.Retained {
			return 0, ErrConflict
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO handoff_source_snapshot(handoff_id,ordinal,snapshot_id) VALUES(?,?,?)", id, ordinal, snapshot.ID); err != nil {
			return 0, storageError(ctx, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO frozen_reference(snapshot_id,owner) VALUES(?,?)", snapshot.ID, id); err != nil {
			return 0, storageError(ctx, err)
		}
		if request.Fork {
			if _, err := tx.ExecContext(ctx, "INSERT INTO conversation_prefix(conversation_id,ordinal,snapshot_id) VALUES(?,?,?)", child, ordinal, snapshot.ID); err != nil {
				return 0, storageError(ctx, err)
			}
			if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO frozen_reference(snapshot_id,owner) VALUES(?,?)", snapshot.ID, child); err != nil {
				return 0, storageError(ctx, err)
			}
		}
		request.SnapshotIDs = append(request.SnapshotIDs, snapshot.ID)
		for _, ref := range snapshot.Records {
			if previous, exists := seen[ref.ID]; exists {
				if previous != ref.Hash {
					return 0, ErrConflict
				}
				continue
			}
			seen[ref.ID] = ref.Hash
			position++
			if position > historysource.MaxRecords {
				return 0, historysource.ErrLimit
			}
			if !ref.Exportable() {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO handoff_item(handoff_id,item_id,selected,position,source_hash) VALUES(?,?,0,?,?)`, id, ref.ID, position, ref.Hash); err != nil {
				return 0, storageError(ctx, err)
			}
		}
	}
	return position, nil
}

func completeFrozenSelection(ctx context.Context, tx *sql.Tx, id string, result PreviewResult) error {
	var total int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM handoff_item WHERE handoff_id=?", id).Scan(&total); err != nil {
		return storageError(ctx, err)
	}
	if total != len(result.SelectedItemIDs)+len(result.OmittedItemIDs)+len(result.CoveredItemIDs) {
		return ErrConflict
	}
	seen := make(map[string]bool, total)
	for selected, ids := range [][]string{result.OmittedItemIDs, result.SelectedItemIDs, result.CoveredItemIDs} {
		for _, item := range ids {
			if item == "" || seen[item] {
				return ErrInvalid
			}
			seen[item] = true
		}
		if len(ids) == 0 {
			continue
		}
		encoded, err := encodePreviewItems(ids)
		if err != nil {
			return err
		}
		if selected == 2 {
			deleted, err := tx.ExecContext(ctx, "DELETE FROM handoff_item WHERE handoff_id=? AND item_id IN (SELECT value FROM json_each(?))", id, string(encoded))
			if err != nil {
				return storageError(ctx, err)
			}
			count, err := deleted.RowsAffected()
			if err != nil || count != int64(len(ids)) {
				return ErrConflict
			}
			continue
		}
		updated, err := tx.ExecContext(ctx, "UPDATE handoff_item SET selected=? WHERE handoff_id=? AND item_id IN (SELECT value FROM json_each(?))", selected, id, string(encoded))
		if err != nil {
			return storageError(ctx, err)
		}
		count, err := updated.RowsAffected()
		if err != nil {
			return storageError(ctx, err)
		}
		if count != int64(len(ids)) {
			return ErrConflict
		}
	}
	for _, item := range result.SelectedItemIDs {
		hash := result.RepresentationHashes[item]
		if hash == "" || result.ProjectionPolicy == "" {
			return ErrInvalid
		}
		if _, err := tx.ExecContext(ctx, "UPDATE handoff_item SET representation=?,representation_hash=? WHERE handoff_id=? AND item_id=?", result.ProjectionPolicy, hash, id, item); err != nil {
			return storageError(ctx, err)
		}
	}
	return nil
}
