package wire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/workspace"
)

func gitConflictFileSummary(s workspace.ConflictFileSummary) contract.GitConflictFileSummary {
	return contract.GitConflictFileSummary(s)
}

func GitInProgressOperation(p workspace.InProgressOperation) contract.GitInProgressOperation {
	return contract.GitInProgressOperation{
		Active:           p.Active,
		Kind:             p.Kind,
		WorktreeID:       p.WorktreeID,
		WorktreePath:     p.WorktreePath,
		CurrentBranch:    p.CurrentBranch,
		Detached:         p.Detached,
		HeadOID:          p.HeadOID,
		HeadSubject:      p.HeadSubject,
		OrigHeadOID:      p.OrigHeadOID,
		OrigHeadSubject:  p.OrigHeadSubject,
		SourceLabel:      p.SourceLabel,
		SourceOID:        p.SourceOID,
		SourceSubject:    p.SourceSubject,
		TargetLabel:      p.TargetLabel,
		TargetOID:        p.TargetOID,
		TargetSubject:    p.TargetSubject,
		StepCurrent:      p.StepCurrent,
		StepTotal:        p.StepTotal,
		StoppedCommitOID: p.StoppedCommitOID,
		StoppedSubject:   p.StoppedSubject,
		ConflictedCount:  p.ConflictedCount,
		ConflictedFiles:  each(p.ConflictedFiles, gitConflictFileSummary),
		CanContinue:      p.CanContinue,
		CanAbort:         p.CanAbort,
		ContinueBlockers: Issues(p.ContinueBlockers),
	}
}

func GitDivergencePreview(p workspace.DivergencePreview) contract.GitDivergencePreview {
	return contract.GitDivergencePreview{
		ConfirmationID:   p.ConfirmationID,
		ExpiresAt:        p.ExpiresAt,
		Kind:             p.Kind,
		RepositoryID:     p.RepositoryID,
		WorktreeID:       p.WorktreeID,
		WorktreePath:     p.WorktreePath,
		DirectionSummary: p.DirectionSummary,
		RecoveryHint:     p.RecoveryHint,
		CurrentBranch:    p.CurrentBranch,
		HeadOID:          p.HeadOID,
		HeadSubject:      p.HeadSubject,
		TargetRef:        p.TargetRef,
		TargetKind:       p.TargetKind,
		TargetOID:        p.TargetOID,
		TargetSubject:    p.TargetSubject,
		MergeBaseOID:     p.MergeBaseOID,
		FastForward:      p.FastForward,
		AlreadyUpToDate:  p.AlreadyUpToDate,
		Commits:          CommitList(p.Commits),
		Sessions:         Sessions(p.Sessions),
		Blockers:         Issues(p.Blockers),
		Warnings:         Issues(p.Warnings),
	}
}

func GitAbortOperationPreview(p workspace.AbortOperationPreview) contract.GitAbortOperationPreview {
	return contract.GitAbortOperationPreview{
		ConfirmationID:   p.ConfirmationID,
		ExpiresAt:        p.ExpiresAt,
		RepositoryID:     p.RepositoryID,
		WorktreeID:       p.WorktreeID,
		WorktreePath:     p.WorktreePath,
		Kind:             p.Kind,
		CurrentBranch:    p.CurrentBranch,
		HeadOID:          p.HeadOID,
		RestoreOID:       p.RestoreOID,
		RestoreSubject:   p.RestoreSubject,
		DirectionSummary: p.DirectionSummary,
		AffectedPaths:    required(p.AffectedPaths),
		Sessions:         Sessions(p.Sessions),
		Blockers:         Issues(p.Blockers),
		Warnings:         Issues(p.Warnings),
	}
}

func GitDivergenceResult(r workspace.DivergenceResult) contract.GitDivergenceResult {
	return contract.GitDivergenceResult{
		OperationID: r.OperationID,
		Kind:        r.Kind,
		Outcome:     r.Outcome,
		Summary:     r.Summary,
		HeadOID:     r.HeadOID,
		Subject:     r.Subject,
		RecoveryRef: r.RecoveryRef,
		InProgress:  GitInProgressOperation(r.InProgress),
		Worktree:    Worktree(r.Worktree),
	}
}

func gitConflictSide(s workspace.ConflictSide) contract.GitConflictSide {
	return contract.GitConflictSide(s)
}

func gitConflictSegment(s workspace.ConflictSegment) contract.GitConflictSegment {
	return contract.GitConflictSegment{
		ID:         s.ID,
		Kind:       s.Kind,
		BaseStart:  s.BaseStart,
		BaseLines:  required(s.BaseLines),
		LeftStart:  s.LeftStart,
		LeftLines:  required(s.LeftLines),
		RightStart: s.RightStart,
		RightLines: required(s.RightLines),
	}
}

func gitConflictIdentity(id workspace.ConflictSnapshotIdentity) contract.GitConflictSnapshotIdentity {
	return contract.GitConflictSnapshotIdentity(id)
}

func GitConflictFileDetail(d workspace.ConflictFileDetail) contract.GitConflictFileDetail {
	return contract.GitConflictFileDetail{
		RepositoryID:          d.RepositoryID,
		WorktreeID:            d.WorktreeID,
		WorktreePath:          d.WorktreePath,
		Path:                  d.Path,
		XY:                    d.XY,
		Kind:                  d.Kind,
		TextMergeable:         d.TextMergeable,
		DegradeReason:         d.DegradeReason,
		OperationKind:         d.OperationKind,
		Identity:              gitConflictIdentity(d.Identity),
		Base:                  gitConflictSide(d.Base),
		Left:                  gitConflictSide(d.Left),
		Right:                 gitConflictSide(d.Right),
		Segments:              each(d.Segments, gitConflictSegment),
		InitialResultText:     d.InitialResultText,
		DiskText:              d.DiskText,
		DiskHasConflictMarker: d.DiskHasConflictMarker,
		DiskModifiedFromBase:  d.DiskModifiedFromBase,
	}
}

func GitSaveConflictRequest(repositoryID, worktreeID string, req contract.GitSaveConflictRequest) workspace.SaveConflictResultRequest {
	return workspace.SaveConflictResultRequest{
		RepositoryID: repositoryID,
		WorktreeID:   worktreeID,
		Identity:     workspace.ConflictSnapshotIdentity(req.Identity),
		ResultText:   req.ResultText,
	}
}

func GitSaveConflictResponse(r workspace.SaveConflictResultResponse) contract.GitSaveConflictResponse {
	return contract.GitSaveConflictResponse{
		Path:     r.Path,
		Identity: gitConflictIdentity(r.Identity),
	}
}

func GitResolveConflictRequest(repositoryID, worktreeID string, req contract.GitResolveConflictRequest) workspace.ResolveConflictRequest {
	return workspace.ResolveConflictRequest{
		RepositoryID: repositoryID,
		WorktreeID:   worktreeID,
		Identity:     workspace.ConflictSnapshotIdentity(req.Identity),
		Resolution:   req.Resolution,
		ResultText:   req.ResultText,
	}
}

func GitResolveConflictResponse(r workspace.ResolveConflictResponse) contract.GitResolveConflictResponse {
	return contract.GitResolveConflictResponse{
		Path:       r.Path,
		InProgress: GitInProgressOperation(r.InProgress),
	}
}
