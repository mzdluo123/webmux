package workspace

import (
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/rainchan/webmux/internal/pathutil"
	"github.com/rainchan/webmux/internal/statefile"
)

const (
	gitOperationsVersion = 1

	GitOpCommit         = "commit"
	GitOpPush           = "push"
	GitOpFetch          = "fetch"
	GitOpFastForward    = "fast_forward"
	GitOpSwitch         = "switch"
	GitOpMerge          = "merge"
	GitOpRebase         = "rebase"
	GitOpCherryPick     = "cherry_pick"
	GitOpRevert         = "revert"
	GitOpContinue       = "continue"
	GitOpAbort          = "abort"
	GitOpStashPush      = "stash_push"
	GitOpStashApply     = "stash_apply"
	GitOpStashPop       = "stash_pop"
	GitOpStashDrop      = "stash_drop"
	GitOpBranchRename   = "branch_rename"
	GitOpBranchUpstream = "branch_upstream"
	GitOpTagCreate      = "tag_create"
	GitOpTagDelete      = "tag_delete"
	GitOpRemoteAdd      = "remote_add"
	GitOpRemoteUpdate   = "remote_update"
	GitOpRemoteRemove   = "remote_remove"
	GitOpDiscard        = "discard"
	GitOpSmartCheckout  = "smart_checkout"

	GitOpPending     = "pending"
	GitOpRunning     = "running"
	GitOpSucceeded   = "succeeded"
	GitOpNeedsReview = "needs_review"
	GitOpFailed      = "failed"
	GitOpUncertain   = "uncertain"

	MaxOperationsPerRepo       = 100
	MaxOperationsGlobal        = 1000
	MaxOperationOutputBytes    = 4096
	MaxClientRequestIDLen      = 128
	OperationTTL               = 7 * 24 * time.Hour
	DefaultOperationsListLimit = 20
	MaxOperationsListLimit     = 100
)

// GitOperationPreview stores the bounded snapshot bound when a write operation starts.
type GitOperationPreview struct {
	WorktreePath    string   `json:"worktreePath,omitempty"`
	HeadState       string   `json:"headState,omitempty"`
	Branch          string   `json:"branch,omitempty"`
	HeadOID         string   `json:"headOid,omitempty"`
	ExpectedTreeOID string   `json:"expectedTreeOid,omitempty"`
	ExpectedParents []string `json:"expectedParents,omitempty"`
	Amend           bool     `json:"amend,omitempty"`
	AuthorOverride  string   `json:"authorOverride,omitempty"`
	Signoff         bool     `json:"signoff,omitempty"`
	StashRef        string   `json:"stashRef,omitempty"`
	StashOID        string   `json:"stashOid,omitempty"`
	Remote          string   `json:"remote,omitempty"`
	Target          string   `json:"target,omitempty"`
	TargetOID       string   `json:"targetOid,omitempty"`
	Publish         bool     `json:"publish,omitempty"`
	Detached        bool     `json:"detached,omitempty"`
}

// GitOperation is one durable entry in git-operations.json.
type GitOperation struct {
	ID              string              `json:"id"`
	RepositoryID    string              `json:"repositoryId"`
	WorktreeID      string              `json:"worktreeId,omitempty"`
	ClientRequestID string              `json:"clientRequestId,omitempty"`
	RequestHash     string              `json:"requestHash,omitempty"`
	Kind            string              `json:"kind"`
	Status          string              `json:"status"`
	Phase           string              `json:"phase,omitempty"`
	ConfirmationID  string              `json:"confirmationId,omitempty"`
	StartedAt       time.Time           `json:"startedAt"`
	FinishedAt      *time.Time          `json:"finishedAt,omitempty"`
	Summary         string              `json:"summary"`
	SanitizedOutput string              `json:"sanitizedOutput,omitempty"`
	FailureKind     string              `json:"failureKind,omitempty"`
	StashOID        string              `json:"stashOid,omitempty"`
	RecoveryOID     string              `json:"recoveryOid,omitempty"`
	RecoveryCommand string              `json:"recoveryCommand,omitempty"`
	RemoteStatus    string              `json:"remoteStatus,omitempty"`
	FetchResults    []FetchResult       `json:"fetchResults,omitempty"`
	Preview         GitOperationPreview `json:"preview"`
	ActualOID       string              `json:"actualOid,omitempty"`
	ActualTreeOID   string              `json:"actualTreeOid,omitempty"`
	Parents         []string            `json:"parents,omitempty"`
	Subject         string              `json:"subject,omitempty"`
	AuthorName      string              `json:"authorName,omitempty"`
	AuthorEmail     string              `json:"authorEmail,omitempty"`
	CommitterName   string              `json:"committerName,omitempty"`
	CommitterEmail  string              `json:"committerEmail,omitempty"`
	ReviewReasons   []string            `json:"reviewReasons,omitempty"`
}

