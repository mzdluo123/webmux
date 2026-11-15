package conversation

import (
	"context"
	"database/sql"

	"github.com/rainchan/webmux/internal/historysource"
)

// NativeBindings returns identities and source locations, never source bodies.
func (s *Service) NativeBindings(ctx context.Context, id string) ([]historysource.Binding, error) {
	out := make([]historysource.Binding, 0)
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.agent_name,p.agent_session_id,p.native_root,coalesce(b.path,''),coalesce(b.branch,'') FROM provider_thread p LEFT JOIN native_source_binding b ON b.provider_thread_id=p.id AND b.native_id=p.agent_session_id WHERE p.conversation_id=? AND p.status!='imported' AND NOT EXISTS(SELECT 1 FROM history_gap g WHERE g.provider_thread_id=p.id AND g.reason='capture_disabled') ORDER BY p.created_at,p.rowid LIMIT 1001`, id)
	if err != nil {
		return out, storageError(ctx, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		binding := historysource.Binding{ConversationID: id}
		if err := rows.Scan(&binding.ProviderThreadID, &binding.Source.Agent, &binding.Source.NativeID, &binding.Root, &binding.Source.Path, &binding.Source.Branch); err != nil {
			return nil, storageError(ctx, err)
		}
		out = append(out, binding)
	}
	if len(out) > 1000 {
		return nil, historysource.ErrLimit
	}
	return out, storageError(ctx, rows.Err())
}

// BindNativeSource is called only after a reader has verified the source. CAS
// against the current provider identity prevents a concurrent rebind from routing
// another session's history into this conversation.
func (s *Service) BindNativeSource(ctx context.Context, thread string, source historysource.Source) error {
	if !s.HistoryEnabled() {
		return ErrUnavailable
	}
	if source.Path == "" || source.NativeID == "" {
		return ErrInvalid
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var agent, native string
		if err := tx.QueryRowContext(ctx, "SELECT agent_name,agent_session_id FROM provider_thread WHERE id=?", thread).Scan(&agent, &native); err != nil {
			return storageError(ctx, err)
		}
		if agent != source.Agent || native != source.NativeID {
			return ErrConflict
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO native_source_binding(provider_thread_id,native_id,path,branch) VALUES(?,?,?,?) ON CONFLICT(provider_thread_id) DO UPDATE SET native_id=excluded.native_id,path=excluded.path,branch=excluded.branch`, thread, native, source.Path, source.Branch)
		return storageError(ctx, err)
	})
}

func (s *Service) PrefixSnapshots(ctx context.Context, id string) ([]string, error) {
	out := make([]string, 0)
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	rows, err := s.db.QueryContext(ctx, "SELECT snapshot_id FROM conversation_prefix WHERE conversation_id=? ORDER BY ordinal LIMIT 1001", id)
	if err != nil {
		return out, storageError(ctx, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var snapshot string
		if err := rows.Scan(&snapshot); err != nil {
			return nil, storageError(ctx, err)
		}
		out = append(out, snapshot)
	}
	if len(out) > 1000 {
		return nil, historysource.ErrLimit
	}
	return out, storageError(ctx, rows.Err())
}
