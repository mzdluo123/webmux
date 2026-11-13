package api

import (
	"context"
	"errors"
	"github.com/rainchan/webmux/internal/artifacts"
	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/files"
	"github.com/rainchan/webmux/internal/forward"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/push"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/tailscalesettings"
	"github.com/rainchan/webmux/internal/tunnelsettings"
	"github.com/rainchan/webmux/internal/voice"
	"github.com/rainchan/webmux/internal/voicesettings"
	"github.com/rainchan/webmux/internal/wire"
	"github.com/rainchan/webmux/internal/workspace"
	"log/slog"
	"net"
	"net/http"
	"os"
)

// Services report transport-neutral error kinds; this file (together with
// errors_hybrid.go) is the only place that turns them into HTTP statuses.

func cancelled(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// sessionStatus maps a session close or removal error: a missing session is
// 404, a running one 409, and any other failure a server error.
func sessionStatus(err error) int {
	switch {
	case errors.Is(err, session.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, session.ErrRunning), errors.Is(err, session.ErrExited),
		errors.Is(err, session.ErrNoRun), errors.Is(err, session.ErrNotController):
		return http.StatusConflict
	case errors.Is(err, session.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, session.ErrShuttingDown):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// pushStatus maps a push subscription store error: a full store is 409,
// anything else a failed save the caller logs.
func pushStatus(err error) int {
	if errors.Is(err, push.ErrFull) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// uploadReadStatus classifies a failed request body read: the body limit is
// 413, a read deadline 408, anything else a malformed request.
func uploadReadStatus(err error) int {
	var tooLarge *http.MaxBytesError
	var networkError net.Error
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge
	} else if errors.As(err, &networkError) && networkError.Timeout() {
		return http.StatusRequestTimeout
	}
	return http.StatusBadRequest
}

// workspaceStatus maps workspace error kinds; cancellation is a timeout and
// anything else a server error.
func workspaceStatus(err error) int {
	switch {
	case errors.Is(err, workspace.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, workspace.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, workspace.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, workspace.ErrTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, workspace.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, workspace.ErrTimeout), cancelled(err):
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// codeStatus maps code view error kinds. Missing files are 404, and the
// workspace size and timeout failures it surfaces keep their statuses.
func codeStatus(err error) int {
	switch {
	case errors.Is(err, codeview.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, codeview.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, codeview.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, codeview.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, codeview.ErrGone):
		return http.StatusGone
	case errors.Is(err, codeview.ErrTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, codeview.ErrBusy):
		return http.StatusTooManyRequests
	case errors.Is(err, codeview.ErrInternal):
		return http.StatusInternalServerError
	case errors.Is(err, codeview.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, codeview.ErrTimeout), cancelled(err):
		return http.StatusGatewayTimeout
	case errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, workspace.ErrTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, workspace.ErrTimeout):
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// gitStatus maps codeview and workspace error kinds for Git workbench routes.
// Expired snapshots and contexts map to 409 so clients receive a uniform
// structured conflict reason ("stale") alongside index/HEAD state conflicts.
func gitStatus(err error) int {
	var bad *badRequest
	switch {
	case errors.As(err, &bad):
		return http.StatusBadRequest
	case errors.Is(err, codeview.ErrInvalid), errors.Is(err, workspace.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, codeview.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, codeview.ErrNotFound), errors.Is(err, workspace.ErrNotFound), errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, codeview.ErrGone), errors.Is(err, codeview.ErrConflict), errors.Is(err, workspace.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, codeview.ErrTooLarge), errors.Is(err, workspace.ErrTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, codeview.ErrBusy):
		return http.StatusTooManyRequests
	case errors.Is(err, codeview.ErrUnavailable), errors.Is(err, workspace.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, codeview.ErrTimeout), errors.Is(err, workspace.ErrTimeout), cancelled(err):
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// gitReason classifies a 409 Git workbench conflict so the frontend can
// distinguish snapshot refresh ("stale") from external lock contention ("busy")
// or read-only bare/repository contexts ("readonly").
func gitReason(err error) string {
	var draftConflict *workspace.DraftConflictError
	switch {
	case errors.As(err, &draftConflict):
		return "draft_conflict"
	case errors.Is(err, workspace.ErrLocked):
		return "busy"
	case errors.Is(err, workspace.ErrReadOnly), errors.Is(err, codeview.ErrReadOnly):
		return "readonly"
	case errors.Is(err, codeview.ErrContextMismatch):
		return "context_mismatch"
	case errors.Is(err, codeview.ErrNoTaskBaseline):
		return "no_task_baseline"
	case errors.Is(err, workspace.ErrConflicted):
		return "conflicted"
	case errors.Is(err, workspace.ErrBlocked):
		return "blocked"
	case errors.Is(err, workspace.ErrHookFailed):
		return "commit_failed"
	case errors.Is(err, workspace.ErrIdempotency):
		return "idempotency_conflict"
	case errors.Is(err, workspace.ErrUncertain):
		return "uncertain"
	default:
		return "stale"
	}
}

// gitFailure answers a Git workbench error with Cache-Control: no-store,
// structured 409 reason codes, and zero logging of file, diff, or commit text.
func gitFailure(w http.ResponseWriter, err error) {
	w.Header().Set("Cache-Control", "no-store")
	status := gitStatus(err)
	if status == http.StatusConflict {
		body := contract.GitConflict{Error: err.Error(), Reason: gitReason(err)}
		var draftConflict *workspace.DraftConflictError
		if errors.As(err, &draftConflict) {
			cur := wire.GitCommitDraft(draftConflict.Current)
			body.CurrentDraft = &cur
		}
		reply(w, status, body)
		return
	}
	if status == http.StatusInternalServerError {
		slog.Error("git request failed", "status", status)
		reply(w, status, contract.ErrorResponse{Error: http.StatusText(status)})
		return
	}
	var bad *badRequest
	if errors.As(err, &bad) {
		err = bad.err
	}
	failure(w, status, err)
}

// launchStatus maps launch store errors to HTTP statuses; anything else is a
// failed save.
func launchStatus(err error) int {
	switch {
	case errors.Is(err, launch.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, launch.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, launch.ErrDuplicate):
		return http.StatusConflict
	case errors.Is(err, launch.ErrUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// launchFailure answers err. Store errors name fields and paths, never
// environment values.
func launchFailure(w http.ResponseWriter, err error) {
	status := launchStatus(err)
	if status == http.StatusInternalServerError {
		slog.Error("launch config save failed", "error", err)
	}
	failure(w, status, err)
}

func conversationFailure(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, conversation.ErrInvalid), errors.Is(err, launch.ErrInvalid), errors.Is(err, workspace.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, conversation.ErrNotFound), errors.Is(err, launch.ErrNotFound), errors.Is(err, workspace.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, conversation.ErrConflict), errors.Is(err, workspace.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, conversation.ErrUnavailable), errors.Is(err, launch.ErrUnavailable), errors.Is(err, workspace.ErrUnavailable):
		status = http.StatusServiceUnavailable
	case cancelled(err):
		status = http.StatusGatewayTimeout
	}
	failure(w, status, err)
}

// filesFailure answers a files service error. A failed read of the upload
// body keeps the request-body classification.
func filesFailure(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, files.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, files.ErrTooLarge):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, files.ErrRead):
		status = uploadReadStatus(err)
	}
	failure(w, status, err)
}

// artifactsFailure answers an artifacts folder error.
func artifactsFailure(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, artifacts.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, artifacts.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, artifacts.ErrTooLarge):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, artifacts.ErrUnavailable):
		status = http.StatusServiceUnavailable
	}
	failure(w, status, err)
}

func workspaceFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, workspace.ErrIdempotency) || errors.Is(err, workspace.ErrUncertain) {
		reply(w, http.StatusConflict, contract.GitConflict{Error: err.Error(), Reason: gitReason(err)})
		return
	}
	failure(w, workspaceStatus(err), err)
}

