package pluginwire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/workspace"
)

func GitDiscardPreviewRequest(repositoryID, worktreeID string, req contract.GitDiscardPreviewRequest) workspace.DiscardPreviewRequest {
	var identities []workspace.StageFileIdentity
	if len(req.Identities) > 0 {
		identities = make([]workspace.StageFileIdentity, len(req.Identities))
		for i, id := range req.Identities {
			identities[i] = gitStageIdentityRequest(id)
		}
	}
	var singleIdentity workspace.StageFileIdentity
	if req.Identity != nil {
		singleIdentity = gitStageIdentityRequest(*req.Identity)
	}
	var lineSelections []workspace.LineSelection
	if len(req.LineSelections) > 0 {
		lineSelections = make([]workspace.LineSelection, len(req.LineSelections))
		for i, sel := range req.LineSelections {
			lineSelections[i] = gitLineSelectionRequest(sel)
		}
	}
	return workspace.DiscardPreviewRequest{
		RepositoryID:   repositoryID,
		WorktreeID:     worktreeID,
		Scope:          req.Scope,
		Paths:          req.Paths,
		Identities:     identities,
		Identity:       singleIdentity,
		HunkIDs:        req.HunkIDs,
		LineSelections: lineSelections,
	}
}

func GitDiscardPreview(p workspace.DiscardPreview, project func(session.Info) contract.SessionInfo) contract.GitDiscardPreview {
	return contract.GitDiscardPreview{
		ConfirmationID:        p.ConfirmationID,
		ExpiresAt:             p.ExpiresAt,
		RepositoryID:          p.RepositoryID,
		WorktreeID:            p.WorktreeID,
		WorktreePath:          p.WorktreePath,
		Scope:                 p.Scope,
		TrackedOverwritePaths: required(p.TrackedOverwritePaths),
		UntrackedDeletePaths:  required(p.UntrackedDeletePaths),
		Items: each(p.Items, func(it workspace.DiscardPreviewItem) contract.GitDiscardPreviewItem {
			return contract.GitDiscardPreviewItem(it)
		}),
		HunkCount: p.HunkCount,
		LineCount: p.LineCount,
		Summary:   p.Summary,
		Sessions:  projectedSessions(p.Sessions, project),
		Blockers:  Issues(p.Blockers),
		Warnings:  Issues(p.Warnings),
	}
}

func GitDiscardResult(r workspace.DiscardResult) contract.GitDiscardResult {
	return contract.GitDiscardResult{
		OperationID:     r.OperationID,
		Scope:           r.Scope,
		Summary:         r.Summary,
		RecoveryOID:     r.RecoveryOID,
		RecoveryRef:     r.RecoveryRef,
		RecoveryCommand: r.RecoveryCommand,
		DiscardedPaths:  required(r.DiscardedPaths),
		Worktree:        Worktree(r.Worktree),
	}
}

func GitSmartCheckoutPreviewRequest(repositoryID, worktreeID string, req contract.GitSmartCheckoutPreviewRequest) workspace.SmartCheckoutPreviewRequest {
	return workspace.SmartCheckoutPreviewRequest{
		RepositoryID:     repositoryID,
		WorktreeID:       worktreeID,
		Target:           SwitchTarget(req.Target),
		IncludeUntracked: req.IncludeUntracked,
	}
}

func GitSmartCheckoutPreview(p workspace.SmartCheckoutPreview, project func(session.Info) contract.SessionInfo) contract.GitSmartCheckoutPreview {
	return contract.GitSmartCheckoutPreview{
		ConfirmationID:   p.ConfirmationID,
		ExpiresAt:        p.ExpiresAt,
		RepositoryID:     p.RepositoryID,
		WorktreeID:       p.WorktreeID,
		Worktree:         Worktree(p.Worktree),
		Target:           contract.SwitchTargetInfo(p.Target),
		IncludeUntracked: p.IncludeUntracked,
		StagedCount:      p.StagedCount,
		UnstagedCount:    p.UnstagedCount,
		UntrackedCount:   p.UntrackedCount,
		DirtyPaths:       required(p.DirtyPaths),
		HasActiveAgent:   p.HasActiveAgent,
		Sessions:         projectedSessions(p.Sessions, project),
		Blockers:         Issues(p.Blockers),
		Warnings:         Issues(p.Warnings),
	}
}

func GitSmartCheckoutResult(r workspace.SmartCheckoutResult) contract.GitSmartCheckoutResult {
	return contract.GitSmartCheckoutResult{
		OperationID:     r.OperationID,
		Outcome:         r.Outcome,
		Phase:           r.Phase,
		Summary:         r.Summary,
		ErrorMessage:    r.ErrorMessage,
		StashOID:        r.StashOID,
		StashRef:        r.StashRef,
		StashDropped:    r.StashDropped,
		StashPreserved:  r.StashPreserved,
		StashRestored:   r.StashRestored,
		RecoveryCommand: r.RecoveryCommand,
		InProgress:      GitInProgressOperation(r.InProgress),
		Worktree:        Worktree(r.Worktree),
	}
}
