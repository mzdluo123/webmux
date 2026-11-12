package workspace

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// PutTaskBaselineRequest specifies an explicit task baseline commit or defaults
// to the worktree's current HEAD (or Unborn=true if HEAD is unborn).
type PutTaskBaselineRequest struct {
	BaseOID string
}

// TaskScopeSummary reports the task baseline and live commit/uncommitted counts
// for one worktree without mutating Git state.
type TaskScopeSummary struct {
	Status           string
	Reason           string
	Baseline         *TaskBaseline
	HeadOID          string
	HeadUnborn       bool
	CommitCount      int
	UncommittedCount int
}

const (
	TaskScopeReady          = "ready"
	TaskScopeNoTaskBaseline = "no_task_baseline"
)

func newTaskBaselineID() (string, error) {
	raw, err := randomID()
	if err != nil {
		return "", err
	}
	return "tb_" + raw[:24], nil
}

// liveWorktreeStatusChanges reads porcelain v2 status for path, returning
// per-area change counts and the distinct count of uncommitted paths.
func (s *Service) liveWorktreeStatusChanges(ctx context.Context, path string) (WorktreeChanges, int, error) {
	data, err := s.git(ctx, path, "status", "--porcelain=v2", "-z", "--untracked-files=normal")
	if err != nil {
		return WorktreeChanges{}, 0, err
	}
	parsed, err := parseStatus(data)
	if err != nil {
		return WorktreeChanges{}, 0, err
	}
	distinct := countStatusEntries(data)
	return parsed.changes, distinct, nil
}

func countStatusEntries(data []byte) int {
	if len(data) == 0 || data[len(data)-1] != 0 {
		return 0
	}
	count := 0
	records := bytes.Split(data[:len(data)-1], []byte{0})
	for i := 0; i < len(records); i++ {
		rec := records[i]
		if len(rec) < 2 {
			continue
		}
		switch rec[0] {
		case '1', 'u', '?':
			count++
		case '2':
			count++
			i++
		}
	}
	return count
}

func (s *Service) writeOrClearBaselineRef(ctx context.Context, anchorPath, worktreeID, baseOID string, unborn bool) error {
	if !validBaselineRefID(worktreeID) {
		return problemf(ErrInvalid, "工作树标识无效")
	}
	if unborn {
		s.dropBaselineRef(ctx, anchorPath, worktreeID)
		return nil
	}
	if !validNonZeroObjectID(baseOID) {
		return problemf(ErrInvalid, "任务基线提交 OID 无效")
	}
	if _, err := s.git(ctx, anchorPath, "update-ref", baselineRef(worktreeID), baseOID); err != nil {
		return fmt.Errorf("写入任务基线保护引用: %w", err)
	}
	return nil
}

// captureWorktreeCreateBaseline records the fixed task baseline immediately
// after git worktree add completes and before any session/agent starts.
func (s *Service) captureWorktreeCreateBaseline(ctx context.Context, r *repositoryState, repositoryID string, tree Worktree) {
	if err := s.requireTaskBaselines(); err != nil {
		slog.Warn("task baseline store unavailable on worktree create", "worktree", tree.ID, "error", err)
		return
	}
	if tree.Kind == "bare" || !tree.Available {
		return
	}
	id, err := newTaskBaselineID()
	if err != nil {
		slog.Warn("task baseline id generation failed", "worktree", tree.ID, "error", err)
		return
	}
	baseOID := s.headCommit(ctx, r, tree)
	unborn := baseOID == ""
	var changes WorktreeChanges
	if tree.Changes != nil {
		changes = *tree.Changes
	} else if live, _, sErr := s.liveWorktreeStatusChanges(ctx, tree.Path); sErr == nil {
		changes = live
	}
	if err := s.writeOrClearBaselineRef(ctx, r.entry.AnchorPath, tree.ID, baseOID, unborn); err != nil {
		slog.Warn("task baseline ref write failed on worktree create", "worktree", tree.ID, "error", err)
		return
	}
	entry := TaskBaseline{
		BaselineID:         id,
		RepositoryID:       repositoryID,
		WorktreeID:         tree.ID,
		WorktreePath:       tree.Path,
		BaseOID:            baseOID,
		Unborn:             unborn,
		CapturedAt:         time.Now().UTC(),
		Source:             TaskBaselineWorktreeCreate,
		CleanAtStart:       changes == (WorktreeChanges{}),
		PreExistingChanges: changes,
	}
	if err := s.upsertTaskBaselineEntry(entry); err != nil {
		slog.Warn("task baseline save failed on worktree create", "worktree", tree.ID, "error", err)
	}
}

