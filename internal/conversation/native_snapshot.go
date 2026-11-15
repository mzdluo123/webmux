package conversation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/rainchan/webmux/internal/codexrollout"
)

type NativeSnapshot struct {
	NativeID                       string
	ID, SourceID, Parser, Checksum string
	Model                          string
	Consumed                       int64
	Gaps                           []string
}

// FreezeNative binds immutable committed activities, not a parent's current
// state. Forgetting the owning conversation deletes its snapshots by cascade.
func (s *Service) FreezeNative(ctx context.Context, source string) (NativeSnapshot, error) {
	var out NativeSnapshot
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var conversation string
		if err := tx.QueryRowContext(ctx, `SELECT id,parser,consumed,checksum,conversation_id,model,native_id FROM native_source WHERE id=?`, source).Scan(&out.SourceID, &out.Parser, &out.Consumed, &out.Checksum, &conversation, &out.Model, &out.NativeID); err != nil {
			return storageError(ctx, err)
		}
		out.Gaps = []string{}
		rows, err := tx.QueryContext(ctx, `SELECT reason FROM history_gap WHERE conversation_id=? AND resolved=0 ORDER BY reason`, conversation)
		if err != nil {
			return storageError(ctx, err)
		}
		for rows.Next() {
			var reason string
			if err = rows.Scan(&reason); err != nil {
				_ = rows.Close()
				return storageError(ctx, err)
			}
			out.Gaps = append(out.Gaps, reason)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return storageError(ctx, err)
		}
		var pending bool
		if err := tx.QueryRowContext(ctx, "SELECT pending FROM native_source WHERE id=?", source).Scan(&pending); err != nil {
			return storageError(ctx, err)
		}
		if pending {
			out.Gaps = append(out.Gaps, "pending_tail")
		}
		var orphan bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM native_record r WHERE r.source_id=? AND r.record_kind='tool_result' AND NOT EXISTS(SELECT 1 FROM native_record c WHERE c.source_id=r.source_id AND c.call_id=r.call_id AND c.offset<r.offset AND c.record_kind IN ('tool','tool_arguments')))`, source).Scan(&orphan); err != nil {
			return storageError(ctx, err)
		}
		if orphan {
			out.Gaps = append(out.Gaps, "orphan_tool_result")
		}
		var missing bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM native_record c WHERE c.source_id=? AND c.record_kind IN ('tool','tool_arguments') AND NOT EXISTS(SELECT 1 FROM native_record r WHERE r.source_id=c.source_id AND r.record_kind='tool_result' AND r.call_id=c.call_id))`, source).Scan(&missing); err != nil {
			return storageError(ctx, err)
		}
		if missing {
			out.Gaps = append(out.Gaps, "missing_tool_result")
		}
		encoded := []byte(strings.Join(out.Gaps, "\n"))
		out.ID = digest([]byte(out.SourceID + out.Parser + out.Checksum + string(encoded)))
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO history_snapshot VALUES(?,?,?,?,?,?,?)`, out.ID, out.SourceID, out.Parser, out.Consumed, out.Checksum, encoded, out.Model); err != nil {
			return storageError(ctx, err)
		}
		if err = freezeNativeGaps(ctx, tx, out); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO history_snapshot_item SELECT ?,a.id,a.position FROM activity a JOIN native_record r ON r.activity_id=a.id WHERE r.source_id=? AND r.end_offset<=?`, out.ID, source, out.Consumed)
		return storageError(ctx, err)
	})
	return out, err
}

func (s *Service) NativeSnapshot(ctx context.Context, id string) (NativeSnapshot, error) {
	var out NativeSnapshot
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	var gaps []byte
	err := s.db.QueryRowContext(ctx, `SELECT s.id,s.source_id,s.parser,s.consumed,s.checksum,s.diagnostics,s.model,n.native_id FROM history_snapshot s JOIN native_source n ON n.id=s.source_id WHERE s.id=?`, id).Scan(&out.ID, &out.SourceID, &out.Parser, &out.Consumed, &out.Checksum, &gaps, &out.Model, &out.NativeID)
	if err != nil {
		return out, storageError(ctx, err)
	}
	out.Gaps = []string{}
	if len(gaps) > 0 {
		out.Gaps = strings.Split(string(gaps), "\n")
	}
	return out, nil
}

