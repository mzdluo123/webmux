package workspace

import (
	"time"

	"github.com/rainchan/webmux/internal/session"
)

const (
	DiscardScopeFiles = "files"
	DiscardScopeHunks = "hunks"
	DiscardScopeLines = "lines"

	maxDiscardFiles        = 200
	maxDiscardRecoveryRefs = 20
	discardRecoveryPrefix  = "refs/webmux/discard-recovery/"
)

// DiscardPreviewItem describes the exact destructive effect on one path.
type DiscardPreviewItem struct {
	Path   string `json:"path"`
	Status string `json:"status"` // "M", "D", "T", "?"
	Action string `json:"action"` // "overwrite_tracked" | "restore_deleted" | "delete_untracked"
}

// DiscardPreviewRequest requests a two-phase preview for discarding unstaged or
// untracked working tree changes without mutating staged .git/index entries.
type DiscardPreviewRequest struct {
	RepositoryID   string
	WorktreeID     string
	Scope          string // "files" | "hunks" | "lines"
	Paths          []string
	Identities     []StageFileIdentity
	Identity       StageFileIdentity
	HunkIDs        []string
	LineSelections []LineSelection
}

// DiscardPreview summarizes irreversible working tree overwrites/deletions
// before two-step confirmation.
type DiscardPreview struct {
	ConfirmationID        string               `json:"confirmationId"`
	ExpiresAt             *time.Time           `json:"expiresAt"`
	RepositoryID          string               `json:"repositoryId"`
	WorktreeID            string               `json:"worktreeId"`
	WorktreePath          string               `json:"worktreePath"`
	Scope                 string               `json:"scope"`
	TrackedOverwritePaths []string             `json:"trackedOverwritePaths"`
	UntrackedDeletePaths  []string             `json:"untrackedDeletePaths"`
	Items                 []DiscardPreviewItem `json:"items"`
	HunkCount             int                  `json:"hunkCount"`
	LineCount             int                  `json:"lineCount"`
	Summary               string               `json:"summary"`
	Sessions              []session.Info       `json:"sessions"`
	Blockers              []Issue              `json:"blockers"`
	Warnings              []Issue              `json:"warnings"`
}

// ExecuteDiscardRequest confirms and executes a previewed Discard Changes action.
type ExecuteDiscardRequest struct {
	RepositoryID        string
	WorktreeID          string
	ConfirmationID      string
	ClientRequestID     string
	AcknowledgeSessions bool
}

// DiscardResult reports the completed Discard Changes operation and recovery handle.
type DiscardResult struct {
	OperationID     string   `json:"operationId,omitempty"`
	Scope           string   `json:"scope"`
	Summary         string   `json:"summary"`
	RecoveryOID     string   `json:"recoveryOid,omitempty"`
	RecoveryRef     string   `json:"recoveryRef,omitempty"`
	RecoveryCommand string   `json:"recoveryCommand,omitempty"`
	DiscardedPaths  []string `json:"discardedPaths"`
	Worktree        Worktree `json:"worktree"`
}

type discardTargetSpec struct {
	identity       StageFileIdentity
	action         string // "overwrite_tracked" | "restore_deleted" | "delete_untracked" | "partial_text"
	status         string
	hunkIDs        []string
	lineSelections []LineSelection
}

type discardPreviewState struct {
	commonDir             string
	worktreeID            string
	worktreePath          string
	scope                 string
	targets               []discardTargetSpec
	trackedOverwritePaths []string
	untrackedDeletePaths  []string
	items                 []DiscardPreviewItem
	hunkCount             int
	lineCount             int
	summary               string
	sessions              []session.Info
	blockers              []Issue
	warnings              []Issue
}
