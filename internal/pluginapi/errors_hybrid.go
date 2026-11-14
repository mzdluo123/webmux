package pluginapi

import (
	"errors"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	wire "github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/session"
	corewire "github.com/rainchan/webmux/internal/wire"
)

// hybridStatus maps hybrid session error kinds; ok is false for other
// errors. ErrAgent and ErrAgentCall are bad gateways, never 500s: their
// messages carry agent output for the requesting user and must not be
// logged.
func hybridStatus(err error) (status int, ok bool) {
	switch {
	case errors.Is(err, hybrid.ErrInvalid):
		return http.StatusBadRequest, true
	case errors.Is(err, hybrid.ErrTooLarge):
		return http.StatusRequestEntityTooLarge, true
	case errors.Is(err, hybrid.ErrNotFound):
		return http.StatusNotFound, true
	case errors.Is(err, hybrid.ErrLimit), errors.Is(err, hybrid.ErrConflict),
		errors.Is(err, hybrid.ErrWrongSide), errors.Is(err, hybrid.ErrAnswered),
		errors.Is(err, hybrid.ErrSwitching), errors.Is(err, hybrid.ErrSent),
		errors.Is(err, hybrid.ErrSteeringUnavailable), errors.Is(err, hybrid.ErrBusy),
		errors.Is(err, hybrid.ErrNotEligible),
		errors.Is(err, hybrid.ErrSessionMissing), errors.Is(err, hybrid.ErrConfigMissing),
		errors.Is(err, hybrid.ErrConnectorMissing), errors.Is(err, hybrid.ErrSessionAmbiguous),
		errors.Is(err, hybrid.ErrExternalWriter), errors.Is(err, hybrid.ErrNoChatView),
		errors.Is(err, hybrid.ErrNotActive), errors.Is(err, hybrid.ErrNotArchived),
		errors.Is(err, session.ErrInputFailed), errors.Is(err, session.ErrWrongInputDestination):
		return http.StatusConflict, true
	case errors.Is(err, hybrid.ErrAgent), errors.Is(err, hybrid.ErrAgentCall):
		return http.StatusBadGateway, true
	case errors.Is(err, hybrid.ErrUnavailable):
		return http.StatusServiceUnavailable, true
	}
	return 0, false
}

// hybridReason is the contract.SessionConflict reason of a 409 hybrid
// error; empty for a plain conflict.
func hybridReason(err error) string {
	switch {
	case errors.Is(err, hybrid.ErrLimit):
		return "limit"
	case errors.Is(err, hybrid.ErrSwitching):
		return "switching"
	case errors.Is(err, hybrid.ErrWrongSide):
		return "wrong_view"
	case errors.Is(err, session.ErrWrongInputDestination):
		return "wrong_view"
	case errors.Is(err, hybrid.ErrNoChatView):
		return "no_chat_view"
	case errors.Is(err, hybrid.ErrNotActive):
		return "not_active"
	case errors.Is(err, hybrid.ErrNotArchived):
		return "not_archived"
	case errors.Is(err, session.ErrInputFailed):
		return "input_failed"
	case errors.Is(err, hybrid.ErrAnswered):
		return "answered"
	case errors.Is(err, hybrid.ErrSent):
		return "sent"
	case errors.Is(err, hybrid.ErrSteeringUnavailable):
		return "steering_unavailable"
	case errors.Is(err, hybrid.ErrBusy):
		return "busy"
	case errors.Is(err, hybrid.ErrNotEligible):
		return "not_eligible"
	case errors.Is(err, hybrid.ErrSessionMissing):
		return "session_missing"
	case errors.Is(err, hybrid.ErrConfigMissing):
		return "config_missing"
	case errors.Is(err, hybrid.ErrConnectorMissing):
		return "connector_missing"
	case errors.Is(err, hybrid.ErrSessionAmbiguous):
		return "session_ambiguous"
	case errors.Is(err, hybrid.ErrExternalWriter):
		return "occupied_external"
	}
	return ""
}

// hybridFailure answers a hybrid error kind, with a SessionConflict body
// for 409s that have a reason (not_eligible with its eligibility reason,
// session_missing and session_ambiguous with their candidates,
// connector_missing with its connector); false for other errors.
func hybridFailure(w http.ResponseWriter, err error) bool {
	status, ok := hybridStatus(err)
	if !ok {
		return false
	}
	if reason := hybridReason(err); reason != "" {
		conflict := contract.SessionConflict{Error: err.Error(), Reason: reason}
		var ineligible *hybrid.IneligibleError
		if errors.As(err, &ineligible) {
			conflict.Eligibility = ineligible.Reason
		}
		var missing *hybrid.SessionMissingError
		if errors.As(err, &missing) {
			conflict.Candidates = wire.AgentCandidates(missing.Candidates)
		}
		var ambiguous *hybrid.SessionAmbiguousError
		if errors.As(err, &ambiguous) {
			conflict.Candidates = wire.AgentCandidates(ambiguous.Candidates)
		}
		var connector *hybrid.ConnectorMissingError
		if errors.As(err, &connector) {
			conflict.Connector = wire.AgentConnector(connector.Connector)
		}
		var input *session.InputFailedError
		if errors.As(err, &input) {
			submission := corewire.SubmissionStatus(input.Status)
			conflict.Submission = &submission
		}
		reply(w, status, conflict)
		return true
	}
	failure(w, status, err)
	return true
}

// sessionCommandFailure answers a failed session or chat command: hybrid
// kinds with their SessionConflict, session errors with their statuses; a
// request that ended first is a timeout.
func sessionCommandFailure(w http.ResponseWriter, err error) {
	var bad *badRequest
	switch {
	case errors.As(err, &bad):
		failure(w, http.StatusBadRequest, bad.err)
	case hybridFailure(w, err):
	case errors.Is(err, session.ErrNotFound), errors.Is(err, session.ErrRunning), errors.Is(err, session.ErrExited),
		errors.Is(err, session.ErrNoRun), errors.Is(err, session.ErrNotController),
		errors.Is(err, session.ErrInvalid), errors.Is(err, session.ErrShuttingDown):
		failure(w, sessionStatus(err), err)
	case cancelled(err):
		failure(w, http.StatusGatewayTimeout, err)
	default:
		failure(w, http.StatusInternalServerError, err)
	}
}

// sessionFailure answers a session start: a saved launch config keeps its
// launch statuses and a hybrid session its hybrid statuses (a full record
// store is a 409 with reason limit); everything else is a workspace error.
func sessionFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, launch.ErrInvalid) || errors.Is(err, launch.ErrNotFound) || errors.Is(err, launch.ErrDuplicate) || errors.Is(err, launch.ErrUnavailable) {
		launchFailure(w, err)
		return
	}
	if hybridFailure(w, err) {
		return
	}
	failure(w, workspaceStatus(err), err)
}
