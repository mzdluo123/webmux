// Package history reads native OpenCode SQLite history without running OpenCode.
package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/rainchan/webmux/internal/historysource"
	_ "modernc.org/sqlite"
)

const ParserVersion = "opencode-sqlite-1"

type boundary struct {
	Generation       string
	Instance         string
	Message, Part    int64
	Digest           string
	NativeID, Branch string
}
type message struct {
	id, role, parent, data string
	position               int64
}
type part struct {
	id, message, data string
	position          int64
}

// Read captures a transactionally consistent session. A cursor freezes both the
// message and part insertion ranges and authenticates their original contents.
func Read(ctx context.Context, req historysource.Request) (out historysource.Document, err error) {
	out.Source = req.Source
	defer func() {
		if err != nil {
			out = historysource.Document{Source: req.Source}
		}
	}()
	out.ParserVersion = ParserVersion
	if req.Source.Agent != "opencode" || req.Source.NativeID == "" {
		return out, historysource.ErrIdentity
	}
	b := boundary{NativeID: req.Source.NativeID, Branch: req.Source.Branch}
	if req.Cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(req.Cursor)
		if e != nil || len(raw) > 4096 || json.Unmarshal(raw, &b) != nil || b.NativeID != req.Source.NativeID || b.Branch != req.Source.Branch || b.Message < 0 || b.Part < 0 || b.Digest == "" {
			return out, historysource.ErrChanged
		}
	}
	before, e := os.Lstat(req.Source.Path)
	if e != nil || !before.Mode().IsRegular() {
		return out, historysource.ErrUnavailable
	}
	db, e := open(ctx, req.Source.Path)
	if e != nil {
		return out, safe(ctx, e)
	}
	defer db.Close()
	tx, e := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, safe(ctx, e)
	}
	defer tx.Rollback()
	legacy, v2 := false, false
	if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='session')").Scan(&legacy); e != nil {
		return out, safe(ctx, e)
	}
	if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='session_v2')").Scan(&v2); e != nil {
		return out, safe(ctx, e)
	}
	var generation string
	var sessionParent, fork, revert sql.NullString
	found := 0
	if legacy {
		var id string
		e = tx.QueryRowContext(ctx, "SELECT id,parent_id,revert FROM session WHERE id=?", req.Source.NativeID).Scan(&id, &sessionParent, &revert)
		if e == nil {
			generation = "legacy"
			found++
		} else if !errors.Is(e, sql.ErrNoRows) {
			return out, safe(ctx, e)
		}
	}
	if v2 {
		var id string
		var p, f, r sql.NullString
		e = tx.QueryRowContext(ctx, "SELECT id,coalesce(fork_session_id,parent_id),fork_boundary,revert FROM session_v2 WHERE id=?", req.Source.NativeID).Scan(&id, &p, &f, &r)
		if e == nil {
			generation = "v2"
			sessionParent = p
			fork = f
			revert = r
			found++
		} else if !errors.Is(e, sql.ErrNoRows) {
			return out, safe(ctx, e)
		}
	}
	if found == 0 {
		return out, historysource.ErrIdentity
	}
	if found != 1 {
		return out, historysource.ErrFormat
	}
	if req.Cursor != "" && b.Generation != generation {
		return out, historysource.ErrChanged
	}
	b.Generation = generation
	instance := historysource.FileIdentity(before)
	if req.Cursor != "" && b.Instance != instance {
		return out, historysource.ErrChanged
	}
	b.Instance = instance
	table := "message"
	if generation == "v2" {
		table = "session_message"
	}
	if req.Cursor == "" {
		if e = tx.QueryRowContext(ctx, "SELECT coalesce(max(rowid),0) FROM "+table+" WHERE session_id=?", req.Source.NativeID).Scan(&b.Message); e != nil {
			return out, safe(ctx, e)
		}
		if generation == "legacy" {
			if e = tx.QueryRowContext(ctx, "SELECT coalesce(max(rowid),0) FROM part WHERE session_id=?", req.Source.NativeID).Scan(&b.Part); e != nil {
				return out, safe(ctx, e)
			}
		}
	}
	query := "SELECT id,rowid,'',substr(CAST(data AS BLOB),1,4194305) FROM message WHERE session_id=? AND rowid<=? ORDER BY time_created,id LIMIT 100001"
	if generation == "v2" {
		query = "SELECT id,rowid,type,substr(CAST(data AS BLOB),1,4194305) FROM session_message WHERE session_id=? AND rowid<=? ORDER BY seq,id LIMIT 100001"
	}
	rows, e := tx.QueryContext(ctx, query, req.Source.NativeID, b.Message)
	if e != nil {
		return out, safe(ctx, e)
	}
	var messages []message
	total := 0
	hash := sha256.New()
	fmt.Fprintf(hash, "%q %q %q %q", generation, sessionParent.String, fork.String, revert.String)
	for rows.Next() {
		var m message
		if e = rows.Scan(&m.id, &m.position, &m.role, &m.data); e != nil {
			rows.Close()
			return out, safe(ctx, e)
		}
		total += len(m.data)
		if len(messages) >= historysource.MaxRecords || len(m.data) > historysource.MaxRecordBytes || total > historysource.MaxBytes {
			rows.Close()
			return out, historysource.ErrLimit
		}
		var info struct {
			Role   string `json:"role"`
			Parent string `json:"parentID"`
		}
		if json.Unmarshal([]byte(m.data), &info) != nil {
			rows.Close()
			return out, historysource.ErrFormat
		}
		if generation == "legacy" {
			m.role = info.Role
		}
		m.parent = info.Parent
		fmt.Fprintf(hash, "%q %d %q %q", m.id, m.position, m.role, m.data)
		messages = append(messages, m)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, safe(ctx, e)
	}
	parts := map[string][]part{}
	if generation == "legacy" {
		rows, e = tx.QueryContext(ctx, "SELECT id,message_id,rowid,substr(CAST(data AS BLOB),1,4194305) FROM part WHERE session_id=? AND rowid<=? ORDER BY time_created,id LIMIT 100001", req.Source.NativeID, b.Part)
		if e != nil {
			return out, safe(ctx, e)
		}
		count := len(messages)
		for rows.Next() {
			var p part
			if e = rows.Scan(&p.id, &p.message, &p.position, &p.data); e != nil {
				rows.Close()
				return out, safe(ctx, e)
			}
			count++
			total += len(p.data)
			if count > historysource.MaxRecords || len(p.data) > historysource.MaxRecordBytes || total > historysource.MaxBytes {
				rows.Close()
				return out, historysource.ErrLimit
			}
			fmt.Fprintf(hash, "%q %q %d %q", p.id, p.message, p.position, p.data)
			parts[p.message] = append(parts[p.message], p)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return out, safe(ctx, e)
		}
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if req.Cursor != "" && b.Digest != digest {
		return out, historysource.ErrChanged
	}
	b.Digest = digest
	selected := map[string]bool{}
	if req.Source.Branch != "" && generation == "v2" {
		found := false
		for _, m := range messages {
			selected[m.id] = true
			if m.id == req.Source.Branch {
				found = true
				break
			}
		}
		if !found {
			return out, historysource.ErrIdentity
		}
	}
	if req.Source.Branch != "" && generation == "legacy" {
		byID := map[string]message{}
		for _, m := range messages {
			byID[m.id] = m
		}
		id := req.Source.Branch
		for id != "" {
			if selected[id] {
				return out, historysource.ErrFormat
			}
			m, ok := byID[id]
			if !ok {
				if id == req.Source.Branch {
					return out, historysource.ErrIdentity
				}
				out.Gaps = append(out.Gaps, historysource.Gap{RecordID: id, Category: "missing-ancestor"})
				break
			}
			selected[id] = true
			id = m.parent
		}
	}
	var reverted struct {
		MessageID string `json:"messageID"`
	}
	if revert.Valid && revert.String != "" && json.Unmarshal([]byte(revert.String), &reverted) != nil {
		return out, historysource.ErrFormat
	}
	for _, m := range messages {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		if m.id == reverted.MessageID {
			out.Gaps = append(out.Gaps, historysource.Gap{RecordID: m.id, Category: "reverted"})
			break
		}
		if req.Source.Branch != "" && !selected[m.id] {
			continue
		}
		if m.role != "user" && m.role != "assistant" {
			out.Gaps = append(out.Gaps, historysource.Gap{RecordID: m.id, Category: "unsupported-record"})
			continue
		}
		if generation == "legacy" {
			for _, p := range parts[m.id] {
				if e = appendPart(&out, m, p.id, p.data, p.position); e != nil {
					return out, e
				}
			}
		} else {
			var data struct {
				Text    string            `json:"text"`
				Content []json.RawMessage `json:"content"`
			}
			if json.Unmarshal([]byte(m.data), &data) != nil {
				return out, historysource.ErrFormat
			}
			if data.Text != "" {
				out.Records = append(out.Records, historysource.Record{ID: m.id, NativeID: m.id, ParentID: m.parent, Kind: "message", Role: m.role, Text: data.Text, Offset: m.position, End: m.position + 1})
			}
			for i, p := range data.Content {
				if e = appendPart(&out, m, fmt.Sprintf("%s/%d", m.id, i), string(p), m.position); e != nil {
					return out, e
				}
			}
		}
		if len(out.Records)+len(out.Gaps) > historysource.MaxRecords {
			return out, historysource.ErrLimit
		}
	}
	if sessionParent.Valid && sessionParent.String != "" {
		out.Gaps = append(out.Gaps, historysource.Gap{RecordID: sessionParent.String, Category: "external-ancestor"})
	}
	if e = tx.Commit(); e != nil {
		return out, safe(ctx, e)
	}
	after, e := os.Lstat(req.Source.Path)
	if e != nil || !os.SameFile(before, after) {
		return out, historysource.ErrChanged
	}
	out.FormatVersion = "opencode-sqlite-" + generation
	out.Generation = instance
	out.Fingerprint = instance + ":" + digest
	raw, _ := json.Marshal(b)
	out.Cursor = base64.RawURLEncoding.EncodeToString(raw)
	return out, nil
}

