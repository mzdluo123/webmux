package pluginapi

import (
	"errors"
	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/contract"
	"net/http"
)

func (s *Server) agentGuide(w http.ResponseWriter, r *http.Request) {
	if s.Guide == nil {
		failure(w, http.StatusServiceUnavailable, errGuideUnavailable)
		return
	}
	id := r.PathValue("id")
	if _, err := s.Manager.Get(id); err != nil {
		failure(w, http.StatusNotFound, err)
		return
	}
	dir, err := s.Artifacts.Ensure(id)
	if err != nil {
		artifactsFailure(w, err)
		return
	}
	out := contract.AgentGuidePrompt{
		Prompt:   s.Guide.Brief(agentguide.BriefInput{SessionID: id, Dir: dir}),
		GuideURL: s.Guide.GuideURL(),
		Dir:      dir,
	}
	if bin := s.Guide.Bin; bin != "" {
		out.Bin = &bin
	}
	if socket := s.Guide.Socket(); socket != "" {
		out.Socket = &socket
	}
	reply(w, http.StatusOK, out)
}

var errGuideUnavailable = errors.New("WebMux 工具说明不可用")
