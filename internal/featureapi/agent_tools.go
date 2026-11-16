package featureapi

import (
	"errors"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/featurewire"
	"github.com/rainchan/webmux/internal/transport"
)

func (s *Server) agentToolsRoutes(mux transport.Registrar) {
	if s.AgentTools == nil {
		return
	}
	const base = "/api/agent-tools"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		refresh, err := parseRefreshQuery(r)
		if err != nil {
			agentToolsFailure(w, err)
			return
		}
		report, err := s.AgentTools.Inspect(r.Context(), refresh)
		if err != nil {
			agentToolsFailure(w, err)
			return
		}
		reply(w, http.StatusOK, wire.AgentToolsReport(report))
	})
	mux.HandleFunc("POST "+base+"/check", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var req contract.AgentToolsCheckRequest
		if !decodeOptional(w, r, &req) {
			return
		}
		report, err := s.AgentTools.CheckUpdates(r.Context(), req.RefreshLocal)
		if err != nil {
			agentToolsFailure(w, err)
			return
		}
		reply(w, http.StatusOK, wire.AgentToolsReport(report))
	})
}

func parseRefreshQuery(r *http.Request) (bool, error) {
	query := r.URL.Query()
	for key, values := range query {
		if key != "refresh" || len(values) != 1 {
			return false, &badRequest{err: errors.New("invalid query parameter")}
		}
	}
	values, ok := query["refresh"]
	if !ok {
		return false, nil
	}
	switch values[0] {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	default:
		return false, &badRequest{err: errors.New("invalid refresh parameter")}
	}
}
