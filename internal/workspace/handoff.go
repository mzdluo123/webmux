package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

// PlanHandoff saves an exact worktree intention before creating resources.
// Session creation and saved-runtime recovery belong to the chat coordinator.
func (s *Service) PlanHandoff(ctx context.Context, in sessioncreate.Input, reservation string) (out sessioncreate.HandoffPlan, err error) {
	defer func() { err = handoffLocationError(err) }()
	if in.ConfigID == nil || s.launches == nil || s.hybrids == nil {
		return out, sessioncreate.ErrUnavailable
	}
	config, err := s.launches.Get(*in.ConfigID)
	if err != nil {
		return out, err
	}
	if in.NewWorktree == nil {
		location, err := s.Prepare(ctx, in)
		if err != nil {
			return out, err
		}
		out.Cwd = location.Cwd
		if out.Cwd == "" {
			out.Cwd = s.defaultCwd
		}
		if in.Workspace != nil {
			out.RepositoryID = in.Workspace.RepositoryID
			out.WorktreeID = in.Workspace.WorktreeID
		}
		if err := s.hybrids.CheckCreate(ctx, hybrid.CreateInput{Cwd: out.Cwd, ConfigID: config.ID, Cmd: config.Cmd, Env: config.Env, StartSide: hybrid.SideACP}); err != nil {
			return out, err
		}
		return out, nil
	}
	out.RepositoryID = in.NewWorktree.RepositoryID
	r, err := s.repository(out.RepositoryID)
	if err != nil {
		return out, err
	}
	snapshot, err := s.snapshot(ctx, r)
	if err != nil {
		return out, err
	}
	if snapshot.Availability != "available" {
		return out, problemf(ErrUnavailable, "%s", snapshot.Error)
	}
	var plan createPlan
	if err := s.planWorktree(ctx, r, in.NewWorktree, &plan); err != nil {
		return out, err
	}
	if err := s.hybrids.CheckCreate(ctx, hybrid.CreateInput{Cwd: r.entry.AnchorPath, ConfigID: config.ID, Cmd: config.Cmd, Env: config.Env, StartSide: hybrid.SideACP}); err != nil {
		return out, err
	}
	out.Path = plan.path + "-" + reservation
	out.Branch, out.Base = plan.branch, plan.base
	oid, err := s.git(ctx, r.entry.AnchorPath, "rev-parse", "--verify", "--end-of-options", out.Base+"^{commit}")
	if err != nil {
		return out, err
	}
	out.Base = strings.TrimSpace(string(oid))
	if _, err := os.Lstat(out.Path); !errors.Is(err, os.ErrNotExist) {
		return out, ErrConflict
	}
	return out, nil
}

func (s *Service) EnsureHandoffWorktree(ctx context.Context, p sessioncreate.HandoffPlan) (out sessioncreate.Location, err error) {
	defer func() { err = handoffLocationError(err) }()
	r, err := s.repository(p.RepositoryID)
	if err != nil {
		return out, err
	}
	snapshot, err := s.snapshot(ctx, r)
	if err != nil {
		return out, err
	}
	for _, tree := range snapshot.Worktrees {
		if filepath.Clean(tree.Path) != filepath.Clean(p.Path) {
			continue
		}
		if !tree.Available || tree.Branch == nil || *tree.Branch != p.Branch {
			return out, ErrConflict
		}
		return handoffLocation(tree), nil
	}
	tree, err := s.CreateWorktree(ctx, p.RepositoryID, CreateRequest{Path: p.Path, Mode: "new", Branch: p.Branch, Base: p.Base})
	if err != nil {
		return out, err
	}
	return handoffLocation(tree), nil
}

func handoffLocation(tree Worktree) sessioncreate.Location {
	return sessioncreate.Location{RepositoryID: tree.RepositoryID, WorktreeID: tree.ID, Path: tree.Path, Detail: tree}
}

func handoffLocationError(err error) error {
	if errors.Is(err, ErrConflict) {
		return errors.Join(sessioncreate.ErrLocationConflict, err)
	}
	return err
}

// HandoffDirectory resolves an existing target without creating resources. A
// source subdirectory is retained only when the caller selected that workspace.
func (s *Service) HandoffDirectory(ctx context.Context, reference sessioncreate.Reference, cwd string) (string, error) {
	r, err := s.repository(reference.RepositoryID)
	if err != nil {
		return "", err
	}
	snapshot, err := s.snapshot(ctx, r)
	if err != nil {
		return "", err
	}
	if snapshot.Availability != "available" {
		return "", problemf(ErrUnavailable, "%s", snapshot.Error)
	}
	tree, err := findTree(snapshot, reference.WorktreeID)
	if err != nil {
		return "", err
	}
	if !tree.Available || tree.Kind == "bare" {
		return "", ErrConflict
	}
	if cwd == "" {
		return tree.Path, nil
	}
	path, err := canonical(cwd)
	if err != nil {
		return "", ErrInvalid
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() || !within(tree.Path, path) {
		return "", ErrInvalid
	}
	return path, nil
}
