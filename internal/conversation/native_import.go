package conversation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rainchan/webmux/internal/codexrollout"
)

type NativeImport struct {
	SourceID             string
	Binding              Binding
	Generation, Consumed int64
	Pending              bool
	Diagnostics          []codexrollout.Diagnostic
}

func digest(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }

// ImportCodex never changes the source. Parsing and bounded verification precede
// the database transaction; identities, records and cursor publish atomically.
// A changed confirmed prefix creates an independent generation, never appends to
// the old conversation. ACP identity alone is insufficient to merge histories.
func (s *Service) ImportCodex(ctx context.Context, path string) (NativeImport, error) {
	var out NativeImport
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return out, ErrInvalid
	}
	f, err := os.Open(path)
	if err != nil {
		return out, ErrNotFound
	}
	defer func() { _ = f.Close() }()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return out, ErrInvalid
	}
	if before.Size() > codexrollout.MaxInput {
		return out, codexrollout.ErrLimit
	}
	data, err := io.ReadAll(io.LimitReader(f, codexrollout.MaxInput+1))
	if err != nil {
		return out, ErrUnavailable
	}
	if len(data) > codexrollout.MaxInput {
		return out, codexrollout.ErrLimit
	}
	var records []codexrollout.Record
	parsed, err := codexrollout.Read(bytes.NewReader(data), func(r codexrollout.Record) error { records = append(records, r); return ctx.Err() })
	if err != nil {
		return out, err
	}
	// Re-read the captured prefix: mtime/inode alone cannot establish identity.
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return out, ErrUnavailable
	}
	verify, err := io.ReadAll(io.LimitReader(f, int64(len(data))))
	if err != nil || digest(verify) != digest(data) {
		return out, ErrConflict
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		return out, ErrConflict
	}
	out.Pending = parsed.Pending
	out.Consumed = parsed.Consumed
	for _, r := range records {
		out.Diagnostics = append(out.Diagnostics, r.Diagnostics...)
	}
	err = s.transact(ctx, func(tx *sql.Tx) error {
		var oldOffset int64
		var oldHash, parser, native, identity string
		err := tx.QueryRowContext(ctx, `SELECT id,generation,conversation_id,provider_thread_id,consumed,checksum,parser,native_id,file_identity FROM native_source WHERE path=? ORDER BY generation DESC LIMIT 1`, path).Scan(&out.SourceID, &out.Generation, &out.Binding.ConversationID, &out.Binding.ProviderThreadID, &oldOffset, &oldHash, &parser, &native, &identity)
		fresh := errors.Is(err, sql.ErrNoRows)
		if err != nil && !fresh {
			return storageError(ctx, err)
		}
		if !fresh && (oldOffset > int64(len(data)) || digest(data[:min(oldOffset, int64(len(data)))]) != oldHash || parser != codexrollout.Version || native != parsed.Metadata.ID || identity != nativeFileIdentity(before)) {
			fresh = true
		}
		if fresh {
			out.Generation++
			oldOffset = 0
			out.SourceID, err = newID()
			if err != nil {
				return ErrUnavailable
			}
			out.Binding.ConversationID, err = newID()
			if err != nil {
				return ErrUnavailable
			}
			out.Binding.ProviderThreadID, err = newID()
			if err != nil {
				return ErrUnavailable
			}
			now := time.Now().UTC().UnixNano()
			if _, err = tx.ExecContext(ctx, `INSERT INTO conversation(id,cwd,active_provider_thread_id,history_origin,created_at,updated_at) VALUES(?,?,?,'bounded_import',?,?)`, out.Binding.ConversationID, parsed.Metadata.Cwd, out.Binding.ProviderThreadID, now, now); err != nil {
				return storageError(ctx, err)
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO provider_thread(id,conversation_id,config_id,agent_name,agent_session_id,status,created_at,updated_at) VALUES(?,?,'','codex',?,'archived',?,?)`, out.Binding.ProviderThreadID, out.Binding.ConversationID, parsed.Metadata.ID, now, now); err != nil {
				return storageError(ctx, err)
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO native_source VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, out.SourceID, path, out.Generation, out.Binding.ConversationID, out.Binding.ProviderThreadID, parsed.Metadata.ID, codexrollout.Version, 0, digest(nil), nativeFileIdentity(before), parsed.Metadata.Timestamp, parsed.Model, parsed.Metadata.ModelProvider, parsed.Metadata.CLIVersion); err != nil {
				return storageError(ctx, err)
			}
			// No text-based reconciliation with live ACP history.
			if err = s.addGap(ctx, tx, out.Binding.ProviderThreadID, "native_acp_unreconciled"); err != nil {
				return err
			}
		}
		for _, r := range records {
			if r.End <= oldOffset {
				continue
			}
			for i, item := range r.Items {
				created := int64(0)
				if timestamp, e := time.Parse(time.RFC3339Nano, r.Timestamp); e == nil {
					created = timestamp.UnixNano()
				}
				id := digest([]byte(out.SourceID + ":" + r.Hash + ":" + strconv.FormatInt(r.Offset, 10) + ":" + strconv.Itoa(i)))
				role := item.Role
				if role == "assistant" {
					role = "agent"
				}
				kind := item.Kind
				if kind == "tool_result" {
					kind = "tool"
				}
				var pos int64
				if err = tx.QueryRowContext(ctx, `UPDATE conversation SET next_activity_position=next_activity_position+1 WHERE id=? RETURNING next_activity_position-1`, out.Binding.ConversationID).Scan(&pos); err != nil {
					return storageError(ctx, err)
				}
				if _, err = tx.ExecContext(ctx, `INSERT INTO activity(id,conversation_id,provider_thread_id,position,kind,role,status,source,provider_item_id,local_key,byte_length,json_bytes,created_at) VALUES(?,?,?,?,?,?,?,'native',?,?,?,?,?)`, id, out.Binding.ConversationID, out.Binding.ProviderThreadID, pos, kind, role, item.Status, item.NativeID, id, len(item.Text), escapedTextBytes(item.Text), created); err != nil {
					return storageError(ctx, err)
				}
				for start := 0; start < len(item.Text); start += MaxActivityBytes {
					end := min(len(item.Text), start+MaxActivityBytes)
					if _, err = tx.ExecContext(ctx, `INSERT INTO activity_chunk VALUES(?,?,?,?)`, id, start, end-start, []byte(item.Text[start:end])); err != nil {
						return storageError(ctx, err)
					}
				}
				if _, err = tx.ExecContext(ctx, `INSERT INTO native_record VALUES(?,?,?,?,?,?,?,?)`, out.SourceID, r.Offset, r.End, r.Hash, id, item.CallID, item.ToolName, item.Kind); err != nil {
					return storageError(ctx, err)
				}
			}
			if len(r.Items) == 0 {
				if _, err = tx.ExecContext(ctx, `INSERT INTO native_record VALUES(?,?,?,?,NULL,'','','')`, out.SourceID, r.Offset, r.End, r.Hash); err != nil {
					return storageError(ctx, err)
				}
			}
			for _, d := range r.Diagnostics {
				if err = s.addGap(ctx, tx, out.Binding.ProviderThreadID, d.Category); err != nil {
					return err
				}
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE native_source SET consumed=?,checksum=? WHERE id=?`, parsed.Consumed, digest(data[:parsed.Consumed]), out.SourceID)
		return storageError(ctx, err)
	})
	return out, err
}

// Small aliases keep native parsing independent of the persistence model.
