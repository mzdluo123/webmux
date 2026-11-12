package wire

import (
	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/workspace"
)

func GitContext(c codeview.Context) contract.GitContext {
	return contract.GitContext(c)
}

func Branches(b workspace.Branches) contract.Branches {
	branches := each(b.Branches, func(v workspace.Branch) contract.Branch {
		return contract.Branch{
			Name: v.Name, Head: v.Head, WorktreePaths: required(v.WorktreePaths), Subject: v.Subject, CommittedAt: v.CommittedAt,
			Upstream: v.Upstream, Ahead: v.Ahead, Behind: v.Behind, UpstreamGone: v.UpstreamGone, MergedIntoDefault: v.MergedIntoDefault,
		}
	})
	bases := each(b.Bases, func(v workspace.Base) contract.BranchBase { return contract.BranchBase(v) })
	tags := each(b.Tags, func(v workspace.Tag) contract.Tag { return contract.Tag(v) })
	return contract.Branches{
		Branches:           branches,
		Bases:              bases,
		Tags:               tags,
		DefaultBase:        b.DefaultBase,
		DefaultBaseSubject: b.DefaultBaseSubject,
		DefaultBranch:      b.DefaultBranch,
		DefaultParent:      b.DefaultParent,
	}
}

func Remotes(r workspace.Remotes) contract.Remotes {
	return contract.Remotes{Remotes: each(r.Remotes, func(v workspace.Remote) contract.Remote { return contract.Remote(v) })}
}

func FetchRemotesRequest(r contract.FetchRemotesRequest) workspace.FetchRequest {
	return workspace.FetchRequest(r)
}

func FetchRemotes(r workspace.FetchRemotesResult) contract.FetchRemotesResponse {
	return contract.FetchRemotesResponse{
		OperationID: r.OperationID,
		Results:     each(r.Results, func(v workspace.FetchResult) contract.FetchResult { return contract.FetchResult(v) }),
		Repository:  Repository(r.Repository),
	}
}

func Commits(in []workspace.Commit) contract.Commits {
	return contract.Commits{Commits: each(in, func(v workspace.Commit) contract.Commit { return contract.Commit(v) })}
}

func RefreshBranchBase(r workspace.BranchBaseRefresh) contract.RefreshBranchBaseResponse {
	return contract.RefreshBranchBaseResponse(r)
}

func gitDocument(d codeview.Document) contract.GitDocument {
	return contract.GitDocument{
		NodeID: d.NodeID, Path: d.Path, Absolute: d.Absolute, Source: d.Source, OID: d.OID,
		Version: d.Version, Kind: d.Kind, Text: d.Text, Lines: d.Lines, Referenceable: d.Referenceable,
		Reason: d.Reason, MediaType: d.MediaType, Size: d.Size, Downloadable: d.Downloadable,
	}
}

func GitFile(f codeview.File) contract.GitFile {
	return contract.GitFile{Snapshot: f.Snapshot, Document: gitDocument(f.Document), Checked: f.Checked}
}

func gitChange(c codeview.Change) contract.GitChange {
	return contract.GitChange(c)
}

func GitChanges(c codeview.Changes) contract.GitChanges {
	return contract.GitChanges{
		Next: c.Next, Total: c.Total, Entries: each(c.Entries, gitChange),
		Version: c.Version, Checked: c.Checked,
	}
}

func GitDiff(d codeview.Diff) contract.GitDiff {
	return contract.GitDiff{
		Snapshot: d.Snapshot, Old: gitDocument(d.Old), New: gitDocument(d.New),
		Kind: d.Kind, Reason: d.Reason, Checked: d.Checked, Change: gitChange(d.Change),
	}
}

func GitCompare(c codeview.Compare) contract.GitCompare {
	out := contract.GitCompare{
		BaseOID:   c.BaseOID,
		TargetOID: c.TargetOID,
		Changes:   GitChanges(c.Changes),
	}
	if c.Diff != nil {
		d := GitDiff(*c.Diff)
		out.Diff = &d
	}
	return out
}

func gitRefBadge(b workspace.HistoryRefBadge) contract.GitRefBadge {
	return contract.GitRefBadge(b)
}

func gitHistoryEdge(e workspace.HistoryEdge) contract.GitHistoryEdge {
	return contract.GitHistoryEdge(e)
}

