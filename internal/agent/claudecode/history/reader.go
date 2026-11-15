// Package history reads Claude Code native transcripts, independently of ACP.
// Evidence: #653 native version 2.1.316, ~/.claude/projects JSONL layout.
// CLI, connector and parser versions are independent; no CLI is launched here.
package history

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	hs "github.com/rainchan/webmux/internal/historysource"
)

const ParserVersion = "claude-jsonl-2"
const FormatVersion = "claude-project-transcript"

type entry struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	ID      string `json:"uuid"`
	Parent  string `json:"parentUuid"`
	Logical string `json:"logicalParentUuid"`
	Session string `json:"sessionId"`
	Agent   string `json:"agentId"`
	Side    bool   `json:"isSidechain"`
	Content string `json:"content"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
		Stop    string          `json:"stop_reason"`
	} `json:"message"`
	Compact struct {
		Preserved struct {
			IDs    []string `json:"uuids"`
			Anchor string   `json:"anchorUuid"`
		} `json:"preservedMessages"`
		Segment struct {
			Head   string `json:"headUuid"`
			Tail   string `json:"tailUuid"`
			Anchor string `json:"anchorUuid"`
		} `json:"preservedSegment"`
	} `json:"compactMetadata"`
	offset, end int64
}
type block struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	CallID  string          `json:"tool_use_id"`
	Error   bool            `json:"is_error"`
	Content json.RawMessage `json:"content"`
	Source  struct {
		Type string `json:"type"`
		Mime string `json:"media_type"`
	} `json:"source"`
}
type boundary struct{ Cursor, Fingerprint, Branch, NativeID string }

// Read selects one exact native ancestor graph. Explicit compaction preservation
// lists are authoritative, including parallel tool results, not adjacent lines.
func Read(ctx context.Context, req hs.Request) (hs.Document, error) {
	if err := ctx.Err(); err != nil {
		return hs.Document{}, err
	}
	if req.Source.Agent != "claudecode" || req.Source.NativeID == "" {
		return hs.Document{}, hs.ErrIdentity
	}
	original := req
	var frozen boundary
	if req.Cursor != "" {
		data, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || json.Unmarshal(data, &frozen) != nil || frozen.Cursor == "" || frozen.NativeID != req.Source.NativeID || (req.Source.Branch != "" && req.Source.Branch != frozen.Branch) {
			return hs.Document{}, hs.ErrChanged
		}
		req.Cursor = frozen.Cursor
		req.Source.Branch = frozen.Branch
	}
	entries := make([]entry, 0)
	index := make(map[string]int)
	identity := ""
	doc, err := hs.ReadJSONL(ctx, req, func(line []byte, off, end int64) error {
		if !utf8.Valid(line) {
			return hs.ErrFormat
		}
		var e entry
		if json.Unmarshal(line, &e) != nil {
			return hs.ErrFormat
		}
		if e.Session != "" {
			if identity != "" && identity != e.Session {
				return hs.ErrIdentity
			}
			identity = e.Session
		}
		if e.ID == "" {
			return nil
		} // Progress and file snapshots are not conversation graph nodes.
		if _, exists := index[e.ID]; exists {
			return hs.ErrFormat
		}
		e.offset, e.end = off, end
		index[e.ID] = len(entries)
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return hs.Document{}, err
	}
	if identity != req.Source.NativeID {
		return hs.Document{}, hs.ErrIdentity
	}
	if frozen.Fingerprint != "" && frozen.Fingerprint != doc.Fingerprint {
		return hs.Document{}, hs.ErrChanged
	}
	leaf := req.Source.Branch
	if leaf == "" && len(entries) > 0 {
		leaf = entries[len(entries)-1].ID
	}
	li, ok := index[leaf]
	if !ok {
		return hs.Document{}, hs.ErrIdentity
	}
	group := entries[li]
	selected := make(map[string]bool)
	preserved := false
	var gaps []hs.Gap
	for id := leaf; id != ""; {
		if err := ctx.Err(); err != nil {
			return hs.Document{}, err
		}
		if selected[id] {
			return hs.Document{}, hs.ErrFormat
		}
		i, ok := index[id]
		if !ok {
			gaps = append(gaps, hs.Gap{RecordID: id, Category: "external_parent"})
			break
		}
		e := entries[i]
		if e.Side != group.Side || e.Agent != group.Agent {
			gaps = append(gaps, hs.Gap{RecordID: id, Category: "external_branch", Offset: e.offset})
			break
		}
		selected[id] = true
		if e.Subtype == "compact_boundary" {
			preserved = true
			gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "compaction", Offset: e.offset})
			for _, keep := range e.Compact.Preserved.IDs {
				k, ok := index[keep]
				if !ok {
					gaps = append(gaps, hs.Gap{RecordID: keep, Category: "external_preserved", Offset: e.offset})
					continue
				}
				if entries[k].Side != group.Side || entries[k].Agent != group.Agent || k >= i {
					return hs.Document{}, hs.ErrFormat
				}
				selected[keep] = true
			}
			for _, ref := range []string{e.Compact.Preserved.Anchor, e.Compact.Segment.Anchor} {
				if ref != "" && !selected[ref] {
					gaps = append(gaps, hs.Gap{RecordID: ref, Category: "external_compaction_anchor", Offset: e.offset})
				}
			}
			if e.Logical != "" {
				gaps = append(gaps, hs.Gap{RecordID: e.Logical, Category: "logical_parent", Offset: e.offset})
			}
			break
		}
		id = e.Parent
	}
	var records []hs.Record
	calls := make(map[string]string)
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return hs.Document{}, err
		}
		if len(records) >= hs.MaxRecords {
			return hs.Document{}, hs.ErrLimit
		}
		if !selected[e.ID] {
			continue
		}
		r := hs.Record{ID: e.ID, NativeID: e.ID, ParentID: e.Parent, Kind: "message", Role: e.Message.Role, Status: e.Message.Stop, Offset: e.offset, End: e.end}
		if e.Subtype == "compact_boundary" {
			r.Kind = "compaction"
			r.Role = "system"
			r.Text = e.Content
			records = append(records, r)
			continue
		}
		if e.Type != "user" && e.Type != "assistant" {
			r.Kind = "metadata"
			r.Role = ""
			records = append(records, r)
			continue
		}
		var text string
		if json.Unmarshal(e.Message.Content, &text) == nil {
			r.Text = text
		} else {
			var blocks []block
			if len(e.Message.Content) > 0 && json.Unmarshal(e.Message.Content, &blocks) != nil {
				return hs.Document{}, hs.ErrFormat
			}
			for position, b := range blocks {
				if len(records) >= hs.MaxRecords-1 {
					return hs.Document{}, hs.ErrLimit
				}
				switch b.Type {
				case "text":
					if r.Text != "" {
						r.Text += "\n"
					}
					r.Text += b.Text
				case "tool_use":
					calls[b.ID] = b.Name
					records = append(records, hs.Record{ID: blockRecordID(e.ID, position, index), NativeID: e.ID, ParentID: e.Parent, Kind: "tool_call", Role: "assistant", CallID: b.ID, ToolName: b.Name, Status: "requested", Offset: e.offset, End: e.end})
				case "tool_result":
					result := hs.Record{ID: blockRecordID(e.ID, position, index), NativeID: e.ID, ParentID: e.Parent, Kind: "tool_result", Role: "tool", CallID: b.CallID, ToolName: calls[b.CallID], Status: "completed", Offset: e.offset, End: e.end}
					if b.Error {
						result.Status = "error"
					}
					if result.ToolName == "" {
						gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "external_tool_call", Offset: e.offset})
					}
					if json.Unmarshal(b.Content, &result.Text) != nil {
						var nested []block
						if json.Unmarshal(b.Content, &nested) != nil {
							return hs.Document{}, hs.ErrFormat
						}
						for _, n := range nested {
							if n.Type == "text" {
								if result.Text != "" {
									result.Text += "\n"
								}
								result.Text += n.Text
							} else if n.Type == "image" || n.Type == "document" {
								result.Attachments = append(result.Attachments, hs.Attachment{MimeType: n.Source.Mime})
								gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "attachment_body_omitted", Offset: e.offset})
							}
						}
					}
					records = append(records, result)
				case "image", "document":
					r.Attachments = append(r.Attachments, hs.Attachment{MimeType: b.Source.Mime})
					gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "attachment_body_omitted", Offset: e.offset})
				case "thinking", "redacted_thinking": // Never expose reasoning, signatures or encrypted bodies.
				default:
					gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "unsupported_content", Offset: e.offset})
				}
			}
		}
		records = append(records, r)
		if preserved && e.Parent != "" && !selected[e.Parent] {
			gaps = append(gaps, hs.Gap{RecordID: e.Parent, Category: "external_parent", Offset: e.offset})
		}
	}
	doc.Source = original.Source
	doc.Source.Branch = leaf
	doc.FormatVersion = FormatVersion
	doc.ParserVersion = ParserVersion
	doc.Records = records
	doc.Gaps = gaps
	data, _ := json.Marshal(boundary{doc.Cursor, doc.Fingerprint, leaf, identity})
	doc.Cursor = base64.RawURLEncoding.EncodeToString(data)
	return doc, nil
}

// Discover accepts an explicit projects root, never guesses cwd/title matches,
// follows no symlinks, and bounds depth, visited entries and returned sessions.
func Discover(ctx context.Context, root string) ([]hs.Source, error) {
	var out []hs.Source
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if err != nil {
			return hs.ErrUnavailable
		}
		count++
		if count > 10000 {
			return hs.ErrLimit
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return hs.ErrUnavailable
		}
		if strings.Count(rel, string(filepath.Separator)) > 5 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		id := ""
		_, e = hs.ReadJSONL(ctx, hs.Request{Source: hs.Source{Path: path}}, func(line []byte, _, _ int64) error {
			var h struct {
				Session string `json:"sessionId"`
			}
			if json.Unmarshal(line, &h) != nil {
				return hs.ErrFormat
			}
			if h.Session != "" {
				if id != "" && id != h.Session {
					return hs.ErrIdentity
				}
				id = h.Session
			}
			return nil
		})
		if e != nil {
			return e
		}
		if id == "" {
			return nil
		}
		doc, e := Read(ctx, hs.Request{Source: hs.Source{Agent: "claudecode", Path: path, NativeID: id}})
		if e != nil {
			return e
		}
		out = append(out, doc.Source)
		if len(out) > 1000 {
			return hs.ErrLimit
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Keep the native message node for parent edges; projected blocks get an
// injective, position-based namespace that cannot collide with native nodes.
func blockRecordID(native string, position int, nodes map[string]int) string {
	id := "block:" + base64.RawURLEncoding.EncodeToString([]byte(native)) + ":" + strconv.Itoa(position)
	for {
		if _, exists := nodes[id]; !exists {
			return id
		}
		id += ":"
	}
}