// ensureUnbornWorktreeBaseline captures an unborn task baseline when a session
// starts in an unborn worktree that has no baseline yet.
func (s *Service) ensureUnbornWorktreeBaseline(ctx context.Context, r *repositoryState, repositoryID string, tree Worktree) {
	if tree.Head != nil || tree.Kind == "bare" || !tree.Available {
		return
	}
	if s.findTaskBaseline(repositoryID, tree.ID) != nil {
		return
	}
	s.captureWorktreeCreateBaseline(ctx, r, repositoryID, tree)
}

func (s *Service) ensureUnbornAssociationBaseline(ctx context.Context, repositoryID, worktreeID, worktreePath string) {
	if repositoryID == "" || worktreeID == "" || worktreePath == "" {
		return
	}
	if s.findTaskBaseline(repositoryID, worktreeID) != nil {
		return
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return
	}
	if _, revErr := s.git(ctx, worktreePath, "rev-parse", "--verify", "--end-of-options", "HEAD^{commit}"); revErr == nil {
		return
	}
	s.captureWorktreeCreateBaseline(ctx, r, repositoryID, Worktree{
		ID:           worktreeID,
		RepositoryID: repositoryID,
		Path:         worktreePath,
		Kind:         "worktree",
		Available:    true,
	})
}

// GetTaskBaseline returns the persisted task baseline of worktreeID, or nil
// when the worktree exists and has no active baseline.
func (s *Service) GetTaskBaseline(ctx context.Context, repositoryID, worktreeID string) (*TaskBaseline, error) {
	if err := s.requireTaskBaselines(); err != nil {
		return nil, err
	}
	loc, err := s.LocateWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return nil, err
	}
	if loc.Kind == "bare" {
		return nil, problemf(ErrConflict, "bare 工作树不支持任务审阅基线")
	}
	if !loc.Available {
		return nil, problemf(ErrConflict, "%s", loc.Reason)
	}
	return s.findTaskBaseline(repositoryID, worktreeID), nil
}