func tunnelStatus(err error) int {
	switch {
	case errors.Is(err, tunnelsettings.ErrNotPersisted):
		return http.StatusServiceUnavailable
	case errors.Is(err, tunnelsettings.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, tunnelsettings.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, tunnelsettings.ErrEnableBlocked):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func tunnelFailure(w http.ResponseWriter, err error) {
	status := tunnelStatus(err)
	message := err.Error()
	if status == http.StatusInternalServerError {
		message = "tunnel settings operation failed"
	}
	reply(w, status, contract.TunnelSettingsError{Error: message, NotPersisted: errors.Is(err, tunnelsettings.ErrNotPersisted)})
}

// localTunnelFailure preserves the local callback's refusal contract while
// keeping every service-error status decision in this file.
func localTunnelFailure(w http.ResponseWriter, err error) {
	if tunnelStatus(err) == http.StatusInternalServerError {
		failure(w, http.StatusConflict, err)
		return
	}
	tunnelFailure(w, err)
}

func forwardRedeemStatus(err error) int {
	if errors.Is(err, forward.ErrUnauthorized) {
		return http.StatusUnauthorized
	}
	return http.StatusInternalServerError
}

func voiceStatus(err error) int {
	if errors.Is(err, voice.ErrDisabled) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

// tailscaleStatus maps settings errors; a shutdown or other refusal is 409.
func tailscaleStatus(err error) int {
	switch {
	case errors.Is(err, tailscalesettings.ErrInvalid), errors.Is(err, tailscalesettings.ErrFunnelUnconfirmed):
		return http.StatusBadRequest
	case errors.Is(err, tailscalesettings.ErrUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusConflict
	}
}

// voiceSettingsStatus maps recognizer settings errors; an unreadable store
// is 503, anything else a rejected value.
func voiceSettingsStatus(err error) int {
	switch {
	case errors.Is(err, voicesettings.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, voicesettings.ErrUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func settingsStatus(err error) int {
	switch {
	case errors.Is(err, settings.ErrInvalid):
		return 400
	case errors.Is(err, settings.ErrConflict):
		return 409
	case errors.Is(err, settings.ErrUnavailable):
		return 503
	default:
		return 500
	}
}

// forwardFailure maps forward registry errors: a bad port or option is 400,
// a port already forwarded 409, a missing forward 404.
func forwardFailure(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, forward.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, forward.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, forward.ErrNotFound):
		status = http.StatusNotFound
	}
	failure(w, status, err)
}
