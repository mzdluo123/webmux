package mcpcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/mcpprotocol"
)

var generation = regexp.MustCompile(`^[0-9a-f]{16}$`)
var decimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,19})$`)

func unsigned(s string) (uint64, error) {
	if !decimal.MatchString(s) {
		return 0, errors.New("invalid cursor")
	}
	return strconv.ParseUint(s, 10, 64)
}
func (c Client) readTools() []mcpprotocol.Tool {
	views := []string{"terminal"}
	description := "Read a bounded raw terminal replay page encoded as base64. Cursors are decimal strings. Reports truncation and resets; never acquires terminal control."
	if c.hasCapability("chat.acp") {
		views = append(views, "activity")
		description = "Read committed exportable chat activity, or a bounded raw terminal replay page encoded as base64. Cursors are decimal strings. Reports truncation, resets and missing history; never acquires terminal control."
	}
	properties := map[string]any{
		"sessionId":  sessionIDSchema(),
		"view":       desc(enumeration(views...), "terminal: raw PTY replay bytes; activity (when available): committed chat items."),
		"generation": desc(text(), "terminal view: replay generation from a previous page; supply generation and offset together."),
		"offset":     desc(text(), "terminal view: decimal cursor, pass back the previous page's nextOffset."),
		"maxBytes":   desc(integer(1, 65536), "terminal view: bytes per page (default 65536)."),
	}
	if c.hasCapability("chat.acp") {
		properties["snapshotId"] = desc(text(), "activity view: frozen snapshotId from the first page; pass it for subsequent pages.")
		properties["afterPosition"] = desc(text(), "activity view: return items after this position (decimal string; \"0\" reads from the start).")
		properties["itemId"] = desc(text(), "activity view: read a single item by id, to continue a truncated item.")
		properties["textOffset"] = desc(text(), "activity view: UTF-8 byte offset inside itemId (decimal string).")
		properties["limit"] = desc(integer(1, 100), "activity view: items per page (default 20).")
		properties["maxBytesPerItem"] = desc(integer(4, 65536), "activity view: per-item byte cap (default 4000).")
	}
	return []mcpprotocol.Tool{tool("session_read", description, true, properties, []string{"sessionId"}, c.read)}
}

func (c Client) read(ctx context.Context, raw json.RawMessage) (any, error) {
	a := contract.MCPSessionReadRequest{View: "activity", AfterPosition: "0", TextOffset: "0", Limit: 20, MaxBytesPerItem: 4000, MaxBytes: 65536}
	if !c.hasCapability("chat.acp") {
		a.View = "terminal"
	}
	if decode(raw, &a) != nil || !identity.MatchString(a.SessionID) {
		return nil, errors.New("invalid session_read arguments")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	switch a.View {
	case "activity":
		if a.Generation != "" || a.Offset != "" || a.MaxBytes != 65536 {
			return nil, errors.New("terminal cursors require terminal view")
		}
		return c.readActivity(ctx, a)
	case "terminal":
		if a.AfterPosition != "0" || a.TextOffset != "0" || a.ItemID != "" || a.Limit != 20 || a.MaxBytesPerItem != 4000 {
			return nil, errors.New("activity cursors require activity view")
		}
		return c.readTerminal(ctx, a)
	default:
		return nil, errors.New("view must be activity or terminal")
	}
}
func (c Client) readActivity(ctx context.Context, a contract.MCPSessionReadRequest) (any, error) {
	after, e1 := unsigned(a.AfterPosition)
	offset, e2 := unsigned(a.TextOffset)
	if e1 != nil || e2 != nil || after > 1<<63-1 || offset > 1<<63-1 || a.Limit < 1 || a.Limit > 100 || a.MaxBytesPerItem < 4 || a.MaxBytesPerItem > 65536 || a.Limit*a.MaxBytesPerItem > 256<<10 || a.ItemID == "" && offset != 0 || a.ItemID != "" && !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(a.ItemID) {
		return nil, errors.New("invalid activity cursor or page")
	}
	if err := c.requireCapability(ctx, "chat.acp"); err != nil {
		return nil, err
	}
	q := url.Values{"snapshotId": {a.SnapshotID}, "afterPosition": {a.AfterPosition}, "textOffset": {a.TextOffset}, "itemId": {a.ItemID}, "limit": {strconv.Itoa(a.Limit)}, "maxBytes": {strconv.Itoa(a.MaxBytesPerItem)}}
	var page contract.NativeResult
	if err := c.API.GetBounded(ctx, "/api/sessions/"+url.PathEscape(a.SessionID)+"/native-history?"+q.Encode(), responseLimit, &page); err != nil {
		return nil, apiError(err)
	}
	return contract.MCPActivityPageResult{View: "activity", NativeResult: page, Instructions: agentguide.ReferenceInstructions(), Done: !page.More}, nil
}

func (c Client) readTerminal(ctx context.Context, a contract.MCPSessionReadRequest) (any, error) {
	if a.MaxBytes < 1 || a.MaxBytes > 65536 || (a.Generation == "") != (a.Offset == "") {
		return nil, errors.New("generation and offset must be supplied together")
	}
	var offset uint64
	if a.Generation != "" {
		var err error
		offset, err = unsigned(a.Offset)
		if err != nil || !generation.MatchString(a.Generation) {
			return nil, errors.New("invalid terminal cursor")
		}
	}
	q := url.Values{"maxBytes": {strconv.Itoa(a.MaxBytes)}}
	if a.Generation != "" {
		q.Set("generation", a.Generation)
		q.Set("offset", strconv.FormatUint(offset, 10))
	}
	var page contract.TerminalOutput
	if err := c.API.GetBounded(ctx, "/api/sessions/"+url.PathEscape(a.SessionID)+"/output?"+q.Encode(), 256<<10, &page); err != nil {
		return nil, apiError(err)
	}
	return contract.MCPTerminalPageResult{View: "terminal", TerminalOutput: page, Instructions: agentguide.ReferenceInstructions()}, nil
}
