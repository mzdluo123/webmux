package pluginwire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/workspace"
)

func gitStashEntry(e workspace.StashEntry) contract.GitStashEntry {
	return contract.GitStashEntry{
		StashRef:     e.StashRef,
		Index:        e.Index,
		OID:          e.OID,
		ShortOID:     e.ShortOID,
		BaseOID:      e.BaseOID,
		IndexOID:     e.IndexOID,
		UntrackedOID: e.UntrackedOID,
		Parents:      required(e.Parents),
		HasUntracked: e.HasUntracked,
		Branch:       e.Branch,
		Summary:      e.Summary,
		CreatedAt:    e.CreatedAt,
	}
}

func GitStashList(in []workspace.StashEntry) contract.GitStashList {
	return contract.GitStashList{
		Stashes: each(in, gitStashEntry),
	}
}

func gitStashFileChange(f workspace.StashFileChange) contract.GitStashFileChange {
	return contract.GitStashFileChange(f)
}

func GitStashDetail(d workspace.StashDetail) contract.GitStashDetail {
	return contract.GitStashDetail{
		Entry:          gitStashEntry(d.Entry),
		AuthorName:     d.AuthorName,
		AuthorEmail:    d.AuthorEmail,
		TrackedCount:   d.TrackedCount,
		UntrackedCount: d.UntrackedCount,
		Files:          each(d.Files, gitStashFileChange),
	}
}

func GitStashCreatePreview(p workspace.StashCreatePreview, project func(session.Info) contract.SessionInfo) contract.GitStashCreatePreview {
	return contract.GitStashCreatePreview{
		ConfirmationID:   p.ConfirmationID,
		ExpiresAt:        p.ExpiresAt,
		RepositoryID:     p.RepositoryID,
		WorktreeID:       p.WorktreeID,
		WorktreePath:     p.WorktreePath,
		CurrentBranch:    p.CurrentBranch,
		Detached:         p.Detached,
		HeadOID:          p.HeadOID,
		HeadSubject:      p.HeadSubject,
		Message:          p.Message,
		IncludeUntracked: p.IncludeUntracked,
		KeepIndex:        p.KeepIndex,
		StagedCount:      p.StagedCount,
		UnstagedCount:    p.UnstagedCount,
		UntrackedCount:   p.UntrackedCount,
		AffectedPaths:    required(p.AffectedPaths),
		Sessions:         projectedSessions(p.Sessions, project),
		Blockers:         Issues(p.Blockers),
		Warnings:         Issues(p.Warnings),
	}
}

func GitStashCreateResult(r workspace.StashCreateResult) contract.GitStashCreateResult {
	return contract.GitStashCreateResult{
		OperationID: r.OperationID,
		Entry:       gitStashEntry(r.Entry),
		Worktree:    Worktree(r.Worktree),
	}
}

func GitStashApplyPreview(p workspace.StashApplyPreview, project func(session.Info) contract.SessionInfo) contract.GitStashApplyPreview {
	return contract.GitStashApplyPreview{
		ConfirmationID:   p.ConfirmationID,
		ExpiresAt:        p.ExpiresAt,
		RepositoryID:     p.RepositoryID,
		WorktreeID:       p.WorktreeID,
		WorktreePath:     p.WorktreePath,
		Mode:             p.Mode,
		Entry:            gitStashEntry(p.Entry),
		ReinstateIndex:   p.ReinstateIndex,
		CurrentBranch:    p.CurrentBranch,
		Detached:         p.Detached,
		HeadOID:          p.HeadOID,
		HeadSubject:      p.HeadSubject,
		StagedCount:      p.StagedCount,
		UnstagedCount:    p.UnstagedCount,
		UntrackedCount:   p.UntrackedCount,
		OverlappingPaths: required(p.OverlappingPaths),
		Sessions:         projectedSessions(p.Sessions, project),
		Blockers:         Issues(p.Blockers),
		Warnings:         Issues(p.Warnings),
	}
}

func GitStashApplyResult(r workspace.StashApplyResult) contract.GitStashApplyResult {
	return contract.GitStashApplyResult{
		OperationID:    r.OperationID,
		Kind:           r.Kind,
		Outcome:        r.Outcome,
		StashRef:       r.StashRef,
		StashOID:       r.StashOID,
		StashDropped:   r.StashDropped,
		StashPreserved: r.StashPreserved,
		Summary:        r.Summary,
		InProgress:     GitInProgressOperation(r.InProgress),
		Worktree:       Worktree(r.Worktree),
	}
}

func GitStashDropPreview(p workspace.StashDropPreview) contract.GitStashDropPreview {
	return contract.GitStashDropPreview{
		ConfirmationID:  p.ConfirmationID,
		ExpiresAt:       p.ExpiresAt,
		RepositoryID:    p.RepositoryID,
		Entry:           gitStashEntry(p.Entry),
		FileCount:       p.FileCount,
		RecoveryCommand: p.RecoveryCommand,
		Blockers:        Issues(p.Blockers),
		Warnings:        Issues(p.Warnings),
	}
}

func GitStashDropResult(r workspace.StashDropResult) contract.GitStashDropResult {
	return contract.GitStashDropResult{
		OperationID:     r.OperationID,
		StashRef:        r.StashRef,
		StashOID:        r.StashOID,
		RecoveryCommand: r.RecoveryCommand,
		Stashes:         each(r.Stashes, gitStashEntry),
	}
}
