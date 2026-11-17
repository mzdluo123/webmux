// Package history reads omp's native session JSONL without runtime capabilities.
// Format evidence: #653 fixtures and the local 18.22.0 capture, session schema 3.
// CLI release numbers are evidence; session schema selects parser compatibility.
package history

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	hs "github.com/rainchan/webmux/internal/historysource"
)

const ParserVersion = "omp-jsonl-4"
const FormatVersion = "omp-session-3"

type entry struct {
	Type          string  `json:"type"`
	ID            string  `json:"id"`
	Parent        string  `json:"parentId"`
	Version       int     `json:"version"`
	ParentSession string  `json:"parentSession"`
	Summary       string  `json:"summary"`
	First         string  `json:"firstKeptEntryId"`
	Through       string  `json:"providerReplayThroughEntryId"`
	Message       message `json:"message"`
	offset, end   int64
}
type message struct {
	Role               string          `json:"role"`
	Content            json.RawMessage `json:"content"`
	CallID             string          `json:"toolCallId"`
	Tool               string          `json:"toolName"`
	Error              bool            `json:"isError"`
	Stop               string          `json:"stopReason"`
	Command            string          `json:"command"`
	Code               string          `json:"code"`
	Output             string          `json:"output"`
	ExitCode           *int            `json:"exitCode"`
	Cancelled          bool            `json:"cancelled"`
	Truncated          bool            `json:"truncated"`
	ExcludeFromContext bool            `json:"excludeFromContext"`
	Files              []struct {
		Path string `json:"path"`
	} `json:"files"`
}
type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Mime      string          `json:"mimeType"`
	Arguments json.RawMessage `json:"arguments"`
}
type boundary struct{ Cursor, Fingerprint, Branch, NativeID string }

