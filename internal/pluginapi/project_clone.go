package pluginapi

import (
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/transport"
)

func (s *Server) projectCloneRoutes(mux transport.Registrar) {
	serveIn(mux, "POST /api/projects/clone", http.StatusAccepted, workspaceFailure, func(r *http.Request, req contract.ProjectCloneRequest) (contract.ProjectCloneOperation, error) {
		op, err := s.Workspace.StartProjectClone(r.Context(), wire.ProjectCloneRequest(req))
		return wire.ProjectClone(op), err
	})
	serve(mux, "GET /api/projects/operations/{id}", http.StatusOK, workspaceFailure, func(r *http.Request) (contract.ProjectCloneOperation, error) {
		op, err := s.Workspace.ProjectClone(r.PathValue("id"))
		return wire.ProjectClone(op), err
	})
	serve(mux, "POST /api/projects/operations/{id}/register", http.StatusAccepted, workspaceFailure, func(r *http.Request) (contract.ProjectCloneOperation, error) {
		op, err := s.Workspace.RetryProjectCloneRegistration(r.Context(), r.PathValue("id"))
		return wire.ProjectClone(op), err
	})
}
