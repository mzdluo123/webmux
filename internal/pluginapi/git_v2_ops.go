package pluginapi

import (
	"context"
	"github.com/rainchan/webmux/internal/transport"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/workspace"
)

func (s *Server) gitV2Routes(mux transport.Registrar) {
	const repo = " /api/repositories/{id}"
	const gitBase = " /api/repositories/{id}/git"

	serveGit(mux, "GET"+gitBase+"/file-history", func(ctx context.Context, r *http.Request) (contract.GitFileHistoryPage, error) {
		q := r.URL.Query()
		limit, err := parseOptionalPositiveInt(q.Get("limit"))
		if err != nil {
			return contract.GitFileHistoryPage{}, err
		}
		v, err := s.Workspace.FileHistory(ctx, workspace.FileHistoryQuery{
			RepositoryID: r.PathValue("id"),
			WorktreeID:   q.Get("worktreeId"),
			StartOID:     q.Get("startOid"),
			Path:         q.Get("path"),
			Limit:        limit,
			Cursor:       q.Get("cursor"),
		})
		return wire.GitFileHistoryPage(v), err
	})

	serveGit(mux, "GET"+gitBase+"/blame", func(ctx context.Context, r *http.Request) (contract.GitBlameResult, error) {
		q := r.URL.Query()
		v, err := s.Workspace.Blame(ctx, workspace.BlameQuery{
			RepositoryID: r.PathValue("id"),
			WorktreeID:   q.Get("worktreeId"),
			Rev:          q.Get("rev"),
			Path:         q.Get("path"),
		})
		return wire.GitBlameResult(v), err
	})

	mux.HandleFunc("POST"+repo+"/branches/rename-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitBranchRenamePreviewRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewBranchRename(ctx, r.PathValue("id"), workspace.BranchRenamePreviewRequest{
			OldName: req.OldName,
			NewName: req.NewName,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitBranchRenamePreview(preview, s.sessionSnapshot))
	})

	mux.HandleFunc("POST"+repo+"/branches/rename", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitBranchRenameRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		res, err := s.Workspace.RenameBranch(ctx, r.PathValue("id"), workspace.RenameBranchRequest{
			ConfirmationID:      req.ConfirmationID,
			AcknowledgeSessions: req.AcknowledgeSessions,
			ClientRequestID:     req.ClientRequestID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitBranchMutationResult(res))
	})

	mux.HandleFunc("POST"+repo+"/branches/upstream", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitBranchUpstreamRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		res, err := s.Workspace.SetBranchUpstream(ctx, r.PathValue("id"), workspace.BranchUpstreamRequest{
			Branch:          req.Branch,
			Upstream:        req.Upstream,
			ClientRequestID: req.ClientRequestID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitBranchMutationResult(res))
	})

	mux.HandleFunc("POST"+repo+"/tags", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitTagCreateRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		res, err := s.Workspace.CreateTag(ctx, r.PathValue("id"), workspace.CreateTagRequest{
			Name:            req.Name,
			TargetOID:       req.TargetOID,
			Annotated:       req.Annotated,
			Message:         req.Message,
			ClientRequestID: req.ClientRequestID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusCreated, wire.GitTagCreateResult(res))
	})

	mux.HandleFunc("POST"+repo+"/tags/delete", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitTagDeleteRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		res, err := s.Workspace.DeleteTag(ctx, r.PathValue("id"), workspace.DeleteTagRequest{
			Name:              req.Name,
			ExpectedTargetOID: req.ExpectedTargetOID,
			ClientRequestID:   req.ClientRequestID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitTagDeleteResult(res))
	})

	mux.HandleFunc("POST"+repo+"/remotes", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitRemoteAddRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		res, err := s.Workspace.AddRemote(ctx, r.PathValue("id"), workspace.AddRemoteRequest{
			Name:            req.Name,
			FetchURL:        req.FetchURL,
			PushURL:         req.PushURL,
			ClientRequestID: req.ClientRequestID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusCreated, wire.GitRemoteMutationResult(res))
	})

	mux.HandleFunc("PATCH"+repo+"/remotes/{name}", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitRemoteUpdateRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		res, err := s.Workspace.UpdateRemote(ctx, r.PathValue("id"), r.PathValue("name"), workspace.UpdateRemoteRequest{
			FetchURL:        req.FetchURL,
			PushURL:         req.PushURL,
			ClearPushURL:    req.ClearPushURL,
			ClientRequestID: req.ClientRequestID,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitRemoteMutationResult(res))
	})

	mux.HandleFunc("DELETE"+repo+"/remotes/{name}", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitRemoteRemoveRequest
		if !decodeOptional(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitStageTimeout)
		defer cancel()
		res, err := s.Workspace.RemoveRemote(ctx, r.PathValue("id"), r.PathValue("name"), req.ClientRequestID)
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitRemoteMutationResult(res))
	})
}
