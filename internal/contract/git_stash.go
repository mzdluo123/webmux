package contract

import "time"

// GitStashEntry summarizes one entry in refs/stash reflog.
type GitStashEntry struct {
	StashRef     string   `json:"stashRef"`
	Index        int      `json:"index"`
	OID          string   `json:"oid"`
	ShortOID     string   `json:"shortOid"`
	BaseOID      string   `json:"baseOid,omitempty"`
	IndexOID     string   `json:"indexOid,omitempty"`
	UntrackedOID string   `json:"untrackedOid,omitempty"`
	Parents      []string `json:"parents"`
	HasUntracked bool     `json:"hasUntracked"`
	Branch       string   `json:"branch,omitempty"`
	Summary      string   `json:"summary"`
	CreatedAt    string   `json:"createdAt"`
}

// GitStashList answers GET /api/repositories/{id}/git/stashes.
type GitStashList struct {
	Stashes []GitStashEntry `json:"stashes"`
}

// GitStashFileChange summarizes one tracked or untracked file in a stash entry.
type GitStashFileChange struct {
	OldPath     string `json:"oldPath"`
	NewPath     string `json:"newPath"`
	Status      string `json:"status"`
	OldMode     string `json:"oldMode"`
	NewMode     string `json:"newMode"`
	OldOID      string `json:"oldOid"`
	NewOID      string `json:"newOid"`
	Binary      bool   `json:"binary"`
	ModeChanged bool   `json:"modeChanged"`
	Untracked   bool   `json:"untracked"`
}

// GitStashDetail answers GET /api/repositories/{id}/git/stashes/{oid}.
type GitStashDetail struct {
	Entry          GitStashEntry        `json:"entry"`
	AuthorName     string               `json:"authorName"`
	AuthorEmail    string               `json:"authorEmail"`
	TrackedCount   int                  `json:"trackedCount"`
	UntrackedCount int                  `json:"untrackedCount"`
	Files          []GitStashFileChange `json:"files"`
}

// GitStashCreatePreviewRequest is the body of POST .../git/stash-preview.
type GitStashCreatePreviewRequest struct {
	Message          string `json:"message,omitempty"`
	IncludeUntracked bool   `json:"includeUntracked,omitempty"`
	KeepIndex        bool   `json:"keepIndex,omitempty"`
}

// GitStashCreatePreview answers POST .../git/stash-preview.
type GitStashCreatePreview struct {
	ConfirmationID   string        `json:"confirmationId"`
	ExpiresAt        *time.Time    `json:"expiresAt"`
	RepositoryID     string        `json:"repositoryId"`
	WorktreeID       string        `json:"worktreeId"`
	WorktreePath     string        `json:"worktreePath"`
	CurrentBranch    string        `json:"currentBranch,omitempty"`
	Detached         bool          `json:"detached"`
	HeadOID          string        `json:"headOid,omitempty"`
	HeadSubject      string        `json:"headSubject,omitempty"`
	Message          string        `json:"message,omitempty"`
	IncludeUntracked bool          `json:"includeUntracked"`
	KeepIndex        bool          `json:"keepIndex"`
	StagedCount      int           `json:"stagedCount"`
	UnstagedCount    int           `json:"unstagedCount"`
	UntrackedCount   int           `json:"untrackedCount"`
	AffectedPaths    []string      `json:"affectedPaths"`
	Sessions         []SessionInfo `json:"sessions"`
	Blockers         []Issue       `json:"blockers"`
	Warnings         []Issue       `json:"warnings"`
}

// GitStashExecuteRequest is the body of POST .../git/stash, stash-apply, and stash-pop.
type GitStashExecuteRequest struct {
	ConfirmationID      string `json:"confirmationId"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
}

// GitStashCreateResult answers POST .../git/stash.
type GitStashCreateResult struct {
	OperationID string        `json:"operationId,omitempty"`
	Entry       GitStashEntry `json:"entry"`
	Worktree    Worktree      `json:"worktree"`
}

// GitStashApplyPreviewRequest is the body of POST .../git/stash-apply-preview.
type GitStashApplyPreviewRequest struct {
	Mode           string `json:"mode"` // "apply" | "pop"
	StashRef       string `json:"stashRef"`
	StashOID       string `json:"stashOid"`
	ReinstateIndex bool   `json:"reinstateIndex,omitempty"`
}

// GitStashApplyPreview answers POST .../git/stash-apply-preview.
type GitStashApplyPreview struct {
	ConfirmationID   string        `json:"confirmationId"`
	ExpiresAt        *time.Time    `json:"expiresAt"`
	RepositoryID     string        `json:"repositoryId"`
	WorktreeID       string        `json:"worktreeId"`
	WorktreePath     string        `json:"worktreePath"`
	Mode             string        `json:"mode"`
	Entry            GitStashEntry `json:"entry"`
	ReinstateIndex   bool          `json:"reinstateIndex"`
	CurrentBranch    string        `json:"currentBranch,omitempty"`
	Detached         bool          `json:"detached"`
	HeadOID          string        `json:"headOid,omitempty"`
	HeadSubject      string        `json:"headSubject,omitempty"`
	StagedCount      int           `json:"stagedCount"`
	UnstagedCount    int           `json:"unstagedCount"`
	UntrackedCount   int           `json:"untrackedCount"`
	OverlappingPaths []string      `json:"overlappingPaths"`
	Sessions         []SessionInfo `json:"sessions"`
	Blockers         []Issue       `json:"blockers"`
	Warnings         []Issue       `json:"warnings"`
}

// GitStashApplyResult answers POST .../git/stash-apply and POST .../git/stash-pop.
type GitStashApplyResult struct {
	OperationID    string                 `json:"operationId,omitempty"`
	Kind           string                 `json:"kind"`
	Outcome        string                 `json:"outcome"`
	StashRef       string                 `json:"stashRef"`
	StashOID       string                 `json:"stashOid"`
	StashDropped   bool                   `json:"stashDropped"`
	StashPreserved bool                   `json:"stashPreserved"`
	Summary        string                 `json:"summary"`
	InProgress     GitInProgressOperation `json:"inProgress"`
	Worktree       Worktree               `json:"worktree"`
}

// GitStashDropPreviewRequest is the body of POST /api/repositories/{id}/git/stashes/drop-preview.
type GitStashDropPreviewRequest struct {
	StashRef string `json:"stashRef"`
	StashOID string `json:"stashOid"`
}

// GitStashDropPreview answers POST /api/repositories/{id}/git/stashes/drop-preview.
type GitStashDropPreview struct {
	ConfirmationID  string        `json:"confirmationId"`
	ExpiresAt       *time.Time    `json:"expiresAt"`
	RepositoryID    string        `json:"repositoryId"`
	Entry           GitStashEntry `json:"entry"`
	FileCount       int           `json:"fileCount"`
	RecoveryCommand string        `json:"recoveryCommand"`
	Blockers        []Issue       `json:"blockers"`
	Warnings        []Issue       `json:"warnings"`
}

// GitStashDropRequest is the body of POST /api/repositories/{id}/git/stashes/drop.
type GitStashDropRequest struct {
	ConfirmationID  string `json:"confirmationId"`
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// GitStashDropResult answers POST /api/repositories/{id}/git/stashes/drop.
type GitStashDropResult struct {
	OperationID     string          `json:"operationId,omitempty"`
	StashRef        string          `json:"stashRef"`
	StashOID        string          `json:"stashOid"`
	RecoveryCommand string          `json:"recoveryCommand"`
	Stashes         []GitStashEntry `json:"stashes"`
}
