package pluginapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/hybridresume"
	"github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"github.com/rainchan/webmux/internal/transport"
	"github.com/rainchan/webmux/internal/workspace"
)

func (s *Server) sessionExtensionRoutes(mux transport.Registrar) {
	if s.Hybrid != nil {
		serveIn(mux, "POST /api/sessions/{id}/view", http.StatusOK, sessionCommandFailure, func(r *http.Request, req contract.ViewChangeRequest) (contract.ViewChangeResult, error) {
			id := r.PathValue("id")
			if err := s.chatTarget(id); err != nil {
				return contract.ViewChangeResult{Dropped: []string{}, Unsupported: []string{}}, err
			}
			result, err := s.Hybrid.Switch(r.Context(), id, pluginwire.ViewChangeInput(req))
			return pluginwire.ViewChangeResult(result), err
		})
		serveIn(mux, "POST /api/sessions/{id}/resume", http.StatusOK, sessionCommandFailure, func(r *http.Request, req contract.ResumeRequest) (contract.SessionInfo, error) {
			id := r.PathValue("id")
			chat, err := s.sessionKind(id)
			switch {
			case err == nil:
				return contract.SessionInfo{}, hybrid.ErrNotArchived
			case !chat || !errors.Is(err, hybrid.ErrNotActive):
				return contract.SessionInfo{}, err
			}
			item, err := s.Resume.Resume(r.Context(), id, hybridresume.Input{ClientRequestID: req.ClientRequestID, Side: resumeSide(req.View), AgentSessionID: req.AgentSessionID, ConfirmWithoutEnv: req.ConfirmWithoutEnv})
			if err != nil {
				return contract.SessionInfo{}, err
			}
			return s.sessionSnapshot(item.Info()), nil
		})
	}
}
func (s *Server) ListArchived(_ context.Context, states []contract.SessionState) ([]contract.SessionInfo, error) {
	out := []contract.SessionInfo{}
	if s.Hybrid == nil {
		return out, nil
	}
	records, err := s.Hybrid.Records(true)
	if err != nil {
		return out, err
	}
	for _, rec := range records {
		if slices.Contains(states, contract.SessionState(rec.State)) {
			out = append(out, pluginwire.ArchivedSession(rec, s.archived(rec)))
		}
	}
	return out, nil
}
func (s *Server) archived(rec hybrid.Record) pluginwire.Archived {
	out := pluginwire.Archived{ConfigMissing: s.configMissing(rec.ConfigID)}
	if s.Conversations != nil {
		if binding, err := s.Conversations.Binding(context.Background(), rec.ID); err == nil {
			out.ConversationID, out.ProviderThreadID = binding.ConversationID, binding.ProviderThreadID
		}
	}
	if s.Hybrid != nil {
		out.AgentTitle, out.AgentIcon = s.Hybrid.AgentLabel(rec.Agent)
	}
	return out
}
func (s *Server) Info(_ context.Context, id string) (contract.SessionInfo, error) {
	return s.sessionInfo(id)
}
func (s *Server) sessionInfo(id string) (contract.SessionInfo, error) {
	item, err := s.Manager.Get(id)
	if err == nil && s.sessionSnapshot(item.Info()).State == contract.SessionStateActive {
		return s.sessionSnapshot(item.Info()), nil
	}
	if s.Hybrid != nil {
		rec, recErr := s.Hybrid.Record(id)
		switch {
		case recErr == nil && rec.State != hybrid.StateActive:
			return pluginwire.ArchivedSession(rec, s.archived(rec)), nil
		case err != nil && errors.Is(recErr, hybrid.ErrUnavailable):
			return contract.SessionInfo{}, recErr
		}
	}
	if err == nil {
		return s.sessionSnapshot(item.Info()), nil
	}
	return contract.SessionInfo{}, err
}
func (s *Server) Kind(id string) (bool, error) { return s.sessionKind(id) }
func (s *Server) sessionKind(id string) (bool, error) {
	if item, err := s.Manager.Get(id); err == nil {
		if s.Hybrid == nil || !s.Hybrid.HasChatView(id) {
			return false, nil
		}
		if item.Info().Status == session.StatusExited {
			return true, hybrid.ErrNotActive
		}
		return true, nil
	}
	if s.Hybrid != nil {
		switch _, err := s.Hybrid.Record(id); {
		case err == nil:
			return true, hybrid.ErrNotActive
		case errors.Is(err, hybrid.ErrUnavailable):
			return false, err
		}
	}
	return false, session.ErrNotFound
}
func (s *Server) chatTarget(id string) error {
	chat, err := s.sessionKind(id)
	switch {
	case !chat && err == nil:
		return hybrid.ErrNoChatView
	case !chat:
		return err
	case s.Hybrid == nil:
		return hybrid.ErrNotFound
	}
	return nil
}
func (s *Server) Input(ctx context.Context, id string, in session.InputRequest) (session.InputResult, error) {
	return s.Hybrid.Input(ctx, id, in)
}
func (s *Server) Interrupt(ctx context.Context, id, request, label string) error {
	return s.Hybrid.Interrupt(ctx, id, request, label)
}
func (s *Server) Forget(ctx context.Context, id string) error { return s.forget(ctx, id) }
func (s *Server) forget(ctx context.Context, id string) error {
	chat, err := s.sessionKind(id)
	if err != nil && !errors.Is(err, hybrid.ErrNotActive) {
		return err
	}
	if chat && s.Hybrid != nil {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		rec, recErr := s.Hybrid.Record(id)
		if err := s.Hybrid.Forget(ctx, id); err != nil {
			return err
		}
		if recErr == nil && rec.Released != nil && s.Workspace != nil {
			if err := s.Workspace.DropRelease(ctx, *rec.Released); err != nil {
				slog.Warn("released ref removal failed", "release", rec.Released.ID, "error", err)
			}
		}
	} else if err := s.Manager.Remove(id); err != nil {
		return err
	}
	if s.Artifacts != nil {
		s.Artifacts.Remove(id)
	}
	return nil
}
func (s *Server) CreatedLocation(location *sessioncreate.Location) *contract.Worktree {
	if location == nil {
		return nil
	}
	tree, ok := location.Detail.(workspace.Worktree)
	if !ok {
		return nil
	}
	out := pluginwire.Worktree(tree)
	return &out
}
func (s *Server) CreateFailure(w http.ResponseWriter, err error)  { s.createFailure(w, err) }
func (s *Server) CommandFailure(w http.ResponseWriter, err error) { sessionCommandFailure(w, err) }
func (s *Server) stagedUpload(w http.ResponseWriter, r *http.Request) {
	upload, received := s.storeUpload(w, r, s.Artifacts.EnsureStaging)
	if !received {
		if upload != nil {
			upload.Discard()
		}
		return
	}
	reply(w, http.StatusCreated, contract.UploadResult{Path: upload.Path})
}
func resumeSide(view contract.SessionView) string {
	if view == contract.ViewChat {
		return "acp"
	}
	if view == contract.ViewTerminal {
		return "tui"
	}
	return string(view)
}
