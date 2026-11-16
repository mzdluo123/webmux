package featureapi

import (
	"errors"
	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/featurewire"
	"github.com/rainchan/webmux/internal/handoff"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/transport"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) nativeRoutes(mux transport.Registrar) {
	mux.HandleFunc("GET /api/sessions/{id}/handoff-history", s.handoffHistory)
	mux.HandleFunc("GET /api/sessions/{id}/native-history", s.runtimeHistory)
	retired := func(w http.ResponseWriter, r *http.Request) {
		failure(w, http.StatusGone, errors.New("该历史协议已停用，请使用短期原生历史快照"))
	}
	mux.HandleFunc("/api/conversations/{path...}", retired)
	mux.HandleFunc("/api/handoffs/{path...}", retired)
	mux.HandleFunc("POST /api/native-history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if s.NativeHistory == nil || s.HistoryProjection == nil {
			historyFailure(w, handoff.ErrUnavailable)
			return
		}
		var in contract.NativeRequest
		if !decodeStrict(w, r, &in, 64<<10, handoff.ErrInvalid) {
			return
		}
		if in.Action == "project" || in.Action == "render" {
			cfg := s.HistoryProjection.Config()
			if in.OutputRunes != 0 {
				cfg.OutputRunes = in.OutputRunes
			}
			if cfg.OutputRunes < 1 || cfg.OutputRunes > cfg.MaxInputBytes {
				historyFailure(w, handoff.ErrInvalid)
				return
			}
			p, err := s.HistoryProjection.Project(r.Context(), in.SnapshotID, in.CurrentTask, cfg, in.Action == "render")
			if err != nil {
				historyFailure(w, err)
				return
			}
			reply(w, http.StatusOK, wire.NativePreview(p, ""))
			return
		}
		out := wire.EmptyNativeResult()
		var err error
		switch in.Action {
		case "list":
			files, e := s.NativeHistory.Discover(r.Context(), in.Agent, in.Root)
			err = e
			out.Files = wire.NativeFiles(files)
		case "freeze":
			scope, e := s.NativeHistory.FreezeRuntime(r.Context(), in.SessionID)
			err = e
			out = wire.NativeSnapshot(scope.Snapshot)
			out.ExpiresAt = scope.ExpiresAt.Format(time.RFC3339Nano)
		case "read":
			after, offset := 0, 0
			if in.AfterPosition != "" {
				after, err = strconv.Atoi(in.AfterPosition)
			}
			if err == nil && in.TextOffset != "" {
				offset, err = strconv.Atoi(in.TextOffset)
			}
			if err == nil {
				page, e := s.NativeHistory.Page(r.Context(), in.SnapshotID, historysource.PageRequest{After: after, TextOffset: offset, Limit: 20, MaxBytes: 4000})
				err = e
				out = wire.NativePage(page, after, offset)
			} else {
				err = handoff.ErrInvalid
			}
		default:
			err = handoff.ErrInvalid
		}
		if err != nil {
			historyFailure(w, err)
			return
		}
		reply(w, http.StatusOK, out)
	})
}

