package conversation

import (
	"context"
	"database/sql"

	"github.com/rainchan/webmux/internal/historysource"
)

type SourceFact struct {
	Hash, RepresentationHash string
}

// SourceCoverage is scoped to the target's current native context. Unknown
// checkpoints and changed projection policies require a fresh context, not an
// optimistic numeric watermark over a different provider's history.
func (s *Service) SourceCoverage(ctx context.Context, target string, checkpoint historysource.ContextState, available map[string]string, policy string) (map[string]SourceFact, bool, error) {
	covered := make(map[string]SourceFact)
	reconcile := false
	err := s.transact(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT coalesce(c.context_key,''),coalesce(c.head_id,''),coalesce(c.head_hash,'') FROM dispatch_receipt r JOIN provider_thread p ON p.id=r.target_provider_thread_id LEFT JOIN dispatch_context c ON c.execution_id=r.execution_id WHERE p.id=? AND r.native_thread_id=p.agent_session_id AND r.status='inline' LIMIT 100001`, target)
		if err != nil {
			return storageError(ctx, err)
		}
		count := 0
		for rows.Next() {
			var key, head, hash string
			if err := rows.Scan(&key, &head, &hash); err != nil {
				_ = rows.Close()
				return storageError(ctx, err)
			}
			count++
			if count > historysource.MaxRecords {
				_ = rows.Close()
				return historysource.ErrLimit
			}
			if checkpoint.Key == "" || key != checkpoint.Key || head == "" || available[head] != hash {
				reconcile = true
			}
		}
		err = rows.Err()
		closeErr := rows.Close()
		if err != nil {
			return storageError(ctx, err)
		}
		if closeErr != nil {
			return storageError(ctx, closeErr)
		}
		rows, err = tx.QueryContext(ctx, `SELECT s.item_id,s.source_hash,s.representation,s.representation_hash FROM dispatch_source_item s JOIN dispatch_receipt r ON r.execution_id=s.execution_id JOIN dispatch_context c ON c.execution_id=r.execution_id JOIN provider_thread p ON p.id=r.target_provider_thread_id WHERE p.id=? AND r.native_thread_id=p.agent_session_id AND r.status='inline' AND c.context_key=? LIMIT 100001`, target, checkpoint.Key)
		if err != nil {
			return storageError(ctx, err)
		}
		defer func() { _ = rows.Close() }()
		count = 0
		for rows.Next() {
			var id, representation string
			var fact SourceFact
			if err := rows.Scan(&id, &fact.Hash, &representation, &fact.RepresentationHash); err != nil {
				return storageError(ctx, err)
			}
			count++
			if count > historysource.MaxRecords {
				return historysource.ErrLimit
			}
			if representation != policy {
				reconcile = true
				continue
			}
			if previous, ok := covered[id]; ok && previous != fact {
				reconcile = true
			}
			covered[id] = fact
		}
		return storageError(ctx, rows.Err())
	})
	return covered, reconcile, err
}

func (s *Service) DispatchSourceScope(ctx context.Context, execution string) (historysource.DispatchScope, error) {
	out := historysource.DispatchScope{ExecutionID: execution}
	if s.broken != nil {
		return out, s.broken
	}
	err := s.db.QueryRowContext(ctx, `SELECT t.conversation_id,t.id,e.native_thread_id,coalesce(json_extract(CAST(p.request AS TEXT),'$.ContextKey'),'') FROM handoff_execution e JOIN handoff_delivery d ON d.id=e.delivery_id JOIN provider_thread t ON t.id=d.target_provider_thread_id JOIN handoff_preview p ON p.handoff_id=d.handoff_id WHERE e.id=? AND e.status='inline'`, execution).Scan(&out.ConversationID, &out.ProviderThreadID, &out.NativeID, &out.BeforeContextKey)
	return out, storageError(ctx, err)
}

// RecordSourceContext cannot change a dispatch outcome or attach an old receipt
// to a replacement native identity. It records only a reusable observed context.
func (s *Service) RecordSourceContext(ctx context.Context, scope historysource.DispatchScope, state historysource.ContextState) error {
	if state.Key == "" || state.HeadID == "" || scope.NativeID == "" {
		return ErrInvalid
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO dispatch_context(execution_id,context_key,head_id,head_hash) SELECT r.execution_id,?,?,? FROM dispatch_receipt r JOIN provider_thread p ON p.id=r.target_provider_thread_id WHERE r.execution_id=? AND r.status='inline' AND r.native_thread_id=? AND p.agent_session_id=? AND p.id=? ON CONFLICT(execution_id) DO UPDATE SET context_key=excluded.context_key,head_id=excluded.head_id,head_hash=excluded.head_hash`, state.Key, state.HeadID, state.HeadHash, scope.ExecutionID, scope.NativeID, scope.NativeID, scope.ProviderThreadID)
		return storageError(ctx, err)
	})
}
