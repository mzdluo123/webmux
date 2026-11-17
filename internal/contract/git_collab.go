package contract

import "time"

// GitCommitDraft answers GET and PUT
// /api/repositories/{id}/worktrees/{worktreeId}/git/commit-draft.
type GitCommitDraft struct {
	RepositoryID string     `json:"repositoryId"`
	WorktreeID   string     `json:"worktreeId"`
	Subject      string     `json:"subject"`
	Body         string     `json:"body"`
	Revision     int        `json:"revision"`
	UpdatedAt    *time.Time `json:"updatedAt"`
}

// GitPutCommitDraftRequest is the body of
// PUT /api/repositories/{id}/worktrees/{worktreeId}/git/commit-draft.
type GitPutCommitDraftRequest struct {
	Subject      string `json:"subject"`
	Body         string `json:"body"`
	BaseRevision int    `json:"baseRevision"`
}

// GitReviewMarkIdentity binds a single file's reviewed state to exact
// server-observed content hashes/OIDs and file modes on both sides.
type GitReviewMarkIdentity struct {
	Scope          string `json:"scope"`
	Area           string `json:"area"`
	Path           string `json:"path"`
	OldPath        string `json:"oldPath,omitempty"`
	BaseMode       string `json:"baseMode,omitempty"`
	BaseIdentity   string `json:"baseIdentity,omitempty"`
	TargetMode     string `json:"targetMode,omitempty"`
	TargetIdentity string `json:"targetIdentity,omitempty"`
}

// GitReviewFileState summarizes one live changed file's review progress and
// content-bound identity in "uncommitted" or "task" scope.
type GitReviewFileState struct {
	Scope          string     `json:"scope"`
	Area           string     `json:"area"`
	Path           string     `json:"path"`
	OldPath        string     `json:"oldPath,omitempty"`
	Status         string     `json:"status"`
	BaseMode       string     `json:"baseMode,omitempty"`
	BaseIdentity   string     `json:"baseIdentity,omitempty"`
	TargetMode     string     `json:"targetMode,omitempty"`
	TargetIdentity string     `json:"targetIdentity,omitempty"`
	Reviewed       bool       `json:"reviewed"`
	ReviewedAt     *time.Time `json:"reviewedAt"`
}

// GitReviewsResponse answers GET and PUT
// /api/repositories/{id}/worktrees/{worktreeId}/git/reviews.
type GitReviewsResponse struct {
	Scope          string               `json:"scope"`
	Status         string               `json:"status"`
	Reason         string               `json:"reason,omitempty"`
	TotalFiles     int                  `json:"totalFiles"`
	ReviewedFiles  int                  `json:"reviewedFiles"`
	Files          []GitReviewFileState `json:"files"`
	Applied        bool                 `json:"applied"`
	StaleRejected  bool                 `json:"staleRejected,omitempty"`
	RejectedReason string               `json:"rejectedReason,omitempty"`
}

// GitPutReviewsRequest is the body of
// PUT /api/repositories/{id}/worktrees/{worktreeId}/git/reviews.
type GitPutReviewsRequest struct {
	Scope  string                 `json:"scope"`
	Action string                 `json:"action"`
	Item   *GitReviewMarkIdentity `json:"item,omitempty"`
}
