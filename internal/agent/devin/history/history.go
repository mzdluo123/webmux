// Package history reads the committed Devin SQLite message chain, never inferred
// sibling nodes or independent ATIF transcript exports.
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
	"strconv"
	"strings"

	"github.com/rainchan/webmux/internal/historysource"
	_ "modernc.org/sqlite"
)

const ParserVersion = "devin-sqlite-2"

type boundary struct {
	NativeID, Branch, Digest, Instance string
	Head                               int64
	Empty                              bool
}
type node struct {
	id             int64
	parent         sql.NullInt64
	body, metadata string
}

type nativeToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolState struct {
	Status    string          `json:"status"`
	Title     string          `json:"title"`
	Content   json.RawMessage `json:"content"`
	RawInput  json.RawMessage `json:"rawInput"`
	RawOutput json.RawMessage `json:"rawOutput"`
}

func parseToolState(raw string) (toolState, bool, error) {
	var state toolState
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &fields) != nil {
		return state, false, historysource.ErrFormat
	}
	gap := false
	for key, value := range fields {
		switch key {
		case "status":
			if json.Unmarshal(value, &state.Status) != nil {
				return state, false, historysource.ErrFormat
			}
		case "title":
			if json.Unmarshal(value, &state.Title) != nil {
				return state, false, historysource.ErrFormat
			}
		case "content":
			state.Content = value
		case "rawInput":
			state.RawInput = value
		case "rawOutput":
			state.RawOutput = value
		case "toolCallId", "kind", "_meta":
		default:
			gap = gap || string(value) != "null"
		}
	}
	return state, gap, nil
}

func parseToolCall(raw json.RawMessage) (nativeToolCall, bool, error) {
	var call nativeToolCall
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return call, false, historysource.ErrFormat
	}
	gap := false
	for key, value := range fields {
		switch key {
		case "id":
			if json.Unmarshal(value, &call.ID) != nil {
				return call, false, historysource.ErrFormat
			}
		case "name":
			if json.Unmarshal(value, &call.Name) != nil {
				return call, false, historysource.ErrFormat
			}
		case "arguments":
			call.Arguments = value
		case "index", "kind":
		default:
			gap = gap || string(value) != "null"
		}
	}
	return call, gap, nil
}

