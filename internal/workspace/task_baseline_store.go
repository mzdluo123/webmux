package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/pathutil"
	"github.com/rainchan/webmux/internal/statefile"
)

const (
	gitTaskBaselinesVersion = 1
	MaxTaskBaselines        = 2000

	// baselineRefPrefix holds private refs that protect task baseline commits
	// from git gc when an agent or user amends/rebases; Git neither fetches nor
	// pushes them by default, and branch/tag enumeration skips refs/webmux/.
	baselineRefPrefix = "refs/webmux/baselines/"

	TaskBaselineWorktreeCreate = "worktree_create"
	TaskBaselineManualSelect   = "manual_select"
)

// TaskBaseline records the fixed review origin of one worktree. Once captured,
// BaseOID (or Unborn=true for an empty repository) stays fixed across git
// fetch, upstream advances, commits, amends and rebases until the worktree is
// removed or the user explicitly resets/clears the baseline.
type TaskBaseline struct {
	BaselineID         string          `json:"baselineId"`
	RepositoryID       string          `json:"repositoryId"`
	WorktreeID         string          `json:"worktreeId"`
	WorktreePath       string          `json:"worktreePath"`
	BaseOID            string          `json:"baseOid,omitempty"`
	Unborn             bool            `json:"unborn,omitempty"`
	CapturedAt         time.Time       `json:"capturedAt"`
	Source             string          `json:"source"`
	CleanAtStart       bool            `json:"cleanAtStart"`
	PreExistingChanges WorktreeChanges `json:"preExistingChanges"`
}

type taskBaselineStore struct {
	V         int            `json:"v"`
	Baselines []TaskBaseline `json:"baselines"`
}

func baselineRef(worktreeID string) string {
	return baselineRefPrefix + worktreeID
}

func validBaselineRefID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for i := 0; i < len(id); i++ {
		ch := id[i]
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && ch != '-' && ch != '_' {
			return false
		}
	}
	return true
}

func validNonZeroObjectID(oid string) bool {
	return isObjectID(oid) && strings.Trim(oid, "0") != ""
}

func validBaselineSource(source string) bool {
	return source == TaskBaselineWorktreeCreate || source == TaskBaselineManualSelect
}

func validateTaskBaselineStore(stored taskBaselineStore) error {
	if stored.V != gitTaskBaselinesVersion || stored.Baselines == nil {
		return errors.New("不支持或损坏的任务基线清单")
	}
	if len(stored.Baselines) > MaxTaskBaselines {
		return fmt.Errorf("任务基线清单超过 %d 条记录", MaxTaskBaselines)
	}
	ids := make(map[string]bool, len(stored.Baselines))
	targets := make(map[string]bool, len(stored.Baselines))
	for _, item := range stored.Baselines {
		if item.BaselineID == "" || item.RepositoryID == "" || !validBaselineRefID(item.WorktreeID) || !filepath.IsAbs(item.WorktreePath) || item.CapturedAt.IsZero() {
			return errors.New("任务基线记录身份或路径无效")
		}
		if !validBaselineSource(item.Source) {
			return errors.New("任务基线记录来源无效")
		}
		if item.Unborn {
			if item.BaseOID != "" {
				return errors.New("unborn 任务基线不允许携带非空 baseOid")
			}
		} else if !validNonZeroObjectID(item.BaseOID) {
			return errors.New("任务基线 baseOid 无效")
		}
		if item.PreExistingChanges.Staged < 0 || item.PreExistingChanges.Unstaged < 0 || item.PreExistingChanges.Untracked < 0 || item.PreExistingChanges.Conflicted < 0 {
			return errors.New("任务基线变更计数无效")
		}
		key := item.RepositoryID + "\x00" + item.WorktreeID
		if ids[item.BaselineID] || targets[key] {
			return errors.New("任务基线记录重复")
		}
		ids[item.BaselineID] = true
		targets[key] = true
	}
	return nil
}

func (s *Service) loadTaskBaselines() error {
	expanded, err := pathutil.Expand(s.stateDir)
	if err != nil {
		return err
	}
	s.stateDir = expanded
	var stored taskBaselineStore
	s.baselinesStore, stored, err = statefile.Open[taskBaselineStore](s.stateDir, statefile.GitTaskBaselines, validateTaskBaselineStore)
	if statefile.IsKind(err, statefile.Missing) {
		return nil
	}
	if err != nil {
		return err
	}
	s.baselines = append([]TaskBaseline(nil), stored.Baselines...)
	return nil
}

func (s *Service) closeTaskBaselines() {
	s.baselinesMu.Lock()
	defer s.baselinesMu.Unlock()
	if s.baselinesStore != nil {
		s.baselinesStore.Close()
	}
	s.baselinesAvailable = false
	s.baselinesError = "任务基线存储已关闭"
}

func (s *Service) requireTaskBaselines() error {
	s.baselinesMu.Lock()
	defer s.baselinesMu.Unlock()
	if !s.baselinesAvailable {
		msg := s.baselinesError
		if msg == "" {
			msg = "任务基线存储不可用"
		}
		return problemf(ErrUnavailable, "%s", msg)
	}
	return nil
}

func pruneTaskBaselines(store *statefile.Handle[taskBaselineStore], list []TaskBaseline) []TaskBaseline {
	out := make([]TaskBaseline, len(list))
	copy(out, list)
	for len(out) > MaxTaskBaselines {
		out = out[1:]
	}
	if store != nil {
		for len(out) > 0 && !store.Fits(taskBaselineStore{V: gitTaskBaselinesVersion, Baselines: out}) {
			out = out[1:]
		}
	}
	return out
}

