package launch

import (
	"context"
	"time"

	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

// Resolve supplies a checked saved command to the terminal creation service.
func (s *Service) Resolve(_ context.Context, id string) (sessioncreate.Command, error) {
	saved, err := s.Get(id)
	if err != nil {
		return sessioncreate.Command{}, err
	}
	out := sessioncreate.Command{Cmd: saved.Cmd, Env: saved.Env, ConfigID: saved.ID, Chat: saved.Hybrid}
	if a := saved.Attention; a != nil {
		out.Attention = &session.AttentionPolicy{Mode: session.AttentionMode(a.Mode), ProgramStatus: a.ProgramStatus, AgentProtocol: a.AgentProtocol, Notify: a.Notify, Bell: a.Bell, MinTurn: time.Duration(a.MinTurnSeconds) * time.Second, DoneDelay: time.Duration(a.DoneDelaySeconds) * time.Second}
	}
	return out, nil
}
