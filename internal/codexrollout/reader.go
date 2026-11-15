// Package codexrollout reads the pinned Codex 0.77 rollout format without
// executing commands, resolving attachments, or retaining reasoning payloads.
package codexrollout

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

const Version = "codex-0.77/v1"
const MaxRecord = 4 << 20
const MaxInput = 64 << 20
const MaxRecords = 100000

var ErrLimit = errors.New("rollout resource limit")
var ErrFormat = errors.New("invalid rollout record")
var ErrVersion = errors.New("unsupported rollout version or lineage")

// ParseError reports position and category without including source content.
type ParseError struct {
	Offset int64
	Cause  error
}

func (e *ParseError) Error() string {
	return "rollout parse failure at byte " + strconv.FormatInt(e.Offset, 10) + ": " + e.Cause.Error()
}
func (e *ParseError) Unwrap() error { return e.Cause }

type Metadata struct {
	ID            string `json:"id"`
	Cwd           string `json:"cwd"`
	Timestamp     string `json:"timestamp"`
	CLIVersion    string `json:"cli_version"`
	ModelProvider string `json:"model_provider"`
}

type Diagnostic struct {
	Offset   int64
	Category string
}
type Attachment struct{ Reference, MimeType string }
type contentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL string `json:"image_url"`
}

type Item struct {
	Attachments                                          []Attachment
	Kind, Role, Text, NativeID, CallID, ToolName, Status string
}
type Record struct {
	Offset, End int64
	Hash        string
	Timestamp   string
	Items       []Item
	Diagnostics []Diagnostic
}
type Result struct {
	Metadata Metadata
	Model    string
	Consumed int64
	Pending  bool
}

type envelope struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// Read confirms newline-terminated records only. A malformed complete record
// stops at its starting byte; errors never include source bodies. Callback errors
// also leave that record unconsumed. Callbacks must stage writes transactionally.
func Read(r io.Reader, visit func(Record) error) (Result, error) {
	var out Result
	count := 0
	b := bufio.NewReaderSize(io.LimitReader(r, MaxInput+1), 64<<10)
	for {
		var line []byte
		for {
			part, err := b.ReadSlice('\n')
			if len(line)+len(part) > MaxRecord {
				return out, ErrLimit
			}
			line = append(line, part...)
			if err == bufio.ErrBufferFull {
				continue
			}
			if err != nil {
				if err != io.EOF {
					return out, err
				}
				out.Pending = len(line) > 0
				return out, nil
			}
			break
		}
		if out.Consumed+int64(len(line)) > MaxInput {
			return out, ErrLimit
		}
		count++
		if count > MaxRecords {
			return out, ErrLimit
		}
		rec := Record{Offset: out.Consumed, End: out.Consumed + int64(len(line))}
		sum := sha256.Sum256(line)
		rec.Hash = hex.EncodeToString(sum[:])
		var e envelope
		if !utf8.Valid(line) || json.Unmarshal(line, &e) != nil || len(e.Payload) == 0 {
			return out, &ParseError{Offset: out.Consumed, Cause: ErrFormat}
		}
		rec.Timestamp = e.Timestamp
		if out.Consumed == 0 && e.Type != "session_meta" {
			return out, &ParseError{Offset: out.Consumed, Cause: ErrFormat}
		}
		if err := decode(e, &out, &rec); err != nil {
			return out, &ParseError{Offset: out.Consumed, Cause: err}
		}
		if visit != nil {
			if err := visit(rec); err != nil {
				return out, err
			}
		}
		out.Consumed = rec.End
	}
}

