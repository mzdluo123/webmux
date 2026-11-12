package workspace

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/pathutil"
	"github.com/rainchan/webmux/internal/statefile"
)

const (
	gitOperationsVersion = 1

	GitOpCommit      = "commit"
	GitOpPush        = "push"
	GitOpFetch       = "fetch"
	GitOpFastForward = "fast_forward"
	GitOpSwitch      = "switch"

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
	WorktreePath    string `json:"worktreePath,omitempty"`
	HeadState       string `json:"headState,omitempty"`
	Branch          string `json:"branch,omitempty"`
	HeadOID         string `json:"headOid,omitempty"`
	ExpectedTreeOID string `json:"expectedTreeOid,omitempty"`
	Remote          string `json:"remote,omitempty"`
	Target          string `json:"target,omitempty"`
	TargetOID       string `json:"targetOid,omitempty"`
	Publish         bool   `json:"publish,omitempty"`
	Detached        bool   `json:"detached,omitempty"`
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
	ConfirmationID  string              `json:"confirmationId,omitempty"`
	StartedAt       time.Time           `json:"startedAt"`
	FinishedAt      *time.Time          `json:"finishedAt,omitempty"`
	Summary         string              `json:"summary"`
	SanitizedOutput string              `json:"sanitizedOutput,omitempty"`
	FailureKind     string              `json:"failureKind,omitempty"`
	RemoteStatus    string              `json:"remoteStatus,omitempty"`
	FetchResults    []FetchResult       `json:"fetchResults,omitempty"`
	Preview         GitOperationPreview `json:"preview"`
	ActualOID       string              `json:"actualOid,omitempty"`
	ActualTreeOID   string              `json:"actualTreeOid,omitempty"`
	Parents         []string            `json:"parents,omitempty"`
	Subject         string              `json:"subject,omitempty"`
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

var (
	authHeaderPattern = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*(?:bearer|basic|token)?\s*)[^\s'"]+`)
	extraHdrPattern   = regexp.MustCompile(`(?i)(extraheader\s*[:=]\s*)[^\r\n]+`)
	knownTokenPattern = regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9_]{16,}|github_pat_[A-Za-z0-9_]{16,}|glpat-[A-Za-z0-9_-]{12,})\b`)
)

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

func validateOperationLedger(stored gitOperationLedger) error {
	if stored.V != gitOperationsVersion || stored.Operations == nil {
		return errors.New("不支持或损坏的 Git 操作账本")
	}
	if len(stored.Operations) > MaxOperationsGlobal {
		return fmt.Errorf("操作账本超过 %d 条记录", MaxOperationsGlobal)
	}
	ids := make(map[string]bool, len(stored.Operations))
	reqKeys := make(map[string]bool, len(stored.Operations))
	for _, op := range stored.Operations {
		if op.ID == "" || op.RepositoryID == "" || ids[op.ID] {
			return errors.New("操作记录身份无效或重复")
		}
		if !validOpKind(op.Kind) || !validOpStatus(op.Status) {
			return errors.New("操作记录类型或状态无效")
		}
		if len(op.SanitizedOutput) > MaxOperationOutputBytes {
			return errors.New("操作记录输出摘要超出上限")
		}
		ids[op.ID] = true
		if op.ClientRequestID != "" {
			if err := validateClientRequestID(op.ClientRequestID); err != nil {
				return errors.New("操作记录 clientRequestId 无效")
			}
			key := op.RepositoryID + "\x00" + op.ClientRequestID
			if reqKeys[key] {
				return errors.New("操作记录 clientRequestId 重复")
			}
			reqKeys[key] = true
		}
	}
	return nil
}

func validOpKind(kind string) bool {
	switch kind {
	case GitOpCommit, GitOpPush, GitOpFetch, GitOpFastForward, GitOpSwitch:
		return true
	}
	return false
}

func validOpStatus(status string) bool {
	switch status {
	case GitOpPending, GitOpRunning, GitOpSucceeded, GitOpNeedsReview, GitOpFailed, GitOpUncertain:
		return true
	}
	return false
}

func validateClientRequestID(id string) error {
	if id == "" {
		return nil
	}
	if len(id) > MaxClientRequestIDLen {
		return problemf(ErrInvalid, "clientRequestId 长度不能超过 %d 个字符", MaxClientRequestIDLen)
	}
	for i := 0; i < len(id); i++ {
		if id[i] < 0x21 || id[i] > 0x7e {
			return problemf(ErrInvalid, "clientRequestId 包含非法字符")
		}
	}
	return nil
}

func hashOperationRequest(parts ...string) string {
	h := sha256.New()
	var buf [8]byte
	for _, p := range parts {
		binary.BigEndian.PutUint64(buf[:], uint64(len(p)))
		_, _ = h.Write(buf[:])
		_, _ = h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sanitizeOperationOutput(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	rawLines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimRight(line, "\r")
		if idx := strings.LastIndexByte(line, '\r'); idx >= 0 {
			line = line[idx+1:]
		}
		line = redactLine(line)
		line = authHeaderPattern.ReplaceAllString(line, "${1}"+redactedSecret)
		line = extraHdrPattern.ReplaceAllString(line, "${1}"+redactedSecret)
		line = knownTokenPattern.ReplaceAllString(line, redactedSecret)
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	out := strings.TrimSpace(strings.Join(cleaned, "\n"))
	if len(out) <= MaxOperationOutputBytes {
		return out
	}
	const ellipsis = "…"
	limit := MaxOperationOutputBytes - len(ellipsis)
	cut := strings.ToValidUTF8(out[:limit], "")
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut + ellipsis
}

func cloneGitOperation(op GitOperation) GitOperation {
	if op.FetchResults != nil {
		op.FetchResults = append([]FetchResult(nil), op.FetchResults...)
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
