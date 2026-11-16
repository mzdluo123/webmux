package featureapi

import (
	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/featurewire"
	"github.com/rainchan/webmux/internal/handoff"
	"github.com/rainchan/webmux/internal/transport"
	"net/http"
)

func (s *Server) handoffRoutes(mux transport.Registrar) {
	mux.HandleFunc("POST /api/sessions/{id}/handoff-previews", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if s.Handoffs == nil {
			historyFailure(w, handoff.ErrUnavailable)
			return
		}
		var in contract.HandoffPreviewRequest
		if !decodeStrict(w, r, &in, 256<<10, handoff.ErrInvalid) {
			return
		}
		preview, err := s.Handoffs.Preview(r.Context(), r.PathValue("id"), wire.HandoffRequest(in))
		if err != nil {
			if !hybridFailure(w, err) {
				historyFailure(w, err)
			}
			return
		}
		reply(w, http.StatusCreated, wire.HandoffPreview(preview))
	})
	mux.HandleFunc("GET /api/handoff-previews/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if s.Handoffs == nil {
			historyFailure(w, handoff.ErrUnavailable)
			return
		}
		p, err := s.Handoffs.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			historyFailure(w, err)
			return
		}
		reply(w, http.StatusOK, wire.HandoffPreview(p))
	})
	mux.HandleFunc("POST /api/handoff-previews/{id}/send", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if s.Handoffs == nil {
			historyFailure(w, handoff.ErrUnavailable)
			return
		}
		var in contract.HandoffSendRequest
		if !decodeStrict(w, r, &in, 4096, handoff.ErrInvalid) {
			return
		}
		o, err := s.Handoffs.Deliver(r.Context(), r.PathValue("id"), in.OperationID)
		if err != nil {
			historyFailure(w, err)
			return
		}
		reply(w, http.StatusAccepted, wire.HandoffOperation(o))
	})
	mux.HandleFunc("GET /api/handoff-operations/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if s.Handoffs == nil {
			historyFailure(w, handoff.ErrUnavailable)
			return
		}
		o, err := s.Handoffs.Delivery(r.Context(), r.PathValue("id"))
		if err != nil {
			historyFailure(w, err)
			return
		}
		reply(w, http.StatusOK, wire.HandoffOperation(o))
	})
	mux.HandleFunc("POST /api/handoff-operations/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		if s.Handoffs == nil {
			historyFailure(w, handoff.ErrUnavailable)
			return
		}
		o, err := s.Handoffs.Cancel(r.Context(), r.PathValue("id"))
		if err != nil {
			historyFailure(w, err)
			return
		}
		reply(w, http.StatusOK, wire.HandoffOperation(o))
	})
}
