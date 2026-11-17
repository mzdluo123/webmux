package hybrid

import (
	"context"
	"fmt"

	"github.com/rainchan/webmux/internal/acp"
)

// PromptResult reports what a prompt did: Queued with the queue item's
// ItemID and 1-based Position, or sent as turn TurnID for prompt PromptID
// (steering sends neither).
type PromptResult struct {
	Queued   bool
	ItemID   string
	Position int
	TurnID   string
	PromptID string
}

// PromptInput is a chat prompt. Mode is ModeAuto (default), ModeSteer,
// ModeRefuse or ModeNow. TitleSource, when set, replaces Text as the source
// of the temporary first-prompt title (Text may carry an instruction
// prefix).
type PromptInput struct {
	ExpectedIdentity     string
	AwaitAcknowledgement bool
	ExpectedUsage        *Usage
	ClientRequestID      string
	Text                 string
	TitleSource          string
	Mode                 string
	Attachments          []Attachment
	Images               []PromptImage
}

type promptDispatch struct {
	result          PromptResult
	acknowledgement *promptAcknowledgement
}

// QueueEdit changes a queued prompt: new Text and/or Move MoveUp.
type QueueEdit struct {
	ClientRequestID string
	Text            *string
	Move            string
}

// ConfigInput sets agent option ConfigID: Value for a select option,
// Enabled for a boolean one.
type ConfigInput struct {
	ClientRequestID string
	ConfigID        string
	Value           *string
	Enabled         *bool
}

// PermissionModeInput sets how the chat side answers permission requests.
type PermissionModeInput struct {
	ClientRequestID string
	Mode            PermissionMode
}

// Every chat command below carries the client's request ID: a retry with
// the same ID within 30 minutes (and among the session's last 256) gets the
// first outcome, success or error, without running again. Commands fail
// with ErrSwitching during a switch and ErrWrongSide outside the chat side.
// A prompt sent while the agent is reconnecting waits in the queue until
// the agent is back.

// command runs fn on session id's actor once per client request ID.
func command[T any](ctx context.Context, s *Service, id, requestID, name string, fn func(a *actor) (T, error)) (T, error) {
	a, err := s.actor(id)
	if err != nil {
		var zero T
		return zero, err
	}
	return once(ctx, &a.recent, requestID, name, func(ctx context.Context) (T, error) {
		return call(ctx, a, func() (T, error) { return fn(a) })
	})
}

// agentCommand is command for commands that end in an ACP call: prepare
// runs on the actor, the call outside it.
func agentCommand(ctx context.Context, s *Service, id, requestID, name string, prepare func(a *actor) (agentCall, error)) error {
	a, err := s.actor(id)
	if err != nil {
		return err
	}
	_, err = once(ctx, &a.recent, requestID, name, func(ctx context.Context) (struct{}, error) {
		run, err := call(ctx, a, func() (agentCall, error) { return prepare(a) })
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, run(ctx)
	})
	return err
}

