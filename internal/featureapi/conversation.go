package featureapi

import (
	"context"
	"github.com/rainchan/webmux/internal/transport"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
	wire "github.com/rainchan/webmux/internal/featurewire"
	"github.com/rainchan/webmux/internal/historysource"
)

var conversationID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var activityID = regexp.MustCompile(`^[a-f0-9]{32}([a-f0-9]{32})?$`)
var conversationCursor = regexp.MustCompile(`^(0|[1-9][0-9]{0,18})$`)

func (s *Server) conversationRoutes(mux transport.Registrar) {
	s.nativeRoutes(mux)
	mux.HandleFunc("GET /api/conversations/{id}/activities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		value, err := s.conversationActivities(r)
		if err != nil {
			conversationFailure(w, err)
			return
		}
		reply(w, http.StatusOK, value)
	})
	mux.HandleFunc("GET /api/conversations/{id}/activities/{activityId}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		value, err := s.conversationActivity(r)
		if err != nil {
			conversationFailure(w, err)
			return
		}
		reply(w, http.StatusOK, value)
	})
}

func readConversationQuery(r *http.Request, allowed ...string) (url.Values, error) {
	values, err := transport.ParseQuery(r.URL.RawQuery, allowed...)
	if err != nil {
		return nil, conversation.ErrInvalid
	}
	return values, nil
}

func conversationNumber(q url.Values, key string, fallback int64) (int64, error) {
	if !q.Has(key) {
		return fallback, nil
	}
	text := q.Get(key)
	if !conversationCursor.MatchString(text) {
		return 0, conversation.ErrInvalid
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, conversation.ErrInvalid
	}
	return n, nil
}

func (s *Server) conversationActivities(r *http.Request) (contract.ConversationActivities, error) {
	var out contract.ConversationActivities
	if !s.Conversations.HistoryEnabled() || s.NativeHistory == nil {
		return out, conversation.ErrUnavailable
	}
	id := r.PathValue("id")
	if !conversationID.MatchString(id) {
		return out, conversation.ErrInvalid
	}
	q, err := readConversationQuery(r, "afterPosition", "limit", "maxBytesPerItem", "handoffId", "snapshotIds")
	if err != nil {
		return out, err
	}
	after, err := conversationNumber(q, "afterPosition", 0)
	if err != nil {
		return out, err
	}
	limit, err := conversationNumber(q, "limit", 20)
	if err != nil || limit < 1 || limit > 100 {
		return out, conversation.ErrInvalid
	}
	bytes, err := conversationNumber(q, "maxBytesPerItem", 4000)
	if err != nil || bytes < 4 || bytes > conversation.MaxActivityBytes {
		return out, conversation.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	doc, ids, hashes, err := s.conversationHistory(ctx, id, q)
	if err != nil {
		return out, err
	}
	if after > int64(len(doc.Records)) {
		return out, conversation.ErrInvalid
	}
	out = contract.ConversationActivities{Items: []contract.ConversationActivity{}, SnapshotIDs: ids, NextPosition: strconv.FormatInt(after, 10), Gaps: []contract.ConversationGap{}, HistoryOrigin: "complete"}
	for _, gap := range doc.Gaps {
		out.Gaps = append(out.Gaps, contract.ConversationGap{ID: gap.RecordID, Reason: gap.Category})
	}
	out.ReconcileRequired = len(doc.Gaps) != 0
	end := min(len(doc.Records), int(after)+int(limit))
	for index := int(after); index < end; index++ {
		item, err := wire.ConversationActivity(doc.Records[index], id, index+1, hashes[doc.Records[index].ID], 0, int(bytes))
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, item)
		out.NextPosition = item.Position
	}
	return out, nil
}