type gitOperationLedger struct {
	V          int            `json:"v"`
	Operations []GitOperation `json:"operations"`
}

type opFlight struct {
	done        chan struct{}
	opID        string
	requestHash string
	kind        string
	result      any
	err         error
}

func (s *Service) loadOperations() error {
	expanded, err := pathutil.Expand(s.stateDir)
	if err != nil {
		return err
	}
	s.stateDir = expanded
	var stored gitOperationLedger
	s.opsStore, stored, err = statefile.Open[gitOperationLedger](s.stateDir, statefile.GitOperations, validateOperationLedger)
	if statefile.IsKind(err, statefile.Missing) {
		return nil
	}
	if err != nil {
		return err
	}
	s.operations = cloneGitOperations(stored.Operations)
	return nil
}

func (s *Service) closeOperations() {
	s.opsMu.Lock()
	defer s.opsMu.Unlock()
	if s.opsStore != nil {
		s.opsStore.Close()
	}
	s.opsAvailable = false
	s.opsError = "Git 操作账本已关闭"
}

func cloneGitOperation(op GitOperation) GitOperation {
	if op.FetchResults != nil {
		op.FetchResults = append([]FetchResult(nil), op.FetchResults...)
	}
	if op.Preview.ExpectedParents != nil {
		op.Preview.ExpectedParents = append([]string(nil), op.Preview.ExpectedParents...)
	}
	if op.Parents != nil {
		op.Parents = append([]string(nil), op.Parents...)
	}
	if op.ReviewReasons != nil {
		op.ReviewReasons = append([]string(nil), op.ReviewReasons...)
	}
	if op.FinishedAt != nil {
		t := *op.FinishedAt
		op.FinishedAt = &t
	}
	return op
}

func cloneGitOperations(list []GitOperation) []GitOperation {
	out := make([]GitOperation, len(list))
	for i, op := range list {
		out[i] = cloneGitOperation(op)
	}
	return out
}

func isTerminalOpStatus(status string) bool {
	return status != GitOpPending && status != GitOpRunning
}

// pruneOperations enforces TTL, per-repository, global, and byte-size budgets.
// Entries in list are ordered oldest to newest; non-terminal entries are kept.
func (s *Service) pruneOperations(list []GitOperation, now time.Time) []GitOperation {
	cutoff := now.Add(-OperationTTL)
	kept := make([]GitOperation, 0, len(list))
	for _, op := range list {
		if isTerminalOpStatus(op.Status) && op.StartedAt.Before(cutoff) {
			continue
		}
		kept = append(kept, op)
	}
	repoCounts := make(map[string]int)
	revKept := make([]GitOperation, 0, len(kept))
	for i := len(kept) - 1; i >= 0; i-- {
		op := kept[i]
		if !isTerminalOpStatus(op.Status) {
			revKept = append(revKept, op)
			continue
		}
		if repoCounts[op.RepositoryID] >= MaxOperationsPerRepo {
			continue
		}
		repoCounts[op.RepositoryID]++
		revKept = append(revKept, op)
	}
	slices.Reverse(revKept)
	for len(revKept) > MaxOperationsGlobal {
		dropped := false
		for i, op := range revKept {
			if isTerminalOpStatus(op.Status) {
				revKept = append(revKept[:i], revKept[i+1:]...)
				dropped = true
				break
			}
		}
		if !dropped {
			break
		}
	}
	if s.opsStore != nil {
		for !s.opsStore.Fits(gitOperationLedger{V: gitOperationsVersion, Operations: revKept}) {
			dropped := false
			for i, op := range revKept {
				if isTerminalOpStatus(op.Status) {
					revKept = append(revKept[:i], revKept[i+1:]...)
					dropped = true
					break
				}
			}
			if !dropped {
				break
			}
		}
	}
	return revKept
}

func (s *Service) commitOperationsLocked(next []GitOperation) error {
	next = s.pruneOperations(next, time.Now().UTC())
	err := s.opsStore.Commit(gitOperationLedger{V: gitOperationsVersion, Operations: next})
	if statefile.IsKind(err, statefile.CommitUncertain) {
		message := "Git 操作账本已替换但目录同步失败，持久结果不确定；操作记录已停用，请重启重新读取: " + err.Error()
		s.opsMu.Lock()
		s.opsAvailable = false
		s.opsError = message
		s.opsMu.Unlock()
		return errors.New(message)
	}
	if err != nil {
		return err
	}
	s.opsMu.Lock()
	s.operations = next
	s.opsMu.Unlock()
	return nil
}

