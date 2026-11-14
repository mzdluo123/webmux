package workspace

import (
	"context"

	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

// CreatedLocation marks a worktree snapshot returned by the location port.
func (Worktree) CreatedLocation() {}

// Prepare contributes only repository planning and identity protection. Generic
// process, configuration and first-input coordination belongs to sessioncreate.
func (s *Service) Prepare(ctx context.Context, in sessioncreate.Input) (out sessioncreate.LocationPlan, err error) {
	defer func() { err = handoffLocationError(err) }()
	out = sessioncreate.LocationPlan{Cwd: in.Cwd}
	var reference *Reference
	if in.Workspace != nil {
		reference = &Reference{RepositoryID: in.Workspace.RepositoryID, WorktreeID: in.Workspace.WorktreeID}
	}
	var selected *Worktree
	repositoryID := ""
	if reference != nil {
		repositoryID = reference.RepositoryID
	}
	if in.NewWorktree != nil {
		repositoryID = in.NewWorktree.RepositoryID
	}
	if repositoryID != "" {
		r, err := s.repository(repositoryID)
		if err != nil {
			return out, err
		}
		readCtx, cancel := context.WithTimeout(ctx, readTimeout)
		defer cancel()
		snapshot, err := s.snapshot(readCtx, r)
		if err != nil {
			return out, err
		}
		if snapshot.Availability != "available" {
			return out, problemf(ErrUnavailable, "%s", snapshot.Error)
		}
		if in.UseInitInstruction {
			out.Instruction, err = s.expandRepositoryInstruction(snapshot.InitInstruction)
			if err != nil {
				return out, err
			}
		}
		if reference != nil {
			tree, err := findTree(snapshot, reference.WorktreeID)
			if err != nil {
				return out, err
			}
			if !tree.Available || tree.Kind == "bare" {
				return out, problemf(ErrConflict, "所选工作树不可作为启动目录: %s", tree.Error)
			}
			selected = &tree
			if out.Cwd == "" {
				out.Cwd = tree.Path
			}
		}
		if nw := in.NewWorktree; nw != nil {
			var plan createPlan
			if err := s.planWorktree(readCtx, r, nw, &plan); err != nil {
				return out, err
			}
			// Chat preflight must use an existing repository directory, not the future
			// worktree. The created location replaces this directory after preflight.
			out.Cwd = r.entry.AnchorPath
			out.Create = func(ctx context.Context) (*sessioncreate.Location, error) {
				tree, err := s.CreateWorktree(ctx, nw.RepositoryID, CreateRequest{Path: plan.path, Mode: nw.Mode, Branch: plan.branch, Base: plan.base})
				if err != nil {
					return nil, handoffLocationError(err)
				}
				selected = &tree
				reference = &Reference{RepositoryID: nw.RepositoryID, WorktreeID: tree.ID}
				return &sessioncreate.Location{RepositoryID: tree.RepositoryID, WorktreeID: tree.ID, Path: tree.Path, Detail: tree}, nil
			}
		}
	}
	var association *Association
	out.Acquire = func(ctx context.Context, path string) (resolved string, releaseStart func(), err error) {
		defer func() { err = handoffLocationError(err) }()
		readCtx, cancel := context.WithTimeout(ctx, readTimeout)
		defer cancel()
		if selected != nil && !within(selected.Path, path) {
			return "", nil, problemf(ErrInvalid, "cwd 与所选工作树不一致")
		}
		if err := s.beginStart(path); err != nil {
			return "", nil, err
		}
		release := func() { s.endStart(path) }
		association = s.association(readCtx, path)
		if selected != nil && (association == nil || association.RepositoryID != reference.RepositoryID || association.WorktreeID != reference.WorktreeID) {
			release()
			return "", nil, problemf(ErrConflict, "所选工作树身份已变化或 cwd 属于嵌套仓库，请刷新")
		}
		if association != nil {
			s.ensureUnbornAssociationBaseline(readCtx, association.RepositoryID, association.WorktreeID, association.WorktreePath)
		}
		return path, release, nil
	}
	out.Associate = func(item *session.Session) { s.commitAssociation(item.Info(), association, false) }
	return out, nil
}
