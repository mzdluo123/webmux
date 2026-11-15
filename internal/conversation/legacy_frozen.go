package conversation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/historysource"
)

// migrateLegacyFrozen runs inside the schema transaction, before any legacy
// body tables are removed. It never reads a provider file or changes receipts.
func migrateLegacyFrozen(ctx context.Context, tx *sql.Tx) error {
	conversations, err := legacyFrozenStrings(ctx, tx, `SELECT id FROM conversation ORDER BY id`)
	if err != nil {
		return err
	}
	for _, owner := range conversations {
		var ambiguousFork bool
		if err := tx.QueryRowContext(ctx, `SELECT relationship='fork' AND NOT EXISTS(SELECT 1 FROM context_handoff h JOIN handoff_preview p ON p.handoff_id=h.id WHERE h.target_conversation_id=c.id AND h.source_conversation_id=c.parent_id) FROM conversation c WHERE id=?`, owner).Scan(&ambiguousFork); err != nil {
			return err
		}
		gap := ""
		if ambiguousFork {
			gap = "legacy_fork_boundary_unknown"
		}
		id, err := legacyFreezeRange(ctx, tx, owner, "", -1, gap)
		if err != nil {
			return err
		}
		if err = legacyPrefix(ctx, tx, owner, id); err != nil {
			return err
		}
	}
	snapshots, err := legacyFrozenStrings(ctx, tx, `SELECT id FROM history_snapshot ORDER BY id`)
	if err != nil {
		return err
	}
	for _, id := range snapshots {
		var owner string
		if err := tx.QueryRowContext(ctx, `SELECT n.conversation_id FROM history_snapshot s JOIN native_source n ON n.id=s.source_id WHERE s.id=?`, id).Scan(&owner); err != nil {
			return err
		}
		if _, err := legacyFreezeRange(ctx, tx, owner, id, 0, ""); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO frozen_reference SELECT snapshot_id,owner FROM history_snapshot_reference WHERE snapshot_id=?`, id); err != nil {
			return err
		}
	}
	handoffs, err := legacyFrozenStrings(ctx, tx, `SELECT h.id FROM context_handoff h WHERE NOT EXISTS(SELECT 1 FROM handoff_source_snapshot s WHERE s.handoff_id=h.id) ORDER BY h.created_at,h.id`)
	if err != nil {
		return err
	}
	for _, handoff := range handoffs {
		var source, target string
		var boundary sql.NullInt64
		var started bool
		err := tx.QueryRowContext(ctx, `SELECT h.source_conversation_id,h.target_conversation_id,p.source_position,EXISTS(SELECT 1 FROM handoff_delivery d JOIN handoff_execution e ON e.delivery_id=d.id WHERE d.handoff_id=h.id AND e.call_started=1) OR EXISTS(SELECT 1 FROM activity a WHERE a.conversation_id=h.target_conversation_id AND a.position IS NOT NULL) OR EXISTS(SELECT 1 FROM native_injection n JOIN provider_thread t ON t.id=n.target_provider_thread_id WHERE t.conversation_id=h.target_conversation_id AND n.call_started=1) FROM context_handoff h LEFT JOIN handoff_preview p ON p.handoff_id=h.id WHERE h.id=?`, handoff).Scan(&source, &target, &boundary, &started)
		if err != nil {
			return err
		}
		if !boundary.Valid {
			continue
		}
		snapshot, err := legacyFreezeRange(ctx, tx, source, "", boundary.Int64, "")
		if err != nil {
			return err
		}
		// Ancestor ranges are independent segments; never substitute the source's
		// current full prefix for the handoff's exact committed boundary.
		inherited, err := legacyFrozenStrings(ctx, tx, `SELECT p.snapshot_id FROM conversation_prefix p JOIN frozen_snapshot s ON s.id=p.snapshot_id WHERE p.conversation_id=? AND json_extract(s.manifest,'$.Source.NativeID')!=? ORDER BY p.ordinal`, source, source)
		if err != nil {
			return err
		}
		segments := append(inherited, snapshot)
		if len(segments) > 1000 {
			return historysource.ErrLimit
		}
		for ordinal, id := range segments {
			if _, err = tx.ExecContext(ctx, `INSERT INTO handoff_source_snapshot VALUES(?,?,?)`, handoff, ordinal, id); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO frozen_reference VALUES(?,?)`, id, handoff); err != nil {
				return err
			}
		}
		if source != target && started {
			for i := len(segments) - 1; i >= 0; i-- {
				if err = legacyPrefix(ctx, tx, target, segments[i]); err != nil {
					return err
				}
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE handoff_item SET source_hash=(SELECT json_extract(r.value,'$.Hash') FROM handoff_source_snapshot h JOIN frozen_snapshot s ON s.id=h.snapshot_id,json_each(s.manifest,'$.Records') r WHERE h.handoff_id=handoff_item.handoff_id AND json_extract(r.value,'$.ID')=handoff_item.item_id LIMIT 1) WHERE source_hash='' AND EXISTS(SELECT 1 FROM handoff_source_snapshot h JOIN frozen_snapshot s ON s.id=h.snapshot_id,json_each(s.manifest,'$.Records') r WHERE h.handoff_id=handoff_item.handoff_id AND json_extract(r.value,'$.ID')=handoff_item.item_id)`); err != nil {
		return err
	}
	// A parent pointer is not evidence of a transmitted range. Retain recovery
	// evidence under the child without making it part of the child's projection.
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO frozen_reference(snapshot_id,owner) SELECT p.snapshot_id,c.id FROM conversation c JOIN conversation_prefix p ON p.conversation_id=c.parent_id WHERE c.relationship='fork' AND c.parent_id IS NOT NULL`); err != nil {
		return err
	}
	return nil
}

