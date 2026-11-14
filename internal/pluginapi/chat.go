package pluginapi

import (
	"errors"
	"github.com/rainchan/webmux/internal/transport"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	wire "github.com/rainchan/webmux/internal/pluginwire"
)

// chatRoutes registers the chat view's own channel: the event stream and
// the commands only the chat view has. Prompts and cancellation are the
// session's input and interrupt. Commands answer 204; a session without a
// chat view answers 409 no_chat_view.
func (s *Server) chatRoutes(mux transport.Registrar) {
	serve(mux, "GET /api/chat/search", http.StatusOK, sessionCommandFailure, s.chatSearch)
	const base = "/api/sessions/{id}/chat/"
	serveInDone(mux, "PATCH "+base+"queue/{qid}", sessionCommandFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatQueueRequest) error {
		return h.EditQueued(r.Context(), r.PathValue("id"), r.PathValue("qid"), wire.ChatQueueEdit(req))
	}))
	serveInDone(mux, "DELETE "+base+"queue/{qid}", sessionCommandFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatQueueRequest) error {
		return h.RemoveQueued(r.Context(), r.PathValue("id"), req.ClientRequestID, r.PathValue("qid"))
	}))
	serveInDone(mux, "POST "+base+"queue/{qid}/send-now", sessionCommandFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatQueueRequest) error {
		return h.SendQueuedNow(r.Context(), r.PathValue("id"), req.ClientRequestID, r.PathValue("qid"))
	}))
	serveInDone(mux, "POST "+base+"queue/resume", sessionCommandFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatQueueRequest) error {
		return h.ResumeQueue(r.Context(), r.PathValue("id"), req.ClientRequestID)
	}))
	serveInDone(mux, "POST "+base+"queue/clear", sessionCommandFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatQueueRequest) error {
		return h.ClearQueue(r.Context(), r.PathValue("id"), req.ClientRequestID)
	}))
	serveInDone(mux, "POST "+base+"requests/{requestId}", sessionCommandFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatAnswerRequest) error {
		answer, err := wire.ChatAnswer(req)
		if err != nil {
			return &badRequest{err}
		}
		return h.Answer(r.Context(), r.PathValue("id"), req.ClientRequestID, r.PathValue("requestId"), answer)
	}))
	serveInDone(mux, "PUT "+base+"config/{configId}", sessionCommandFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatConfigRequest) error {
		return h.SetConfig(r.Context(), r.PathValue("id"), wire.ChatConfigInput(r.PathValue("configId"), req))
	}))
	serveInDone(mux, "PUT "+base+"permission-mode", sessionCommandFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatPermissionModeRequest) error {
		return h.SetPermissionMode(r.Context(), r.PathValue("id"), wire.ChatPermissionModeInput(req))
	}))
}

// chatCommand adapts a command on the hybrid service to a serveInDone
// handler; a session without a chat view is refused first.
func chatCommand[Req any](s *Server, run func(h *hybrid.Service, r *http.Request, req Req) error) func(*http.Request, Req) error {
	return func(r *http.Request, req Req) error {
		if err := s.chatTarget(r.PathValue("id")); err != nil {
			return err
		}
		return run(s.Hybrid, r, req)
	}
}

// configMissing reports whether launch config id was deleted; a store that
// cannot be read cannot tell and says no.
func (s *Server) configMissing(id string) bool {
	_, err := s.Launch.Get(id)
	return errors.Is(err, launch.ErrNotFound)
}

func (s *Server) chatSearch(r *http.Request) ([]contract.ChatMessageMatch, error) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if n := utf8.RuneCountInString(query); n < 2 || n > 120 {
		return nil, &badRequest{errors.New("聊天搜索需要 2 至 120 个字符")}
	}
	if s.Hybrid == nil {
		return []contract.ChatMessageMatch{}, nil
	}
	matches, err := s.Hybrid.SearchMessages(r.Context(), query)
	if err != nil {
		return nil, err
	}
	return wire.ChatMessageMatches(matches), nil
}
