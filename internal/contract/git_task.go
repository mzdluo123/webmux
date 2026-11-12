package contract

import "time"

// GitTaskBaseline records the fixed review origin of one worktree.
type GitTaskBaseline struct {
	BaselineID         string          `json:"baselineId"`
	RepositoryID       string          `json:"repositoryId"`
	WorktreeID         string          `json:"worktreeId"`
	WorktreePath       string          `json:"worktreePath"`
	BaseOID            string          `json:"baseOid,omitempty"`
	Unborn             bool            `json:"unborn"`
	CapturedAt         time.Time       `json:"capturedAt"`
	Source             string          `json:"source"`
	CleanAtStart       bool            `json:"cleanAtStart"`
	PreExistingChanges WorktreeChanges `json:"preExistingChanges"`
}

// GitTaskBaselineResponse answers GET and PUT
// /api/repositories/{id}/worktrees/{worktreeId}/git/task-baseline.
type GitTaskBaselineResponse struct {
	Baseline *GitTaskBaseline `json:"baseline"`
}

// GitPutTaskBaselineRequest is the optional body of
// PUT /api/repositories/{id}/worktrees/{worktreeId}/git/task-baseline.
type GitPutTaskBaselineRequest struct {
	BaseOID string `json:"baseOid,omitempty"`
}

// GitTaskChanges answers GET /api/repositories/{id}/worktrees/{worktreeId}/git/task-changes.
type GitTaskChanges struct {
	Status           string           `json:"status"`
	Reason           string           `json:"reason,omitempty"`
	Baseline         *GitTaskBaseline `json:"baseline"`
	HeadOID          string           `json:"headOid,omitempty"`
	HeadUnborn       bool             `json:"headUnborn"`
	CommitCount      int              `json:"commitCount"`
	UncommittedCount int              `json:"uncommittedCount"`
	Changes          GitChanges       `json:"changes"`
}