func legacyFrozenStrings(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if len(out) >= historysource.MaxRecords {
			return nil, historysource.ErrLimit
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func legacyPrefix(ctx context.Context, tx *sql.Tx, owner, id string) error {
	// A parent's frozen range precedes the child's own committed body.
	_, err := tx.ExecContext(ctx, `INSERT INTO conversation_prefix(conversation_id,ordinal,snapshot_id) SELECT ?,coalesce(min(ordinal),1)-1,? FROM conversation_prefix WHERE conversation_id=? HAVING NOT EXISTS(SELECT 1 FROM conversation_prefix WHERE conversation_id=? AND snapshot_id=?)`, owner, id, owner, owner, id)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO frozen_reference VALUES(?,?)`, id, owner)
	return err
}

// A negative boundary means all committed records; zero means an empty range.
func legacyFreezeRange(ctx context.Context, tx *sql.Tx, owner, oldID string, boundary int64, extraGap string) (string, error) {
	query := `SELECT id FROM activity WHERE conversation_id=? AND position IS NOT NULL AND (?<0 OR position<=?) ORDER BY position`
	args := []any{owner, boundary, boundary}
	if oldID != "" {
		query = `SELECT activity_id FROM history_snapshot_item WHERE snapshot_id=? ORDER BY position`
		args = []any{oldID}
	}
	ids, err := legacyFrozenStrings(ctx, tx, query, args...)
	if err != nil {
		return "", err
	}
	origin := "unknown"
	if err = tx.QueryRowContext(ctx, `SELECT history_origin FROM conversation WHERE id=?`, owner).Scan(&origin); err != nil {
		return "", err
	}
	category := "legacy_history_origin_unknown"
	if origin != "unknown" {
		category = "legacy_history_origin_partial"
	}
	doc := historysource.Document{Source: historysource.Source{Agent: "legacy-webmux", NativeID: owner}, FormatVersion: "legacy-activity/v1", ParserVersion: "legacy-activity/v1", Generation: "legacy-activity/v1", Fingerprint: owner, Cursor: strconv.FormatInt(boundary, 10), Gaps: []historysource.Gap{{Category: category}}}
	if oldID != "" {
		doc.Generation = "legacy-codex-snapshot/v1"
		doc.ParserVersion = doc.Generation
		doc.FormatVersion = doc.Generation
		doc.Cursor = oldID
	}
	if extraGap != "" {
		doc.Gaps = append(doc.Gaps, historysource.Gap{Category: extraGap})
	}
	var total int64
	for _, id := range ids {
		var a legacyFrozenActivity
		if err := tx.QueryRowContext(ctx, `SELECT id,kind,role,status,position,heading_length,byte_length FROM activity WHERE id=? AND position IS NOT NULL`, id).Scan(&a.ID, &a.Kind, &a.Role, &a.Status, &a.Position, &a.HeadingLength, &a.OutputLength); err != nil {
			return "", err
		}
		if !legacyFrozenExportable(a.Kind, a.Role) {
			continue
		}
		length := a.HeadingLength + a.OutputLength
		if a.HeadingLength < 0 || a.OutputLength < 0 || length < 0 || length > historysource.MaxRecordBytes || total+length > historysource.MaxBytes {
			return "", historysource.ErrLimit
		}
		var body strings.Builder
		body.Grow(int(length))
		if err := legacyFrozenReadChunks(ctx, tx, "activity_heading_chunk", id, a.HeadingLength, &body); err != nil {
			return "", err
		}
		if err := legacyFrozenReadChunks(ctx, tx, "activity_chunk", id, a.OutputLength, &body); err != nil {
			return "", err
		}
		if !utf8.ValidString(body.String()) {
			return "", ErrUnavailable
		}
		record := historysource.Record{ID: a.ID, Kind: a.Kind, Role: a.Role, Status: a.Status, Text: body.String(), Offset: a.Position, End: a.Position + 1, ByteLength: length}
		rows, err := tx.QueryContext(ctx, `SELECT reference,mime_type,name,size FROM activity_attachment WHERE activity_id=? ORDER BY position`, id)
		if err != nil {
			return "", err
		}
		for rows.Next() {
			var attachment historysource.Attachment
			if err = rows.Scan(&attachment.Reference, &attachment.MimeType, &attachment.Name, &attachment.Size); err != nil {
				break
			}
			if len(record.Attachments) >= historysource.MaxRecords || len(doc.Gaps) >= historysource.MaxRecords {
				err = historysource.ErrLimit
				break
			}
			record.Attachments = append(record.Attachments, attachment)
			doc.Gaps = append(doc.Gaps, historysource.Gap{RecordID: id, Category: "legacy_attachment_body_unavailable", Offset: a.Position})
		}
		rowErr := rows.Err()
		_ = rows.Close()
		if err != nil {
			return "", err
		}
		if rowErr != nil {
			return "", rowErr
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			return "", err
		}
		total += int64(len(encoded))
		if total > historysource.MaxBytes {
			return "", historysource.ErrLimit
		}
		doc.Records = append(doc.Records, record)
	}
	gapQuery := `SELECT id,reason FROM history_gap WHERE conversation_id=? AND resolved=0 ORDER BY created_at,id`
	gapArgs := []any{owner}
	if oldID != "" {
		gapQuery = `SELECT id,reason FROM history_snapshot_gap WHERE snapshot_id=? ORDER BY position`
		gapArgs = []any{oldID}
	}
	rows, err := tx.QueryContext(ctx, gapQuery, gapArgs...)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var gap historysource.Gap
		if err = rows.Scan(&gap.RecordID, &gap.Category); err != nil {
			break
		}
		if len(doc.Gaps) >= historysource.MaxRecords {
			err = historysource.ErrLimit
			break
		}
		doc.Gaps = append(doc.Gaps, gap)
	}
	rowErr := rows.Err()
	_ = rows.Close()
	if err != nil {
		return "", err
	}
	if rowErr != nil {
		return "", rowErr
	}
	if oldID != "" {
		var diagnostics []byte
		if err := tx.QueryRowContext(ctx, `SELECT diagnostics FROM history_snapshot WHERE id=?`, oldID).Scan(&diagnostics); err != nil {
			return "", err
		}
		for _, gap := range strings.Split(string(diagnostics), "\n") {
			if gap != "" {
				if len(doc.Gaps) >= historysource.MaxRecords {
					return "", historysource.ErrLimit
				}
				doc.Gaps = append(doc.Gaps, historysource.Gap{Category: gap})
			}
		}
	}
	doc.Pending = false
	doc.RecordRefs, doc.References = nil, nil
	contentHash, err := legacyFrozenHash(doc)
	if err != nil {
		return "", err
	}
	export := historysource.Export{Records: doc.Records}
	exportHash, err := legacyFrozenHash(export)
	if err != nil {
		return "", err
	}
	snapshot := historysource.Snapshot{Source: doc.Source, Cursor: doc.Cursor, Generation: doc.Generation, Fingerprint: doc.Fingerprint, FormatVersion: doc.FormatVersion, ParserVersion: doc.ParserVersion, ContentHash: contentHash, ExportHash: exportHash, RecordCount: len(doc.Records), Gaps: doc.Gaps, Retained: true, Records: make([]historysource.RecordRef, 0, len(doc.Records))}
	for i, record := range doc.Records {
		hash, err := legacyFrozenHash(record)
		if err != nil {
			return "", err
		}
		snapshot.Records = append(snapshot.Records, historysource.RecordRef{ID: record.ID, Hash: hash, Kind: record.Kind, Position: i + 1})
	}
	snapshot.ID, err = legacyFrozenHash(snapshot)
	if err != nil {
		return "", err
	}
	if oldID != "" {
		snapshot.ID = oldID
	}
	manifest, err := encodeFrozen(snapshot)
	if err != nil {
		return "", err
	}
	if len(manifest) > historysource.MaxBytes {
		return "", historysource.ErrLimit
	}
	body, err := encodeFrozen(export)
	if err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO frozen_export VALUES(?,?)`, exportHash, body); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO frozen_snapshot VALUES(?,?,?)`, snapshot.ID, manifest, exportHash); err != nil {
		return "", err
	}
	var existing []byte
	if err = tx.QueryRowContext(ctx, `SELECT manifest FROM frozen_snapshot WHERE id=?`, snapshot.ID).Scan(&existing); err != nil {
		return "", err
	}
	if !bytes.Equal(existing, manifest) {
		return "", ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO frozen_reference VALUES(?,?)`, snapshot.ID, owner); err != nil {
		return "", err
	}
	return snapshot.ID, nil
}

