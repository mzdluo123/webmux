package featureapi

import (
	"context"
	"errors"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/featurewire"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/workspace"
)

// detailsTimeout bounds optional Git inspection, outside all process locks.
const detailsTimeout = 5 * time.Second

// EnrichDetails is prebound by cmd; the core does not construct a business
// metadata service when no feature is enabled.
func (s *Server) EnrichDetails(ctx context.Context, info session.Info, out *contract.SessionDetails) {
	out.Session = s.sessionSnapshot(info)
	if s.Workspace != nil {
		if association := s.Workspace.Association(info.ID); association != nil {
			lookup, cancel := context.WithTimeout(ctx, detailsTimeout)
			s.detailsWorkspace(lookup, association, out)
			cancel()
		}
	}
	if info.ConfigID != "" && s.Launch != nil {
		config, err := s.Launch.Get(info.ConfigID)
		switch {
		case err == nil:
			out.LaunchConfig = &contract.DetailsLaunchConfig{ID: config.ID, Name: config.Name, Icon: config.Icon, Chat: config.Hybrid}
		case errors.Is(err, launch.ErrNotFound):
			out.LaunchConfigMissing = true
		}
	}
	if out.Session.State != contract.SessionStateActive {
		if described, err := s.sessionInfo(ctx, info.ID); err == nil {
			out.Session = described
		}
	}
	if s.Artifacts != nil {
		if dir, err := s.Artifacts.Existing(info.ID); err == nil {
			out.ArtifactsDir = &dir
		}
	}
	if s.Forward != nil {
		for _, f := range s.Forward.List() {
			if f.Session == info.ID {
				out.Forwards = append(out.Forwards, wire.Forward(f))
			}
		}
	}
}

func (s *Server) detailsWorkspace(ctx context.Context, ref *workspace.Association, out *contract.SessionDetails) {
	repository, tree, err := s.Workspace.WorktreeInfo(ctx, ref.RepositoryID, ref.WorktreeID)
	if err != nil {
		return
	}
	out.Repository = &contract.DetailsRepository{ID: repository.ID, Name: repository.Name, Path: repository.AnchorPath}
	if tree != nil {
		out.Worktree = &contract.DetailsWorktree{ID: tree.ID, Branch: tree.Branch, Path: tree.Path, Main: tree.Kind == "main"}
	}
}
