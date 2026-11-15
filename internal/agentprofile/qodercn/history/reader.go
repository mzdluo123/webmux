// Package history reads QoderCN JSONL without loading runtime configuration.
package history

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rainchan/webmux/internal/historysource"
)

const ParserVersion = "qodercn-jsonl/1"

// ObservedWriterVersion is CLI evidence from the real fixtures, not a native
// format version or a connector/injection protocol version.
const ObservedWriterVersion = "1.1.84"

type envelope struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	ID        string `json:"uuid"`
	Parent    string `json:"parentUuid"`
	Leaf      string `json:"leafUuid"`
	Version   string `json:"version"`
	IsMeta    bool   `json:"isMeta"`
	Subtype   string `json:"subtype"`
	Message   struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}
type node struct {
	envelope
	offset, end int64
}
type boundary struct{ End, Fingerprint string }

func Read(ctx context.Context, req historysource.Request) (historysource.Document, error) {
	return ReadAs(ctx, req, "qodercn")
}

// ReadAs is shared with the provisional international Qoder adapter. It does
// not establish that the international writer actually uses this format.
func ReadAs(ctx context.Context, req historysource.Request, agent string) (historysource.Document, error) {
	empty := historysource.Document{Source: req.Source}
	if (agent != "qoder" && agent != "qodercn") || req.Source.Agent != agent || req.Source.NativeID == "" {
		return empty, historysource.ErrIdentity
	}
	frozen := boundary{}
	if req.Cursor != "" {
		data, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || json.Unmarshal(data, &frozen) != nil || frozen.End == "" || frozen.Fingerprint == "" {
			return empty, historysource.ErrChanged
		}
	}
	raw := req
	raw.Cursor = frozen.End
	nodes := make(map[string]node)
	leaf, version := "", ""
	identity := false
	doc, err := historysource.ReadJSONL(ctx, raw, func(data []byte, offset, end int64) error {
		var e envelope
		if json.Unmarshal(data, &e) != nil {
			return historysource.ErrFormat
		}
		if e.SessionID != "" {
			if e.SessionID != req.Source.NativeID {
				return historysource.ErrIdentity
			}
			identity = true
		}
		if e.Version != "" {
			if e.Version != ObservedWriterVersion {
				return historysource.ErrFormat
			}
			version = e.Version
		}
		if e.Type == "active-leaf" {
			leaf = e.Leaf
		}
		if e.ID != "" {
			if _, exists := nodes[e.ID]; exists {
				return historysource.ErrFormat
			}
			nodes[e.ID] = node{e, offset, end}
		}
		return nil
	})
	if err != nil {
		return empty, err
	}
	if !identity {
		return empty, historysource.ErrIdentity
	}
	if version == "" {
		return empty, historysource.ErrFormat
	}
	if frozen.Fingerprint != "" && frozen.Fingerprint != doc.Fingerprint {
		return empty, historysource.ErrChanged
	}
	if req.Source.Branch != "" {
		leaf = req.Source.Branch
	}
	if leaf == "" {
		return empty, historysource.ErrFormat
	}
	doc.Source.Branch = leaf
	doc.FormatVersion = "qoder-jsonl/unversioned"
	doc.ParserVersion = ParserVersion
	b, _ := json.Marshal(boundary{doc.Cursor, doc.Fingerprint})
	doc.Cursor = base64.RawURLEncoding.EncodeToString(b)
	chain := make([]node, 0)
	seen := make(map[string]bool)
	for id := leaf; id != ""; {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if seen[id] {
			doc.Gaps = append(doc.Gaps, historysource.Gap{RecordID: id, Category: "cycle"})
			break
		}
		seen[id] = true
		n, ok := nodes[id]
		if !ok {
			doc.Gaps = append(doc.Gaps, historysource.Gap{RecordID: id, Category: "external-ancestor"})
			break
		}
		chain = append(chain, n)
		id = n.Parent
	}
	calls := make(map[string]string)
	for i := len(chain) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		n := chain[i]
		r := historysource.Record{ID: n.ID, NativeID: n.ID, ParentID: n.Parent, Offset: n.offset, End: n.end}
		switch n.Type {
		case "user", "assistant":
			if n.Message.Role != n.Type {
				return empty, historysource.ErrFormat
			}
			if n.IsMeta {
				r.Kind = "metadata"
				doc.Records = append(doc.Records, r)
				continue
			}
			r.Kind = "message"
			r.Role = n.Type
			var text string
			if json.Unmarshal(n.Message.Content, &text) == nil {
				r.Text = text
			} else {
				var parts []struct {
					Type, Text, ID, Name string
					ToolUseID            string          `json:"tool_use_id"`
					Content              json.RawMessage `json:"content"`
					IsError              bool            `json:"is_error"`
				}
				if json.Unmarshal(n.Message.Content, &parts) != nil {
					return empty, historysource.ErrFormat
				}
				for j, p := range parts {
					if err := ctx.Err(); err != nil {
						return empty, err
					}
					switch p.Type {
					case "text":
						if p.Text != "" {
							textRecord := r
							textRecord.ID = n.ID + ":" + strconv.Itoa(j)
							textRecord.Text = p.Text
							doc.Records = append(doc.Records, textRecord)
						}
					case "tool_use", "tool_result":
						tool := historysource.Record{ID: n.ID + ":" + strconv.Itoa(j), NativeID: n.ID, ParentID: n.ID, Kind: p.Type, Role: n.Type, CallID: p.ID, ToolName: p.Name, Offset: n.offset, End: n.end}
						if p.Type == "tool_use" {
							calls[p.ID] = p.Name
						} else {
							tool.Role = "tool"
							tool.CallID = p.ToolUseID
							tool.ToolName = calls[p.ToolUseID]
							tool.Status = "completed"
							if p.IsError {
								tool.Status = "error"
							}
							if json.Unmarshal(p.Content, &tool.Text) != nil {
								var output []struct{ Type, Text string }
								if json.Unmarshal(p.Content, &output) != nil {
									return empty, historysource.ErrFormat
								}
								var text strings.Builder
								for _, part := range output {
									if part.Type == "text" {
										text.WriteString(part.Text)
									}
								}
								tool.Text = text.String()
							}
						}
						doc.Records = append(doc.Records, tool)
					}
					if len(doc.Records) > historysource.MaxRecords {
						return empty, historysource.ErrLimit
					}
				}
			}
			if r.Text != "" {
				doc.Records = append(doc.Records, r)
			}
		case "system", "attachment":
			r.Kind = "metadata"
			if n.Subtype == "compact_boundary" {
				r.Kind = "compaction"
				doc.Gaps = append(doc.Gaps, historysource.Gap{RecordID: n.ID, Category: "compaction", Offset: n.offset})
			}
			// Native metadata bodies may contain configuration and are never returned.
			doc.Records = append(doc.Records, r)
		default:
			doc.Gaps = append(doc.Gaps, historysource.Gap{RecordID: n.ID, Category: "unsupported-record", Offset: n.offset})
		}
	}
	if len(doc.Records) > historysource.MaxRecords {
		return empty, historysource.ErrLimit
	}
	return doc, nil
}

func Discover(ctx context.Context, root string) ([]historysource.Source, error) {
	return DiscoverAs(ctx, root, "qodercn")
}

// DiscoverAs scans only the explicitly supplied projects data root.
func DiscoverAs(ctx context.Context, root, agent string) ([]historysource.Source, error) {
	if agent != "qoder" && agent != "qodercn" {
		return nil, historysource.ErrIdentity
	}
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
			if strings.Count(rel, string(filepath.Separator)) >= 3 {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		id := ""
		_, err = historysource.ReadJSONL(ctx, historysource.Request{Source: historysource.Source{Path: path}}, func(data []byte, _, _ int64) error {
			var e envelope
			if json.Unmarshal(data, &e) != nil {
				return historysource.ErrFormat
			}
			if e.SessionID != "" {
				if id != "" && id != e.SessionID {
					return historysource.ErrIdentity
				}
				id = e.SessionID
			}
			return nil
		})
		if err != nil {
			return err
		}
		if id != "" {
			sources = append(sources, historysource.Source{Agent: agent, Path: path, NativeID: id})
		}
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
