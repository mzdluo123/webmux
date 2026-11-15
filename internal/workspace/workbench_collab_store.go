package workspace

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/pathutil"
	"github.com/rainchan/webmux/internal/statefile"
)

const (
	gitWorkbenchStateVersion = 1
	MaxCommitDraftBytes      = 32 << 10
	MaxCommitDraftRecords    = 500
	MaxReviewMarkRecords     = 4000
	MaxWorktreeReviewMarks   = 500

	ReviewScopeUncommitted = "uncommitted"
	ReviewScopeTask        = "task"

	ReviewAreaStaged   = "staged"
	ReviewAreaUnstaged = "unstaged"
	ReviewAreaTask     = "task"
)

// CommitDraft represents a worktree's shared server-side commit message draft
// protected by monotonic optimistic concurrency revision checking.
type CommitDraft struct {
	RepositoryID string     `json:"repositoryId"`
	WorktreeID   string     `json:"worktreeId"`
	Subject      string     `json:"subject"`
	Body         string     `json:"body"`
	Revision     int        `json:"revision"`
	UpdatedAt    *time.Time `json:"updatedAt,omitempty"`
}

// ReviewMarkIdentity binds a single file's reviewed state to exact server-observed
// content hashes/OIDs and file modes on both sides of the comparison.
type ReviewMarkIdentity struct {
	Scope          string `json:"scope"`
	Area           string `json:"area"`
	Path           string `json:"path"`
	OldPath        string `json:"oldPath,omitempty"`
	BaseMode       string `json:"baseMode,omitempty"`
	BaseIdentity   string `json:"baseIdentity,omitempty"`
	TargetMode     string `json:"targetMode,omitempty"`
	TargetIdentity string `json:"targetIdentity,omitempty"`
}