func (s *Service) recordOperationStart(op GitOperation) error {
	op.SanitizedOutput = sanitizeOperationOutput(op.SanitizedOutput)
	s.opsSaveMu.Lock()
	defer s.opsSaveMu.Unlock()
	s.opsMu.Lock()
	if !s.opsAvailable {
		errText := s.opsError
		s.opsMu.Unlock()
		if op.ClientRequestID != "" {
			return problemf(ErrUnavailable, "%s", errText)
		}
		return nil
	}
	next := append(cloneGitOperations(s.operations), cloneGitOperation(op))
	s.opsMu.Unlock()
	if err := s.commitOperationsLocked(next); err != nil {
		if op.ClientRequestID != "" {
			return problemf(ErrUnavailable, "保存 Git 操作记录失败: %v", err)
		}
		slog.Warn("failed to record git operation start", "operation", op.ID, "kind", op.Kind, "err", err)
	}
	return nil
}

func (s *Service) finalizeOperation(id string, update func(*GitOperation)) {
	s.opsSaveMu.Lock()
	defer s.opsSaveMu.Unlock()
	s.opsMu.Lock()
	if !s.opsAvailable {
		s.opsMu.Unlock()
		return
	}
	next := cloneGitOperations(s.operations)
	s.opsMu.Unlock()
	idx := -1
	for i := len(next) - 1; i >= 0; i-- {
		if next[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	update(&next[idx])
	next[idx].SanitizedOutput = sanitizeOperationOutput(next[idx].SanitizedOutput)
	if next[idx].FinishedAt == nil && isTerminalOpStatus(next[idx].Status) {
		now := time.Now().UTC()
		next[idx].FinishedAt = &now
	}
	if err := s.commitOperationsLocked(next); err != nil {
		slog.Warn("failed to finalize git operation", "operation", id, "err", err)
	}
}

func (s *Service) purgeRepositoryOperations(repositoryID string) {
	s.opsSaveMu.Lock()
	defer s.opsSaveMu.Unlock()
	s.opsMu.Lock()
	if !s.opsAvailable {
		s.opsMu.Unlock()
		return
	}
	hasAny := false
	next := make([]GitOperation, 0, len(s.operations))
	for _, op := range s.operations {
		if op.RepositoryID == repositoryID {
			hasAny = true
			continue
		}
		next = append(next, cloneGitOperation(op))
	}
	s.opsMu.Unlock()
	if !hasAny {
		return
	}
	if err := s.commitOperationsLocked(next); err != nil {
		slog.Warn("failed to purge repository git operations", "repository", repositoryID, "err", err)
	}
}

// ListGitOperations lists bounded durable operations for a registered repository,
// newest first, optionally filtered by worktreeID.
func (s *Service) ListGitOperations(repositoryID, worktreeID string, limit int) ([]GitOperation, error) {
	if _, err := s.repository(repositoryID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = DefaultOperationsListLimit
	} else if limit > MaxOperationsListLimit {
		limit = MaxOperationsListLimit
	}
	s.opsMu.Lock()
	defer s.opsMu.Unlock()
	if !s.opsAvailable {
		return nil, problemf(ErrUnavailable, "%s", s.opsError)
	}
	out := make([]GitOperation, 0, min(limit, len(s.operations)))
	for i := len(s.operations) - 1; i >= 0 && len(out) < limit; i-- {
		op := s.operations[i]
		if op.RepositoryID != repositoryID {
			continue
		}
		if worktreeID != "" && op.WorktreeID != worktreeID {
			continue
		}
		out = append(out, cloneGitOperation(op))
	}
	return out, nil
}

// GetGitOperation returns a single durable operation of a registered repository.
func (s *Service) GetGitOperation(repositoryID, operationID string) (GitOperation, error) {
	if _, err := s.repository(repositoryID); err != nil {
		return GitOperation{}, err
	}
	if operationID == "" {
		return GitOperation{}, problemf(ErrInvalid, "缺少 operationId")
	}
	s.opsMu.Lock()
	defer s.opsMu.Unlock()
	if !s.opsAvailable {
		return GitOperation{}, problemf(ErrUnavailable, "%s", s.opsError)
	}
	for i := len(s.operations) - 1; i >= 0; i-- {
		if s.operations[i].RepositoryID == repositoryID && s.operations[i].ID == operationID {
			return cloneGitOperation(s.operations[i]), nil
		}
	}
	return GitOperation{}, problemf(ErrNotFound, "Git 操作记录不存在")
}
