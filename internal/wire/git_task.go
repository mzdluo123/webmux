package wire

import (
	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/workspace"
)

func GitTaskBaseline(b workspace.TaskBaseline) contract.GitTaskBaseline {
	return contract.GitTaskBaseline{
		BaselineID:   b.BaselineID,
		RepositoryID: b.RepositoryID,
		WorktreeID:   b.WorktreeID,
		WorktreePath: b.WorktreePath,
		BaseOID:      b.BaseOID,
		Unborn:       b.Unborn,
		CapturedAt:   b.CapturedAt,
		Source:       b.Source,
		CleanAtStart: b.CleanAtStart,
		PreExistingChanges: contract.WorktreeChanges{
			Staged:     b.PreExistingChanges.Staged,
			Unstaged:   b.PreExistingChanges.Unstaged,
			Untracked:  b.PreExistingChanges.Untracked,
			Conflicted: b.PreExistingChanges.Conflicted,
		},
	}
}

func GitTaskBaselinePtr(b *workspace.TaskBaseline) *contract.GitTaskBaseline {
	if b == nil {
		return nil
	}
	v := GitTaskBaseline(*b)
	return &v
}

func GitTaskBaselineResponse(b *workspace.TaskBaseline) contract.GitTaskBaselineResponse {
	return contract.GitTaskBaselineResponse{Baseline: GitTaskBaselinePtr(b)}
}

func GitTaskChanges(tc codeview.TaskChanges) contract.GitTaskChanges {
	return contract.GitTaskChanges{
		Status:           tc.Status,
		Reason:           tc.Reason,
		Baseline:         GitTaskBaselinePtr(tc.Baseline),
		HeadOID:          tc.HeadOID,
		HeadUnborn:       tc.HeadUnborn,
		CommitCount:      tc.CommitCount,
		UncommittedCount: tc.UncommittedCount,
		Changes:          GitChanges(tc.Changes),
	}
}