func (s *Server) handoffHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.Handoffs == nil || s.NativeHistory == nil {
		historyFailure(w, handoff.ErrUnavailable)
		return
	}
	q, err := transport.ParseQuery(r.URL.RawQuery, "snapshotId", "afterPosition", "textOffset", "itemId", "limit", "maxBytes")
	if err != nil {
		historyFailure(w, handoff.ErrInvalid)
		return
	}
	scope := q.Get("snapshotId")
	if err := s.Handoffs.HistoryAllowed(r.Context(), r.PathValue("id"), scope); err != nil {
		historyFailure(w, err)
		return
	}
	parse := func(name string, fallback int) (int, error) {
		if q.Get(name) == "" {
			return fallback, nil
		}
		return strconv.Atoi(q.Get(name))
	}
	after, err := parse("afterPosition", 0)
	if err != nil {
		historyFailure(w, handoff.ErrInvalid)
		return
	}
	offset, err := parse("textOffset", 0)
	if err != nil {
		historyFailure(w, handoff.ErrInvalid)
		return
	}
	limit, err := parse("limit", 20)
	if err != nil {
		historyFailure(w, handoff.ErrInvalid)
		return
	}
	maxBytes, err := parse("maxBytes", 4000)
	if err != nil {
		historyFailure(w, handoff.ErrInvalid)
		return
	}
	if q.Get("itemId") != "" {
		doc, err := s.NativeHistory.ScopeDocument(r.Context(), scope)
		if err != nil {
			historyFailure(w, err)
			return
		}
		found := false
		for i, item := range doc.Records {
			if item.ID == q.Get("itemId") {
				after = i
				limit = 1
				found = true
				break
			}
		}
		if !found {
			historyFailure(w, handoff.ErrInvalid)
			return
		}
	}
	page, err := s.NativeHistory.Page(r.Context(), scope, historysource.PageRequest{After: after, TextOffset: offset, Limit: limit, MaxBytes: maxBytes})
	if err != nil {
		historyFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.NativePage(page, after, offset))
}

func (s *Server) runtimeHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.Hybrid == nil || s.NativeHistory == nil {
		historyFailure(w, handoff.ErrUnavailable)
		return
	}
	q, err := transport.ParseQuery(r.URL.RawQuery, "snapshotId", "afterPosition", "textOffset", "itemId", "limit", "maxBytes")
	if err != nil {
		historyFailure(w, handoff.ErrInvalid)
		return
	}
	rec, err := s.Hybrid.Record(r.PathValue("id"))
	if err != nil {
		if !hybridFailure(w, err) {
			historyFailure(w, err)
		}
		return
	}
	scopeID := q.Get("snapshotId")
	if scopeID == "" {
		if (q.Get("afterPosition") != "" && q.Get("afterPosition") != "0") || (q.Get("textOffset") != "" && q.Get("textOffset") != "0") || q.Get("itemId") != "" {
			historyFailure(w, handoff.ErrInvalid)
			return
		}
		scope, err := s.NativeHistory.FreezeRuntime(r.Context(), rec.ID)
		if err != nil {
			historyFailure(w, err)
			return
		}
		scopeID = scope.ID
	}
	scope, err := s.NativeHistory.Scope(scopeID)
	if err != nil {
		historyFailure(w, err)
		return
	}
	if scope.Snapshot.Source.Agent != rec.Agent || scope.Snapshot.Source.NativeID != rec.AgentSessionID {
		historyFailure(w, handoff.ErrConflict)
		return
	}
	after, offset, limit, maxBytes := 0, 0, 20, 4000
	for key, dest := range map[string]*int{"afterPosition": &after, "textOffset": &offset, "limit": &limit, "maxBytes": &maxBytes} {
		if q.Get(key) != "" {
			*dest, err = strconv.Atoi(q.Get(key))
			if err != nil {
				historyFailure(w, handoff.ErrInvalid)
				return
			}
		}
	}
	if item := q.Get("itemId"); item != "" {
		doc, err := s.NativeHistory.ScopeDocument(r.Context(), scopeID)
		if err != nil {
			historyFailure(w, err)
			return
		}
		found := false
		for i, record := range doc.Records {
			if record.ID == item {
				after = i
				limit = 1
				found = true
				break
			}
		}
		if !found {
			historyFailure(w, handoff.ErrInvalid)
			return
		}
	}
	page, err := s.NativeHistory.Page(r.Context(), scopeID, historysource.PageRequest{After: after, TextOffset: offset, Limit: limit, MaxBytes: maxBytes})
	if err != nil {
		historyFailure(w, err)
		return
	}
	out := wire.NativePage(page, after, offset)
	out.ExpiresAt = scope.ExpiresAt.Format(time.RFC3339Nano)
	reply(w, http.StatusOK, out)
}
