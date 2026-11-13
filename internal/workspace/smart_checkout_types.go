package workspace

import (
	"context"
	"time"

	"github.com/rainchan/webmux/internal/session"
)

const (
	SmartCheckoutPhaseStashing   = "stashing"
	SmartCheckoutPhaseSwitching  = "switching"
	SmartCheckoutPhaseUnstashing = "unstashing"
	SmartCheckoutPhaseCompleted  = "completed"
	SmartCheckoutPhaseRolledBack = "rolled_back"
	SmartCheckoutPhaseConflict   = "conflict_needs_resolution"

	SmartCheckoutCompleted  = "completed"
	SmartCheckoutRolledBack = "rolled_back"
	SmartCheckoutConflict   = "conflict_needs_resolution"

	smartCheckoutRefPrefix = "refs/webmux/smart-checkout/"
)

// SmartCheckoutPreviewRequest requests a preview for 3-phase tracked-stash
// Smart Checkout when local changes block plain branch switching.
type SmartCheckoutPreviewRequest struct {
	RepositoryID     string
	WorktreeID       string
	Target           SwitchTarget
	IncludeUntracked *bool
}

// SmartCheckoutPreview describes the 3-phase Smart Checkout plan, dirty file
// counts, active CLI agent sessions, blockers, and confirmation token.
type SmartCheckoutPreview struct {
	ConfirmationID   string
	ExpiresAt        *time.Time
	RepositoryID     string
	WorktreeID       string
	Worktree         Worktree
	Target           SwitchTargetInfo
	IncludeUntracked bool
	StagedCount      int
	UnstagedCount    int
	UntrackedCount   int
	DirtyPaths       []string
	HasActiveAgent   bool
	Sessions         []session.Info
	Blockers         []Issue
	Warnings         []Issue
}

// ExecuteSmartCheckoutRequest confirms and runs the 3-phase Smart Checkout.
type ExecuteSmartCheckoutRequest struct {
	RepositoryID        string
	WorktreeID          string
	ConfirmationID      string
	ClientRequestID     string
	AcknowledgeSessions bool
}

// SmartCheckoutResult reports the final phase and state of a Smart Checkout.
type SmartCheckoutResult struct {
	OperationID     string
	Outcome         string // "completed" | "rolled_back" | "conflict_needs_resolution"
	Phase           string
	Summary         string
	ErrorMessage    string
	StashOID        string
	StashRef        string
	StashDropped    bool
	StashPreserved  bool
	StashRestored   bool
	RecoveryCommand string
	InProgress      InProgressOperation
	Worktree        Worktree
}

type smartCheckoutState struct {
	commonDir        string
	worktreeID       string
	tree             Worktree
	plan             switchPlan
	includeUntracked bool
	stagedCount      int
	unstagedCount    int
	untrackedCount   int
	dirtyPaths       []string
	dirtyDigest      string
	hasActiveAgent   bool
	sessions         []session.Info
	blockers         []Issue
	warnings         []Issue
}

type smartCheckoutFaultHooks struct {
	afterStash    func(opID, stashOID string) error
	failSwitch    func(opID, stashOID string) error
	afterSwitch   func(opID, stashOID string) error
	beforeUnstash func(opID, stashOID string) error
	afterUnstash  func(opID, stashOID string) error
}

type smartCheckoutFaultHookKey struct{}

func getSmartCheckoutHooks(ctx context.Context) smartCheckoutFaultHooks {
	if h, ok := ctx.Value(smartCheckoutFaultHookKey{}).(smartCheckoutFaultHooks); ok {
		return h
	}
	return smartCheckoutFaultHooks{}
}
