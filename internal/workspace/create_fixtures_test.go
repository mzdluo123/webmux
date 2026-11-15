package workspace

import (
	"context"
	"errors"

	"github.com/rainchan/webmux/internal/artifacts"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

func newCreateFixture(s *Service, files *artifacts.Service, brief func(string, string) string) *sessioncreate.Service {
	options := sessioncreate.Options{DefaultCwd: s.defaultCwd, Locations: s, Brief: brief}
	if s.launches != nil {
		options.Commands = s.launches
	}
	if s.hybrids != nil {
		options.Chat = s.hybrids.Creation()
	}
	if files != nil {
		options.Files = files
	}
	return sessioncreate.New(s.manager, options)
}
func createdTree(location *sessioncreate.Location) *Worktree {
	if location == nil {
		return nil
	}
	tree := location.Detail.(Worktree)
	return &tree
}

// CreateSession is the single-step fixture convenience; every process is
// created through the real core coordinator with the enabled contributions.
func (s *Service) CreateSession(ctx context.Context, in sessioncreate.Input) (*session.Session, error) {
	result, err := newCreateFixture(s, nil, nil).Create(ctx, in)
	return fixtureSession(result, err)
}
func fixtureSession(result sessioncreate.Result, err error) (*session.Session, error) {
	var failed *sessioncreate.Error
	if errors.As(err, &failed) {
		err = failed.Err
	}
	if errors.Is(err, session.ErrInvalid) {
		err = errors.Join(ErrInvalid, err)
	}
	return result.Session, err
}
