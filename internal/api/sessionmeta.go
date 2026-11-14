package api

import (
	"net/http"
	"strings"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
)

func (s *Server) sessionMetaRoutes(mux *http.ServeMux) {
	serve(mux, "GET /api/sessions/{id}/details", http.StatusOK, renameFailure, s.details)
	serveIn(mux, "PATCH /api/sessions/{id}/labels", http.StatusOK, renameFailure, func(r *http.Request, req contract.LabelsUpdate) (contract.SessionInfo, error) {
		info, err := s.Manager.UpdateLabels(r.PathValue("id"), req.Set, req.Remove)
		return s.sessionSnapshot(info), err
	})
	serveIn(mux, "POST /api/sessions/{id}/attention", http.StatusOK, renameFailure, func(r *http.Request, req contract.AttentionRequest) (contract.SessionInfo, error) {
		info, err := s.Manager.RaiseAttention(r.PathValue("id"), req.Kind, req.Message)
		return s.sessionSnapshot(info), err
	})
	serveDone(mux, "DELETE /api/sessions/{id}/attention", renameFailure, func(r *http.Request) error {
		_, err := s.Manager.ClearAttention(r.PathValue("id"))
		return err
	})
}
func (s *Server) callerSession(r *http.Request) string {
	if !s.local {
		return ""
	}
	pid, ok := r.Context().Value(peerKey{}).(int)
	if !ok || pid <= 0 {
		return ""
	}
	id, _ := s.Manager.ProcessSession(pid)
	return id
}
func (s *Server) adopt(r *http.Request, id string, labels map[string]string) (session.Info, error) {
	s.Manager.SetParent(id, s.callerSession(r))
	if len(labels) > 0 {
		return s.Manager.UpdateLabels(id, labels, nil)
	}
	item, err := s.Manager.Get(id)
	if err != nil {
		return session.Info{}, err
	}
	return item.Info(), nil
}
func (s *Server) details(r *http.Request) (contract.SessionDetails, error) {
	item, err := s.Manager.Get(r.PathValue("id"))
	if err != nil {
		return contract.SessionDetails{}, err
	}
	info := item.Info()
	out := contract.SessionDetails{Session: s.sessionSnapshot(info), ParentID: info.ParentID, Labels: info.Labels, Children: s.Manager.Children(info.ID), Forwards: []contract.Forward{}}
	if out.Labels == nil {
		out.Labels = map[string]string{}
	}
	if s.LocalURL != "" {
		link := strings.TrimSuffix(s.LocalURL, "/") + "/s/" + info.ID
		out.Link = &link
	}
	if s.EnrichDetails != nil {
		s.EnrichDetails(r.Context(), info, &out)
	}
	return out, nil
}