// SnapshotActivities uses the same bounded text reader as live history. Each
// activity retains its source ID, byte length and continuation offset.
func (s *Service) SnapshotActivities(ctx context.Context, id string, after int64, limit, maxBytes int) (ActivityPage, error) {
	out := ActivityPage{HistoryOrigin: "bounded_import", Items: []ActivityPart{}, Gaps: []HistoryGap{}, NextPosition: after}
	if !s.HistoryEnabled() {
		return out, ErrUnavailable
	}
	limit, maxBytes, err := pageBounds(limit, maxBytes)
	if err != nil || after < 0 {
		return out, ErrInvalid
	}
	err = s.transact(ctx, func(tx *sql.Tx) error {
		var exists string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM history_snapshot WHERE id=?`, id).Scan(&exists); err != nil {
			return storageError(ctx, err)
		}
		out.Gaps, err = readNativeGaps(ctx, tx, id)
		if err != nil {
			return err
		}
		out.ReconcileRequired = len(out.Gaps) > 0
		rows, err := tx.QueryContext(ctx, `SELECT a.id,a.conversation_id FROM history_snapshot_item i JOIN activity a ON a.id=i.activity_id WHERE i.snapshot_id=? AND i.position>? ORDER BY i.position LIMIT ?`, id, after, limit)
		if err != nil {
			return storageError(ctx, err)
		}
		var ids [][2]string
		for rows.Next() {
			var pair [2]string
			if err = rows.Scan(&pair[0], &pair[1]); err != nil {
				_ = rows.Close()
				return storageError(ctx, err)
			}
			ids = append(ids, pair)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return storageError(ctx, err)
		}
		for _, pair := range ids {
			part, err := readActivity(ctx, tx, pair[1], pair[0], 0, maxBytes)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, part)
			out.NextPosition = part.Activity.Position
		}
		return nil
	})
	return out, err
}

// SnapshotInput exports original bodies; only snapcompact.Project may truncate
// or normalize them. CurrentTask remains an independent unmodified field.
func (s *Service) SnapshotInput(ctx context.Context, id, currentTask string) (snapcompact.Input, error) {
	input := snapcompact.Input{History: []snapcompact.Record{}, CurrentTask: currentTask}
	if _, err := s.NativeSnapshot(ctx, id); err != nil {
		return input, err
	}
	var total int
	var after int64
	for {
		page, err := s.SnapshotActivities(ctx, id, after, 20, 4000)
		if err != nil {
			return input, err
		}
		if len(page.Items) == 0 {
			break
		}
		for _, part := range page.Items {
			a := part.Activity
			if !part.Exportable {
				continue
			}
			text := a.Text
			for part.Truncated {
				part, err = s.Activity(ctx, a.ConversationID, a.ID, part.NextTextOffset, MaxActivityBytes)
				if err != nil {
					return input, err
				}
				text += part.Activity.Text
				if len(text) > codexrollout.MaxInput {
					return input, codexrollout.ErrLimit
				}
			}
			total += len(text)
			if total > codexrollout.MaxInput {
				return input, codexrollout.ErrLimit
			}
			r := snapcompact.Record{ID: a.ID, Role: a.Role, Status: a.Status, Text: text}
			var kind string
			if err = s.db.QueryRowContext(ctx, `SELECT call_id,tool_name,record_kind FROM native_record WHERE activity_id=?`, a.ID).Scan(&r.CallID, &r.ToolName, &kind); err != nil {
				return input, storageError(ctx, err)
			}

			if kind == "tool_result" && r.ToolName == "" && r.CallID != "" {
				e := s.db.QueryRowContext(ctx, `SELECT c.tool_name FROM native_record c JOIN native_record r ON r.source_id=c.source_id AND r.call_id=c.call_id WHERE r.activity_id=? AND c.offset<r.offset AND c.record_kind IN ('tool','tool_arguments') ORDER BY c.offset DESC LIMIT 1`, a.ID).Scan(&r.ToolName)
				if e != nil && !errors.Is(e, sql.ErrNoRows) {
					return input, storageError(ctx, e)
				}
			}
			if r.Role == "agent" {
				r.Role = "assistant"
			}
			switch kind {
			case "tool_arguments":
				decoder := json.NewDecoder(strings.NewReader(r.Text))
				decoder.UseNumber()
				var args map[string]any
				if decoder.Decode(&args) == nil && args != nil {
					r.Arguments = args
					r.Text = ""
					r.Role = "tool_call"
				} else {
					r.Role = "tool_call"
					r.CallBody = r.Text
					r.Text = ""
				}
			case "tool":
				r.Role = "tool_call"
				r.CallBody = r.Text
				r.Text = ""
			case "tool_result":
				r.Role = "tool"
			}
			input.History = append(input.History, r)
		}
		after = page.NextPosition
	}
	return input, nil
}

// DeleteNativeSource explicitly forgets imported history and every dependent
// snapshot. It never removes the native file. Retained fork references are not
// supported; callers must not imply that snapshots survive this operation.
func (s *Service) DeleteNativeSource(ctx context.Context, source string) error {
	if !s.HistoryEnabled() {
		return ErrUnavailable
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var owner string
		if err := tx.QueryRowContext(ctx, "SELECT conversation_id FROM native_source WHERE id=?", source).Scan(&owner); err != nil {
			return storageError(ctx, err)
		}
		var retained bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM history_snapshot_reference r JOIN history_snapshot s ON s.id=r.snapshot_id WHERE s.source_id=?)`, source).Scan(&retained); err != nil {
			return storageError(ctx, err)
		}
		if retained {
			return ErrConflict
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM conversation WHERE id=?", owner)
		return storageError(ctx, err)
	})
}

