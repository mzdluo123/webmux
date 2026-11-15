// Package hybridresume coordinates saved chat recovery without requiring Git.
package hybridresume

import (
	"context"
	"errors"
	"slices"

	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

// Input restores the recorded side unless Side selects tui or acp.
// ConfirmWithoutEnv explicitly accepts a deleted launch configuration.
type Input struct {
	ContextExecutionID  string
	RequireNativeResume bool
	ClientRequestID     string
	Side                string
	AgentSessionID      string
	ConfirmWithoutEnv   bool
}

// Locations protects startup against concurrent worktree removal. It is absent
// when the workspace plugin is disabled; ordinary directory recovery still works.
type Locations interface {
	PrepareResume(context.Context, string) (func(*session.Session), func(), error)
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
	var commit func(*session.Session)
	if s.options.ResumeLocations != nil {
		var release func()
		commit, release, err = s.options.ResumeLocations.PrepareResume(ctx, rec.Cwd)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	item, err := s.hybrids.Resume(ctx, id, hybrid.ResumeInput{ContextExecutionID: in.ContextExecutionID, RequireNativeResume: in.RequireNativeResume, ClientRequestID: in.ClientRequestID, Side: hybrid.Side(in.Side), AgentSessionID: in.AgentSessionID, Env: env, Attention: attention})
	if err != nil {
		return nil, err
	}
	if commit != nil {
		commit(item)
	}
	return item, nil
}

// EnsureHandoffSession uses the durable reservation for fresh creation and
// archived recovery alike. It never substitutes a second runtime shell.
func (s *Service) EnsureHandoffSession(ctx context.Context, in sessioncreate.Input, identity sessioncreate.HandoffIdentity) (*session.Session, error) {
	if s.hybrids == nil || s.launches == nil || s.options.Creation == nil {
		return nil, sessioncreate.ErrUnavailable
	}
	if (in.Workspace != nil || in.NewWorktree != nil) && s.options.Locations == nil {
		return nil, sessioncreate.ErrUnavailable
	}
	if rec, err := s.hybrids.Record(identity.RuntimeID); err == nil {
		if in.ConfigID == nil || rec.ConfigID != *in.ConfigID {
			return nil, sessioncreate.ErrLocationConflict
		}
		current, err := s.launches.Get(*in.ConfigID)
		if err != nil {
			return nil, err
		}
		if !slices.Equal(rec.Cmd, current.Cmd) {
			return nil, hybrid.ErrProviderConfigChanged
		}
		if rec.State == hybrid.StateActive {
			item, err := s.hybrids.LiveSession(identity.RuntimeID)
			if err != nil {
				return nil, sessioncreate.ErrLocationConflict
			}
			return item, nil
		}
		return s.Resume(ctx, identity.RuntimeID, Input{ClientRequestID: "handoff-resume-" + identity.ExecutionID, Side: string(hybrid.SideACP), ContextExecutionID: identity.ExecutionID, RequireNativeResume: identity.RequireNativeResume})
	} else if !errors.Is(err, hybrid.ErrNotFound) {
		return nil, err
	}
	if in.ConfigID == nil {
		return nil, hybrid.ErrInvalid
	}
	command, err := s.launches.Resolve(ctx, *in.ConfigID)
	if err != nil {
		return nil, err
	}
	cwd := in.Cwd
	if cwd == "" {
		cwd = s.options.DefaultCwd
	}
	var location sessioncreate.LocationPlan
	if s.options.Locations != nil {
		location, err = s.options.Locations.Prepare(ctx, in)
		if err != nil {
			return nil, err
		}
		if location.Cwd != "" {
			cwd = location.Cwd
		}
		if location.Create != nil {
			return nil, hybrid.ErrInvalid
		}
	}
	cwd, err = sessioncreate.Directory(cwd)
	if err != nil {
		return nil, err
	}
	if location.Acquire != nil {
		var release func()
		cwd, release, err = location.Acquire(ctx, cwd)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	if err := s.options.Creation.Check(ctx, in, command, cwd); err != nil {
		return nil, err
	}
	item, err := s.hybrids.Create(ctx, hybrid.CreateInput{RuntimeID: identity.RuntimeID, ProviderThreadID: identity.ProviderThreadID, ContextExecutionID: identity.ExecutionID, Cwd: cwd, Name: in.Name, ConfigID: command.ConfigID, Cmd: command.Cmd, Env: command.Env, StartSide: hybrid.SideACP, Cols: in.Cols, Rows: in.Rows, Attention: command.Attention})
	if err != nil {
		return nil, err
	}
	if location.Associate != nil {
		location.Associate(item)
	}
	return item, nil
}
