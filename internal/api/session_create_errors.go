package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"github.com/rainchan/webmux/internal/wire"
)

func cancelled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
func sessionStatus(err error) int {
	switch {
	case errors.Is(err, session.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, session.ErrRunning), errors.Is(err, session.ErrExited), errors.Is(err, session.ErrNoRun), errors.Is(err, session.ErrNotController), errors.Is(err, session.ErrInputFailed), errors.Is(err, session.ErrWrongInputDestination):
		return http.StatusConflict
	case errors.Is(err, session.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, session.ErrShuttingDown), errors.Is(err, sessioncreate.ErrUnavailable):
		return http.StatusServiceUnavailable
	case cancelled(err):
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
func sessionCommandFailure(w http.ResponseWriter, err error) {
	var bad *badRequest
	if errors.As(err, &bad) {
		failure(w, http.StatusBadRequest, bad.err)
		return
	}
	if errors.Is(err, session.ErrInputFailed) {
		body := contract.SessionConflict{Error: err.Error(), Reason: "input_failed"}
		var failed *session.InputFailedError
		if errors.As(err, &failed) {
			status := wire.SubmissionStatus(failed.Status)
			body.Submission = &status
		}
		reply(w, http.StatusConflict, body)
		return
	}
	if errors.Is(err, session.ErrWrongInputDestination) {
		reply(w, http.StatusConflict, contract.SessionConflict{Error: err.Error(), Reason: "wrong_view"})
		return
	}
	failure(w, sessionStatus(err), err)
}
func renameFailure(w http.ResponseWriter, err error) { sessionCommandFailure(w, err) }
func createStatus(err error) int                     { return sessionStatus(err) }
func createFailure(w http.ResponseWriter, err error) {
	body := contract.CreateSessionFailure{Step: sessioncreate.StepPrecheck}
	var failed *sessioncreate.Error
	if errors.As(err, &failed) {
		body.Step, body.SessionID = failed.Step, failed.SessionID
		err = failed.Err
	}
	status := createStatus(err)
	body.Error = err.Error()
	if status == http.StatusInternalServerError {
		slog.Error("session create failed", "step", body.Step, "error", err)
		body.Error = http.StatusText(status)
	}
	reply(w, status, body)
}
