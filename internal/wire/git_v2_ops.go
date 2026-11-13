package wire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/workspace"
)

func GitBranchRenamePreview(p workspace.BranchRenamePreview) contract.GitBranchRenamePreview {
	return contract.GitBranchRenamePreview{
		ConfirmationID:    p.ConfirmationID,
		ExpiresAt:         p.ExpiresAt,
		RepositoryID:      p.RepositoryID,
		OldName:           p.OldName,
		NewName:           p.NewName,
		HeadOID:           p.HeadOID,
		Subject:           p.Subject,
		Upstream:          p.Upstream,
		AffectedWorktrees: each(p.AffectedWorktrees, Worktree),
		Sessions:          Sessions(p.Sessions),
		Blockers:          Issues(p.Blockers),
		Warnings:          Issues(p.Warnings),
	}
}

func GitBranchMutationResult(r workspace.BranchMutationResult) contract.GitBranchMutationResult {
	return contract.GitBranchMutationResult{
		OperationID: r.OperationID,
		Branch:      r.Branch,
		Upstream:    r.Upstream,
		Repository:  Repository(r.Repository),
		Branches:    Branches(r.Branches),
	}
}

func GitTagCreateResult(r workspace.CreateTagResult) contract.GitTagCreateResult {
	return contract.GitTagCreateResult{
		OperationID: r.OperationID,
		Tag:         contract.Tag(r.Tag),
		Branches:    Branches(r.Branches),
	}
}

func GitTagDeleteResult(r workspace.DeleteTagResult) contract.GitTagDeleteResult {
	return contract.GitTagDeleteResult{
		OperationID:     r.OperationID,
		Name:            r.Name,
		TargetOID:       r.TargetOID,
		RecoveryCommand: r.RecoveryCommand,
		Branches:        Branches(r.Branches),
	}
}

func GitRemoteMutationResult(r workspace.RemoteMutationResult) contract.GitRemoteMutationResult {
	return contract.GitRemoteMutationResult{
		OperationID: r.OperationID,
		Remotes:     each(r.Remotes, func(v workspace.Remote) contract.Remote { return contract.Remote(v) }),
		Repository:  Repository(r.Repository),
	}
}

func gitFileHistoryRow(r workspace.FileHistoryRow) contract.GitFileHistoryRow {
	return contract.GitFileHistoryRow{
		OID:               r.OID,
		ShortOID:          r.ShortOID,
		Parents:           required(r.Parents),
		ComparedParentOID: r.ComparedParentOID,
		Subject:           r.Subject,
		AuthorName:        r.AuthorName,
		AuthorEmail:       r.AuthorEmail,
		AuthorAt:          r.AuthorAt,
		Status:            r.Status,
		Path:              r.Path,
		OldPath:           r.OldPath,
	}
}

func GitFileHistoryPage(p workspace.FileHistoryPage) contract.GitFileHistoryPage {
	return contract.GitFileHistoryPage{
		Status:     p.Status,
		Path:       p.Path,
		StartOID:   p.StartOID,
		Commits:    each(p.Commits, gitFileHistoryRow),
		NextCursor: p.NextCursor,
	}
}

func gitBlameCommit(c workspace.BlameCommit) contract.GitBlameCommit {
	return contract.GitBlameCommit(c)
}

func gitBlameLine(l workspace.BlameLine) contract.GitBlameLine {
	return contract.GitBlameLine(l)
}

func GitBlameResult(r workspace.BlameResult) contract.GitBlameResult {
	return contract.GitBlameResult{
		CommitOID:     r.CommitOID,
		ShortOID:      r.ShortOID,
		Path:          r.Path,
		BlobOID:       r.BlobOID,
		Mode:          r.Mode,
		Kind:          r.Kind,
		DegradeReason: r.DegradeReason,
		TotalLines:    r.TotalLines,
		Commits:       each(r.Commits, gitBlameCommit),
		Lines:         each(r.Lines, gitBlameLine),
	}
}
