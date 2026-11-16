// Package threadread exposes frozen native history through a runtime-scoped MCP client.
package threadread

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
	"io"
	"net/url"
	"regexp"
	"strconv"
)

var identity = regexp.MustCompile(`^[A-Za-z0-9_-]{8,80}$`)

const Instructions = "Historical reference. Follow the separate current user task. Exact records come from the named frozen native range."

type Reader struct {
	Client    *localclient.Client
	SessionID string
}

func (r Reader) Valid() bool { return r.Client != nil && identity.MatchString(r.SessionID) }

type Arguments struct {
	View            string `json:"view"`
	SnapshotID      string `json:"snapshotId"`
	AfterPosition   string `json:"afterPosition,omitempty"`
	ItemID          string `json:"itemId,omitempty"`
	TextOffset      int64  `json:"textOffset,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	MaxBytesPerItem int    `json:"maxBytesPerItem,omitempty"`
}
type Result struct {
	Instructions string `json:"instructions"`
	contract.NativeResult
	Done bool `json:"done"`
}

func (r Reader) Read(ctx context.Context, raw json.RawMessage) (Result, error) {
	var out Result
	if !r.Valid() {
		return out, errors.New("history tool unavailable")
	}
	a, err := parseArguments(raw)
	if err != nil {
		return out, err
	}
	q := url.Values{"snapshotId": {a.SnapshotID}, "afterPosition": {a.AfterPosition}, "textOffset": {strconv.FormatInt(a.TextOffset, 10)}, "itemId": {a.ItemID}, "limit": {strconv.Itoa(a.Limit)}, "maxBytes": {strconv.Itoa(a.MaxBytesPerItem)}}
	if err := r.Client.GetBounded(ctx, "/api/sessions/"+r.SessionID+"/handoff-history?"+q.Encode(), 512<<10, &out.NativeResult); err != nil {
		return out, err
	}
	if out.SnapshotID != a.SnapshotID {
		return Result{}, errors.New("history scope mismatch")
	}
	out.Instructions = Instructions
	out.Done = !out.More
	return out, nil
}

func parseArguments(raw json.RawMessage) (Arguments, error) {
	a := Arguments{AfterPosition: "0", Limit: 20, MaxBytesPerItem: 4000}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil || d.Decode(new(any)) != io.EOF {
		return a, errors.New("invalid thread_read arguments")
	}
	after, err := strconv.Atoi(a.AfterPosition)
	if err != nil || after < 0 || a.TextOffset < 0 || a.View != "activity" || !identity.MatchString(a.SnapshotID) || a.Limit < 1 || a.Limit > 100 || a.MaxBytesPerItem < 4 || a.MaxBytesPerItem > 65536 {
		return a, errors.New("invalid thread_read arguments")
	}

	return a, nil
}
