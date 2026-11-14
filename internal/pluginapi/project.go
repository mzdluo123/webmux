package pluginapi

import (
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/transport"
)

func (s *Server) projectRoutes(mux transport.Registrar) {
	serve(mux, "GET /api/projects", http.StatusOK, workspaceFailure, func(r *http.Request) ([]contract.Project, error) {
		projects, err := s.Workspace.ListProjects()
		return wire.Projects(projects), err
	})
	serve(mux, "GET /api/projects/directories", http.StatusOK, workspaceFailure, func(r *http.Request) (contract.DirectoryList, error) {
		dirs, err := s.Workspace.ProjectDirectories(r.URL.Query().Get("prefix"))
		return contract.DirectoryList(dirs), err
	})
	mux.HandleFunc("POST /api/projects", func(w http.ResponseWriter, r *http.Request) {
		var req contract.OpenProjectRequest
		if !decode(w, r, &req) {
			return
		}
		project, created, err := s.Workspace.OpenProject(r.Context(), req.Path)
		if err != nil {
			workspaceFailure(w, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		reply(w, status, wire.Project(project))
	})
	mux.HandleFunc("POST /api/projects/create", func(w http.ResponseWriter, r *http.Request) {
		var req contract.CreateProjectRequest
		if !decode(w, r, &req) {
			return
		}
		project, err := s.Workspace.CreateProject(r.Context(), req.Parent, req.Name, req.InitializeGit)
		if err != nil {
			if project.Path != "" {
				reply(w, workspaceStatus(err), contract.ProjectCreationFailure{Error: err.Error(), Path: project.Path})
			} else {
				workspaceFailure(w, err)
			}
			return
		}
		reply(w, http.StatusCreated, wire.Project(project))
	})
}
