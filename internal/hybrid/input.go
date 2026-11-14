package hybrid

import (
	"context"
	"fmt"
	"time"

	"github.com/rainchan/webmux/internal/session"
)

// view is the side hybrid session id settled on, for a command that acts
// on it: ErrNotActive once it is closed or interrupted, ErrSwitching while
// it moves.
func (s *Service) view(ctx context.Context, id string) (*actor, Side, error) {
	a, err := s.actor(id)
	if err != nil {
		return nil, "", err
	}
	side, err := call(ctx, a, func() (Side, error) {
		switch {
		case a.state != StateActive || a.closing:
			return "", ErrNotActive
		case a.side == SideSwitching:
			return "", ErrSwitching
		}
		return a.side, nil
	})
	return a, side, err
}

// Input sends in to hybrid session id on its current side.
func (s *Service) Input(ctx context.Context, id string, in session.InputRequest) (session.InputResult, error) {
	a, side, err := s.view(ctx, id)
	if err != nil {
		return session.InputResult{}, err
	}
	if side == SideTUI {
		return a.shell.Input(ctx, in)
	}
	if len(in.Keys) > 0 {
		return session.InputResult{}, fmt.Errorf("%w: 按键只适用于终端视图", ErrWrongSide)
	}
	attachments := make([]Attachment, len(in.Attachments))
	for i, at := range in.Attachments {
		attachments[i] = Attachment{Path: at.Path, Name: at.Name, MimeType: at.MimeType}
	}
	result, err := s.Prompt(ctx, id, PromptInput{ClientRequestID: in.ClientRequestID, Text: in.Text, Mode: in.Mode, Attachments: attachments})
	return session.InputResult{Side: string(SideACP), Prompt: session.InputPromptResult(result)}, err
}

// Interrupt stops what hybrid session id is doing on its current side: on
// the chat it cancels the running turn (nothing without one), on the
// terminal it writes the profile's interrupt key presses.
func (s *Service) Interrupt(ctx context.Context, id, requestID, label string) error {
	a, side, err := s.view(ctx, id)
	if err != nil {
		return err
	}
	if side == SideACP {
		return s.Cancel(ctx, id, requestID)
	}
	keys := a.cmd.Agent.InterruptKeys()
	for i, key := range keys {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interruptKeyGap):
			}
		}
		// Each press is its own receipt; a retry of the request finds every
		// press recorded and writes none of them again.
		in := session.ServerInput{RequestID: fmt.Sprintf("%s-%d", requestID, i), Label: label, Keys: [][]byte{key}}
		if err := session.WriteTerminal(ctx, a.shell, in); err != nil {
			return err
		}
	}
	return nil
}

// HasChatView reports whether session id has a chat view: a running hybrid
// session or an archive record.
func (s *Service) HasChatView(id string) bool {
	if _, err := s.actor(id); err == nil {
		return true
	}
	_, err := s.Record(id)
	return err == nil
}