func appendPart(out *historysource.Document, m message, id, raw string, pos int64) error {
	var p struct {
		Type, Text, Tool, Name, CallID, ID string
		State                              struct {
			Status, Output, Error string
			Content               []struct{ Type, Text string }
		}
	}
	if json.Unmarshal([]byte(raw), &p) != nil {
		return historysource.ErrFormat
	}
	r := historysource.Record{ID: id, NativeID: m.id, ParentID: m.id, Kind: "message", Role: m.role, Offset: pos, End: pos + 1}
	switch p.Type {
	case "text":
		r.Text = p.Text
	case "tool":
		r.Kind = "tool-result"
		r.Role = "tool"
		r.CallID = p.CallID
		if r.CallID == "" {
			r.CallID = p.ID
		}
		r.ToolName = p.Tool
		if r.ToolName == "" {
			r.ToolName = p.Name
		}
		r.Status = p.State.Status
		r.Text = p.State.Output
		for _, c := range p.State.Content {
			if c.Type == "text" {
				r.Text += c.Text
			}
		}
		if r.Status == "error" {
			r.Text = p.State.Error
		}
		out.Records = append(out.Records, historysource.Record{ID: id + "/call", NativeID: m.id, ParentID: m.id, Kind: "tool-call", Role: "assistant", CallID: r.CallID, ToolName: r.ToolName, Status: r.Status, Offset: pos, End: pos + 1})
	case "compaction":
		r.Kind = "compaction"
		out.Gaps = append(out.Gaps, historysource.Gap{RecordID: id, Category: "compaction"})
	case "reasoning":
		out.Gaps = append(out.Gaps, historysource.Gap{RecordID: id, Category: "reasoning-omitted"})
		return nil
	case "step-start", "step-finish", "snapshot", "patch":
		return nil
	default:
		out.Gaps = append(out.Gaps, historysource.Gap{RecordID: id, Category: "unsupported-part"})
		return nil
	}
	out.Records = append(out.Records, r)
	return nil
}

