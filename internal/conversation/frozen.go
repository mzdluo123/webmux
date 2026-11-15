package conversation

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/rainchan/webmux/internal/historysource"
)

const frozenSchema = `
CREATE TABLE frozen_export (
 hash TEXT PRIMARY KEY, body BLOB NOT NULL
);
CREATE TABLE frozen_snapshot (
 id TEXT PRIMARY KEY, manifest BLOB NOT NULL,
 export_hash TEXT REFERENCES frozen_export(hash) ON DELETE RESTRICT
);
CREATE TABLE frozen_reference (
 snapshot_id TEXT NOT NULL REFERENCES frozen_snapshot(id) ON DELETE CASCADE,
 owner TEXT NOT NULL, PRIMARY KEY(snapshot_id,owner)
);
CREATE INDEX frozen_reference_owner ON frozen_reference(owner);
CREATE TABLE native_source_binding (
 provider_thread_id TEXT PRIMARY KEY REFERENCES provider_thread(id) ON DELETE CASCADE,
 native_id TEXT NOT NULL, path TEXT NOT NULL, branch TEXT NOT NULL
);
CREATE TABLE conversation_prefix (
 conversation_id TEXT NOT NULL REFERENCES conversation(id) ON DELETE CASCADE,
 ordinal INTEGER NOT NULL, snapshot_id TEXT NOT NULL REFERENCES frozen_snapshot(id) ON DELETE RESTRICT,
 PRIMARY KEY(conversation_id,ordinal)
);
CREATE TABLE handoff_source_snapshot (
 handoff_id TEXT NOT NULL REFERENCES context_handoff(id) ON DELETE CASCADE,
 ordinal INTEGER NOT NULL, snapshot_id TEXT NOT NULL REFERENCES frozen_snapshot(id) ON DELETE RESTRICT,
 PRIMARY KEY(handoff_id,ordinal)
);
PRAGMA user_version=13;
`

func encodeFrozen(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil || len(body) > historysource.MaxBytes*2 {
		return nil, ErrInvalid
	}
	return body, nil
}

// SaveFrozen atomically publishes a manifest, an optional one-time export and
// its lifetime owner. No Activity or source cursor is mutated by this operation.
func (s *Service) SaveFrozen(ctx context.Context, data historysource.SnapshotData, owner string) (historysource.Snapshot, error) {
	out := data.Snapshot
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	if out.ID == "" || out.ContentHash == "" || owner == "" || (out.Retained != (data.Export != nil)) || (out.Retained && out.ExportHash == "") {
		return out, ErrInvalid
	}
	manifest, err := encodeFrozen(out)
	if err != nil {
		return out, err
	}
	var export []byte
	if data.Export != nil {
		export, err = encodeFrozen(data.Export)
		if err != nil {
			return out, err
		}
	}
	err = s.transact(ctx, func(tx *sql.Tx) error {
		var exportHash any
		if out.Retained {
			exportHash = out.ExportHash
			if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO frozen_export(hash,body) VALUES(?,?)", out.ExportHash, export); err != nil {
				return storageError(ctx, err)
			}
		}
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO frozen_snapshot(id,manifest,export_hash) VALUES(?,?,?)", out.ID, manifest, exportHash); err != nil {
			return storageError(ctx, err)
		}
		var existing []byte
		if err := tx.QueryRowContext(ctx, "SELECT manifest FROM frozen_snapshot WHERE id=?", out.ID).Scan(&existing); err != nil {
			return storageError(ctx, err)
		}
		if !bytes.Equal(existing, manifest) {
			return ErrConflict
		}
		_, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO frozen_reference(snapshot_id,owner) VALUES(?,?)", out.ID, owner)
		return storageError(ctx, err)
	})
	return out, err
}

func (s *Service) Frozen(ctx context.Context, id string) (historysource.SnapshotData, error) {
	var out historysource.SnapshotData
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var manifest, export []byte
		err := tx.QueryRowContext(ctx, "SELECT s.manifest,e.body FROM frozen_snapshot s LEFT JOIN frozen_export e ON e.hash=s.export_hash WHERE s.id=?", id).Scan(&manifest, &export)
		if err != nil {
			return storageError(ctx, err)
		}
		if len(manifest) > historysource.MaxBytes || json.Unmarshal(manifest, &out.Snapshot) != nil || out.Snapshot.ID != id {
			return ErrUnavailable
		}
		if out.Snapshot.Retained {
			if len(export) > historysource.MaxBytes*2 {
				return ErrUnavailable
			}
			out.Export = new(historysource.Export)
			if json.Unmarshal(export, out.Export) != nil {
				return ErrUnavailable
			}
		}
		return nil
	})
	return out, err
}

func (s *Service) RetainFrozen(ctx context.Context, id, owner string) error {
	if !s.HistoryEnabled() {
		return ErrUnavailable
	}
	if owner == "" {
		return ErrInvalid
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var found string
		if err := tx.QueryRowContext(ctx, "SELECT id FROM frozen_snapshot WHERE id=?", id).Scan(&found); err != nil {
			return storageError(ctx, err)
		}
		_, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO frozen_reference(snapshot_id,owner) VALUES(?,?)", id, owner)
		return storageError(ctx, err)
	})
}

func (s *Service) ReleaseFrozen(ctx context.Context, id, owner string) error {
	if !s.HistoryEnabled() {
		return ErrUnavailable
	}
	if id == "" || owner == "" {
		return ErrInvalid
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		return releaseFrozen(ctx, tx, id, owner)
	})
}

func releaseFrozen(ctx context.Context, tx *sql.Tx, id, owner string) error {
	// A handoff's exact payload identity and frozen source outlive source files.
	// Neither explicit release nor migration may unlock pending/unknown delivery.
	var pending int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM handoff_delivery d JOIN handoff_execution e ON e.delivery_id=d.id WHERE d.handoff_id=? AND e.status IN ('pending','unknown')`, owner).Scan(&pending); err != nil {
		return storageError(ctx, err)
	}
	if pending != 0 {
		return ErrConflict
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM frozen_reference WHERE snapshot_id=? AND owner=?", id, owner); err != nil {
		return storageError(ctx, err)
	}
	return collectFrozen(ctx, tx)
}

func collectFrozen(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM frozen_snapshot WHERE NOT EXISTS (SELECT 1 FROM frozen_reference r WHERE r.snapshot_id=frozen_snapshot.id)"); err != nil {
		return storageError(ctx, err)
	}
	_, err := tx.ExecContext(ctx, "DELETE FROM frozen_export WHERE NOT EXISTS (SELECT 1 FROM frozen_snapshot s WHERE s.export_hash=frozen_export.hash)")
	return storageError(ctx, err)
}

// FrozenForOwner returns stable manifest IDs only, never an unbounded body list.
func (s *Service) FrozenForOwner(ctx context.Context, owner string) ([]string, error) {
	out := make([]string, 0)
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	rows, err := s.db.QueryContext(ctx, "SELECT snapshot_id FROM frozen_reference WHERE owner=? ORDER BY snapshot_id LIMIT 1001", owner)
	if err != nil {
		return out, storageError(ctx, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return out, storageError(ctx, err)
		}
		out = append(out, id)
	}
	if len(out) > 1000 {
		return nil, historysource.ErrLimit
	}
	if err := rows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, storageError(ctx, err)
	}
	return out, nil
}
