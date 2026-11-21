// Package hybridresume coordinates saved chat recovery without requiring Git.
package hybridresume

import (
	"context"
	"errors"

	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

// Input restores the recorded side unless Side selects tui or acp.
// ConfirmWithoutEnv explicitly accepts a deleted launch configuration.
type Input struct {
	ClientRequestID   string
	Side              string
	AgentSessionID    string
	ConfirmWithoutEnv bool
}

// Locations protects startup against concurrent worktree removal. It is absent
// when the workspace feature is disabled; ordinary directory recovery still works.
type Locations interface {
	PrepareResume(context.Context, string) (func(*session.Session) func(), func(*session.Session), func(), error)
}

type Options struct {
	DefaultCwd      string
	Creation        sessioncreate.Chat
	Locations       sessioncreate.Locations
	ResumeLocations Locations
}

type Service struct {
	hybrids  *hybrid.Service
	launches *launch.Service
	options  Options
}

func New(hybrids *hybrid.Service, launches *launch.Service, options Options) *Service {
	return &Service{hybrids: hybrids, launches: launches, options: options}
}

// Resume retains the record's command snapshot, reads the current launch
// environment, and never replaces a released or unrecoverable record.
func (s *Service) Resume(ctx context.Context, id string, in Input) (*session.Session, error) {
	if s.hybrids == nil || s.launches == nil {
		return nil, sessioncreate.ErrUnavailable
	}
	rec, err := s.hybrids.Recoverable(id)
	if err != nil {
		return nil, err
	}
	var env map[string]string
	var attention *session.AttentionPolicy
	saved, err := s.launches.Resolve(ctx, rec.ConfigID)
	switch {
	case err == nil:
		env = saved.Env
		attention = saved.Attention
	case !errors.Is(err, launch.ErrNotFound):
		return nil, err
	case !in.ConfirmWithoutEnv:
		return nil, hybrid.ErrConfigMissing
	}
	var stage func(*session.Session) func()
	var commit func(*session.Session)
	if s.options.ResumeLocations != nil {
		var release func()
		stage, commit, release, err = s.options.ResumeLocations.PrepareResume(ctx, rec.Cwd)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	item, err := s.hybrids.Resume(ctx, id, hybrid.ResumeInput{ClientRequestID: in.ClientRequestID, Side: hybrid.Side(in.Side), AgentSessionID: in.AgentSessionID, Env: env, Attention: attention, Prepare: stage})
	if err != nil {
		return nil, err
	}
	if commit != nil {
		commit(item)
	}
	return item, nil
}