func (s *Server) conversationActivity(r *http.Request) (contract.ConversationActivity, error) {
	var out contract.ConversationActivity
	if !s.Conversations.HistoryEnabled() || s.NativeHistory == nil {
		return out, conversation.ErrUnavailable
	}
	id, item := r.PathValue("id"), r.PathValue("activityId")
	if !conversationID.MatchString(id) || !activityID.MatchString(item) {
		return out, conversation.ErrInvalid
	}
	q, err := readConversationQuery(r, "textOffset", "maxBytes", "handoffId", "snapshotIds")
	if err != nil {
		return out, err
	}
	offset, err := conversationNumber(q, "textOffset", 0)
	if err != nil {
		return out, err
	}
	bytes, err := conversationNumber(q, "maxBytes", 4000)
	if err != nil || bytes < 4 || bytes > conversation.MaxActivityBytes {
		return out, conversation.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	doc, ids, hashes, err := s.conversationHistory(ctx, id, q)
	if err != nil {
		return out, err
	}
	for index, record := range doc.Records {
		if record.ID == item {
			if offset > int64(len(record.Text)) || (offset < int64(len(record.Text)) && !utf8.RuneStart(record.Text[offset])) {
				return out, conversation.ErrInvalid
			}
			out, err = wire.ConversationActivity(record, id, index+1, hashes[record.ID], int(offset), int(bytes))
			out.SnapshotIDs = ids
			return out, err
		}
	}
	return out, conversation.ErrNotFound
}

// conversationHistory resolves the permission boundary before opening any body.
// A handoff owns its complete frozen range, not only budget-selected items.
func (s *Server) conversationHistory(ctx context.Context, id string, q url.Values) (historysource.Document, []string, map[string]string, error) {
	var empty historysource.Document
	if _, err := s.Conversations.Conversation(ctx, id); err != nil {
		return empty, nil, nil, err
	}
	var ids []string
	var point int64 = -1
	var err error
	if q.Has("handoffId") {
		if !conversationID.MatchString(q.Get("handoffId")) || q.Has("snapshotIds") {
			return empty, nil, nil, conversation.ErrInvalid
		}
		selection, e := s.Conversations.Selection(ctx, q.Get("handoffId"), 0, 1, "")
		if e != nil {
			return empty, nil, nil, e
		}
		if selection.SourceConversationID != id {
			return empty, nil, nil, historysource.ErrIdentity
		}
		point = selection.SourcePosition
		ids, err = s.Conversations.HandoffSnapshots(ctx, q.Get("handoffId"))
	} else if q.Has("snapshotIds") {
		ids = strings.Split(q.Get("snapshotIds"), ",")
		if len(ids) > 1000 {
			return empty, nil, nil, conversation.ErrInvalid
		}
		owned, e := s.Conversations.FrozenForOwner(ctx, id)
		if e != nil {
			return empty, nil, nil, e
		}
		prefix, e := s.Conversations.PrefixSnapshots(ctx, id)
		if e != nil {
			return empty, nil, nil, e
		}
		allowed := make(map[string]bool, len(owned)+len(prefix))
		for _, snapshot := range append(owned, prefix...) {
			allowed[snapshot] = true
		}
		seen := make(map[string]bool, len(ids))
		for _, snapshot := range ids {
			if len(snapshot) != 64 || !activityID.MatchString(snapshot) || !allowed[snapshot] || seen[snapshot] {
				return empty, nil, nil, historysource.ErrIdentity
			}
			seen[snapshot] = true
		}
	} else {
		snapshots, e := s.NativeHistory.FreezeConversation(ctx, id, id, false)
		err = e
		ids = make([]string, 0, len(snapshots))
		for _, snapshot := range snapshots {
			ids = append(ids, snapshot.ID)
		}
	}
	if err != nil {
		return empty, nil, nil, err
	}
	if len(ids) == 0 {
		return empty, nil, nil, historysource.ErrUnavailable
	}
	doc, err := s.NativeHistory.Documents(ctx, ids)
	if err != nil {
		return empty, nil, nil, err
	}
	if point >= 0 && int64(len(doc.Records)) != point {
		return empty, nil, nil, historysource.ErrChanged
	}
	// Documents already carries the verified frozen references. Loading each
	// snapshot again would decode every retained export a second time just to
	// retrieve these hashes, making a bounded page depend on duplicate full reads.
	hashes := make(map[string]string, len(doc.RecordRefs))
	for _, record := range doc.RecordRefs {
		hashes[record.ID] = record.Hash
	}
	return doc, ids, hashes, nil
}
