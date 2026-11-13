package api

import (
	"context"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/wire"
	"github.com/rainchan/webmux/internal/workspace"
)

func (s *Server) gitV3Routes(mux *http.ServeMux) {
	const base = " /api/repositories/{id}/worktrees/{worktreeId}/git"

	mux.HandleFunc("POST"+base+"/discard-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitDiscardPreviewRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewDiscard(
			ctx,
			wire.GitDiscardPreviewRequest(r.PathValue("id"), r.PathValue("worktreeId"), req),
		)
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitDiscardPreview(preview))
	})

	mux.HandleFunc("POST"+base+"/discard", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitDiscardRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitCommitTimeout)
		defer cancel()
		res, err := s.Workspace.ExecuteDiscard(ctx, workspace.ExecuteDiscardRequest{
			RepositoryID:        r.PathValue("id"),
			WorktreeID:          r.PathValue("worktreeId"),
			ConfirmationID:      req.ConfirmationID,
			ClientRequestID:     req.ClientRequestID,
			AcknowledgeSessions: req.AcknowledgeSessions,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitDiscardResult(res))
	})

	mux.HandleFunc("POST"+base+"/smart-checkout-preview", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitSmartCheckoutPreviewRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		preview, err := s.Workspace.PreviewSmartCheckout(
			ctx,
			wire.GitSmartCheckoutPreviewRequest(r.PathValue("id"), r.PathValue("worktreeId"), req),
		)
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitSmartCheckoutPreview(preview))
	})

	mux.HandleFunc("POST"+base+"/smart-checkout", func(w http.ResponseWriter, r *http.Request) {
		var req contract.GitSmartCheckoutRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), gitCommitTimeout)
		defer cancel()
		res, err := s.Workspace.ExecuteSmartCheckout(ctx, workspace.ExecuteSmartCheckoutRequest{
			RepositoryID:        r.PathValue("id"),
			WorktreeID:          r.PathValue("worktreeId"),
			ConfirmationID:      req.ConfirmationID,
			ClientRequestID:     req.ClientRequestID,
			AcknowledgeSessions: req.AcknowledgeSessions,
		})
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, wire.GitSmartCheckoutResult(res))
	})
}
