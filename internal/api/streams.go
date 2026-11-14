package api

import (
	"context"
	"net/http"

	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/transport"
)

func (s *Server) coreStreams() []transport.Stream {
	return []transport.Stream{
		{Pattern: "GET /ws/sessions", Exposure: transport.All, Open: func(ctx context.Context, r *http.Request) (func(hub.MessageConn), func(), error) {
			return func(conn hub.MessageConn) { hub.List(ctx, s.Manager, conn, s.SessionProjection) }, nil, nil
		}},
		{Pattern: "GET /ws/s/{id}", Exposure: transport.All, Open: func(ctx context.Context, r *http.Request) (func(hub.MessageConn), func(), error) {
			item, err := s.Manager.Get(r.PathValue("id"))
			return func(conn hub.MessageConn) {
				if err != nil {
					_ = conn.Close(hub.StatusNotFound, "session not found")
					return
				}
				query := r.URL.Query()
				hub.Attach(ctx, item, conn, hub.AttachOptions(query), query.Get("zreplay") == "1" && query.Get("compress") != "1", s.SessionProjection)
			}, nil, nil
		}},
		{Pattern: "GET /ws/settings", Exposure: transport.All, CloseCode: func(error) hub.StatusCode { return hub.StatusInternalError }, Failure: func(w http.ResponseWriter, err error) { failure(w, settingsStatus(err), err) }, Open: func(ctx context.Context, r *http.Request) (func(hub.MessageConn), func(), error) {
			if _, err := s.Settings.PruneSessionPins(s.keepSessionPin); err != nil {
				return nil, nil, err
			}
			subscription, err := s.Settings.Subscribe()
			if err != nil {
				return nil, nil, err
			}
			return func(conn hub.MessageConn) { hub.Settings(ctx, subscription, conn) }, subscription.Close, nil
		}},
	}
}
