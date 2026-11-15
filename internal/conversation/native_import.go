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
	NativeID             string
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
	var priorID, priorHash string
	var priorOffset int64
	priorErr := s.db.QueryRowContext(ctx, "SELECT s.id,s.consumed,s.checksum FROM native_source s JOIN native_location l ON l.source_id=s.id WHERE l.path=?", path).Scan(&priorID, &priorOffset, &priorHash)
	if priorErr != nil && !errors.Is(priorErr, sql.ErrNoRows) {
		return out, storageError(ctx, priorErr)
	}
	f, err := codexrollout.OpenFile(path)
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
	var captured bytes.Buffer
	var records []codexrollout.Record
	parsed, err := codexrollout.Read(io.TeeReader(io.LimitReader(f, codexrollout.MaxInput+1), &captured), func(r codexrollout.Record) error { records = append(records, r); return ctx.Err() })
	if err != nil {
		return out, err
	}
	if parsed.Metadata.ID == "" {
		if parsed.Pending {
			return NativeImport{Pending: true}, nil
		}
		return out, codexrollout.ErrFormat
	}
	data := captured.Bytes()
	if s.beforeNativeVerify != nil {
		s.beforeNativeVerify()
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
	out.NativeID = parsed.Metadata.ID
	out.Pending = parsed.Pending
	out.Consumed = parsed.Consumed
	for _, r := range records {
		out.Diagnostics = append(out.Diagnostics, r.Diagnostics...)
	}
	err = s.transact(ctx, func(tx *sql.Tx) error {
		var private bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_thread p JOIN history_gap g ON g.provider_thread_id=p.id WHERE p.agent_name='codex' AND p.agent_session_id=? AND g.reason='capture_disabled')`, parsed.Metadata.ID).Scan(&private); err != nil {
			return storageError(ctx, err)
		}
		if private {
			return ErrUnavailable
		}
		var oldOffset int64
		var oldHash, parser, native, identity string
		err := tx.QueryRowContext(ctx, `SELECT s.id,s.generation,s.conversation_id,s.provider_thread_id,s.consumed,s.checksum,s.parser,s.native_id,s.file_identity FROM native_source s JOIN native_location l ON l.source_id=s.id WHERE l.path=?`, path).Scan(&out.SourceID, &out.Generation, &out.Binding.ConversationID, &out.Binding.ProviderThreadID, &oldOffset, &oldHash, &parser, &native, &identity)
		fresh := errors.Is(err, sql.ErrNoRows)
		if err != nil && !fresh {
			return storageError(ctx, err)
		}

		if nativeFileIdentity(before) != "unknown" && (fresh || identity != nativeFileIdentity(before)) {
			// A rename/archive or hard link is the same observed source only
			// with native identity, platform identity and confirmed content.
			rows, e := tx.QueryContext(ctx, `SELECT id,generation,conversation_id,provider_thread_id,consumed,checksum,parser,native_id,file_identity FROM native_source WHERE file_identity=? AND native_id=? AND parser=? ORDER BY rowid DESC LIMIT 1`, nativeFileIdentity(before), parsed.Metadata.ID, codexrollout.Version)
			if e != nil {
				return storageError(ctx, e)
			}
			for rows.Next() {
				var candidate NativeImport
				var offset int64
				var checksum, p, n, i string
				if e = rows.Scan(&candidate.SourceID, &candidate.Generation, &candidate.Binding.ConversationID, &candidate.Binding.ProviderThreadID, &offset, &checksum, &p, &n, &i); e != nil {
					_ = rows.Close()
					return storageError(ctx, e)
				}
				if offset <= int64(len(data)) && digest(data[:offset]) == checksum {
					out.SourceID = candidate.SourceID
					out.Generation = candidate.Generation
					out.Binding = candidate.Binding
					oldOffset = offset
					oldHash = checksum
					parser = p
					native = n
					identity = i
					fresh = false
					break
				}
			}
			e = rows.Err()
			_ = rows.Close()
			if e != nil {
				return storageError(ctx, e)
			}
		}
		// An importer with an older captured view must not turn a concurrent
		// append (or replacement already imported by another caller) into a
		// spurious generation. Retry against the newly committed cursor.
		changed := priorID != out.SourceID || priorOffset != oldOffset || priorHash != oldHash
		compatible := !fresh && oldOffset <= int64(len(data)) && digest(data[:oldOffset]) == oldHash && identity == nativeFileIdentity(before) && native == parsed.Metadata.ID && parser == codexrollout.Version
		if changed && !compatible && (!fresh || priorID != "") {
			return ErrConflict
		}
		if !fresh && (oldOffset > int64(len(data)) || digest(data[:min(oldOffset, int64(len(data)))]) != oldHash || parser != codexrollout.Version || native != parsed.Metadata.ID || identity != nativeFileIdentity(before)) {
			fresh = true
		}
		if fresh {
			if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(generation),0)+1 FROM native_source WHERE path=?", path).Scan(&out.Generation); err != nil {
				return storageError(ctx, err)
			}
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
			if _, err = tx.ExecContext(ctx, `INSERT INTO native_source VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, out.SourceID, path, out.Generation, out.Binding.ConversationID, out.Binding.ProviderThreadID, parsed.Metadata.ID, codexrollout.Version, 0, digest(nil), nativeFileIdentity(before), parsed.Metadata.Timestamp, parsed.Model, parsed.Metadata.ModelProvider, parsed.Metadata.CLIVersion, parsed.Pending); err != nil {
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
				if timestamp, e := time.Parse(time.RFC3339Nano, r.Timestamp); e == nil && time.Unix(0, timestamp.UnixNano()).Equal(timestamp) {
					created = timestamp.UnixNano()
				}
				id := digest([]byte(out.SourceID + ":" + r.Hash + ":" + strconv.FormatInt(r.Offset, 10) + ":" + strconv.Itoa(i)))
				role := item.Role
				if role == "assistant" {
					role = "agent"
				}
				kind := item.Kind
				if kind == "tool_result" || kind == "tool_arguments" {
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
				for position, a := range item.Attachments {
					if _, err = tx.ExecContext(ctx, "INSERT INTO activity_attachment VALUES(?,?,?,?,?,?)", id, position, "native image", a.MimeType, a.Reference, -1); err != nil {
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
		if _, err = tx.ExecContext(ctx, "INSERT INTO native_location VALUES(?,?) ON CONFLICT(path) DO UPDATE SET source_id=excluded.source_id", path, out.SourceID); err != nil {
			return storageError(ctx, err)
		}
		_, err = tx.ExecContext(ctx, `UPDATE native_source SET consumed=?,checksum=?,pending=?,model=? WHERE id=?`, parsed.Consumed, digest(data[:parsed.Consumed]), parsed.Pending, parsed.Model, out.SourceID)
		return storageError(ctx, err)
	})
	return out, err
}

// Small aliases keep native parsing independent of the persistence model.
