package conversation

import (
	"context"
	"database/sql"

	"github.com/rainchan/webmux/internal/historysource"
)

// NativeContextReferences never pairs records by text alone. Native identity,
// provider item ID, role and transmitted digest must all agree at the reader.
func (s *Service) NativeContextReferences(ctx context.Context, source historysource.Source) ([]historysource.ContextReference, error) {
	out := make([]historysource.ContextReference, 0)
	if source.ProviderThreadID == "" {
		return out, nil
	}
	err := s.transact(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT r.execution_id,m.provider_item_id,m.role,m.text_hash,NULL,i.projection_policy FROM native_injection i JOIN native_injection_item m ON m.delivery_id=i.delivery_id JOIN dispatch_receipt r ON r.delivery_id=i.delivery_id WHERE i.target_provider_thread_id=? AND i.native_thread_id=? AND i.state='injected' AND m.provider_item_id!='' UNION ALL SELECT i.execution_id,i.provider_item_id,'user',i.prompt_hash,i.user_text,i.projection_policy FROM inline_receipt i JOIN dispatch_receipt r ON r.execution_id=i.execution_id WHERE i.target_provider_thread_id=? AND i.native_thread_id=? AND i.provider_item_id!='' AND r.status='inline' LIMIT 100001`, source.ProviderThreadID, source.NativeID, source.ProviderThreadID, source.NativeID)
		if err != nil {
			return storageError(ctx, err)
		}
		var executions []string
		var policies []string
		textBytes := 0
		for rows.Next() {
			var execution, policy string
			var ref historysource.ContextReference
			var text sql.NullString
			if err := rows.Scan(&execution, &ref.NativeID, &ref.Role, &ref.TextHash, &text, &policy); err != nil {
				_ = rows.Close()
				return storageError(ctx, err)
			}
			if text.Valid {
				value := text.String
				ref.UserText = &value
				textBytes += len(value)
			}
			if len(out) >= historysource.MaxRecords || textBytes > historysource.MaxBytes {
				_ = rows.Close()
				return historysource.ErrLimit
			}
			out = append(out, ref)
			executions = append(executions, execution)
			policies = append(policies, policy)
		}
		err = rows.Err()
		closeErr := rows.Close()
		if err != nil {
			return storageError(ctx, err)
		}
		if closeErr != nil {
			return storageError(ctx, closeErr)
		}
		for index := range out {
			if policies[index] == "" {
				continue
			}
			out[index].Policy = policies[index]
			items, err := tx.QueryContext(ctx, "SELECT item_id,source_hash FROM dispatch_source_item WHERE execution_id=? ORDER BY item_id LIMIT 100001", executions[index])
			if err != nil {
				return storageError(ctx, err)
			}
			for items.Next() {
				var item historysource.RecordRef
				if err := items.Scan(&item.ID, &item.Hash); err != nil {
					_ = items.Close()
					return storageError(ctx, err)
				}
				if len(out[index].Originals) >= historysource.MaxRecords {
					_ = items.Close()
					return historysource.ErrLimit
				}
				out[index].Originals = append(out[index].Originals, item)
			}
			err = items.Err()
			closeErr = items.Close()
			if err != nil {
				return storageError(ctx, err)
			}
			if closeErr != nil {
				return storageError(ctx, closeErr)
			}
		}
		return nil
	})
	return out, err
}