func legacyFrozenHash(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

type legacyFrozenActivity struct {
	ID, Kind, Role, Status                string
	Position, HeadingLength, OutputLength int64
}

// This is the old ProjectActivity allowlist, not the native reader allowlist.
// In particular reasoning, permissions and private/control events stay private.
func legacyFrozenExportable(kind, role string) bool {
	switch kind {
	case "message":
		return role == "user" || role == "agent"
	case "command", "tool", "error", "interrupt", "file_change", "plan", "compaction":
		return true
	default:
		return false
	}
}

// Read each legacy chunk through a bounded SQL slice. Contiguity and exact
// declared lengths are checked so damaged bodies cannot silently become exports.
// UTF-8 may straddle chunks; validation happens after both sections are joined.
func legacyFrozenReadChunks(ctx context.Context, tx *sql.Tx, table, id string, length int64, out *strings.Builder) error {
	const chunkLimit = 64 << 10
	rows, err := tx.QueryContext(ctx, `SELECT byte_offset,byte_length,length(text),substr(text,1,?) FROM `+table+` WHERE activity_id=? ORDER BY byte_offset`, chunkLimit+1, id)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	var next int64
	for rows.Next() {
		var offset, declared, actual int64
		var chunk []byte
		if err := rows.Scan(&offset, &declared, &actual, &chunk); err != nil {
			return err
		}
		if declared > chunkLimit {
			return historysource.ErrLimit
		}
		if offset != next || declared <= 0 || declared != actual || int64(len(chunk)) != declared || declared > length-next {
			return ErrUnavailable
		}
		out.Write(chunk)
		next += declared
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if next != length {
		return ErrUnavailable
	}
	return nil
}
