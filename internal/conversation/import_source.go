package conversation

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/rainchan/webmux/internal/historysource"
)

// ImportFrozenSource transfers a frozen range's pending lifetime to a new
// history-only conversation. Its provider identity is metadata, not a runtime
// binding: the prefix is the sole source of imported conversation history.
func (s *Service) ImportFrozenSource(ctx context.Context, snapshotID, pendingOwner string) (historysource.Binding, error) {
	var out historysource.Binding
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	if snapshotID == "" || pendingOwner == "" {
		return out, ErrInvalid
	}
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var manifest []byte
		if err := tx.QueryRowContext(ctx, `SELECT s.manifest FROM frozen_snapshot s JOIN frozen_reference r ON r.snapshot_id=s.id WHERE s.id=? AND r.owner=?`, snapshotID, pendingOwner).Scan(&manifest); err != nil {
			return storageError(ctx, err)
		}
		var snapshot historysource.Snapshot
		if len(manifest) > historysource.MaxBytes || json.Unmarshal(manifest, &snapshot) != nil || snapshot.ID != snapshotID || snapshot.Source.Agent == "" || snapshot.Source.NativeID == "" {
			return ErrInvalid
		}
		var err error
		out.ConversationID, err = s.allocateID(ctx, tx, "conversation")
		if err != nil {
			return err
		}
		out.ProviderThreadID, err = s.allocateID(ctx, tx, "provider_thread")
		if err != nil {
			return err
		}
		out.Source = snapshot.Source
		out.Source.ProviderThreadID = out.ProviderThreadID
		now := time.Now().UTC().UnixNano()
		// Source locations are not evidence of cwd, repository, runtime config,
		// or discovery root. Leave all unknown metadata empty.
		if _, err := tx.ExecContext(ctx, `INSERT INTO conversation(id,cwd,active_provider_thread_id,history_origin,created_at,updated_at) VALUES(?,'',?,'bounded_import',?,?)`, out.ConversationID, out.ProviderThreadID, now, now); err != nil {
			return storageError(ctx, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO provider_thread(id,conversation_id,config_id,agent_name,agent_session_id,native_root,status,created_at,updated_at) VALUES(?,?,'',?,?,'','imported',?,?)`, out.ProviderThreadID, out.ConversationID, out.Source.Agent, out.Source.NativeID, now, now); err != nil {
			return storageError(ctx, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO conversation_prefix(conversation_id,ordinal,snapshot_id) VALUES(?,1,?)`, out.ConversationID, snapshotID); err != nil {
			return storageError(ctx, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO frozen_reference(snapshot_id,owner) VALUES(?,?)`, snapshotID, out.ConversationID); err != nil {
			return storageError(ctx, err)
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM frozen_reference WHERE snapshot_id=? AND owner=?`, snapshotID, pendingOwner)
		return storageError(ctx, err)
	})
	if err != nil {
		return historysource.Binding{}, err
	}
	return out, nil
}
