package pluginapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"github.com/rainchan/webmux/internal/workspace"
)

func createStatus(err error) int {
	if errors.Is(err, launch.ErrInvalid) || errors.Is(err, launch.ErrNotFound) || errors.Is(err, launch.ErrDuplicate) || errors.Is(err, launch.ErrUnavailable) {
		return launchStatus(err)
	}
	if status, ok := hybridStatus(err); ok {
		return status
	}
	if errors.Is(err, session.ErrInvalid) || errors.Is(err, session.ErrShuttingDown) {
		return sessionStatus(err)
	}
	if errors.Is(err, sessioncreate.ErrUnavailable) {
		return http.StatusServiceUnavailable
	}
	return workspaceStatus(err)
}
func (s *Server) createFailure(w http.ResponseWriter, err error) {
	body := contract.CreateSessionFailure{Step: sessioncreate.StepPrecheck}
	var failed *sessioncreate.Error
	if errors.As(err, &failed) {
		body.Step, body.Worktree, body.SessionID = failed.Step, s.CreatedLocation(failed.Location), failed.SessionID
		err = failed.Err
	}
	status := createStatus(err)
	body.Error = err.Error()
	if status == http.StatusInternalServerError {
		slog.Error("session create failed", "step", body.Step, "error", err)
		body.Error = http.StatusText(status)
	}
	if _, ok := hybridStatus(err); ok {
		body.Reason = hybridReason(err)
		var ineligible *hybrid.IneligibleError
		if errors.As(err, &ineligible) {
			body.Eligibility = ineligible.Reason
		}
		var connector *hybrid.ConnectorMissingError
		if errors.As(err, &connector) {
			body.Connector = pluginwire.AgentConnector(connector.Connector)
		}
	} else if errors.Is(err, workspace.ErrConflict) {
		body.Reason = "conflict"
	}
	reply(w, status, body)
}
