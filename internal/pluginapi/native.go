package pluginapi

import (
	"errors"
	snapcompact "github.com/mzdluo123/snapcompact-go"
	"net/http"
	"strconv"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
	wire "github.com/rainchan/webmux/internal/pluginwire"
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
		if !s.Conversations.HistoryEnabled() {
			conversationFailure(w, conversation.ErrUnavailable)
			return
		}

		if request.Action == "project" || request.Action == "render" {
			cfg := snapcompact.DefaultPreprocessConfig()
			if request.OutputRunes != 0 {
				cfg.OutputRunes = request.OutputRunes
			}
			preview, err := s.Conversations.PreviewNative(r.Context(), request.SnapshotID, request.CurrentTask, cfg, request.Action == "render")
			if errors.Is(err, conversation.ErrNotFound) || errors.Is(err, conversation.ErrUnavailable) || cancelled(err) {
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
		switch request.Action {
		case "list":
			files, e := s.Conversations.DiscoverCodex(request.Root)
			err = e
			out.Files = wire.NativeFiles(files)
		case "import":
			imported, e := s.Conversations.ImportCodex(r.Context(), request.Path)
			err = e
			out = wire.NativeImported(imported)
		case "delete":
			err = s.Conversations.DeleteNativeSource(r.Context(), request.SourceID)
		case "freeze":
			snapshot, e := s.Conversations.FreezeNative(r.Context(), request.SourceID)
			err = e
			out = wire.NativeSnapshot(snapshot)
		case "read":
			snapshot, e := s.Conversations.NativeSnapshot(r.Context(), request.SnapshotID)
			err = e
			out = wire.NativeSnapshot(snapshot)
			if err == nil {
				var after int64
				if request.AfterPosition != "" {
					after, err = strconv.ParseInt(request.AfterPosition, 10, 64)
				}
				if err != nil {
					err = conversation.ErrInvalid
				} else {
					page, e := s.Conversations.SnapshotActivities(r.Context(), request.SnapshotID, after, 20, 4000)
					err = e
					out.Activities = wire.ConversationActivities(page)
					if err == nil {
						provenance, e := s.Conversations.SnapshotProvenance(r.Context(), request.SnapshotID, after)
						err = e
						out.Provenance = wire.NativeProvenance(provenance)
					}
				}
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
