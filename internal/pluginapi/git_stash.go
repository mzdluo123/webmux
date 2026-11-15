package pluginapi

import (
	"context"
	"github.com/rainchan/webmux/internal/transport"
	"net/http"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/workspace"
)

const gitStashTimeout = 2*time.Minute + 15*time.Second

func (s *Server) gitStashRoutes(mux transport.Registrar) {
	const repoBase = " /api/repositories/{id}/git"
	const wtBase = " /api/repositories/{id}/worktrees/{worktreeId}/git"

	serveGit(mux, "GET"+repoBase+"/stashes", func(ctx context.Context, r *http.Request) (contract.GitStashList, error) {
		v, err := s.Workspace.ListStashes(ctx, r.PathValue("id"))
		return wire.GitStashList(v), err
	})

	serveGit(mux, "GET"+repoBase+"/stashes/{oid}", func(ctx context.Context, r *http.Request) (contract.GitStashDetail, error) {
		v, err := s.Workspace.InspectStash(ctx, r.PathValue("id"), r.PathValue("oid"))
		return wire.GitStashDetail(v), err
	})

	mux.HandleFunc("POST"+repoBase+"/stashes/drop-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitStashDropPreviewRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewStashDrop(ctx, workspace.StashDropPreviewRequest{
			RepositoryID: r.PathValue("id"),
			StashRef:     req.StashRef,
			StashOID:     req.StashOID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitStashDropPreview(preview))
	})

	mux.HandleFunc("POST"+repoBase+"/stashes/drop", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitStashDropRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		res, err := s.Workspace.ExecuteStashDrop(ctx, workspace.ExecuteStashDropRequest{
			RepositoryID:    r.PathValue("id"),
			ConfirmationID:  req.ConfirmationID,
			ClientRequestID: req.ClientRequestID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitStashDropResult(res))
	})

	mux.HandleFunc("POST"+wtBase+"/stash-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitStashCreatePreviewRequest
		if !decodeOptional(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewStashCreate(ctx, workspace.StashCreatePreviewRequest{
			RepositoryID:     r.PathValue("id"),
			WorktreeID:       r.PathValue("worktreeId"),
			Message:          req.Message,
			IncludeUntracked: req.IncludeUntracked,
			KeepIndex:        req.KeepIndex,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitStashCreatePreview(preview, s.sessionSnapshot))
	})

	mux.HandleFunc("POST"+wtBase+"/stash", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitStashExecuteRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStashTimeout)
		defer cancel()
		res, err := s.Workspace.ExecuteStashCreate(ctx, workspace.ExecuteStashCreateRequest{
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
		reply(w, http.StatusOK, wire.GitStashCreateResult(res))
	})

	mux.HandleFunc("POST"+wtBase+"/stash-apply-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitStashApplyPreviewRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewStashApply(ctx, workspace.StashApplyPreviewRequest{
			RepositoryID:   r.PathValue("id"),
			WorktreeID:     r.PathValue("worktreeId"),
			Mode:           req.Mode,
			StashRef:       req.StashRef,
			StashOID:       req.StashOID,
			ReinstateIndex: req.ReinstateIndex,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitStashApplyPreview(preview, s.sessionSnapshot))
	})

	mux.HandleFunc("POST"+wtBase+"/stash-apply", func(w http.ResponseWriter, r *http.Request) {
		s.handleExecuteStashApplyOrPop(w, r, s.Workspace.ExecuteStashApply)
	})

	mux.HandleFunc("POST"+wtBase+"/stash-pop", func(w http.ResponseWriter, r *http.Request) {
		s.handleExecuteStashApplyOrPop(w, r, s.Workspace.ExecuteStashPop)
	})
}

func (s *Server) handleExecuteStashApplyOrPop(
	w http.ResponseWriter,
	r *http.Request,
	fn func(context.Context, workspace.ExecuteStashApplyRequest) (workspace.StashApplyResult, error),
) {
	var req contract.GitStashExecuteRequest
	if !decode(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), gitStashTimeout)
	defer cancel()
	res, err := fn(ctx, workspace.ExecuteStashApplyRequest{
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
	reply(w, http.StatusOK, wire.GitStashApplyResult(res))
}