// Prompt sends a prompt. ModeAuto starts a turn when the session is idle
// and its queue empty and running, and queues the text otherwise; queued
// prompts go out one by one as turns end normally. ModeSteer injects the
// text into the running turn and fails with ErrSteeringUnavailable when
// the agent cannot. ModeRefuse fails with ErrBusy instead of queueing.
// ModeNow cancels the running turn and sends the prompt next. The text is
// at most MaxPromptBytes and may be blank when the prompt has attachments.
func (s *Service) Prompt(ctx context.Context, id string, in PromptInput) (PromptResult, error) {
	if len(in.Images) > 0 && (in.Mode != ModeRefuse || len(in.Attachments) > 0) {
		return PromptResult{}, fmt.Errorf("%w: 冻结图片仅支持直接发送", ErrInvalid)
	}
	a, err := s.actor(id)
	if err != nil {
		return PromptResult{}, err
	}
	// Resolve attachments inside the deduped command: a retried request ID
	// must return the first outcome without reading the files again.
	switch in.Mode {
	case "", ModeAuto:
		return once(ctx, &a.recent, in.ClientRequestID, "prompt", func(ctx context.Context) (PromptResult, error) {
			attachments, err := resolveAttachments(s.uploadCheck(id), in.Attachments)
			if err != nil {
				return PromptResult{}, err
			}
			if err := validText(in.Text, len(attachments)); err != nil {
				return PromptResult{}, err
			}
			return call(ctx, a, func() (PromptResult, error) {
				return a.prompt(in.Text, in.TitleSource, attachments)
			})
		})
	case ModeSteer:
		_, err = once(ctx, &a.recent, in.ClientRequestID, "steer", func(ctx context.Context) (struct{}, error) {
			attachments, err := resolveAttachments(s.uploadCheck(id), in.Attachments)
			if err != nil {
				return struct{}{}, err
			}
			if err := validText(in.Text, len(attachments)); err != nil {
				return struct{}{}, err
			}
			run, err := call(ctx, a, func() (agentCall, error) {
				return a.steer(in.Text, attachments)
			})
			if err != nil {
				return struct{}{}, err
			}
			return struct{}{}, run(ctx)
		})
		return PromptResult{}, err
	case ModeRefuse:
		dispatch, err := once(ctx, &a.recent, in.ClientRequestID, "prompt-refuse", func(ctx context.Context) (promptDispatch, error) {
			attachments, err := resolveAttachments(s.uploadCheck(id), in.Attachments)
			if err != nil {
				return promptDispatch{}, err
			}
			if err := validText(in.Text, len(attachments)+len(in.Images)); err != nil {
				return promptDispatch{}, err
			}
			var acknowledgement *promptAcknowledgement
			result, err := call(ctx, a, func() (PromptResult, error) {
				if in.ExpectedIdentity != "" {
					rec, ok := s.records.get(id)
					if !ok || NativeIdentity(rec) != in.ExpectedIdentity {
						return PromptResult{}, ErrConflict
					}
				}

				if in.ExpectedUsage != nil && (a.chat.Usage == nil || a.chat.Usage.Used != in.ExpectedUsage.Used || a.chat.Usage.Size != in.ExpectedUsage.Size) {
					return PromptResult{}, ErrConflict
				}
				result, err := a.promptRefuse(in.Text, in.TitleSource, attachments, in.Images)
				if err == nil && a.turn != nil {
					acknowledgement = a.turn.acknowledgement
				}
				return result, err
			})
			return promptDispatch{result: result, acknowledgement: acknowledgement}, err
		})
		if err != nil || !in.AwaitAcknowledgement || dispatch.acknowledgement == nil {
			return dispatch.result, err
		}
		// Receipt observation follows the turn, not the 30-second dispatch
		// deadline. Cancelling observation never submits a second prompt.
		select {
		case <-dispatch.acknowledgement.done:
			return dispatch.result, dispatch.acknowledgement.err
		case <-ctx.Done():
			return dispatch.result, ctx.Err()
		}
	case ModeNow:
		return once(ctx, &a.recent, in.ClientRequestID, "prompt-now", func(ctx context.Context) (PromptResult, error) {
			attachments, err := resolveAttachments(s.uploadCheck(id), in.Attachments)
			if err != nil {
				return PromptResult{}, err
			}
			if err := validText(in.Text, len(attachments)); err != nil {
				return PromptResult{}, err
			}
			return call(ctx, a, func() (PromptResult, error) {
				return a.promptNow(in.Text, in.TitleSource, attachments)
			})
		})
	}
	return PromptResult{}, fmt.Errorf("%w: mode 只能是 auto、steer、refuse 或 now", ErrInvalid)
}

// EditQueued edits queued prompt itemID; ErrSent once it was sent or
// removed.
func (s *Service) EditQueued(ctx context.Context, id, itemID string, in QueueEdit) error {
	if in.Text != nil {
		if err := validTextLength(*in.Text); err != nil {
			return err
		}
	}
	if in.Move != "" && in.Move != MoveUp {
		return fmt.Errorf("%w: move 只能是 up", ErrInvalid)
	}
	if in.Text == nil && in.Move == "" {
		return fmt.Errorf("%w: 需要 text 或 move", ErrInvalid)
	}
	_, err := command(ctx, s, id, in.ClientRequestID, "queue.edit", func(a *actor) (struct{}, error) {
		return struct{}{}, a.editQueued(itemID, in.Text, in.Move)
	})
	return err
}

// RemoveQueued deletes queued prompt itemID; ErrSent once it was sent or
// removed.
func (s *Service) RemoveQueued(ctx context.Context, id, requestID, itemID string) error {
	_, err := command(ctx, s, id, requestID, "queue.remove", func(a *actor) (struct{}, error) { return struct{}{}, a.removeQueued(itemID) })
	return err
}

