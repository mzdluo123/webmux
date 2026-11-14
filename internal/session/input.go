package session

import (
	"context"
	"errors"
	"fmt"
)

var ErrInputFailed = errors.New("输入未写入终端")
var ErrWrongInputDestination = errors.New("input is not supported by the current view")

type InputAttachment struct{ Path, Name, MimeType string }
type InputRequest struct {
	ClientRequestID, Label, Text string
	Submit                       bool
	Keys                         [][]byte
	Attachments                  []InputAttachment
	Mode                         string
}
type InputPromptResult struct {
	Queued           bool
	ItemID           string
	Position         int
	TurnID, PromptID string
}
type InputResult struct {
	Side   string
	Prompt InputPromptResult
}
type InputFailedError struct{ Status SubmissionStatus }

func (e *InputFailedError) Error() string {
	return fmt.Sprintf("%v: %s", ErrInputFailed, e.Status.State)
}
func (e *InputFailedError) Unwrap() error { return ErrInputFailed }

// Input uses the same acknowledgement and deduplication gate as native
// terminal input; it does not need an optional chat service.
func (shell *Session) Input(ctx context.Context, in InputRequest) (InputResult, error) {
	if len(in.Attachments) > 0 || in.Mode != "" {
		return InputResult{}, fmt.Errorf("%w: 附件与 mode 只适用于聊天视图", ErrWrongInputDestination)
	}
	var submit []byte
	if in.Submit {
		submit = []byte("\r")
	}
	if err := WriteTerminal(ctx, shell, ServerInput{RequestID: in.ClientRequestID, Label: in.Label, Body: []byte(in.Text), Submit: submit, Keys: in.Keys}); err != nil {
		return InputResult{}, err
	}
	return InputResult{Side: "tui"}, nil
}
func (shell *Session) Interrupt(ctx context.Context, requestID, label string) error {
	return WriteTerminal(ctx, shell, ServerInput{RequestID: requestID, Label: label, Keys: [][]byte{[]byte("\x03")}})
}

// WriteTerminal converts an incomplete receipt to the core input failure.
func WriteTerminal(ctx context.Context, shell *Session, in ServerInput) error {
	status, err := shell.SubmitServer(ctx, in)
	switch {
	case errors.Is(err, ErrNoRun):
		return &InputFailedError{Status: SubmissionStatus{ID: in.RequestID, State: SubmissionMissing}}
	case err != nil:
		return err
	case status.State != SubmissionWritten:
		return &InputFailedError{Status: status}
	}
	return nil
}
