package pluginapi

import (
	"context"
	"errors"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/p2psettings"
	"github.com/rainchan/webmux/internal/transport"
	"net/http"
	"strconv"
	"time"
)

// P2PControl is injected by cmd/. HTTP handlers never construct the P2P
// service or touch its settings file directly.
type P2PControl interface {
	Enabled() bool
	Status() contract.P2PStatus
	UpdateSettings(contract.P2PSettingsUpdate) (contract.P2PStatus, error)
	ResetSettings() (contract.P2PStatus, error)
	Signal(context.Context, contract.P2PSignalRequest) (contract.P2PSignalResponse, error)
}

func (s *Server) p2pRoutes(mux transport.Registrar) {
	mux.HandleFunc("GET /api/p2p", func(w http.ResponseWriter, _ *http.Request) {
		if s.p2pControl(w) == nil {
			return
		}
		reply(w, http.StatusOK, s.P2P.Status())
	})
	mux.HandleFunc("PUT /api/p2p/settings", func(w http.ResponseWriter, r *http.Request) {
		if s.p2pControl(w) == nil {
			return
		}
		var req contract.P2PSettingsUpdate
		if !decodeStrict(w, r, &req, 65536, p2psettings.ErrInvalid) {
			return
		}
		view, err := s.P2P.UpdateSettings(req)
		if err != nil {
			failure(w, p2pSettingsStatus(err), err)
			return
		}
		reply(w, http.StatusOK, view)
	})
	mux.HandleFunc("DELETE /api/p2p/settings", func(w http.ResponseWriter, _ *http.Request) {
		if s.p2pControl(w) == nil {
			return
		}
		view, err := s.P2P.ResetSettings()
		if err != nil {
			failure(w, p2pSettingsStatus(err), err)
			return
		}
		reply(w, http.StatusOK, view)
	})
	mux.HandleFunc("POST /api/p2p/signal", func(w http.ResponseWriter, r *http.Request) {
		if s.p2pControl(w) == nil {
			return
		}
		var req contract.P2PSignalRequest
		if !decode(w, r, &req) {
			return
		}
		resp, err := s.P2P.Signal(r.Context(), req)
		if err != nil {
			failure(w, p2pSignalStatus(err), err)
			return
		}
		reply(w, http.StatusOK, resp)
	})
	mux.HandleFunc("GET /api/p2p/ping", func(w http.ResponseWriter, r *http.Request) {
		seq, _ := strconv.Atoi(r.URL.Query().Get("seq"))
		reply(w, http.StatusOK, contract.P2PPingResponse{
			Seq:      seq,
			ServerTs: time.Now().UnixMilli(),
		})
	})
	collector := mux.(*transport.Collector)
	collector.Exposure = transport.Local
	collector.HandleFunc(contract.PathP2PDiagnostics, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if s.p2pControl(w) != nil {
			reply(w, http.StatusOK, s.P2P.Status())
		}
	})
}

func (s *Server) p2pControl(w http.ResponseWriter) P2PControl {
	if s.P2P == nil {
		failure(w, http.StatusServiceUnavailable, errors.New("p2p service unavailable"))
	}
	return s.P2P
}
