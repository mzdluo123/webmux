package workspace

import (
	"time"

	"github.com/rainchan/webmux/internal/session"
)

const (
	divergenceTimeout = 3 * time.Minute

	DivergenceOutcomeCompleted       = "completed"
	DivergenceOutcomeAlreadyUpToDate = "already_up_to_date"
	DivergenceOutcomeConflicted      = "conflicted"
	DivergenceOutcomeAborted         = "aborted"

	ConflictKindText         = "text"
	ConflictKindBinary       = "binary"
	ConflictKindDeleteModify = "delete_modify"
	ConflictKindSubmodule    = "submodule"
	ConflictKindSymlink      = "symlink"
	ConflictKindTooLarge     = "too_large"
)

// ConflictFileSummary summarizes one unmerged entry in the Git index.
type ConflictFileSummary struct {
	Path       string `json:"path"`
	XY         string `json:"xy"` // porcelain v2 unmerged two-letter code, e.g. "UU", "UD", "DU", "AA"
	Kind       string `json:"kind"`
	Stage1Mode string `json:"stage1Mode,omitempty"`
	Stage1OID  string `json:"stage1Oid,omitempty"`
	Stage2Mode string `json:"stage2Mode,omitempty"`
	Stage2OID  string `json:"stage2Oid,omitempty"`
	Stage3Mode string `json:"stage3Mode,omitempty"`
	Stage3OID  string `json:"stage3Oid,omitempty"`
}

// InProgressOperation describes the live Git sequencer / merge state on disk
// for a worktree, whether started in WebMux or via CLI.
type InProgressOperation struct {
	Active           bool                  `json:"active"`
	Kind             string                `json:"kind,omitempty"` // "merge" | "rebase" | "cherry-pick" | "revert" | "bisect"
	WorktreeID       string                `json:"worktreeId"`
	WorktreePath     string                `json:"worktreePath"`
	CurrentBranch    string                `json:"currentBranch,omitempty"`
	Detached         bool                  `json:"detached"`
	HeadOID          string                `json:"headOid,omitempty"`
	HeadSubject      string                `json:"headSubject,omitempty"`
	OrigHeadOID      string                `json:"origHeadOid,omitempty"`
	OrigHeadSubject  string                `json:"origHeadSubject,omitempty"`
	SourceLabel      string                `json:"sourceLabel,omitempty"`
	SourceOID        string                `json:"sourceOid,omitempty"`
	SourceSubject    string                `json:"sourceSubject,omitempty"`
	TargetLabel      string                `json:"targetLabel,omitempty"`
	TargetOID        string                `json:"targetOid,omitempty"`
	TargetSubject    string                `json:"targetSubject,omitempty"`
	StepCurrent      int                   `json:"stepCurrent,omitempty"`
	StepTotal        int                   `json:"stepTotal,omitempty"`
	StoppedCommitOID string                `json:"stoppedCommitOid,omitempty"`
	StoppedSubject   string                `json:"stoppedSubject,omitempty"`
	ConflictedCount  int                   `json:"conflictedCount"`
	ConflictedFiles  []ConflictFileSummary `json:"conflictedFiles"`
	CanContinue      bool                  `json:"canContinue"`
	CanAbort         bool                  `json:"canAbort"`
	ContinueBlockers []Issue               `json:"continueBlockers"`
}

// DivergencePreviewRequest identifies the branch or commit for Merge, Rebase,
// Cherry-pick, or Revert preview.
type DivergencePreviewRequest struct {
	RepositoryID string `json:"repositoryId"`
	WorktreeID   string `json:"worktreeId"`
	// RefKind is "local" or "remote" for Merge and Rebase.
	RefKind string `json:"refKind,omitempty"`
	// RefName is e.g. "main" (local) or "origin/main" (remote).
	RefName string `json:"refName,omitempty"`
	// CommitOID is the 40/64-hex commit OID for Cherry-pick and Revert.
	CommitOID string `json:"commitOid,omitempty"`
}

