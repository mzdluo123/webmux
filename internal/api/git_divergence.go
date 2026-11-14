package api

import (
	"context"
	"net/http"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/wire"
	"github.com/rainchan/webmux/internal/workspace"
)

const gitDivergenceTimeout = 3*time.Minute + 15*time.Second

func (s *Server) gitDivergenceRoutes(mux *http.ServeMux) {
	const base = " /api/repositories/{id}/worktrees/{worktreeId}/git"

	serveGit(mux, "GET"+base+"/operation-state", func(ctx context.Context, r *http.Request) (contract.GitInProgressOperation, error) {
		v, err := s.Workspace.InspectInProgressOperation(ctx, r.PathValue("id"), r.PathValue("worktreeId"))
		return wire.GitInProgressOperation(v), err
	})

	serveGit(mux, "GET"+base+"/conflict", func(ctx context.Context, r *http.Request) (contract.GitConflictFileDetail, error) {
		v, err := s.Workspace.InspectConflictFile(ctx, r.PathValue("id"), r.PathValue("worktreeId"), r.URL.Query().Get("path"))
		return wire.GitConflictFileDetail(v), err
	})

	mux.HandleFunc("POST"+base+"/conflict/save", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitSaveConflictRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		res, err := s.Workspace.SaveConflictResult(ctx, wire.GitSaveConflictRequest(r.PathValue("id"), r.PathValue("worktreeId"), req))
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitSaveConflictResponse(res))
	})

	mux.HandleFunc("POST"+base+"/conflict/resolve", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitResolveConflictRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		res, err := s.Workspace.ResolveConflict(ctx, wire.GitResolveConflictRequest(r.PathValue("id"), r.PathValue("worktreeId"), req))
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitResolveConflictResponse(res))
	})

	mux.HandleFunc("POST"+base+"/merge-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitBranchDivergencePreviewRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewMerge(ctx, workspace.DivergencePreviewRequest{
			RepositoryID: r.PathValue("id"),
			WorktreeID:   r.PathValue("worktreeId"),
			RefKind:      req.RefKind,
			RefName:      req.RefName,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitDivergencePreview(preview))
	})

	mux.HandleFunc("POST"+base+"/merge", func(w http.ResponseWriter, r *http.Request) {
		s.handleExecuteDivergence(w, r, s.Workspace.ExecuteMerge)
	})

	mux.HandleFunc("POST"+base+"/rebase-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitBranchDivergencePreviewRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewRebase(ctx, workspace.DivergencePreviewRequest{
			RepositoryID: r.PathValue("id"),
			WorktreeID:   r.PathValue("worktreeId"),
			RefKind:      req.RefKind,
			RefName:      req.RefName,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitDivergencePreview(preview))
	})

	mux.HandleFunc("POST"+base+"/rebase", func(w http.ResponseWriter, r *http.Request) {
		s.handleExecuteDivergence(w, r, s.Workspace.ExecuteRebase)
	})

	mux.HandleFunc("POST"+base+"/cherry-pick-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitCommitDivergencePreviewRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewCherryPick(ctx, workspace.DivergencePreviewRequest{
			RepositoryID: r.PathValue("id"),
			WorktreeID:   r.PathValue("worktreeId"),
			CommitOID:    req.CommitOID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitDivergencePreview(preview))
	})

	mux.HandleFunc("POST"+base+"/cherry-pick", func(w http.ResponseWriter, r *http.Request) {
		s.handleExecuteDivergence(w, r, s.Workspace.ExecuteCherryPick)
	})

	mux.HandleFunc("POST"+base+"/revert-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitCommitDivergencePreviewRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewRevert(ctx, workspace.DivergencePreviewRequest{
			RepositoryID: r.PathValue("id"),
			WorktreeID:   r.PathValue("worktreeId"),
			CommitOID:    req.CommitOID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitDivergencePreview(preview))
	})

	mux.HandleFunc("POST"+base+"/revert", func(w http.ResponseWriter, r *http.Request) {
		s.handleExecuteDivergence(w, r, s.Workspace.ExecuteRevert)
	})

	mux.HandleFunc("POST"+base+"/operation-continue", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitContinueOperationRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitDivergenceTimeout)
		defer cancel()
		res, err := s.Workspace.ContinueOperation(ctx, workspace.ContinueOperationRequest{
			RepositoryID:    r.PathValue("id"),
			WorktreeID:      r.PathValue("worktreeId"),
			Kind:            req.Kind,
			ClientRequestID: req.ClientRequestID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitDivergenceResult(res))
	})

	mux.HandleFunc("POST"+base+"/operation-abort-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitAbortOperationPreviewRequest
		if !decodeOptional(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewAbortOperation(ctx, r.PathValue("id"), r.PathValue("worktreeId"))
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitAbortOperationPreview(preview))
	})

	mux.HandleFunc("POST"+base+"/operation-abort", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitAbortOperationRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitDivergenceTimeout)
		defer cancel()
		res, err := s.Workspace.AbortOperation(ctx, workspace.ExecuteAbortOperationRequest{
			RepositoryID:        r.PathValue("id"),
			WorktreeID:          r.PathValue("worktreeId"),
			ConfirmationID:      req.ConfirmationID,
			AcknowledgeSessions: req.AcknowledgeSessions,
			ClientRequestID:     req.ClientRequestID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitDivergenceResult(res))
	})
}

func (s *Server) handleExecuteDivergence(
	w http.ResponseWriter,
	r *http.Request,
	fn func(context.Context, workspace.ExecuteDivergenceRequest) (workspace.DivergenceResult, error),
) {
	var req contract.GitDivergenceExecuteRequest
	if !decode(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), gitDivergenceTimeout)
	defer cancel()
	res, err := fn(ctx, workspace.ExecuteDivergenceRequest{
		RepositoryID:        r.PathValue("id"),
		WorktreeID:          r.PathValue("worktreeId"),
		ConfirmationID:      req.ConfirmationID,
		AcknowledgeSessions: req.AcknowledgeSessions,
		ClientRequestID:     req.ClientRequestID,
	})
	if err != nil {
		gitFailure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	reply(w, http.StatusOK, wire.GitDivergenceResult(res))
}