// PutTaskBaseline explicitly sets or resets worktreeID's task baseline to
// req.BaseOID (or the worktree's current HEAD / unborn state when empty).
func (s *Service) PutTaskBaseline(ctx context.Context, repositoryID, worktreeID string, req PutTaskBaselineRequest) (TaskBaseline, error) {
	if err := s.requireTaskBaselines(); err != nil {
		return TaskBaseline{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	r, err := s.repository(repositoryID)
	if err != nil {
		return TaskBaseline{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return TaskBaseline{}, err
	}
	defer release(r)
	repository := s.readRepository(ctx, r)
	if repository.Availability != "available" {
		return TaskBaseline{}, problemf(ErrUnavailable, "%s", repository.Error)
	}
	tree, err := findTree(repository, worktreeID)
	if err != nil {
		return TaskBaseline{}, err
	}
	if tree.Kind == "bare" {
		return TaskBaseline{}, problemf(ErrConflict, "bare 工作树不支持任务审阅基线")
	}
	if !tree.Available || tree.Prunable {
		return TaskBaseline{}, problemf(ErrConflict, "工作树目录不可用，无法设定任务审阅基线")
	}
	rawOID := strings.TrimSpace(req.BaseOID)
	var baseOID string
	var unborn bool
	if rawOID != "" {
		if strings.HasPrefix(rawOID, "-") || strings.ContainsRune(rawOID, '\x00') || !validNonZeroObjectID(rawOID) {
			return TaskBaseline{}, problemf(ErrInvalid, "baseOid 必须是有效的完整提交十六进制 OID")
		}
		out, vErr := s.git(ctx, tree.Path, "rev-parse", "--verify", "--end-of-options", rawOID+"^{commit}")
		if vErr != nil {
			if errorKind(vErr) == ErrTimeout || errorKind(vErr) == ErrTooLarge {
				return TaskBaseline{}, vErr
			}
			return TaskBaseline{}, problemf(ErrNotFound, "指定的任务起点提交不存在")
		}
		resolved := outputLine(out)
		if resolved != rawOID {
			return TaskBaseline{}, problemf(ErrNotFound, "指定的任务起点提交不存在")
		}
		baseOID = resolved
	} else {
		baseOID = s.headCommit(ctx, r, tree)
		unborn = baseOID == ""
	}
	var changes WorktreeChanges
	if tree.Changes != nil {
		changes = *tree.Changes
	} else {
		live, _, sErr := s.liveWorktreeStatusChanges(ctx, tree.Path)
		if sErr != nil {
			return TaskBaseline{}, sErr
		}
		changes = live
	}
	id, err := newTaskBaselineID()
	if err != nil {
		return TaskBaseline{}, err
	}
	if err := s.writeOrClearBaselineRef(ctx, r.entry.AnchorPath, tree.ID, baseOID, unborn); err != nil {
		return TaskBaseline{}, err
	}
	entry := TaskBaseline{
		BaselineID:         id,
		RepositoryID:       repositoryID,
		WorktreeID:         tree.ID,
		WorktreePath:       tree.Path,
		BaseOID:            baseOID,
		Unborn:             unborn,
		CapturedAt:         time.Now().UTC(),
		Source:             TaskBaselineManualSelect,
		CleanAtStart:       changes == (WorktreeChanges{}),
		PreExistingChanges: changes,
	}
	if err := s.upsertTaskBaselineEntry(entry); err != nil {
		return TaskBaseline{}, err
	}
	return entry, nil
}

// DeleteTaskBaseline removes worktreeID's task baseline entry and deletes its
// private refs/webmux/baselines/<worktreeId> reference.
func (s *Service) DeleteTaskBaseline(ctx context.Context, repositoryID, worktreeID string) error {
	if err := s.requireTaskBaselines(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	r, err := s.repository(repositoryID)
	if err != nil {
		return err
	}
	if err := acquire(ctx, r); err != nil {
		return err
	}
	defer release(r)
	repository, err := s.current(ctx, r)
	if err != nil {
		return err
	}
	if _, findErr := findTree(repository, worktreeID); findErr != nil && s.findTaskBaseline(repositoryID, worktreeID) == nil {
		return findErr
	}
	s.dropBaselineRef(ctx, r.entry.AnchorPath, worktreeID)
	_, err = s.removeTaskBaselineEntry(repositoryID, worktreeID)
	return err
}

// ResolveTaskScope computes the task baseline, live HEAD state, task commit
// count (baseOid..HEAD or root..HEAD for unborn baselines), and live distinct
// uncommitted file count for worktreeID.
func (s *Service) ResolveTaskScope(ctx context.Context, repositoryID, worktreeID string) (TaskScopeSummary, error) {
	if err := s.requireTaskBaselines(); err != nil {
		return TaskScopeSummary{}, err
	}
	loc, err := s.LocateWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return TaskScopeSummary{}, err
	}
	if loc.Kind == "bare" {
		return TaskScopeSummary{}, problemf(ErrConflict, "bare 工作树不支持任务范围审阅")
	}
	if !loc.Available {
		return TaskScopeSummary{}, problemf(ErrConflict, "%s", loc.Reason)
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	_, uncommittedCount, err := s.liveWorktreeStatusChanges(ctx, loc.Path)
	if err != nil {
		return TaskScopeSummary{}, err
	}
	var headOID string
	if out, hErr := s.git(ctx, loc.Path, "rev-parse", "--verify", "--end-of-options", "HEAD^{commit}"); hErr == nil {
		cand := outputLine(out)
		if validNonZeroObjectID(cand) {
			headOID = cand
		}
	}
	headUnborn := headOID == ""
	baseline := s.findTaskBaseline(repositoryID, worktreeID)
	if baseline == nil {
		return TaskScopeSummary{
			Status:           TaskScopeNoTaskBaseline,
			Reason:           "当前工作树尚未设定任务起点基线，可将当前 HEAD 或历史提交设为任务起点。",
			HeadOID:          headOID,
			HeadUnborn:       headUnborn,
			UncommittedCount: uncommittedCount,
		}, nil
	}
	commitCount := 0
	if !headUnborn {
		var revSpec string
		if baseline.Unborn {
			revSpec = headOID
		} else if baseline.BaseOID != headOID {
			revSpec = baseline.BaseOID + ".." + headOID
		}
		if revSpec != "" {
			out, cErr := s.git(ctx, loc.Path, "rev-list", "--count", "--end-of-options", revSpec)
			if cErr != nil {
				return TaskScopeSummary{}, cErr
			}
			if n, pErr := strconv.Atoi(strings.TrimSpace(string(out))); pErr == nil && n >= 0 {
				commitCount = n
			}
		}
	}
	return TaskScopeSummary{
		Status:           TaskScopeReady,
		Baseline:         baseline,
		HeadOID:          headOID,
		HeadUnborn:       headUnborn,
		CommitCount:      commitCount,
		UncommittedCount: uncommittedCount,
	}, nil
}