func decode(e envelope, out *Result, rec *Record) error {
	diag := func(category string) { rec.Diagnostics = append(rec.Diagnostics, Diagnostic{rec.Offset, category}) }
	switch e.Type {
	case "session_meta":
		var fields map[string]json.RawMessage
		if json.Unmarshal(e.Payload, &fields) != nil {
			return ErrFormat
		}
		// Newer reference-based histories must not masquerade as materialized forks.
		for _, k := range []string{"history_base", "forked_from_id", "subagent_history_start_ordinal"} {
			if v, ok := fields[k]; ok && string(v) != "null" {
				return ErrVersion
			}
		}
		var m Metadata
		if json.Unmarshal(e.Payload, &m) != nil || m.ID == "" || m.Cwd == "" {
			return ErrFormat
		}
		if m.CLIVersion != "0.77.0" {
			return ErrVersion
		}
		if out.Metadata.ID == "" {
			out.Metadata = m
		} else {
			diag("inherited_session_metadata")
		}
	case "turn_context":
		var v struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(e.Payload, &v) != nil {
			return ErrFormat
		}
		out.Model = v.Model
		rec.Items = append(rec.Items, Item{Kind: "turn_start", Status: "unknown"})
	case "compacted":
		var v struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(e.Payload, &v) != nil {
			return ErrFormat
		}
		rec.Items = append(rec.Items, Item{Kind: "compaction", Role: "agent", Text: v.Message, Status: "completed"})
		diag("compaction_boundary")
	case "event_msg":
		var v struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(e.Payload, &v) != nil {
			return ErrFormat
		}
		// Message events mirror response_item and are deliberately not imported.
		switch v.Type {
		case "user_message", "agent_message", "agent_reasoning", "agent_reasoning_raw_content", "token_count", "context_compacted":
		case "turn_aborted":
			rec.Items = append(rec.Items, Item{Kind: "interrupt", Status: "cancelled"})
		case "entered_review_mode", "exited_review_mode", "undo_completed":
			diag("history_transition_" + v.Type)
		default:
			diag("unknown_event")
		}
	case "response_item":
		return response(e.Payload, rec)
	default:
		diag("unknown_record")
	}
	return nil
}

func response(raw json.RawMessage, rec *Record) error {
	var v struct {
		Type      string          `json:"type"`
		ID        string          `json:"id"`
		Role      string          `json:"role"`
		CallID    string          `json:"call_id"`
		Name      string          `json:"name"`
		Status    string          `json:"status"`
		Arguments string          `json:"arguments"`
		Input     string          `json:"input"`
		Output    json.RawMessage `json:"output"`
		Content   []contentPart   `json:"content"`
		Action    json.RawMessage `json:"action"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return ErrFormat
	}
	item := Item{NativeID: v.ID, CallID: v.CallID, ToolName: v.Name, Status: v.Status}
	if item.Status == "" {
		item.Status = "unknown"
	}
	diag := func(c string) { rec.Diagnostics = append(rec.Diagnostics, Diagnostic{rec.Offset, c}) }
	switch v.Type {
	case "message":
		if v.Role != "user" && v.Role != "assistant" {
			diag("nonexportable_message")
			return nil
		}
		item.Kind = "message"
		item.Role = v.Role
		if err := appendContent(&item, v.Content, diag); err != nil {
			return err
		}
	case "function_call":
		item.Kind = "tool_arguments"
		item.Text = v.Arguments
	case "custom_tool_call":
		item.Kind = "tool"
		item.Text = v.Input
	case "function_call_output", "custom_tool_call_output":
		item.Kind = "tool_result"
		if json.Unmarshal(v.Output, &item.Text) != nil {
			var content []contentPart
			if json.Unmarshal(v.Output, &content) != nil {
				diag("unknown_tool_output")
				return nil
			}
			if err := appendContent(&item, content, diag); err != nil {
				return err
			}
		}
	case "local_shell_call", "web_search_call":
		item.Kind = "tool"
		item.ToolName = v.Type
		item.Text = string(v.Action)
		if item.CallID == "" {
			diag("tool_without_call_id")
		}
	case "reasoning", "compaction":
		diag("nonexportable_reasoning")
		return nil
	default:
		diag("unknown_response_item")
		return nil
	}
	rec.Items = append(rec.Items, item)
	return nil
}

// Images remain opaque references; inline payloads and credential-bearing URLs
// are never copied into conversation storage or fetched by this parser.
func appendContent(item *Item, parts []contentPart, diag func(string)) error {
	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text":
			item.Text += part.Text
		case "input_image":
			sum := sha256.Sum256([]byte(part.ImageURL))
			item.Attachments = append(item.Attachments, Attachment{Reference: "native-image:sha256:" + hex.EncodeToString(sum[:]), MimeType: "application/octet-stream"})
			if len(item.Attachments) > 64 {
				return ErrLimit
			}
			diag("attachment_body_not_imported")
		default:
			diag("unknown_content")
		}
	}
	return nil
}
