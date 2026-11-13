package contract

import "time"

// Git workbench V3 shapes: Discard Changes (files / hunks / lines) and
// 3-phase tracked-stash Smart Checkout (#551).

// GitDiscardPreviewItem describes the destructive effect on one path.
type GitDiscardPreviewItem struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Action string `json:"action"`
}

// GitDiscardPreviewRequest is the body of
// POST /api/repositories/{id}/worktrees/{worktreeId}/git/discard-preview.
type GitDiscardPreviewRequest struct {
	Scope          string                 `json:"scope"`
	Paths          []string               `json:"paths,omitempty"`
	Identities     []GitStageFileIdentity `json:"identities,omitempty"`
	Identity       *GitStageFileIdentity  `json:"identity,omitempty"`
	HunkIDs        []string               `json:"hunkIds,omitempty"`
	LineSelections []GitLineSelection     `json:"lineSelections,omitempty"`
}

// GitDiscardPreview answers POST .../git/discard-preview.
type GitDiscardPreview struct {
	ConfirmationID        string                  `json:"confirmationId"`
	ExpiresAt             *time.Time              `json:"expiresAt"`
	RepositoryID          string                  `json:"repositoryId"`
	WorktreeID            string                  `json:"worktreeId"`
	WorktreePath          string                  `json:"worktreePath"`
	Scope                 string                  `json:"scope"`
	TrackedOverwritePaths []string                `json:"trackedOverwritePaths"`
	UntrackedDeletePaths  []string                `json:"untrackedDeletePaths"`
	Items                 []GitDiscardPreviewItem `json:"items"`
	HunkCount             int                     `json:"hunkCount"`
	LineCount             int                     `json:"lineCount"`
	Summary               string                  `json:"summary"`
	Sessions              []SessionInfo           `json:"sessions"`
	Blockers              []Issue                 `json:"blockers"`
	Warnings              []Issue                 `json:"warnings"`
}

// GitDiscardRequest is the body of POST .../git/discard.
type GitDiscardRequest struct {
	ConfirmationID      string `json:"confirmationId"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions,omitempty"`
}

// GitDiscardResult answers POST .../git/discard.
type GitDiscardResult struct {
	OperationID     string   `json:"operationId,omitempty"`
	Scope           string   `json:"scope"`
	Summary         string   `json:"summary"`
	RecoveryOID     string   `json:"recoveryOid,omitempty"`
	RecoveryRef     string   `json:"recoveryRef,omitempty"`
	RecoveryCommand string   `json:"recoveryCommand,omitempty"`
	DiscardedPaths  []string `json:"discardedPaths"`
	Worktree        Worktree `json:"worktree"`
}

// GitSmartCheckoutPreviewRequest is the body of
// POST /api/repositories/{id}/worktrees/{worktreeId}/git/smart-checkout-preview.
type GitSmartCheckoutPreviewRequest struct {
	Target           SwitchTarget `json:"target"`
	IncludeUntracked *bool        `json:"includeUntracked,omitempty"`
}

// GitSmartCheckoutPreview answers POST .../git/smart-checkout-preview.
type GitSmartCheckoutPreview struct {
	ConfirmationID   string           `json:"confirmationId"`
	ExpiresAt        *time.Time       `json:"expiresAt"`
	RepositoryID     string           `json:"repositoryId"`
	WorktreeID       string           `json:"worktreeId"`
	Worktree         Worktree         `json:"worktree"`
	Target           SwitchTargetInfo `json:"target"`
	IncludeUntracked bool             `json:"includeUntracked"`
	StagedCount      int              `json:"stagedCount"`
	UnstagedCount    int              `json:"unstagedCount"`
	UntrackedCount   int              `json:"untrackedCount"`
	DirtyPaths       []string         `json:"dirtyPaths"`
	HasActiveAgent   bool             `json:"hasActiveAgent"`
	Sessions         []SessionInfo    `json:"sessions"`
	Blockers         []Issue          `json:"blockers"`
	Warnings         []Issue          `json:"warnings"`
}

// GitSmartCheckoutRequest is the body of POST .../git/smart-checkout.
type GitSmartCheckoutRequest struct {
	ConfirmationID      string `json:"confirmationId"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions,omitempty"`
}

// GitSmartCheckoutResult answers POST .../git/smart-checkout.
type GitSmartCheckoutResult struct {
	OperationID     string                 `json:"operationId,omitempty"`
	Outcome         string                 `json:"outcome"`
	Phase           string                 `json:"phase"`
	Summary         string                 `json:"summary"`
	ErrorMessage    string                 `json:"errorMessage,omitempty"`
	StashOID        string                 `json:"stashOid,omitempty"`
	StashRef        string                 `json:"stashRef,omitempty"`
	StashDropped    bool                   `json:"stashDropped"`
	StashPreserved  bool                   `json:"stashPreserved"`
	StashRestored   bool                   `json:"stashRestored"`
	RecoveryCommand string                 `json:"recoveryCommand,omitempty"`
	InProgress      GitInProgressOperation `json:"inProgress"`
	Worktree        Worktree               `json:"worktree"`
}
