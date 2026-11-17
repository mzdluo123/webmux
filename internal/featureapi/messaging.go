package featureapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/featurewire"
	"github.com/rainchan/webmux/internal/messaging"
	"github.com/rainchan/webmux/internal/transport"
)

func (s *Server) messagingRoutes(mux transport.Registrar) {
	s.messagingAgentRoutes(mux)
	mux.HandleFunc("GET /api/messaging/channels", func(w http.ResponseWriter, r *http.Request) {
		if !s.messagingReady(w) {
			return
		}
		items, err := s.Messaging.Channels()
		if err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, http.StatusOK, featurewire.MessagingChannels(items))
	})
	mux.HandleFunc("POST /api/messaging/channels", func(w http.ResponseWriter, r *http.Request) {
		if !s.messagingReady(w) {
			return
		}
		var in contract.MessagingCreateChannel
		if !decodeStrict(w, r, &in, 16<<10, messaging.ErrInvalid) {
			return
		}
		item, err := s.Messaging.CreateChannel(in.Name, in.Topic)
		if err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, http.StatusCreated, featurewire.MessagingChannel(item))
	})
	mux.HandleFunc("GET /api/messaging/channels/{channel}/messages", func(w http.ResponseWriter, r *http.Request) {
		if !s.messagingReady(w) {
			return
		}
		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				messagingFailure(w, messaging.ErrInvalid)
				return
			}
			limit = n
		}
		items, err := s.Messaging.Messages(r.PathValue("channel"), r.URL.Query().Get("before"), limit)
		if err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, http.StatusOK, featurewire.MessagingMessages(items))
	})
	mux.HandleFunc("POST /api/messaging/channels/{channel}/messages", func(w http.ResponseWriter, r *http.Request) {
		if !s.messagingReady(w) {
			return
		}
		var in contract.MessagingSend
		if !decodeStrict(w, r, &in, 256<<10, messaging.ErrInvalid) {
			return
		}
		item, err := s.Messaging.Send(r.PathValue("channel"), in.Text, in.ClientRequestID)
		if err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, http.StatusCreated, featurewire.MessagingMessage(item))
	})
}

func (s *Server) messagingReady(w http.ResponseWriter) bool {
	if s.Messaging == nil || s.Messaging.Unavailable() != nil {
		failure(w, http.StatusServiceUnavailable, messaging.ErrUnavailable)
		return false
	}
	return true
}

func messagingFailure(w http.ResponseWriter, err error) {
	status := http.StatusServiceUnavailable
	switch {
	case errors.Is(err, messaging.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, messaging.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, messaging.ErrConflict), errors.Is(err, messaging.ErrDuplicate):
		status = http.StatusConflict
	case errors.Is(err, messaging.ErrLimit):
		status = http.StatusConflict
	}
	failure(w, status, err)
}