func gitHistoryRow(c workspace.HistoryRow) contract.GitHistoryRow {
	return contract.GitHistoryRow{
		OID:                  c.OID,
		ShortOID:             c.ShortOID,
		Parents:              required(c.Parents),
		Subject:              c.Subject,
		AuthorName:           c.AuthorName,
		AuthorEmail:          c.AuthorEmail,
		AuthorAt:             c.AuthorAt,
		CommitterName:        c.CommitterName,
		CommitterAt:          c.CommitterAt,
		Refs:                 each(c.Refs, gitRefBadge),
		Lane:                 c.Lane,
		ActiveLanes:          required(c.ActiveLanes),
		Edges:                each(c.Edges, gitHistoryEdge),
		DirectParentsMissing: c.DirectParentsMissing,
	}
}

func GitHistoryPage(p workspace.HistoryPage) contract.GitHistoryPage {
	return contract.GitHistoryPage{
		Status:        p.Status,
		Reason:        p.Reason,
		Scope:         p.Scope,
		LastFetchedAt: p.LastFetchedAt,
		Upstream:      p.Upstream,
		RefsChanged:   p.RefsChanged,
		NextCursor:    p.NextCursor,
		Rows:          each(p.Commits, gitHistoryRow),
	}
}

func gitCommitParent(p workspace.CommitParent) contract.GitCommitParent {
	return contract.GitCommitParent(p)
}

func gitCommitFileChange(f workspace.CommitFileChange) contract.GitCommitFileChange {
	return contract.GitCommitFileChange(f)
}

func GitCommitDetail(d workspace.CommitDetail) contract.GitCommitDetail {
	return contract.GitCommitDetail{
		OID:               d.OID,
		ShortOID:          d.ShortOID,
		Subject:           d.Subject,
		Body:              d.Body,
		Message:           d.Message,
		AuthorName:        d.AuthorName,
		AuthorEmail:       d.AuthorEmail,
		AuthorAt:          d.AuthorAt,
		CommitterName:     d.CommitterName,
		CommitterEmail:    d.CommitterEmail,
		CommitterAt:       d.CommitterAt,
		DistinctCommitter: d.DistinctCommitter,
		Parents:           each(d.Parents, gitCommitParent),
		ComparedParentOID: d.SelectedParent,
		Refs:              each(d.Refs, gitRefBadge),
		Files:             each(d.Files, gitCommitFileChange),
	}
}

func gitStageIdentity(id workspace.StageFileIdentity) contract.GitStageFileIdentity {
	return contract.GitStageFileIdentity{
		Area:         string(id.Area),
		Path:         id.Path,
		OldPath:      id.OldPath,
		OldBaseOID:   id.OldBaseOID,
		OldBaseMode:  id.OldBaseMode,
		BaseOID:      id.BaseOID,
		BaseMode:     id.BaseMode,
		TargetOID:    id.TargetOID,
		TargetSHA256: id.TargetSHA256,
		TargetMode:   id.TargetMode,
	}
}

func gitStagingEntry(e workspace.StagingEntry) contract.GitStagingEntry {
	return contract.GitStagingEntry{
		Path:               e.Path,
		OldPath:            e.OldPath,
		Area:               string(e.Area),
		Status:             e.Status,
		HunkStageAvailable: e.HunkStageAvailable,
		DegradeReason:      e.DegradeReason,
		Identity:           gitStageIdentity(e.Identity),
	}
}

func GitStagingOverview(o workspace.StagingOverview) contract.GitStagingOverview {
	return contract.GitStagingOverview{
		Unborn:     o.Unborn,
		HeadOID:    o.HeadOID,
		Staged:     each(o.Staged, gitStagingEntry),
		Unstaged:   each(o.Unstaged, gitStagingEntry),
		Conflicted: each(o.Conflicted, gitStagingEntry),
	}
}

func gitStagingHunkLine(l workspace.StagingHunkLine) contract.GitStagingHunkLine {
	return contract.GitStagingHunkLine(l)
}

func gitStagingHunk(h workspace.StagingHunk) contract.GitStagingHunk {
	return contract.GitStagingHunk{
		ID:       h.ID,
		Header:   h.Header,
		OldStart: h.OldStart,
		OldLines: h.OldLines,
		NewStart: h.NewStart,
		NewLines: h.NewLines,
		Lines:    each(h.Lines, gitStagingHunkLine),
	}
}

