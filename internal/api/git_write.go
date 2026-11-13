package api

import (
	"context"
	"net/http"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/wire"
	"github.com/rainchan/webmux/internal/workspace"
)

const (
	gitStageTimeout  = 30 * time.Second
	gitCommitTimeout = 2*time.Minute + 15*time.Second
)

func (s *Server) gitWriteRoutes(mux *http.ServeMux) {
	const base = " /api/repositories/{id}/worktrees/{worktreeId}/git"

	serveGit(mux, "GET"+base+"/staging", func(ctx context.Context, r *http.Request) (contract.GitStagingOverview, error) {
		v, err := s.Workspace.InspectStaging(ctx, r.PathValue("id"), r.PathValue("worktreeId"))
		return wire.GitStagingOverview(v), err
	})

	serveGit(mux, "GET"+base+"/staging/diff", func(ctx context.Context, r *http.Request) (contract.GitStagingFileDiff, error) {
		q := r.URL.Query()
		v, err := s.Workspace.InspectStagingFile(
			ctx,
			r.PathValue("id"),
			r.PathValue("worktreeId"),
			workspace.StageArea(q.Get("area")),
			q.Get("path"),
			q.Get("oldPath"),
		)
		return wire.GitStagingFileDiff(v), err
	})

	mux.HandleFunc("POST"+base+"/stage", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitStageRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		if err := s.Workspace.Stage(ctx, wire.GitStageRequest(r.PathValue("id"), r.PathValue("worktreeId"), req)); err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST"+base+"/unstage", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitStageRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		if err := s.Workspace.Unstage(ctx, wire.GitStageRequest(r.PathValue("id"), r.PathValue("worktreeId"), req)); err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST"+base+"/commit-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitCommitPreviewRequest
		if !decodeOptional(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewCommit(ctx, r.PathValue("id"), r.PathValue("worktreeId"))
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitCommitPreview(preview))
	})

	// Commit execution uses an independent bounded window (2m inside workspace)
	// and detaches the git commit subprocess from client cancellation once it
	// starts so browser disconnects never leave a half-written commit.
	mux.HandleFunc("POST"+base+"/commit", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitCommitRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitCommitTimeout)
		defer cancel()
		res, err := s.Workspace.ExecuteCommit(ctx, workspace.ExecuteCommitRequest{
			RepositoryID:   r.PathValue("id"),
			WorktreeID:     r.PathValue("worktreeId"),
			ConfirmationID: req.ConfirmationID,
			Message:        req.Message,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitCommitResult(res))
	})
}