// Read preserves native entry IDs and parent edges. Branch names an exact leaf;
// when omitted, the last native entry is the active leaf of the captured prefix.
func Read(ctx context.Context, req hs.Request) (hs.Document, error) {
	if err := ctx.Err(); err != nil {
		return hs.Document{}, err
	}
	if req.Source.Agent != "omp" || req.Source.NativeID == "" {
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
	external := false
	doc, err := hs.ReadJSONL(ctx, req, func(line []byte, off, end int64) error {
		if !utf8.Valid(line) {
			return hs.ErrFormat
		}
		var e entry
		if json.Unmarshal(line, &e) != nil {
			return hs.ErrFormat
		}
		if e.Type == "session" {
			if identity != "" || e.Version != 3 {
				return hs.ErrFormat
			}
			identity = e.ID
			external = e.ParentSession != ""
			return nil
		}
		if e.ID == "" {
			if e.Type == "title" {
				return nil
			}
			return hs.ErrFormat
		}
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
	if identity != original.Source.NativeID {
		return hs.Document{}, hs.ErrIdentity
	}
	if frozen.Fingerprint != "" && frozen.Fingerprint != doc.Fingerprint {
		return hs.Document{}, hs.ErrChanged
	}
	leaf := req.Source.Branch
	if leaf == "" && len(entries) > 0 {
		leaf = entries[len(entries)-1].ID
	}
	stopAt := ""
	selected := make(map[string]bool)
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
			if id == leaf {
				return hs.Document{}, hs.ErrIdentity
			}
			gaps = append(gaps, hs.Gap{RecordID: id, Category: "external_parent"})
			break
		}
		selected[id] = true
		if stopAt == "" && entries[i].Type == "compaction" {
			stopAt = entries[i].First
		}
		if id == stopAt {
			break
		}
		id = entries[i].Parent
	}
	var records []hs.Record
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
		r := hs.Record{ID: e.ID, NativeID: e.ID, ParentID: e.Parent, Kind: "metadata", Offset: e.offset, End: e.end}
		switch e.Type {
		case "compaction":
			r.Kind = "compaction"
			r.Role = "system"
			r.Text = e.Summary
			gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "compaction", Offset: e.offset})
			for _, ref := range []struct{ id, category string }{{e.First, "first_kept"}, {e.Through, "provider_replay_through"}} {
				if ref.id == "" {
					continue
				}
				category := ref.category
				if !selected[ref.id] {
					category = "external_" + category
				}
				gaps = append(gaps, hs.Gap{RecordID: ref.id, Category: category, Offset: e.offset})
			}
		case "message":
			r.Kind = "message"
			r.Role = e.Message.Role
			r.Status = e.Message.Stop
			switch r.Role {
			case "user", "assistant", "toolResult", "developer":
			case "custom":
				r.Role = "system"
			case "bashExecution", "pythonExecution":
				if e.Message.ExcludeFromContext {
					r.Kind, r.Role, r.Status = "metadata", "", ""
					gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "excluded_from_context", Offset: e.offset})
					break
				}
				r.Kind, r.Role = "tool_result", "tool"
				r.ToolName, r.Text = "bash", e.Message.Command
				if e.Message.Role == "pythonExecution" {
					r.ToolName, r.Text = "python", e.Message.Code
				}
				r.Text += "\n" + e.Message.Output
				r.Status = "completed"
				if e.Message.Cancelled {
					r.Status = "cancelled"
				} else if e.Message.ExitCode == nil {
					r.Status = "unknown"
				} else if *e.Message.ExitCode != 0 {
					r.Status = "error"
				}
				if e.Message.Truncated {
					gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "truncated_output", Offset: e.offset})
				}
			case "fileMention":
				r.Role = "user"
				for _, file := range e.Message.Files {
					r.Attachments = append(r.Attachments, hs.Attachment{Name: file.Path})
				}
				gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "attachment_body_omitted", Offset: e.offset})
			default:
				return hs.Document{}, hs.ErrFormat
			}
			if r.Role == "toolResult" {
				r.Role = "tool"
				r.Kind = "tool_result"
				r.CallID = e.Message.CallID
				r.ToolName = e.Message.Tool
				r.Status = "completed"
				if e.Message.Error {
					r.Status = "error"
				}
			}
			// Execution and file-mention payloads have dedicated safe projections.
			if e.Message.Role == "bashExecution" || e.Message.Role == "pythonExecution" || e.Message.Role == "fileMention" {
				break
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
					case "toolCall":
						records = append(records, hs.Record{ID: blockRecordID(e.ID, position, index), NativeID: e.ID, ParentID: e.Parent, Kind: "tool_call", Role: "assistant", Text: string(b.Arguments), CallID: b.ID, ToolName: b.Name, Status: "requested", Offset: e.offset, End: e.end})
					case "image", "file":
						r.Attachments = append(r.Attachments, hs.Attachment{MimeType: b.Mime})
						gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "attachment_body_omitted", Offset: e.offset})
					case "thinking", "reasoning", "redacted_thinking": // Deliberately omit private reasoning and signatures.
					default:
						gaps = append(gaps, hs.Gap{RecordID: e.ID, Category: "unsupported_content", Offset: e.offset})
					}
				}
			}
		default: // Keep graph nodes, never custom arguments, credentials or provider payloads.
		}
		records = append(records, r)
		if e.Parent != "" && !selected[e.Parent] {
			gaps = append(gaps, hs.Gap{RecordID: e.Parent, Category: "external_parent", Offset: e.offset})
		}
	}
	if external {
		gaps = append(gaps, hs.Gap{Category: "external_session"})
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

// Discover walks only the caller-provided sessions root. It follows no symlinks,
// caps directory entries and depth, and validates identity from native headers.
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
		id, e := discoverIdentity(ctx, path)
		if e != nil {
			return e
		}
		if id == "" {
			return nil
		}
		out = append(out, hs.Source{Agent: "omp", Path: path, NativeID: id})
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

// Discovery reads the bounded title/header prefix only. Read validates the
// selected session's schema and body after runtime identity is unique.
func discoverIdentity(ctx context.Context, path string) (string, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() {
		return "", nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", nil
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !os.SameFile(before, info) {
		return "", hs.ErrChanged
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 64<<10)
	for range 4 {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if !scanner.Scan() {
			return "", nil
		}
		var header struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}
		if json.Unmarshal(scanner.Bytes(), &header) != nil {
			return "", nil
		}
		switch header.Type {
		case "title":
			continue
		case "session":
			return header.ID, nil
		default:
			return "", nil
		}
	}
	return "", nil
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
