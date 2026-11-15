package pluginapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/pluginhub"
	"github.com/rainchan/webmux/internal/transport"
	"github.com/rainchan/webmux/internal/voice"
)

func (s *Server) voiceStream() transport.Stream {
	return transport.Stream{Pattern: "GET /ws/voice", Exposure: transport.All, CloseCode: func(err error) hub.StatusCode {
		if errors.Is(err, voice.ErrDisabled) {
			return hub.StatusNotFound
		}
		return hub.StatusInternalError
	}, Failure: func(w http.ResponseWriter, err error) { failure(w, voiceStatus(err), err) }, Open: func(ctx context.Context, r *http.Request) (func(hub.MessageConn), func(), error) {
		if s.Voice == nil || !s.Voice.Enabled() {
			return nil, nil, voice.ErrDisabled
		}
		recognition, err := s.Voice.Start(ctx)
		if err != nil {
			return nil, nil, err
		}
		return func(conn hub.MessageConn) { pluginhub.Voice(ctx, recognition, s.Voice.FinishTimeout(), conn) }, recognition.Close, nil
	}}
}

func (s *Server) chatStream() transport.Stream {
	return transport.Stream{Pattern: "GET /ws/s/{id}/chat", Exposure: transport.All, Failure: sessionCommandFailure, Open: func(ctx context.Context, r *http.Request) (func(hub.MessageConn), func(), error) {
		id := r.PathValue("id")
		if err := s.chatTarget(id); errors.Is(err, hybrid.ErrNoChatView) {
			return nil, nil, err
		}
		var subscription *hybrid.Subscription
		err := hybrid.ErrNotFound
		if s.Hybrid != nil {
			subscription, err = s.Hybrid.Subscribe(id, pluginhub.ChatResume(r.URL.Query()))
		}
		if err != nil {
			return func(conn hub.MessageConn) { _ = conn.Close(hub.StatusNotFound, "session not found") }, nil, nil
		}
		return func(conn hub.MessageConn) { pluginhub.Chat(ctx, subscription, conn) }, subscription.Close, nil
	}}
}
