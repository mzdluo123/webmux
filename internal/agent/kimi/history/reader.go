// Package history reads Kimi wire protocols 1.4 and 1.5.
package history

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rainchan/webmux/internal/historysource"
)

const ParserVersion = "kimi-wire/2"

type boundary struct{ End, Fingerprint, State, Parser string }
type part struct{ Type, Text string }
type message struct {
	ID      string `json:"id"`
	UUID    string `json:"uuid"`
	Role    string `json:"role"`
	Content []part `json:"content"`
	Origin  struct {
		Kind string `json:"kind"`
	} `json:"origin"`
}
type event struct {
	Type      string          `json:"type"`
	UUID      string          `json:"uuid"`
	Parent    string          `json:"parentUuid"`
	Step      string          `json:"stepUuid"`
	CallID    string          `json:"toolCallId"`
	Name      string          `json:"name"`
	Finish    string          `json:"finishReason"`
	Args      json.RawMessage `json:"args"`
	MessageID string          `json:"messageId"`
	Part      part            `json:"part"`
	Result    struct {
		Output  string `json:"output"`
		Note    string `json:"note"`
		IsError bool   `json:"isError"`
	} `json:"result"`
}

// identity validates the session directory and state relationship, not title or
// working directory. Relocated exports must relocate state.homedir as well.
func identity(ctx context.Context, source historysource.Source) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if source.Agent != "kimi" || source.NativeID == "" || (source.Branch != "" && source.Branch != "main") {
		return nil, historysource.ErrIdentity
	}
	path, err := filepath.Abs(source.Path)
	if err != nil {
		return nil, historysource.ErrUnavailable
	}
	home := filepath.Dir(path)
	session := filepath.Dir(filepath.Dir(home))
	if filepath.Base(path) != "wire.jsonl" || filepath.Base(home) != "main" || filepath.Base(filepath.Dir(home)) != "agents" || filepath.Base(session) != source.NativeID {
		return nil, historysource.ErrIdentity
	}
	statePath := filepath.Join(session, "state.json")
	info, err := os.Lstat(statePath)
	if err != nil || !info.Mode().IsRegular() {
		return nil, historysource.ErrUnavailable
	}
	f, err := os.Open(statePath)
	if err != nil {
		return nil, historysource.ErrUnavailable
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, historysource.MaxRecordBytes+1))
	if err != nil {
		return nil, historysource.ErrUnavailable
	}
	if len(data) > historysource.MaxRecordBytes {
		return nil, historysource.ErrLimit
	}
	var state struct {
		ID     string `json:"id"`
		Agents map[string]struct {
			Home   string  `json:"homedir"`
			Type   string  `json:"type"`
			Parent *string `json:"parentAgentId"`
		} `json:"agents"`
	}
	if json.Unmarshal(data, &state) != nil {
		return nil, historysource.ErrFormat
	}
	if state.ID != "" && state.ID != source.NativeID {
		return nil, historysource.ErrIdentity
	}
	main, ok := state.Agents["main"]
	if !ok || main.Type != "main" || main.Parent != nil || !filepath.IsAbs(main.Home) || filepath.Clean(main.Home) != home {
		return nil, historysource.ErrIdentity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func Read(ctx context.Context, req historysource.Request) (historysource.Document, error) {
	empty := historysource.Document{Source: req.Source}
	state, err := identity(ctx, req.Source)
	if err != nil {
		return empty, err
	}
	var header struct {
		Version *int `json:"version"`
	}
	if json.Unmarshal(state, &header) != nil || (header.Version != nil && *header.Version != 2) {
		return empty, historysource.ErrFormat
	}
	sum := sha256.Sum256(state)
	stateHash := hex.EncodeToString(sum[:])
	frozen := boundary{}
	if req.Cursor != "" {
		data, e := base64.RawURLEncoding.DecodeString(req.Cursor)
		if e != nil || json.Unmarshal(data, &frozen) != nil || frozen.End == "" || frozen.Fingerprint == "" || frozen.State != stateHash || frozen.Parser != ParserVersion {
			return empty, historysource.ErrChanged
		}
	}
	raw := req
	raw.Cursor = frozen.End
	records := []historysource.Record{}
	gaps := []historysource.Gap{}
	seen := make(map[string]bool)
	calls := make(map[string]string)
	messageIndex := make(map[string]int)
	representedMessages := make(map[string]bool)
	stepText := make(map[string]bool)
	protocol := ""
	doc, err := historysource.ReadJSONL(ctx, raw, func(data []byte, offset, end int64) error {
		var row struct {
			Type    string  `json:"type"`
			Version string  `json:"protocol_version"`
			AgentID string  `json:"agentId"`
			Message message `json:"message"`
			Event   event   `json:"event"`
		}
		if json.Unmarshal(data, &row) != nil {
			return historysource.ErrFormat
		}
		if offset == 0 {
			if row.Type != "metadata" || (row.Version != "1.4" && row.Version != "1.5") {
				return historysource.ErrFormat
			}
			protocol = row.Version
			return nil
		}
		r := historysource.Record{ID: "wire:" + strconv.FormatInt(offset, 10), Offset: offset, End: end}
		switch row.Type {
		case "metadata":
			return historysource.ErrFormat
		}
		// Child-agent events and injected runtime instructions are not user history.
		if row.AgentID != "" && row.AgentID != "main" {
			return nil
		}
		switch row.Type {
		case "context.append_message":
			m := row.Message
			if m.ID != "" {
				r.ID = m.ID
			} else if m.UUID != "" {
				r.ID = m.UUID
			}
			if m.Origin.Kind == "injection" {
				return nil
			}
			if m.Role == "user" && m.Origin.Kind != "" && m.Origin.Kind != "user" {
				gaps = append(gaps, historysource.Gap{RecordID: r.ID, Category: "unsupported-message-origin", Offset: offset})
				return nil
			}
			if m.Role != "user" && m.Role != "assistant" && m.Role != "tool" {
				return nil
			}
			if representedMessages[r.ID] {
				return nil
			}
			key := "message:" + r.ID
			if seen[key] {
				return nil
			}
			seen[key] = true
			r.Kind = "message"
			r.Role = m.Role
			var text strings.Builder
			for _, p := range m.Content {
				if p.Type == "text" {
					text.WriteString(p.Text)
				}
			}
			r.Text = text.String()
			if r.Text != "" {
				messageIndex[r.ID] = len(records)
				records = append(records, r)
			}
		case "context.append_loop_event":
			e := row.Event
			key := e.Type + ":" + e.UUID
			if e.UUID == "" || e.Type == "tool.result" {
				key = e.Type + ":" + e.CallID
			}
			if e.UUID != "" || e.CallID != "" {
				if seen[key] {
					return nil
				}
				seen[key] = true
			}
			if e.UUID != "" {
				r.ID = e.UUID
			}
			r.ParentID = e.Step
			switch e.Type {
			case "content.part":
				if e.Part.Type == "text" {
					r.Kind = "message"
					r.Role = "assistant"
					r.Text = e.Part.Text
					stepText[e.Step] = true
					records = append(records, r)
				}
			case "tool.call":
				if e.CallID == "" {
					return historysource.ErrFormat
				}
				r.Kind = "tool_arguments"
				r.Role = "assistant"
				r.CallID = e.CallID
				r.ToolName = e.Name
				if len(e.Args) != 0 && !bytes.Equal(e.Args, []byte("null")) {
					r.Text = string(e.Args)
				}
				calls[e.CallID] = e.Name
				records = append(records, r)
			case "tool.result":
				if e.CallID == "" {
					return historysource.ErrFormat
				}
				r.Kind = "tool_result"
				r.Role = "tool"
				r.CallID = e.CallID
				r.ParentID = e.Parent
				r.ToolName = calls[e.CallID]
				r.Text = e.Result.Output
				if e.Result.Note != "" {
					if r.Text != "" {
						r.Text += "\n"
					}
					r.Text += e.Result.Note
				}
				r.Status = "completed"
				if e.Result.IsError {
					r.Status = "error"
				}
				if r.ToolName == "" {
					gaps = append(gaps, historysource.Gap{RecordID: r.ID, Category: "missing-tool-call", Offset: offset})
				}
				records = append(records, r)
			case "step.end":
				if e.MessageID != "" && stepText[e.UUID] {
					representedMessages[e.MessageID] = true
					if index, ok := messageIndex[e.MessageID]; ok {
						records[index].Kind = ""
					}
				}
				r.Kind = "status"
				r.Status = e.Finish
				records = append(records, r)
			case "step.begin":
			default:
				gaps = append(gaps, historysource.Gap{RecordID: r.ID, Category: "unsupported-event", Offset: offset})
			}
		default:
			if strings.HasPrefix(row.Type, "context.") {
				gaps = append(gaps, historysource.Gap{RecordID: r.ID, Category: "unsupported-context-mutation", Offset: offset})
			}
			// turn.prompt and agent.message.appended mirror canonical context records.
			// Profile/config, thinking and wire bookkeeping are never transcript bodies.
		}
		return nil
	})
	if err != nil {
		return empty, err
	}
	if protocol == "" {
		return empty, historysource.ErrFormat
	}
	if frozen.Fingerprint != "" && frozen.Fingerprint != doc.Fingerprint {
		return empty, historysource.ErrChanged
	}
	after, err := identity(ctx, req.Source)
	if err != nil {
		return empty, err
	}
	if !bytes.Equal(state, after) {
		return empty, historysource.ErrChanged
	}
	doc.Source.Branch = "main"
	doc.FormatVersion = "kimi-wire/" + protocol
	doc.ParserVersion = ParserVersion
	b, _ := json.Marshal(boundary{End: doc.Cursor, Fingerprint: doc.Fingerprint, State: stateHash, Parser: ParserVersion})
	doc.Cursor = base64.RawURLEncoding.EncodeToString(b)
	doc.Fingerprint += ":" + stateHash
	kept := records[:0]
	for _, r := range records {
		if r.Kind != "" {
			kept = append(kept, r)
		}
	}
	records = kept
	doc.Records = records
	doc.Gaps = gaps
	return doc, nil
}

// Discover expects the Kimi sessions root, not a guessed home or workspace.
func Discover(ctx context.Context, root string) ([]historysource.Source, error) {
	var sources []historysource.Source
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return historysource.ErrUnavailable
		}
		count++
		if count > 10000 {
			return historysource.ErrLimit
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return historysource.ErrUnavailable
		}
		if d.IsDir() {
			if strings.Count(rel, string(filepath.Separator)) >= 5 {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || d.Name() != "wire.jsonl" || filepath.Base(filepath.Dir(path)) != "main" {
			return nil
		}
		s := historysource.Source{Agent: "kimi", Path: path, NativeID: filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path)))), Branch: "main"}
		if _, err := identity(ctx, s); err != nil {
			return err
		}
		sources = append(sources, s)
		if len(sources) > 1000 {
			return historysource.ErrLimit
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sources, nil
}