func (s *Service) commitTaskBaselinesLocked(next []TaskBaseline) error {
	next = pruneTaskBaselines(s.baselinesStore, next)
	err := s.baselinesStore.Commit(taskBaselineStore{V: gitTaskBaselinesVersion, Baselines: next})
	if statefile.IsKind(err, statefile.CommitUncertain) {
		msg := "任务基线已替换但目录同步失败，持久结果不确定；任务基线管理已停用，请重启重新读取: " + err.Error()
		s.baselinesMu.Lock()
		s.baselinesAvailable = false
		s.baselinesError = msg
		s.baselinesMu.Unlock()
		return problemf(ErrUnavailable, "%s", msg)
	}
	if err != nil {
		return fmt.Errorf("保存任务基线清单: %w", err)
	}
	s.baselinesMu.Lock()
	s.baselines = next
	s.baselinesMu.Unlock()
	return nil
}

func (s *Service) upsertTaskBaselineEntry(entry TaskBaseline) error {
	s.baselinesSaveMu.Lock()
	defer s.baselinesSaveMu.Unlock()
	if err := s.requireTaskBaselines(); err != nil {
		return err
	}
	s.baselinesMu.Lock()
	next := make([]TaskBaseline, 0, len(s.baselines)+1)
	for _, existing := range s.baselines {
		if existing.RepositoryID == entry.RepositoryID && existing.WorktreeID == entry.WorktreeID {
			continue
		}
		next = append(next, existing)
	}
	next = append(next, entry)
	s.baselinesMu.Unlock()
	return s.commitTaskBaselinesLocked(next)
}

func (s *Service) removeTaskBaselineEntry(repositoryID, worktreeID string) (bool, error) {
	s.baselinesSaveMu.Lock()
	defer s.baselinesSaveMu.Unlock()
	if err := s.requireTaskBaselines(); err != nil {
		return false, err
	}
	s.baselinesMu.Lock()
	next := make([]TaskBaseline, 0, len(s.baselines))
	removed := false
	for _, existing := range s.baselines {
		if existing.RepositoryID == repositoryID && existing.WorktreeID == worktreeID {
			removed = true
			continue
		}
		next = append(next, existing)
	}
	s.baselinesMu.Unlock()
	if !removed {
		return false, nil
	}
	return true, s.commitTaskBaselinesLocked(next)
}

func (s *Service) findTaskBaseline(repositoryID, worktreeID string) *TaskBaseline {
	s.baselinesMu.Lock()
	defer s.baselinesMu.Unlock()
	for i := range s.baselines {
		if s.baselines[i].RepositoryID == repositoryID && s.baselines[i].WorktreeID == worktreeID {
			copyItem := s.baselines[i]
			return &copyItem
		}
	}
	return nil
}

func (s *Service) dropBaselineRef(ctx context.Context, anchorPath, worktreeID string) {
	if s.gitPath == "" || anchorPath == "" || !validBaselineRefID(worktreeID) {
		return
	}
	if _, err := s.git(ctx, anchorPath, "update-ref", "-d", baselineRef(worktreeID)); err != nil {
		slog.Warn("task baseline ref removal failed", "worktree", worktreeID, "error", err)
	}
}

// purgeWorktreeBaseline deletes a removed worktree's persisted task baseline
// and private GC protection ref. Archiving a session never calls this.
func (s *Service) purgeWorktreeBaseline(ctx context.Context, r *repositoryState, repositoryID, worktreeID string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), readTimeout)
	defer cancel()
	if r != nil {
		s.dropBaselineRef(cleanupCtx, r.entry.AnchorPath, worktreeID)
	}
	if _, err := s.removeTaskBaselineEntry(repositoryID, worktreeID); err != nil {
		slog.Warn("task baseline entry removal failed", "repository", repositoryID, "worktree", worktreeID, "error", err)
	}
}

// purgeRepositoryBaselines cleans all persisted task baselines and private refs
// belonging to an unregistered repository.
func (s *Service) purgeRepositoryBaselines(r *repositoryState, repositoryID string) {
	s.baselinesSaveMu.Lock()
	defer s.baselinesSaveMu.Unlock()
	s.baselinesMu.Lock()
	if !s.baselinesAvailable {
		s.baselinesMu.Unlock()
		return
	}
	next := make([]TaskBaseline, 0, len(s.baselines))
	var removedWorktrees []string
	for _, existing := range s.baselines {
		if existing.RepositoryID == repositoryID {
			removedWorktrees = append(removedWorktrees, existing.WorktreeID)
			continue
		}
		next = append(next, existing)
	}
	s.baselinesMu.Unlock()
	if len(removedWorktrees) > 0 {
		if err := s.commitTaskBaselinesLocked(next); err != nil {
			slog.Warn("purge repository task baselines failed", "repository", repositoryID, "error", err)
		}
	}
	if r != nil && s.gitPath != "" {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), readTimeout)
		defer cancel()
		for _, wtID := range removedWorktrees {
			s.dropBaselineRef(cleanupCtx, r.entry.AnchorPath, wtID)
		}
		if out, err := s.git(cleanupCtx, r.entry.AnchorPath, "for-each-ref", "--format=%(refname)", baselineRefPrefix); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if strings.HasPrefix(line, baselineRefPrefix) {
					_, _ = s.git(cleanupCtx, r.entry.AnchorPath, "update-ref", "-d", line)
				}
			}
		}
	}
}