// SendQueuedNow sends queued prompt itemID next: the running turn is
// cancelled first (the client confirmed that).
func (s *Service) SendQueuedNow(ctx context.Context, id, requestID, itemID string) error {
	_, err := command(ctx, s, id, requestID, "queue.send-now", func(a *actor) (struct{}, error) { return struct{}{}, a.sendQueuedNow(itemID) })
	return err
}

// ResumeQueue unpauses the queue, sending from its head.
func (s *Service) ResumeQueue(ctx context.Context, id, requestID string) error {
	_, err := command(ctx, s, id, requestID, "queue.resume", func(a *actor) (struct{}, error) { return struct{}{}, a.resumeQueue() })
	return err
}

// ClearQueue removes every queued prompt.
func (s *Service) ClearQueue(ctx context.Context, id, requestID string) error {
	_, err := command(ctx, s, id, requestID, "queue.clear", func(a *actor) (struct{}, error) { return struct{}{}, a.clearQueue() })
	return err
}

// Cancel stops the running turn: pending requests are answered cancelled
// and the agent gets session/cancel; a non-empty queue pauses. Without a
// running turn it does nothing.
func (s *Service) Cancel(ctx context.Context, id, requestID string) error {
	_, err := command(ctx, s, id, requestID, "cancel", func(a *actor) (struct{}, error) { return struct{}{}, a.cancel() })
	return err
}

// Answer resolves pending request requestID. The first answer wins; later
// ones fail with ErrAnswered.
func (s *Service) Answer(ctx context.Context, id, clientRequestID, requestID string, answer Answer) error {
	_, err := command(ctx, s, id, clientRequestID, "answer", func(a *actor) (struct{}, error) { return struct{}{}, a.answer(requestID, answer) })
	return err
}

// SetConfig changes an agent option through session/set_config_option;
// every device sees the agent's new options, and the record keeps the
// value for later loads. A refusal is ErrAgentCall.
func (s *Service) SetConfig(ctx context.Context, id string, in ConfigInput) error {
	return agentCommand(ctx, s, id, in.ClientRequestID, "config", func(a *actor) (agentCall, error) {
		return a.setConfig(in.ConfigID, in.Value, in.Enabled)
	})
}

// SetPermissionMode sets session id's permission mode, PermissionAsk or
// PermissionAuto; every device sees it and the record keeps it. Unlike the
// other chat commands it is accepted on the terminal side too, for the
// chat side's next run; switching to PermissionAuto approves the requests
// already waiting. Form elicitations always wait for the user.
func (s *Service) SetPermissionMode(ctx context.Context, id string, in PermissionModeInput) error {
	if in.Mode != PermissionAsk && in.Mode != PermissionAuto {
		return fmt.Errorf("%w: mode 只能是 ask 或 auto", ErrInvalid)
	}
	_, err := command(ctx, s, id, in.ClientRequestID, "permission-mode", func(a *actor) (struct{}, error) {
		return struct{}{}, a.setPermissionMode(in.Mode)
	})
	return err
}

// SetName records the user-chosen name of session id (empty for none), so a
// recovered session keeps it. A session without a record answers ErrNotFound.
func (s *Service) SetName(id, name string) error {
	_, err := s.records.update(id, true, func(r *Record) { r.Name = name })
	return err
}

// ChatSettings are the agent options and permission mode a chat runs with.
type ChatSettings struct {
	Config         []ConfigOption
	PermissionMode PermissionMode
}

// Settings returns session id's current agent options and permission mode,
// as every device sees them.
func (s *Service) Settings(ctx context.Context, id string) (ChatSettings, error) {
	a, err := s.actor(id)
	if err != nil {
		return ChatSettings{}, err
	}
	return call(ctx, a, func() (ChatSettings, error) {
		return ChatSettings{Config: a.chat.Config, PermissionMode: a.chat.PermissionMode}, nil
	})
}

// Notice adds a WebMux warning to session id's chat timeline, like the
// notices of options that could not be restored.
func (s *Service) Notice(ctx context.Context, id, text string) error {
	a, err := s.actor(id)
	if err != nil {
		return err
	}
	_, err = call(ctx, a, func() (struct{}, error) {
		a.emit(Event{Kind: EventNotice, Notice: &Notice{Level: acp.NoticeWarning, Text: text}})
		return struct{}{}, nil
	})
	return err
}