// DivergencePreview captures the explicit direction, blast radius, incoming or
// replayed commits, active sessions, and recovery path before mutating Git state.
type DivergencePreview struct {
	ConfirmationID   string         `json:"confirmationId,omitempty"`
	ExpiresAt        *time.Time     `json:"expiresAt,omitempty"`
	Kind             string         `json:"kind"` // "merge" | "rebase" | "cherry_pick" | "revert"
	RepositoryID     string         `json:"repositoryId"`
	WorktreeID       string         `json:"worktreeId"`
	WorktreePath     string         `json:"worktreePath"`
	DirectionSummary string         `json:"directionSummary"`
	RecoveryHint     string         `json:"recoveryHint"`
	CurrentBranch    string         `json:"currentBranch,omitempty"`
	HeadOID          string         `json:"headOid,omitempty"`
	HeadSubject      string         `json:"headSubject,omitempty"`
	TargetRef        string         `json:"targetRef"`
	TargetKind       string         `json:"targetKind"` // "local" | "remote" | "commit"
	TargetOID        string         `json:"targetOid"`
	TargetSubject    string         `json:"targetSubject"`
	MergeBaseOID     string         `json:"mergeBaseOid,omitempty"`
	FastForward      bool           `json:"fastForward"`
	AlreadyUpToDate  bool           `json:"alreadyUpToDate"`
	Commits          CommitList     `json:"commits"`
	Sessions         []session.Info `json:"sessions"`
	Blockers         []Issue        `json:"blockers"`
	Warnings         []Issue        `json:"warnings"`
}

type divergencePreviewState struct {
	kind             string
	purpose          purpose
	commonDir        string
	worktreeID       string
	worktreePath     string
	currentBranch    string
	headOID          string
	headSubject      string
	targetRef        string
	targetFullRef    string
	targetKind       string
	targetOID        string
	targetSubject    string
	mergeBaseOID     string
	fastForward      bool
	alreadyUpToDate  bool
	directionSummary string
	recoveryHint     string
	commits          CommitList
	sessions         []session.Info
	blockers         []Issue
	warnings         []Issue
}

// ExecuteDivergenceRequest executes a previewed Merge, Rebase, Cherry-pick, or Revert.
type ExecuteDivergenceRequest struct {
	RepositoryID        string `json:"repositoryId"`
	WorktreeID          string `json:"worktreeId"`
	ConfirmationID      string `json:"confirmationId"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
}

// ContinueOperationRequest resumes an in-progress Merge, Rebase, Cherry-pick,
// or Revert after all unmerged index entries have been resolved.
type ContinueOperationRequest struct {
	RepositoryID    string `json:"repositoryId"`
	WorktreeID      string `json:"worktreeId"`
	Kind            string `json:"kind"` // expected active operation kind ("merge"|"rebase"|"cherry-pick"|"revert")
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// AbortOperationPreview describes the exact target commit and affected files
// that aborting an in-progress operation will restore and discard.
type AbortOperationPreview struct {
	ConfirmationID   string         `json:"confirmationId,omitempty"`
	ExpiresAt        *time.Time     `json:"expiresAt,omitempty"`
	RepositoryID     string         `json:"repositoryId"`
	WorktreeID       string         `json:"worktreeId"`
	WorktreePath     string         `json:"worktreePath"`
	Kind             string         `json:"kind"`
	CurrentBranch    string         `json:"currentBranch,omitempty"`
	HeadOID          string         `json:"headOid,omitempty"`
	RestoreOID       string         `json:"restoreOid,omitempty"`
	RestoreSubject   string         `json:"restoreSubject,omitempty"`
	DirectionSummary string         `json:"directionSummary"`
	AffectedPaths    []string       `json:"affectedPaths"`
	Sessions         []session.Info `json:"sessions"`
	Blockers         []Issue        `json:"blockers"`
	Warnings         []Issue        `json:"warnings"`
}

type abortPreviewState struct {
	commonDir        string
	worktreeID       string
	worktreePath     string
	kind             string
	currentBranch    string
	headOID          string
	restoreOID       string
	restoreSubject   string
	directionSummary string
	affectedPaths    []string
	sessions         []session.Info
	blockers         []Issue
	warnings         []Issue
}

// ExecuteAbortOperationRequest confirms and executes an operation abort.
type ExecuteAbortOperationRequest struct {
	RepositoryID        string `json:"repositoryId"`
	WorktreeID          string `json:"worktreeId"`
	ConfirmationID      string `json:"confirmationId"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
}

// DivergenceResult reports the outcome of Merge, Rebase, Cherry-pick, Revert,
// Continue, or Abort, including structured conflict state when halted on conflicts.
type DivergenceResult struct {
	OperationID string              `json:"operationId,omitempty"`
	Kind        string              `json:"kind"`
	Outcome     string              `json:"outcome"` // "completed" | "already_up_to_date" | "conflicted" | "aborted"
	Summary     string              `json:"summary"`
	HeadOID     string              `json:"headOid,omitempty"`
	Subject     string              `json:"subject,omitempty"`
	RecoveryRef string              `json:"recoveryRef,omitempty"`
	InProgress  InProgressOperation `json:"inProgress"`
	Worktree    Worktree            `json:"worktree"`
}