// ReviewMarkRecord stores one persisted content-bound file review mark.
type ReviewMarkRecord struct {
	RepositoryID   string    `json:"repositoryId"`
	WorktreeID     string    `json:"worktreeId"`
	Scope          string    `json:"scope"`
	Area           string    `json:"area"`
	Path           string    `json:"path"`
	OldPath        string    `json:"oldPath,omitempty"`
	BaseMode       string    `json:"baseMode,omitempty"`
	BaseIdentity   string    `json:"baseIdentity,omitempty"`
	TargetMode     string    `json:"targetMode,omitempty"`
	TargetIdentity string    `json:"targetIdentity,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type commitDraftRecord struct {
	RepositoryID string    `json:"repositoryId"`
	WorktreeID   string    `json:"worktreeId"`
	Subject      string    `json:"subject"`
	Body         string    `json:"body"`
	Revision     int       `json:"revision"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type gitWorkbenchStateStore struct {
	V       int                 `json:"v"`
	Drafts  []commitDraftRecord `json:"drafts"`
	Reviews []ReviewMarkRecord  `json:"reviews"`
}

func validReviewScope(scope string) bool {
	return scope == ReviewScopeUncommitted || scope == ReviewScopeTask
}

func validReviewAreaForScope(scope, area string) bool {
	switch scope {
	case ReviewScopeUncommitted:
		return area == ReviewAreaStaged || area == ReviewAreaUnstaged
	case ReviewScopeTask:
		return area == ReviewAreaTask
	default:
		return false
	}
}

func validateGitWorkbenchStateStore(stored gitWorkbenchStateStore) error {
	if stored.V != gitWorkbenchStateVersion || stored.Drafts == nil || stored.Reviews == nil {
		return errors.New("不支持或损坏的 Git 工作台协作状态")
	}
	if len(stored.Drafts) > MaxCommitDraftRecords {
		return fmt.Errorf("提交草稿记录超过 %d 条上限", MaxCommitDraftRecords)
	}
	if len(stored.Reviews) > MaxReviewMarkRecords {
		return fmt.Errorf("审阅进度记录超过 %d 条上限", MaxReviewMarkRecords)
	}
	draftTargets := make(map[string]bool, len(stored.Drafts))
	for _, d := range stored.Drafts {
		if d.RepositoryID == "" || !validBaselineRefID(d.WorktreeID) || d.Revision < 1 || d.UpdatedAt.IsZero() {
			return errors.New("提交草稿记录身份或版本号无效")
		}
		if len(d.Subject)+len(d.Body) > MaxCommitDraftBytes || !utf8.ValidString(d.Subject) || !utf8.ValidString(d.Body) {
			return errors.New("提交草稿记录长度超限或编码无效")
		}
		key := d.RepositoryID + "\x00" + d.WorktreeID
		if draftTargets[key] {
			return errors.New("提交草稿记录重复")
		}
		draftTargets[key] = true
	}
	reviewTargets := make(map[string]bool, len(stored.Reviews))
	for _, r := range stored.Reviews {
		if r.RepositoryID == "" || !validBaselineRefID(r.WorktreeID) || r.UpdatedAt.IsZero() {
			return errors.New("审阅进度记录身份无效")
		}
		if !validReviewScope(r.Scope) || !validReviewAreaForScope(r.Scope, r.Area) {
			return errors.New("审阅进度记录范围无效")
		}
		if _, err := validateRepoRelPath(r.Path); err != nil {
			return errors.New("审阅进度记录路径无效")
		}
		if r.OldPath != "" {
			if _, err := validateRepoRelPath(r.OldPath); err != nil {
				return errors.New("审阅进度记录原路径无效")
			}
		}
		key := r.RepositoryID + "\x00" + r.WorktreeID + "\x00" + r.Scope + "\x00" + r.Area + "\x00" + r.Path
		if reviewTargets[key] {
			return errors.New("审阅进度记录重复")
		}
		reviewTargets[key] = true
	}
	return nil
}

func (s *Service) loadWorkbenchCollab() error {
	expanded, err := pathutil.Expand(s.stateDir)
	if err != nil {
		return err
	}
	s.stateDir = expanded
	var stored gitWorkbenchStateStore
	s.collabStore, stored, err = statefile.Open[gitWorkbenchStateStore](s.stateDir, statefile.GitWorkbenchState, validateGitWorkbenchStateStore)
	if statefile.IsKind(err, statefile.Missing) {
		return nil
	}
	if err != nil {
		return err
	}
	s.collabDrafts = append([]commitDraftRecord(nil), stored.Drafts...)
	s.collabReviews = append([]ReviewMarkRecord(nil), stored.Reviews...)
	return nil
}

func (s *Service) closeWorkbenchCollab() {
	s.collabMu.Lock()
	defer s.collabMu.Unlock()
	if s.collabStore != nil {
		s.collabStore.Close()
	}
	s.collabAvailable = false
	s.collabError = "Git 工作台协作状态存储已关闭"
}

func (s *Service) requireWorkbenchCollab() error {
	s.collabMu.Lock()
	defer s.collabMu.Unlock()
	if !s.collabAvailable {
		msg := s.collabError
		if msg == "" {
			msg = "Git 工作台协作状态存储不可用"
		}
		return problemf(ErrUnavailable, "%s", msg)
	}
	return nil
}

func pruneWorkbenchState(
	store *statefile.Handle[gitWorkbenchStateStore],
	drafts []commitDraftRecord,
	reviews []ReviewMarkRecord,
) ([]commitDraftRecord, []ReviewMarkRecord) {
	outDrafts := make([]commitDraftRecord, len(drafts))
	copy(outDrafts, drafts)
	outReviews := make([]ReviewMarkRecord, len(reviews))
	copy(outReviews, reviews)

	slices.SortStableFunc(outDrafts, func(a, b commitDraftRecord) int {
		return a.UpdatedAt.Compare(b.UpdatedAt)
	})
	for len(outDrafts) > MaxCommitDraftRecords {
		outDrafts = outDrafts[1:]
	}

	slices.SortStableFunc(outReviews, func(a, b ReviewMarkRecord) int {
		return a.UpdatedAt.Compare(b.UpdatedAt)
	})
	perWorktree := make(map[string]int)
	filteredReviews := make([]ReviewMarkRecord, 0, len(outReviews))
	for i := len(outReviews) - 1; i >= 0; i-- {
		wtKey := outReviews[i].RepositoryID + "\x00" + outReviews[i].WorktreeID
		if perWorktree[wtKey] >= MaxWorktreeReviewMarks {
			continue
		}
		perWorktree[wtKey]++
		filteredReviews = append(filteredReviews, outReviews[i])
	}
	slices.Reverse(filteredReviews)
	outReviews = filteredReviews
	for len(outReviews) > MaxReviewMarkRecords {
		outReviews = outReviews[1:]
	}

	if store != nil {
		for !store.Fits(gitWorkbenchStateStore{V: gitWorkbenchStateVersion, Drafts: outDrafts, Reviews: outReviews}) {
			if len(outReviews) > 0 {
				outReviews = outReviews[1:]
				continue
			}
			if len(outDrafts) > 0 {
				outDrafts = outDrafts[1:]
				continue
			}
			break
		}
	}
	return outDrafts, outReviews
}

func (s *Service) commitWorkbenchStateLocked(nextDrafts []commitDraftRecord, nextReviews []ReviewMarkRecord) error {
	nextDrafts, nextReviews = pruneWorkbenchState(s.collabStore, nextDrafts, nextReviews)
	err := s.collabStore.Commit(gitWorkbenchStateStore{
		V:       gitWorkbenchStateVersion,
		Drafts:  nextDrafts,
		Reviews: nextReviews,
	})
	if statefile.IsKind(err, statefile.CommitUncertain) {
		msg := "Git 工作台协作状态已替换但目录同步失败，持久结果不确定；协作同步已停用，请重启重新读取: " + err.Error()
		s.collabMu.Lock()
		s.collabAvailable = false
		s.collabError = msg
		s.collabMu.Unlock()
		return problemf(ErrUnavailable, "%s", msg)
	}
	if err != nil {
		return fmt.Errorf("保存 Git 工作台协作状态: %w", err)
	}
	s.collabMu.Lock()
	s.collabDrafts = nextDrafts
	s.collabReviews = nextReviews
	s.collabMu.Unlock()
	return nil
}

// purgeWorktreeCollabState cleans persisted commit drafts and review marks when
// a worktree is removed or released. Session archival never calls this.
func (s *Service) purgeWorktreeCollabState(repositoryID, worktreeID string) {
	s.collabSaveMu.Lock()
	defer s.collabSaveMu.Unlock()
	s.collabMu.Lock()
	if !s.collabAvailable {
		s.collabMu.Unlock()
		return
	}
	nextDrafts := make([]commitDraftRecord, 0, len(s.collabDrafts))
	changed := false
	for _, d := range s.collabDrafts {
		if d.RepositoryID == repositoryID && d.WorktreeID == worktreeID {
			changed = true
			continue
		}
		nextDrafts = append(nextDrafts, d)
	}
	nextReviews := make([]ReviewMarkRecord, 0, len(s.collabReviews))
	for _, r := range s.collabReviews {
		if r.RepositoryID == repositoryID && r.WorktreeID == worktreeID {
			changed = true
			continue
		}
		nextReviews = append(nextReviews, r)
	}
	s.collabMu.Unlock()
	if !changed {
		return
	}
	if err := s.commitWorkbenchStateLocked(nextDrafts, nextReviews); err != nil {
		slog.Warn("purge worktree workbench collab state failed", "repository", repositoryID, "worktree", worktreeID, "error", err)
	}
}

// purgeRepositoryCollabState cleans all persisted commit drafts and review marks
// belonging to an unregistered repository.
func (s *Service) purgeRepositoryCollabState(repositoryID string) {
	s.collabSaveMu.Lock()
	defer s.collabSaveMu.Unlock()
	s.collabMu.Lock()
	if !s.collabAvailable {
		s.collabMu.Unlock()
		return
	}
	nextDrafts := make([]commitDraftRecord, 0, len(s.collabDrafts))
	changed := false
	for _, d := range s.collabDrafts {
		if d.RepositoryID == repositoryID {
			changed = true
			continue
		}
		nextDrafts = append(nextDrafts, d)
	}
	nextReviews := make([]ReviewMarkRecord, 0, len(s.collabReviews))
	for _, r := range s.collabReviews {
		if r.RepositoryID == repositoryID {
			changed = true
			continue
		}
		nextReviews = append(nextReviews, r)
	}
	s.collabMu.Unlock()
	if !changed {
		return
	}
	if err := s.commitWorkbenchStateLocked(nextDrafts, nextReviews); err != nil {
		slog.Warn("purge repository workbench collab state failed", "repository", repositoryID, "error", err)
	}
}
