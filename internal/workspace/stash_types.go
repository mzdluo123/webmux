package workspace

import (
	"time"

	"github.com/rainchan/webmux/internal/session"
)

const (
	StashApplyCompleted   = "completed"
	StashApplyConflicted  = "conflicted"
	StashApplyNeedsReview = "needs_review"
)

// StashEntry represents one entry in refs/stash reflog.
type StashEntry struct {
	StashRef     string   `json:"stashRef"` // e.g. "stash@{0}"
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

// StashFileChange describes one tracked or untracked file stored in a stash entry.
type StashFileChange struct {
	OldPath     string `json:"oldPath"`
	NewPath     string `json:"newPath"`
	Status      string `json:"status"` // M, A, D, R, C, T, U
	OldMode     string `json:"oldMode"`
	NewMode     string `json:"newMode"`
	OldOID      string `json:"oldOid"`
	NewOID      string `json:"newOid"`
	Binary      bool   `json:"binary"`
	ModeChanged bool   `json:"modeChanged"`
	Untracked   bool   `json:"untracked"`
}

// StashDetail contains metadata and changed files (including 3rd-parent untracked files) for one stash entry.
type StashDetail struct {
	Entry          StashEntry        `json:"entry"`
	AuthorName     string            `json:"authorName"`
	AuthorEmail    string            `json:"authorEmail"`
	TrackedCount   int               `json:"trackedCount"`
	UntrackedCount int               `json:"untrackedCount"`
	Files          []StashFileChange `json:"files"`
}

// StashCreatePreviewRequest specifies options when previewing a new stash on a worktree.
type StashCreatePreviewRequest struct {
	RepositoryID     string `json:"repositoryId"`
	WorktreeID       string `json:"worktreeId"`
	Message          string `json:"message,omitempty"`
	IncludeUntracked bool   `json:"includeUntracked"`
	KeepIndex        bool   `json:"keepIndex"`
}

// StashCreatePreview describes what creating a stash will affect on the target worktree.
type StashCreatePreview struct {
	ConfirmationID   string         `json:"confirmationId,omitempty"`
	ExpiresAt        *time.Time     `json:"expiresAt,omitempty"`
	RepositoryID     string         `json:"repositoryId"`
	WorktreeID       string         `json:"worktreeId"`
	WorktreePath     string         `json:"worktreePath"`
	CurrentBranch    string         `json:"currentBranch,omitempty"`
	Detached         bool           `json:"detached"`
	HeadOID          string         `json:"headOid,omitempty"`
	HeadSubject      string         `json:"headSubject,omitempty"`
	Message          string         `json:"message,omitempty"`
	IncludeUntracked bool           `json:"includeUntracked"`
	KeepIndex        bool           `json:"keepIndex"`
	StagedCount      int            `json:"stagedCount"`
	UnstagedCount    int            `json:"unstagedCount"`
	UntrackedCount   int            `json:"untrackedCount"`
	AffectedPaths    []string       `json:"affectedPaths"`
	Sessions         []session.Info `json:"sessions"`
	Blockers         []Issue        `json:"blockers"`
	Warnings         []Issue        `json:"warnings"`
}

type stashCreateState struct {
	commonDir        string
	worktreePath     string
	currentBranch    string
	detached         bool
	headOID          string
	headSubject      string
	message          string
	includeUntracked bool
	keepIndex        bool
	stagedCount      int
	unstagedCount    int
	untrackedCount   int
	affectedPaths    []string
	sessions         []session.Info
	blockers         []Issue
	warnings         []Issue
}

// ExecuteStashCreateRequest executes a confirmed stash push.
type ExecuteStashCreateRequest struct {
	RepositoryID        string `json:"repositoryId"`
	WorktreeID          string `json:"worktreeId"`
	ConfirmationID      string `json:"confirmationId"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
}

// StashCreateResult reports the newly created stash entry and updated worktree.
type StashCreateResult struct {
	OperationID string     `json:"operationId,omitempty"`
	Entry       StashEntry `json:"entry"`
	Worktree    Worktree   `json:"worktree"`
}

// StashApplyPreviewRequest previews applying or popping a stash onto a target worktree.
type StashApplyPreviewRequest struct {
	RepositoryID   string `json:"repositoryId"`
	WorktreeID     string `json:"worktreeId"`
	Mode           string `json:"mode"` // "apply" | "pop"
	StashRef       string `json:"stashRef"`
	StashOID       string `json:"stashOid"`
	ReinstateIndex bool   `json:"reinstateIndex"`
}

// StashApplyPreview describes target worktree cleanliness, overlap risks, and sessions.
type StashApplyPreview struct {
	ConfirmationID   string         `json:"confirmationId,omitempty"`
	ExpiresAt        *time.Time     `json:"expiresAt,omitempty"`
	RepositoryID     string         `json:"repositoryId"`
	WorktreeID       string         `json:"worktreeId"`
	WorktreePath     string         `json:"worktreePath"`
	Mode             string         `json:"mode"` // "apply" | "pop"
	Entry            StashEntry     `json:"entry"`
	ReinstateIndex   bool           `json:"reinstateIndex"`
	CurrentBranch    string         `json:"currentBranch,omitempty"`
	Detached         bool           `json:"detached"`
	HeadOID          string         `json:"headOid,omitempty"`
	HeadSubject      string         `json:"headSubject,omitempty"`
	StagedCount      int            `json:"stagedCount"`
	UnstagedCount    int            `json:"unstagedCount"`
	UntrackedCount   int            `json:"untrackedCount"`
	OverlappingPaths []string       `json:"overlappingPaths"`
	Sessions         []session.Info `json:"sessions"`
	Blockers         []Issue        `json:"blockers"`
	Warnings         []Issue        `json:"warnings"`
}

type stashApplyState struct {
	commonDir        string
	worktreePath     string
	mode             string
	stashRef         string
	stashOID         string
	entry            StashEntry
	reinstateIndex   bool
	currentBranch    string
	detached         bool
	headOID          string
	headSubject      string
	stagedCount      int
	unstagedCount    int
	untrackedCount   int
	overlappingPaths []string
	sessions         []session.Info
	blockers         []Issue
	warnings         []Issue
}

// ExecuteStashApplyRequest executes a confirmed stash apply or safe two-step pop.
type ExecuteStashApplyRequest struct {
	RepositoryID        string `json:"repositoryId"`
	WorktreeID          string `json:"worktreeId"`
	ConfirmationID      string `json:"confirmationId"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
}

// StashApplyResult reports the outcome of stash apply or safe two-step pop.
type StashApplyResult struct {
	OperationID    string              `json:"operationId,omitempty"`
	Kind           string              `json:"kind"`    // "stash_apply" | "stash_pop"
	Outcome        string              `json:"outcome"` // "completed" | "conflicted" | "needs_review"
	StashRef       string              `json:"stashRef"`
	StashOID       string              `json:"stashOid"`
	StashDropped   bool                `json:"stashDropped"`
	StashPreserved bool                `json:"stashPreserved"`
	Summary        string              `json:"summary"`
	InProgress     InProgressOperation `json:"inProgress"`
	Worktree       Worktree            `json:"worktree"`
}

// StashDropPreviewRequest previews dropping a single stash entry.
type StashDropPreviewRequest struct {
	RepositoryID string `json:"repositoryId"`
	StashRef     string `json:"stashRef"`
	StashOID     string `json:"stashOid"`
}

// StashDropPreview presents the target stash summary, underlying OID, and recovery command.
type StashDropPreview struct {
	ConfirmationID  string     `json:"confirmationId,omitempty"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	RepositoryID    string     `json:"repositoryId"`
	Entry           StashEntry `json:"entry"`
	FileCount       int        `json:"fileCount"`
	RecoveryCommand string     `json:"recoveryCommand"`
	Blockers        []Issue    `json:"blockers"`
	Warnings        []Issue    `json:"warnings"`
}

type stashDropState struct {
	stashRef        string
	stashOID        string
	entry           StashEntry
	fileCount       int
	recoveryCommand string
}

// ExecuteStashDropRequest executes a confirmed single-entry stash drop.
type ExecuteStashDropRequest struct {
	RepositoryID    string `json:"repositoryId"`
	ConfirmationID  string `json:"confirmationId"`
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// StashDropResult reports the dropped stash OID and pre-GC recovery command.
type StashDropResult struct {
	OperationID     string       `json:"operationId,omitempty"`
	StashRef        string       `json:"stashRef"`
	StashOID        string       `json:"stashOid"`
	RecoveryCommand string       `json:"recoveryCommand"`
	Stashes         []StashEntry `json:"stashes"`
}
