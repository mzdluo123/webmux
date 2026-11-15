package featureapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
	wire "github.com/rainchan/webmux/internal/featurewire"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/transport"
)

// Native operations use the existing authenticated, Origin-checked registrar.
func (s *Server) nativeRoutes(mux transport.Registrar) {
	mux.HandleFunc("POST /api/conversations/native", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var request contract.NativeRequest
		if !decodeStrict(w, r, &request, 64<<10, conversation.ErrInvalid) {
			return
		}
		if !s.Conversations.HistoryEnabled() || s.NativeHistory == nil || s.HistoryProjection == nil {
			conversationFailure(w, conversation.ErrUnavailable)
			return
		}

		if request.Action == "project" || request.Action == "render" {
			cfg := s.HistoryProjection.Config()
			if request.OutputRunes != 0 {
				cfg.OutputRunes = request.OutputRunes
			}
			if cfg.OutputRunes < 1 || cfg.OutputRunes > cfg.MaxInputBytes {
				conversationFailure(w, conversation.ErrInvalid)
				return
			}
			preview, err := s.HistoryProjection.Project(r.Context(), request.SnapshotID, request.CurrentTask, cfg, request.Action == "render")
			if errors.Is(err, conversation.ErrNotFound) || errors.Is(err, conversation.ErrUnavailable) || errors.Is(err, historysource.ErrUnavailable) || errors.Is(err, historysource.ErrChanged) || errors.Is(err, historysource.ErrIdentity) || errors.Is(err, historysource.ErrFormat) || errors.Is(err, historysource.ErrLimit) || cancelled(err) {
				conversationFailure(w, err)
				return
			}
			problem := ""
			if err != nil {
				problem = "derivation_failed"
			}
			reply(w, http.StatusOK, wire.NativePreview(preview, problem))
			return
		}
		out := wire.EmptyNativeResult()
		var err error
		source := historysource.Source{Agent: request.Agent, Path: request.Path, NativeID: request.NativeID, Branch: request.Branch}
		switch request.Action {
		case "list":
			files, e := s.NativeHistory.Discover(r.Context(), request.Agent, request.Root)
			err = e
			out.Files = wire.NativeFiles(files)
			if errors.Is(err, historysource.ErrLimit) {
				out.Truncated, err = true, nil
			}
		case "bind":
			err = s.NativeHistory.Bind(r.Context(), request.ProviderThreadID, source)
		case "retain":
			err = s.NativeHistory.Retain(r.Context(), request.SnapshotID, request.Owner)
		case "release":
			err = s.NativeHistory.Release(r.Context(), request.SnapshotID, request.Owner)
		case "import":
			binding, snapshot, e := s.NativeHistory.Import(r.Context(), source, request.Retain)
			err = e
			out = wire.NativeSnapshot(snapshot)
			out.ConversationID, out.ProviderThreadID = binding.ConversationID, binding.ProviderThreadID
		case "freeze":
			snapshot, e := s.NativeHistory.Freeze(r.Context(), source, request.Owner, request.Retain)
			err = e
			out = wire.NativeSnapshot(snapshot)
		case "read":
			after, offset := 0, 0
			if request.AfterPosition != "" {
				after, err = strconv.Atoi(request.AfterPosition)
			}
			if err == nil && request.TextOffset != "" {
				offset, err = strconv.Atoi(request.TextOffset)
			}
			if err != nil {
				err = conversation.ErrInvalid
			} else {
				page, e := s.NativeHistory.Page(r.Context(), request.SnapshotID, historysource.PageRequest{After: after, TextOffset: offset, Limit: 20, MaxBytes: 4000})
				err = e
				out = wire.NativePage(page, after, offset)
			}
		default:
			err = conversation.ErrInvalid
		}
		if err != nil {
			conversationFailure(w, err)
			return
		}
		reply(w, http.StatusOK, out)
	})
}
