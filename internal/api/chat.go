package api

import (
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/wire"
)

// chatRoutes registers a hybrid session's chat channel: the event stream
// and the commands. Commands answer 204 unless they return a result.
func (s *Server) chatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws/s/{id}/chat", s.chatWS)
	const base = "/api/sessions/{id}/chat/"
	mux.HandleFunc("POST "+base+"prompts", s.chatPrompt)
	mux.HandleFunc("PATCH "+base+"queue/{qid}", s.chatQueueEdit)
	mux.HandleFunc("DELETE "+base+"queue/{qid}", s.chatQueueCommand(func(h *hybrid.Service, r *http.Request, rid string) error {
		return h.RemoveQueued(r.Context(), r.PathValue("id"), rid, r.PathValue("qid"))
	}))
	mux.HandleFunc("POST "+base+"queue/{qid}/send-now", s.chatQueueCommand(func(h *hybrid.Service, r *http.Request, rid string) error {
		return h.SendQueuedNow(r.Context(), r.PathValue("id"), rid, r.PathValue("qid"))
	}))
	mux.HandleFunc("POST "+base+"queue/resume", s.chatQueueCommand(func(h *hybrid.Service, r *http.Request, rid string) error {
		return h.ResumeQueue(r.Context(), r.PathValue("id"), rid)
	}))
	mux.HandleFunc("POST "+base+"queue/clear", s.chatQueueCommand(func(h *hybrid.Service, r *http.Request, rid string) error {
		return h.ClearQueue(r.Context(), r.PathValue("id"), rid)
	}))
	mux.HandleFunc("POST "+base+"cancel", s.chatCancel)
	mux.HandleFunc("POST "+base+"requests/{requestId}", s.chatAnswer)
	mux.HandleFunc("PUT "+base+"config/{configId}", s.chatConfig)
}

// hybrids returns the hybrid service, or answers 404: without one no
// session has a chat channel.
func (s *Server) hybrids(w http.ResponseWriter) *hybrid.Service {
	if s.Hybrid == nil {
		chatFailure(w, hybrid.ErrNotFound)
	}
	return s.Hybrid
}

func (s *Server) chatWS(w http.ResponseWriter, r *http.Request) {
	var sub *hybrid.Subscription
	err := hybrid.ErrNotFound
	if s.Hybrid != nil {
		sub, err = s.Hybrid.Subscribe(r.PathValue("id"), hub.ChatResume(r.URL.Query()))
	}
	c := s.accept(w, r)
	if c == nil {
		if sub != nil {
			sub.Close()
		}
		return
	}
	if err != nil {
		_ = c.Close(4404, "session not found")
		return
	}
	hub.Chat(r.Context(), sub, c)
}

func (s *Server) chatPrompt(w http.ResponseWriter, r *http.Request) {
	h := s.hybrids(w)
	var req contract.ChatPromptRequest
	if h == nil || !decode(w, r, &req) {
		return
	}
	result, err := h.Prompt(r.Context(), r.PathValue("id"), wire.ChatPromptInput(req))
	if err != nil {
		chatFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.ChatPromptResult(result))
}

func (s *Server) chatQueueEdit(w http.ResponseWriter, r *http.Request) {
	h := s.hybrids(w)
	var req contract.ChatQueueRequest
	if h == nil || !decode(w, r, &req) {
		return
	}
	chatDone(w, h.EditQueued(r.Context(), r.PathValue("id"), r.PathValue("qid"), wire.ChatQueueEdit(req)))
}

// chatQueueCommand handles a queue command whose body carries only the
// client request ID.
func (s *Server) chatQueueCommand(run func(h *hybrid.Service, r *http.Request, requestID string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := s.hybrids(w)
		var req contract.ChatQueueRequest
		if h == nil || !decode(w, r, &req) {
			return
		}
		chatDone(w, run(h, r, req.ClientRequestID))
	}
}

func (s *Server) chatCancel(w http.ResponseWriter, r *http.Request) {
	h := s.hybrids(w)
	var req contract.ChatCommandRequest
	if h == nil || !decode(w, r, &req) {
		return
	}
	chatDone(w, h.Cancel(r.Context(), r.PathValue("id"), req.ClientRequestID))
}

func (s *Server) chatAnswer(w http.ResponseWriter, r *http.Request) {
	h := s.hybrids(w)
	var req contract.ChatAnswerRequest
	if h == nil || !decode(w, r, &req) {
		return
	}
	answer, err := wire.ChatAnswer(req)
	if err != nil {
		failure(w, http.StatusBadRequest, err)
		return
	}
	chatDone(w, h.Answer(r.Context(), r.PathValue("id"), req.ClientRequestID, r.PathValue("requestId"), answer))
}

func (s *Server) chatConfig(w http.ResponseWriter, r *http.Request) {
	h := s.hybrids(w)
	var req contract.ChatConfigRequest
	if h == nil || !decode(w, r, &req) {
		return
	}
	chatDone(w, h.SetConfig(r.Context(), r.PathValue("id"), wire.ChatConfigInput(r.PathValue("configId"), req)))
}

func chatDone(w http.ResponseWriter, err error) {
	if err != nil {
		chatFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