func open(ctx context.Context, path string) (*sql.DB, error) {
	abs, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	u := url.URL{Scheme: "file", Path: abs}
	q := u.Query()
	q.Set("mode", "ro")
	q.Add("_pragma", "query_only(1)")
	u.RawQuery = q.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, e
	}
	return db, nil
}
func safe(ctx context.Context, e error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return historysource.ErrUnavailable
}

// Discover searches only the explicitly supplied data root, without following
// symlinks or inspecting titles, working directories, or transcript exports.
func Discover(ctx context.Context, root string) ([]historysource.Source, error) {
	var out []historysource.Source
	visited := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e != nil {
			return historysource.ErrUnavailable
		}
		visited++
		if visited > 10000 {
			return historysource.ErrLimit
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if strings.Count(rel, string(os.PathSeparator)) >= 8 {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || d.Name() != "opencode.db" {
			return nil
		}
		db, e := open(ctx, path)
		if e != nil {
			return safe(ctx, e)
		}
		defer db.Close()
		for _, table := range []string{"session", "session_v2"} {
			var exists bool
			if e = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)", table).Scan(&exists); e != nil {
				return safe(ctx, e)
			}
			if !exists {
				continue
			}
			rows, e := db.QueryContext(ctx, "SELECT id FROM "+table+" ORDER BY id LIMIT 100001")
			if e != nil {
				return safe(ctx, e)
			}
			for rows.Next() {
				var id string
				if e = rows.Scan(&id); e != nil {
					rows.Close()
					return safe(ctx, e)
				}
				if len(out) >= historysource.MaxRecords {
					rows.Close()
					return historysource.ErrLimit
				}
				out = append(out, historysource.Source{Agent: "opencode", Path: path, NativeID: id})
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return safe(ctx, e)
			}
		}
		return nil
	})
	return out, err
}