func GitStagingFileDiff(d workspace.StagingFileDiff) contract.GitStagingFileDiff {
	return contract.GitStagingFileDiff{
		Identity:           gitStageIdentity(d.Identity),
		Conflicted:         d.Conflicted,
		Binary:             d.Binary,
		HunkStageAvailable: d.HunkStageAvailable,
		DegradeReason:      d.DegradeReason,
		Hunks:              each(d.Hunks, gitStagingHunk),
	}
}

func GitStageRequest(repositoryID, worktreeID string, req contract.GitStageRequest) workspace.StageRequest {
	return workspace.StageRequest{
		RepositoryID: repositoryID,
		WorktreeID:   worktreeID,
		Identity: workspace.StageFileIdentity{
			Area:         workspace.StageArea(req.Identity.Area),
			Path:         req.Identity.Path,
			OldPath:      req.Identity.OldPath,
			OldBaseOID:   req.Identity.OldBaseOID,
			OldBaseMode:  req.Identity.OldBaseMode,
			BaseOID:      req.Identity.BaseOID,
			BaseMode:     req.Identity.BaseMode,
			TargetOID:    req.Identity.TargetOID,
			TargetSHA256: req.Identity.TargetSHA256,
			TargetMode:   req.Identity.TargetMode,
		},
		HunkIDs: req.HunkIDs,
	}
}

func GitCommitPreview(p workspace.CommitPreview) contract.GitCommitPreview {
	return contract.GitCommitPreview{
		ConfirmationID:  p.ConfirmationID,
		ExpiresAt:       p.ExpiresAt,
		RepositoryID:    p.RepositoryID,
		WorktreeID:      p.WorktreeID,
		WorktreePath:    p.WorktreePath,
		HeadState:       p.HeadState,
		Branch:          p.Branch,
		HeadOID:         p.HeadOID,
		ExpectedTreeOID: p.ExpectedTreeOID,
		IndexDigest:     p.IndexDigest,
		StagedFiles:     each(p.StagedFiles, gitCommitFileChange),
		UnstagedCount:   p.UnstagedCount,
		UntrackedCount:  p.UntrackedCount,
		ConflictedCount: p.ConflictedCount,
		Operation:       p.Operation,
		Identity:        contract.GitCommitIdentity(p.Identity),
		ExecutableHooks: required(p.ExecutableHooks),
		Blockers:        Issues(p.Blockers),
		Warnings:        Issues(p.Warnings),
	}
}

func GitCommitResult(r workspace.CommitResult) contract.GitCommitResult {
	return contract.GitCommitResult{
		OperationID:     r.OperationID,
		Outcome:         r.Outcome,
		CommitOID:       r.CommitOID,
		Parents:         required(r.Parents),
		TreeOID:         r.TreeOID,
		ExpectedTreeOID: r.ExpectedTreeOID,
		Subject:         r.Subject,
		ReviewReasons:   required(r.ReviewReasons),
	}
}

func GitOperation(op workspace.GitOperation) contract.GitOperation {
	return contract.GitOperation{
		ID:              op.ID,
		RepositoryID:    op.RepositoryID,
		WorktreeID:      op.WorktreeID,
		ClientRequestID: op.ClientRequestID,
		Kind:            op.Kind,
		Status:          op.Status,
		ConfirmationID:  op.ConfirmationID,
		StartedAt:       op.StartedAt,
		FinishedAt:      op.FinishedAt,
		Summary:         op.Summary,
		SanitizedOutput: op.SanitizedOutput,
		RemoteStatus:    op.RemoteStatus,
		FetchResults:    each(op.FetchResults, func(v workspace.FetchResult) contract.FetchResult { return contract.FetchResult(v) }),
		Preview:         contract.GitOperationPreview(op.Preview),
		ActualOID:       op.ActualOID,
		ActualTreeOID:   op.ActualTreeOID,
		Parents:         required(op.Parents),
		Subject:         op.Subject,
		ReviewReasons:   required(op.ReviewReasons),
	}
}

func GitOperations(in []workspace.GitOperation) contract.GitOperations {
	return contract.GitOperations{
		Operations: each(in, GitOperation),
	}
}
