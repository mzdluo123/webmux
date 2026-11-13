package contract

import "time"

// Git workbench V2 divergence (Merge, Rebase, Cherry-pick, Revert) and
// recovery (Continue, Abort, InProgressOperation) wire contracts.

// GitConflictFileSummary summarizes one unmerged entry in the Git index.
type GitConflictFileSummary struct {
	Path       string `json:"path"`
	XY         string `json:"xy"`
	Kind       string `json:"kind"`
	Stage1Mode string `json:"stage1Mode,omitempty"`
	Stage1OID  string `json:"stage1Oid,omitempty"`
	Stage2Mode string `json:"stage2Mode,omitempty"`
	Stage2OID  string `json:"stage2Oid,omitempty"`
	Stage3Mode string `json:"stage3Mode,omitempty"`
	Stage3OID  string `json:"stage3Oid,omitempty"`
}

// GitInProgressOperation answers GET .../git/operation-state.
type GitInProgressOperation struct {
	Active           bool                     `json:"active"`
	Kind             string                   `json:"kind,omitempty"`
	WorktreeID       string                   `json:"worktreeId"`
	WorktreePath     string                   `json:"worktreePath"`
	CurrentBranch    string                   `json:"currentBranch,omitempty"`
	Detached         bool                     `json:"detached"`
	HeadOID          string                   `json:"headOid,omitempty"`
	HeadSubject      string                   `json:"headSubject,omitempty"`
	OrigHeadOID      string                   `json:"origHeadOid,omitempty"`
	OrigHeadSubject  string                   `json:"origHeadSubject,omitempty"`
	SourceLabel      string                   `json:"sourceLabel,omitempty"`
	SourceOID        string                   `json:"sourceOid,omitempty"`
	SourceSubject    string                   `json:"sourceSubject,omitempty"`
	TargetLabel      string                   `json:"targetLabel,omitempty"`
	TargetOID        string                   `json:"targetOid,omitempty"`
	TargetSubject    string                   `json:"targetSubject,omitempty"`
	StepCurrent      int                      `json:"stepCurrent,omitempty"`
	StepTotal        int                      `json:"stepTotal,omitempty"`
	StoppedCommitOID string                   `json:"stoppedCommitOid,omitempty"`
	StoppedSubject   string                   `json:"stoppedSubject,omitempty"`
	ConflictedCount  int                      `json:"conflictedCount"`
	ConflictedFiles  []GitConflictFileSummary `json:"conflictedFiles"`
	CanContinue      bool                     `json:"canContinue"`
	CanAbort         bool                     `json:"canAbort"`
	ContinueBlockers []Issue                  `json:"continueBlockers"`
}

// GitBranchDivergencePreviewRequest is the body of POST .../git/merge-preview
// and POST .../git/rebase-preview.
type GitBranchDivergencePreviewRequest struct {
	RefKind string `json:"refKind"` // "local" | "remote"
	RefName string `json:"refName"`
}

// GitCommitDivergencePreviewRequest is the body of POST .../git/cherry-pick-preview
// and POST .../git/revert-preview.
type GitCommitDivergencePreviewRequest struct {
	CommitOID string `json:"commitOid"`
}

// GitDivergencePreview answers merge-preview, rebase-preview, cherry-pick-preview,
// and revert-preview. With blockers ConfirmationID is "" and ExpiresAt is null.
type GitDivergencePreview struct {
	ConfirmationID   string        `json:"confirmationId"`
	ExpiresAt        *time.Time    `json:"expiresAt"`
	Kind             string        `json:"kind"`
	RepositoryID     string        `json:"repositoryId"`
	WorktreeID       string        `json:"worktreeId"`
	WorktreePath     string        `json:"worktreePath"`
	DirectionSummary string        `json:"directionSummary"`
	RecoveryHint     string        `json:"recoveryHint"`
	CurrentBranch    string        `json:"currentBranch,omitempty"`
	HeadOID          string        `json:"headOid,omitempty"`
	HeadSubject      string        `json:"headSubject,omitempty"`
	TargetRef        string        `json:"targetRef"`
	TargetKind       string        `json:"targetKind"`
	TargetOID        string        `json:"targetOid"`
	TargetSubject    string        `json:"targetSubject"`
	MergeBaseOID     string        `json:"mergeBaseOid,omitempty"`
	FastForward      bool          `json:"fastForward"`
	AlreadyUpToDate  bool          `json:"alreadyUpToDate"`
	Commits          CommitList    `json:"commits"`
	Sessions         []SessionInfo `json:"sessions"`
	Blockers         []Issue       `json:"blockers"`
	Warnings         []Issue       `json:"warnings"`
}

// GitDivergenceExecuteRequest is the body of POST .../git/merge,
// POST .../git/rebase, POST .../git/cherry-pick, and POST .../git/revert.
type GitDivergenceExecuteRequest struct {
	ConfirmationID      string `json:"confirmationId"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
}

// GitContinueOperationRequest is the body of POST .../git/operation-continue.
type GitContinueOperationRequest struct {
	Kind            string `json:"kind"`
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// GitAbortOperationPreviewRequest is the optional empty JSON body of
// POST .../git/operation-abort-preview.
type GitAbortOperationPreviewRequest struct{}

// GitAbortOperationPreview answers POST .../git/operation-abort-preview.
type GitAbortOperationPreview struct {
	ConfirmationID   string        `json:"confirmationId"`
	ExpiresAt        *time.Time    `json:"expiresAt"`
	RepositoryID     string        `json:"repositoryId"`
	WorktreeID       string        `json:"worktreeId"`
	WorktreePath     string        `json:"worktreePath"`
	Kind             string        `json:"kind"`
	CurrentBranch    string        `json:"currentBranch,omitempty"`
	HeadOID          string        `json:"headOid,omitempty"`
	RestoreOID       string        `json:"restoreOid,omitempty"`
	RestoreSubject   string        `json:"restoreSubject,omitempty"`
	DirectionSummary string        `json:"directionSummary"`
	AffectedPaths    []string      `json:"affectedPaths"`
	Sessions         []SessionInfo `json:"sessions"`
	Blockers         []Issue       `json:"blockers"`
	Warnings         []Issue       `json:"warnings"`
}

// GitAbortOperationRequest is the body of POST .../git/operation-abort.
type GitAbortOperationRequest struct {
	ConfirmationID      string `json:"confirmationId"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
}

// GitDivergenceResult answers merge, rebase, cherry-pick, revert,
// operation-continue, and operation-abort.
type GitDivergenceResult struct {
	OperationID string                 `json:"operationId,omitempty"`
	Kind        string                 `json:"kind"`
	Outcome     string                 `json:"outcome"`
	Summary     string                 `json:"summary"`
	HeadOID     string                 `json:"headOid,omitempty"`
	Subject     string                 `json:"subject,omitempty"`
	RecoveryRef string                 `json:"recoveryRef,omitempty"`
	InProgress  GitInProgressOperation `json:"inProgress"`
	Worktree    Worktree               `json:"worktree"`
}
