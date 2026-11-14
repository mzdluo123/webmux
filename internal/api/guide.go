package api

import (
	"net/http"
	"strings"

	"github.com/rainchan/webmux/internal/agentguide"
)

// guideRoutes serves the shell guide llms.txt-style and the {{webmux}}
// brief. The guide pages lie outside /api/, so like the static app they
// need no token on the TCP listener; there they carry nothing about the
// running instance. The local socket adds its version, executable and
// socket, and names the session the caller runs in, found from the peer
// process. The guide itself, with the command directory cmd/ rendered from
// the command table, is the instance's.
func (s *Server) guideRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /llms.txt", func(w http.ResponseWriter, r *http.Request) {
		guideText(w, "text/plain; charset=utf-8", s.guidePages().Index(s.guideInstance(), s.callerSession(r)))
	})
	mux.HandleFunc("GET /llms-full.txt", func(w http.ResponseWriter, r *http.Request) {
		guideText(w, "text/plain; charset=utf-8", s.guidePages().Full(s.guideInstance(), s.callerSession(r)))
	})
	mux.HandleFunc("GET /llms/{page}", func(w http.ResponseWriter, r *http.Request) {
		name, ok := strings.CutSuffix(r.PathValue("page"), ".md")
		page, found := s.guidePages().Page(name)
		if !ok || !found {
			http.NotFound(w, r)
			return
		}
		guideText(w, "text/markdown; charset=utf-8", page)
	})
}

// guidePages is the injected guide; without one the topics are served with
// an empty command directory.
func (s *Server) guidePages() agentguide.Guide {
	if s.Guide == nil {
		return agentguide.Guide{}
	}
	return s.Guide.Guide
}

// guideInstance is the running instance on the local socket, nil on TCP.
func (s *Server) guideInstance() *agentguide.Instance {
	if !s.local {
		return nil
	}
	return s.Guide
}

func guideText(w http.ResponseWriter, contentType, text string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(text))
}
