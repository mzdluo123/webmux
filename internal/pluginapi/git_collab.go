package pluginapi

import (
	"context"
	"github.com/rainchan/webmux/internal/transport"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/workspace"
)

func (s *Server) gitCollabRoutes(mux transport.Registrar) {
	const base = " /api/repositories/{id}/worktrees/{worktreeId}/git"

	serveGit(mux, "GET"+base+"/commit-draft", func(ctx context.Context, r *http.Request) (contract.GitCommitDraft, error) {
		v, err := s.Workspace.GetCommitDraft(ctx, r.PathValue("id"), r.PathValue("worktreeId"))
		return wire.GitCommitDraft(v), err
	})

	mux.HandleFunc("PUT"+base+"/commit-draft", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitPutCommitDraftRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		v, err := s.Workspace.PutCommitDraft(ctx, r.PathValue("id"), r.PathValue("worktreeId"), workspace.PutCommitDraftRequest{
			Subject:      req.Subject,
			Body:         req.Body,
			BaseRevision: req.BaseRevision,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitCommitDraft(v))
	})

	serveGit(mux, "GET"+base+"/reviews", func(ctx context.Context, r *http.Request) (contract.GitReviewsResponse, error) {
		v, err := s.Workspace.GetReviews(ctx, r.PathValue("id"), r.PathValue("worktreeId"), r.URL.Query().Get("scope"))
		return wire.GitReviewsResponse(v), err
	})

	mux.HandleFunc("PUT"+base+"/reviews", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitPutReviewsRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		v, err := s.Workspace.PutReviews(ctx, r.PathValue("id"), r.PathValue("worktreeId"), wire.GitPutReviewsRequest(req))
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitReviewsResponse(v))
	})
}
