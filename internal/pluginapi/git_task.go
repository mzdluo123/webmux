package pluginapi

import (
	"context"
	"github.com/rainchan/webmux/internal/transport"
	"net/http"

	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/workspace"
)

func (s *Server) gitTaskRoutes(mux transport.Registrar) {
	const base = " /api/repositories/{id}/worktrees/{worktreeId}/git"

	serveGit(mux, "GET"+base+"/task-baseline", func(ctx context.Context, r *http.Request) (contract.GitTaskBaselineResponse, error) {
		v, err := s.Workspace.GetTaskBaseline(ctx, r.PathValue("id"), r.PathValue("worktreeId"))
		return wire.GitTaskBaselineResponse(v), err
	})

	mux.HandleFunc("PUT"+base+"/task-baseline", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitPutTaskBaselineRequest
		if !decodeOptional(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		v, err := s.Workspace.PutTaskBaseline(ctx, r.PathValue("id"), r.PathValue("worktreeId"), workspace.PutTaskBaselineRequest{
			BaseOID: req.BaseOID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitTaskBaselineResponse(&v))
	})

	mux.HandleFunc("DELETE"+base+"/task-baseline", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		if err := s.Workspace.DeleteTaskBaseline(ctx, r.PathValue("id"), r.PathValue("worktreeId")); err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})
}

func (s *Server) gitCodeTaskRoutes(mux transport.Registrar) {
	const base = " /api/repositories/{id}/worktrees/{worktreeId}/git"

	serveGit(mux, "GET"+base+"/task-changes", func(ctx context.Context, r *http.Request) (contract.GitTaskChanges, error) {
		q := r.URL.Query()
		v, err := s.Code.GitTaskChanges(ctx, r.PathValue("id"), r.PathValue("worktreeId"), codeview.ChangeQuery{
			Cursor: q.Get("cursor"),
			Q:      q.Get("q"),
			NodeID: q.Get("nodeId"),
		})
		return wire.GitTaskChanges(v), err
	})

	serveGit(mux, "GET"+base+"/task-diff", func(ctx context.Context, r *http.Request) (contract.GitDiff, error) {
		q := r.URL.Query()
		v, err := s.Code.GitTaskDiff(ctx, r.PathValue("id"), r.PathValue("worktreeId"), q.Get("path"), q.Get("changeId"))
		return wire.GitDiff(v), err
	})
}