type NativeProvenance struct {
	ActivityID, SourceID, Checksum, CallID, ToolName, Kind string
	Offset, End, Position                                  int64
}

func (s *Service) SnapshotProvenance(ctx context.Context, id string, after int64, limit int) ([]NativeProvenance, error) {
	out := []NativeProvenance{}
	if after < 0 || limit < 1 || limit > 100 {
		return out, ErrInvalid
	}
	if _, err := s.NativeSnapshot(ctx, id); err != nil {
		return out, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.activity_id,r.source_id,r.checksum,r.call_id,r.tool_name,r.record_kind,r.offset,r.end_offset,i.position FROM history_snapshot_item i JOIN native_record r ON r.activity_id=i.activity_id WHERE i.snapshot_id=? AND i.position>? ORDER BY i.position LIMIT ?`, id, after, limit)
	if err != nil {
		return out, storageError(ctx, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var p NativeProvenance
		if err = rows.Scan(&p.ActivityID, &p.SourceID, &p.Checksum, &p.CallID, &p.ToolName, &p.Kind, &p.Offset, &p.End, &p.Position); err != nil {
			return out, storageError(ctx, err)
		}
		out = append(out, p)
	}
	return out, storageError(ctx, rows.Err())
}

// RetainNativeSnapshot is an explicit lifetime reference for future consumers.
// It does not create a child conversation or copy running provider state.
func (s *Service) RetainNativeSnapshot(ctx context.Context, id, owner string) error {
	if !s.HistoryEnabled() {
		return ErrUnavailable
	}
	if owner == "" || len(owner) > 256 {
		return ErrInvalid
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var exists string
		if err := tx.QueryRowContext(ctx, "SELECT id FROM history_snapshot WHERE id=?", id).Scan(&exists); err != nil {
			return storageError(ctx, err)
		}
		_, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO history_snapshot_reference VALUES(?,?)", id, owner)
		return storageError(ctx, err)
	})
}

// ReleaseNativeSnapshot removes one owner's reference. Collection remains an
// explicit DeleteNativeSource operation after all owners have released.
func (s *Service) ReleaseNativeSnapshot(ctx context.Context, id, owner string) error {
	if !s.HistoryEnabled() {
		return ErrUnavailable
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "DELETE FROM history_snapshot_reference WHERE snapshot_id=? AND owner=?", id, owner)
		return storageError(ctx, err)
	})
}
