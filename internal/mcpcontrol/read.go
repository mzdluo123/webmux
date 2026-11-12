package mcpcontrol

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/mcpstdio"
)

var generation = regexp.MustCompile(`^[0-9a-f]{16}$`)
var decimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,19})$`)

func unsigned(s string) (uint64, error) {
	if !decimal.MatchString(s) {
		return 0, errors.New("invalid cursor")
	}
	return strconv.ParseUint(s, 10, 64)
}
func (c Client) readTools() []mcpstdio.Tool {
	return []mcpstdio.Tool{tool("session_read", "Read committed exportable chat activity, or a bounded raw terminal replay page encoded as base64. Cursors are decimal strings. Reports truncation, resets and missing history; never acquires terminal control.", true, map[string]any{
		"sessionId": text(), "view": enumeration("activity", "terminal"),
		"afterPosition": text(), "itemId": text(), "textOffset": text(), "limit": integer(1, 100), "maxBytesPerItem": integer(4, 65536),
		"generation": text(), "offset": text(), "maxBytes": integer(1, 65536),
	}, []string{"sessionId"}, c.read)}
}

type readArguments struct {
	SessionID       string `json:"sessionId"`
	View            string `json:"view,omitempty"`
	AfterPosition   string `json:"afterPosition,omitempty"`
	ItemID          string `json:"itemId,omitempty"`
	TextOffset      string `json:"textOffset,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	MaxBytesPerItem int    `json:"maxBytesPerItem,omitempty"`
	Generation      string `json:"generation,omitempty"`
	Offset          string `json:"offset,omitempty"`
	MaxBytes        int    `json:"maxBytes,omitempty"`
}

func (c Client) read(ctx context.Context, raw json.RawMessage) (any, error) {
	a := readArguments{View: "activity", AfterPosition: "0", TextOffset: "0", Limit: 20, MaxBytesPerItem: 4000, MaxBytes: 65536}
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
func (c Client) readActivity(ctx context.Context, a readArguments) (any, error) {
	after, e1 := unsigned(a.AfterPosition)
	offset, e2 := unsigned(a.TextOffset)
	if e1 != nil || e2 != nil || after > 1<<63-1 || offset > 1<<63-1 || a.Limit < 1 || a.Limit > 100 || a.MaxBytesPerItem < 4 || a.MaxBytesPerItem > 65536 || a.Limit*a.MaxBytesPerItem > 256<<10 || a.ItemID == "" && offset != 0 || a.ItemID != "" && !regexpHistoryID.MatchString(a.ItemID) {
		return nil, errors.New("invalid activity cursor or page")
	}
	var session contract.SessionInfo
	if err := c.API.GetBounded(ctx, "/api/sessions/"+url.PathEscape(a.SessionID), responseLimit, &session); err != nil {
		return nil, apiError(err)
	}
	if !regexpHistoryID.MatchString(session.ConversationID) {
		return nil, errors.New("session has no durable conversation history")
	}
	path := "/api/conversations/" + session.ConversationID + "/activities"
	if a.ItemID != "" {
		q := url.Values{"textOffset": {a.TextOffset}, "maxBytes": {strconv.Itoa(a.MaxBytesPerItem)}}
		var item contract.ConversationActivity
		if err := c.API.GetBounded(ctx, path+"/"+a.ItemID+"?"+q.Encode(), responseLimit, &item); err != nil {
			return nil, apiError(err)
		}
		if !item.Exportable {
			return nil, errors.New("activity is not exportable")
		}
		return map[string]any{"view": "activity", "item": item, "done": !item.Truncated, "instructions": instructions}, nil
	}
	q := url.Values{"afterPosition": {a.AfterPosition}, "limit": {strconv.Itoa(a.Limit)}, "maxBytesPerItem": {strconv.Itoa(a.MaxBytesPerItem)}}
	var page contract.ConversationActivities
	if err := c.API.GetBounded(ctx, path+"?"+q.Encode(), responseLimit, &page); err != nil {
		return nil, apiError(err)
	}
	items := make([]contract.ConversationActivity, 0, len(page.Items))
	for _, item := range page.Items {
		if item.Exportable {
			items = append(items, item)
		}
	}
	return map[string]any{"view": "activity", "conversationId": session.ConversationID, "items": items, "nextPosition": page.NextPosition, "done": len(page.Items) < a.Limit, "gaps": page.Gaps, "historyOrigin": page.HistoryOrigin, "reconcileRequired": page.ReconcileRequired, "instructions": instructions}, nil
}

var regexpHistoryID = regexp.MustCompile(`^[a-f0-9]{32}$`)

func (c Client) readTerminal(ctx context.Context, a readArguments) (any, error) {
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
	path := "/ws/s/" + url.PathEscape(a.SessionID)
	if a.Generation != "" {
		path += "?" + url.Values{"generation": {a.Generation}, "offset": {a.Offset}}.Encode()
	}
	conn, err := c.API.DialWebSocket(ctx, path)
	if err != nil {
		return nil, apiError(err)
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(responseLimit)
	var position *contract.StreamPosition
	var replay []byte
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return nil, errors.New("terminal replay unavailable")
		}
		if len(data) == 0 {
			return nil, errors.New("invalid terminal frame")
		}
		switch data[0] {
		case contract.FramePosition:
			var p contract.StreamPosition
			if json.Unmarshal(data[1:], &p) != nil || p.End < p.Start || p.End-p.Start > responseLimit || !generation.MatchString(p.Generation) || (p.Mode != "reset" && p.Mode != "append") {
				return nil, errors.New("invalid terminal position")
			}
			position = &p
			if p.End == p.Start {
				return terminalPage(a, offset, p, nil)
			}
		case contract.FrameSnapshot, contract.FrameOutput:
			if position == nil {
				return nil, errors.New("terminal output without position")
			}
			remaining := position.End - position.Start - uint64(len(replay))
			n := min(uint64(len(data)-1), remaining)
			replay = append(replay, data[1:1+int(n)]...)
			if uint64(len(replay)) == position.End-position.Start {
				return terminalPage(a, offset, *position, replay)
			}
		case contract.FrameExit:
			return nil, errors.New("terminal ended before replay completed")
		}
	}
}
func terminalPage(a readArguments, offset uint64, p contract.StreamPosition, replay []byte) (any, error) {
	reset := a.Generation != "" && (p.Mode == "reset" || a.Generation != p.Generation || offset < p.Start || offset > p.End)
	start := p.Start
	if !reset && a.Generation != "" {
		start = max(start, offset)
	}
	if start > p.End {
		return nil, errors.New("terminal cursor exceeds replay")
	}
	n := min(uint64(a.MaxBytes), p.End-start)
	data := replay[start-p.Start : start-p.Start+n]
	return map[string]any{"view": "terminal", "generation": p.Generation, "start": strconv.FormatUint(start, 10), "end": strconv.FormatUint(p.End, 10), "nextOffset": strconv.FormatUint(start+n, 10), "encoding": "base64", "data": base64.StdEncoding.EncodeToString(data), "reset": reset, "historyTruncated": p.Truncated, "limited": start+n < p.End, "instructions": instructions}, nil
}