// Read follows sessions.main_chain_id inside one read-only transaction. Frozen
// requests keep their original committed head even if the live head advances.
func Read(ctx context.Context, req historysource.Request) (out historysource.Document, err error) {
	out.Source = req.Source
	defer func() {
		if err != nil {
			out = historysource.Document{Source: req.Source}
		}
	}()
	out.ParserVersion = ParserVersion
	out.FormatVersion = "devin-sqlite-message-nodes-1"
	if req.Source.Agent != "devin" || req.Source.NativeID == "" {
		return out, historysource.ErrIdentity
	}
	b := boundary{NativeID: req.Source.NativeID, Branch: req.Source.Branch}
	if req.Cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(req.Cursor)
		if e != nil || len(raw) > 4096 || json.Unmarshal(raw, &b) != nil || b.NativeID != req.Source.NativeID || b.Branch != req.Source.Branch || b.Digest == "" {
			return out, historysource.ErrChanged
		}
	}
	before, e := os.Lstat(req.Source.Path)
	if e != nil || !before.Mode().IsRegular() {
		return out, historysource.ErrUnavailable
	}
	instance := historysource.FileIdentity(before)
	if req.Cursor != "" && b.Instance != instance {
		return out, historysource.ErrChanged
	}
	b.Instance = instance
	db, e := open(ctx, req.Source.Path)
	if e != nil {
		return out, safe(ctx, e)
	}
	defer func() { _ = db.Close() }()
	tx, e := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, safe(ctx, e)
	}
	defer func() { _ = tx.Rollback() }()
	var head sql.NullInt64
	e = tx.QueryRowContext(ctx, "SELECT main_chain_id FROM sessions WHERE id=?", req.Source.NativeID).Scan(&head)
	if errors.Is(e, sql.ErrNoRows) {
		return out, historysource.ErrIdentity
	}
	if e != nil {
		return out, safe(ctx, e)
	}
	if req.Cursor == "" {
		b.Head = head.Int64
		b.Empty = !head.Valid
	}
	nodes := []node{}
	seen := map[int64]bool{}
	total := 0
	id := b.Head
	if !b.Empty {
		for {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			if seen[id] {
				return out, historysource.ErrFormat
			}
			if len(nodes) >= historysource.MaxRecords {
				return out, historysource.ErrLimit
			}
			seen[id] = true
			var n node
			n.id = id
			e = tx.QueryRowContext(ctx, "SELECT parent_node_id,substr(CAST(chat_message AS BLOB),1,4194305),coalesce(substr(CAST(metadata AS BLOB),1,4194305),'') FROM message_nodes WHERE session_id=? AND node_id=?", req.Source.NativeID, id).Scan(&n.parent, &n.body, &n.metadata)
			if errors.Is(e, sql.ErrNoRows) {
				if len(nodes) == 0 {
					return out, historysource.ErrChanged
				}
				out.Gaps = append(out.Gaps, historysource.Gap{RecordID: strconv.FormatInt(id, 10), Category: "missing-ancestor"})
				break
			}
			if e != nil {
				return out, safe(ctx, e)
			}
			total += len(n.body) + len(n.metadata)
			if len(n.body) > historysource.MaxRecordBytes || len(n.metadata) > historysource.MaxRecordBytes || total > historysource.MaxBytes {
				return out, historysource.ErrLimit
			}
			nodes = append(nodes, n)
			if !n.parent.Valid {
				break
			}
			id = n.parent.Int64
		}
	}
	// An explicitly selected branch must belong to the committed chain. Never
	// promote an inference sibling just because its node ID exists in the forest.
	if req.Cursor == "" && req.Source.Branch != "" {
		leaf, e := strconv.ParseInt(req.Source.Branch, 10, 64)
		if e != nil || !seen[leaf] {
			return out, historysource.ErrIdentity
		}
		for i, n := range nodes {
			if n.id == leaf {
				nodes = nodes[i:]
				b.Head = leaf
				break
			}
		}
	}
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%q %d %t", req.Source.NativeID, b.Head, b.Empty)
	calls := make(map[string]historysource.Record)
	for i := len(nodes) - 1; i >= 0; i-- {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		n := nodes[i]
		_, _ = fmt.Fprintf(hash, "%d %v %q %q", n.id, n.parent, n.body, n.metadata)
		var msg struct {
			Role       string            `json:"role"`
			Content    json.RawMessage   `json:"content"`
			ToolCallID string            `json:"tool_call_id"`
			ToolCalls  []json.RawMessage `json:"tool_calls"`
		}
		if json.Unmarshal([]byte(n.body), &msg) != nil {
			return out, historysource.ErrFormat
		}
		id := strconv.FormatInt(n.id, 10)
		parent := ""
		if n.parent.Valid {
			parent = strconv.FormatInt(n.parent.Int64, 10)
		}
		var meta map[string]json.RawMessage
		if n.metadata != "" && json.Unmarshal([]byte(n.metadata), &meta) != nil {
			return out, historysource.ErrFormat
		}
		if v := meta["summarized_from"]; len(v) > 0 && string(v) != "null" {
			out.Gaps = append(out.Gaps, historysource.Gap{RecordID: id, Category: "compaction"})
		}
		var ext map[string]json.RawMessage
		if v := meta["extensions"]; len(v) > 0 {
			if json.Unmarshal(v, &ext) != nil {
				return out, historysource.ErrFormat
			}
			if _, ok := ext["compact/prior_node_ids"]; ok {
				out.Gaps = append(out.Gaps, historysource.Gap{RecordID: id, Category: "compaction"})
			}
		}
		if msg.Role != "user" && msg.Role != "assistant" && msg.Role != "tool" {
			out.Gaps = append(out.Gaps, historysource.Gap{RecordID: id, Category: "system-omitted"})
			continue
		}
		text, gap, e := contentText(msg.Content)
		if e != nil {
			return out, e
		}
		if gap {
			out.Gaps = append(out.Gaps, historysource.Gap{RecordID: id, Category: "unsupported-content"})
		}
		if strings.TrimSpace(text) != "" {
			record := historysource.Record{ID: id, NativeID: id, ParentID: parent, Kind: "message", Role: msg.Role, Text: text, CallID: msg.ToolCallID, NativeCallID: msg.ToolCallID, Offset: n.id, End: n.id + 1}
			if msg.Role == "tool" {
				record.Kind = "tool-result"
				record.ToolName = calls[msg.ToolCallID].ToolName
				record.Status = calls[msg.ToolCallID].Status
			}
			out.Records = append(out.Records, record)
		}
		for j, rawCall := range msg.ToolCalls {
			c, gap, e := parseToolCall(rawCall)
			if e != nil {
				return out, e
			}
			if gap {
				out.Gaps = append(out.Gaps, historysource.Gap{RecordID: fmt.Sprintf("%s/call/%d", id, j), Category: "unsupported-tool-call"})
			}
			args, e := valueText(c.Arguments)
			if e != nil {
				return out, e
			}
			record := historysource.Record{ID: fmt.Sprintf("%s/call/%d", id, j), NativeID: id, ParentID: id, Kind: "tool-call", Role: "assistant", Text: args, CallID: c.ID, NativeCallID: c.ID, ToolName: c.Name, Offset: n.id, End: n.id + 1}
			if c.ID == "" && c.Name == "" && strings.TrimSpace(args) == "" {
				continue
			}
			out.Records = append(out.Records, record)
			callIndex := len(out.Records) - 1
			if c.ID == "" {
				out.Gaps = append(out.Gaps, historysource.Gap{RecordID: record.ID, Category: "tool-call-identity"})
				continue
			}
			var initial, update, sources sql.NullString
			e = tx.QueryRowContext(ctx, "SELECT substr(CAST(tool_call_json AS BLOB),1,4194305),substr(CAST(tool_call_update_json AS BLOB),1,4194305),substr(CAST(source_node_ids AS BLOB),1,4194305) FROM tool_call_state WHERE session_id=? AND tool_call_id=?", req.Source.NativeID, c.ID).Scan(&initial, &update, &sources)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return out, safe(ctx, e)
			}
			if e == nil {
				total += len(initial.String) + len(update.String) + len(sources.String)
				if total > historysource.MaxBytes || len(initial.String) > historysource.MaxRecordBytes || len(update.String) > historysource.MaxRecordBytes || len(sources.String) > historysource.MaxRecordBytes {
					return out, historysource.ErrLimit
				}
				_, _ = fmt.Fprintf(hash, "%q %q %q %q", c.ID, initial.String, update.String, sources.String)
				for k, raw := range []sql.NullString{initial, update} {
					if !raw.Valid || raw.String == "" {
						continue
					}
					state, gap, e := parseToolState(raw.String)
					if e != nil {
						return out, e
					}
					if gap {
						out.Gaps = append(out.Gaps, historysource.Gap{RecordID: record.ID, Category: "unsupported-tool-state"})
					}
					if state.Status != "" {
						out.Records[callIndex].Status = state.Status
					}
					input, e := valueText(state.RawInput)
					if e != nil {
						return out, e
					}
					if input != "" && input != out.Records[callIndex].Text {
						if out.Records[callIndex].Text == "" {
							out.Records[callIndex].Text = input
						} else {
							extra := record
							extra.ID = fmt.Sprintf("%s/state/%d/input", record.ID, k)
							extra.Text = input
							out.Records = append(out.Records, extra)
						}
					}
					body, gap, e := contentText(state.Content)
					if e != nil {
						return out, e
					}
					if gap {
						out.Gaps = append(out.Gaps, historysource.Gap{RecordID: record.ID, Category: "unsupported-tool-content"})
					}
					output, e := valueText(state.RawOutput)
					if e != nil {
						return out, e
					}
					for field, text := range []string{body, output} {
						if strings.TrimSpace(text) == "" || (field == 1 && text == body) {
							continue
						}
						extra := record
						extra.ID = fmt.Sprintf("%s/state/%d/%d", record.ID, k, field)
						extra.Kind, extra.Role, extra.Text = "tool-result", "tool", text
						if k == 0 && field == 0 {
							extra.Kind, extra.Role = "tool", "assistant"
						}
						extra.Status = out.Records[callIndex].Status
						out.Records = append(out.Records, extra)
					}
					if strings.TrimSpace(state.Title) != "" && out.Records[callIndex].Text == "" && body == "" && output == "" {
						out.Records[callIndex].Text = state.Title
					}
				}
			}
			for k := callIndex + 1; k < len(out.Records); k++ {
				out.Records[k].Status = out.Records[callIndex].Status
			}
			calls[c.ID] = out.Records[callIndex]
		}
		if len(out.Records)+len(out.Gaps) > historysource.MaxRecords {
			return out, historysource.ErrLimit
		}
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if req.Cursor != "" && b.Digest != digest {
		return out, historysource.ErrChanged
	}
	b.Digest = digest
	if e = tx.Commit(); e != nil {
		return out, safe(ctx, e)
	}
	after, e := os.Lstat(req.Source.Path)
	if e != nil || !os.SameFile(before, after) {
		return out, historysource.ErrChanged
	}
	raw, _ := json.Marshal(b)
	out.Cursor = base64.RawURLEncoding.EncodeToString(raw)
	out.Generation = instance
	out.Fingerprint = instance + ":" + digest
	return out, nil
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
		_ = db.Close()
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

// Discover examines only native sessions.db files under the caller's root.
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
		if d.Type()&os.ModeSymlink != 0 || d.Name() != "sessions.db" {
			return nil
		}
		db, e := open(ctx, path)
		if e != nil {
			return safe(ctx, e)
		}
		defer func() { _ = db.Close() }()
		rows, e := db.QueryContext(ctx, "SELECT id FROM sessions ORDER BY id LIMIT 100001")
		if e != nil {
			return safe(ctx, e)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				return safe(ctx, e)
			}
			if len(out) >= historysource.MaxRecords {
				return historysource.ErrLimit
			}
			out = append(out, historysource.Source{Agent: "devin", Path: path, NativeID: id})
		}
		if e = rows.Err(); e != nil {
			return safe(ctx, e)
		}
		return nil
	})
	return out, err
}
