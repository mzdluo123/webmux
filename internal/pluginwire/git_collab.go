package pluginwire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/workspace"
)

func GitCommitDraft(d workspace.CommitDraft) contract.GitCommitDraft {
	return contract.GitCommitDraft{
		RepositoryID: d.RepositoryID,
		WorktreeID:   d.WorktreeID,
		Subject:      d.Subject,
		Body:         d.Body,
		Revision:     d.Revision,
		UpdatedAt:    d.UpdatedAt,
	}
}

func gitReviewFileState(f workspace.ReviewFileState) contract.GitReviewFileState {
	return contract.GitReviewFileState{
		Scope:          f.Scope,
		Area:           f.Area,
		Path:           f.Path,
		OldPath:        f.OldPath,
		Status:         f.Status,
		BaseMode:       f.BaseMode,
		BaseIdentity:   f.BaseIdentity,
		TargetMode:     f.TargetMode,
		TargetIdentity: f.TargetIdentity,
		Reviewed:       f.Reviewed,
		ReviewedAt:     f.ReviewedAt,
	}
}

func GitReviewsResponse(r workspace.ReviewsOverview) contract.GitReviewsResponse {
	return contract.GitReviewsResponse{
		Scope:          r.Scope,
		Status:         r.Status,
		Reason:         r.Reason,
		TotalFiles:     r.TotalFiles,
		ReviewedFiles:  r.ReviewedFiles,
		Files:          each(r.Files, gitReviewFileState),
		Applied:        r.Applied,
		StaleRejected:  r.StaleRejected,
		RejectedReason: r.RejectedReason,
	}
}

func GitPutReviewsRequest(req contract.GitPutReviewsRequest) workspace.PutReviewsRequest {
	out := workspace.PutReviewsRequest{
		Scope:  req.Scope,
		Action: req.Action,
	}
	if req.Item != nil {
		out.Item = &workspace.ReviewMarkIdentity{
			Scope:          req.Item.Scope,
			Area:           req.Item.Area,
			Path:           req.Item.Path,
			OldPath:        req.Item.OldPath,
			BaseMode:       req.Item.BaseMode,
			BaseIdentity:   req.Item.BaseIdentity,
			TargetMode:     req.Item.TargetMode,
			TargetIdentity: req.Item.TargetIdentity,
		}
	}
	return out
}
