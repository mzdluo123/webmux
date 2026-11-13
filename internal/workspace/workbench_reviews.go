package workspace

import (
	"context"
	"log/slog"
	"time"
)

// ReviewsOverview summarizes live review progress for one worktree and scope.
type ReviewsOverview struct {
	Scope          string            `json:"scope"`
	Status         string            `json:"status"`
	Reason         string            `json:"reason,omitempty"`
	TotalFiles     int               `json:"totalFiles"`
	ReviewedFiles  int               `json:"reviewedFiles"`
	Files          []ReviewFileState `json:"files"`
	Applied        bool              `json:"applied"`
	StaleRejected  bool              `json:"staleRejected,omitempty"`
	RejectedReason string            `json:"rejectedReason,omitempty"`
}

// PutReviewsRequest marks, unmarks, or clears review progress for a worktree.
type PutReviewsRequest struct {
	Scope  string
	Action string // "mark" | "unmark" | "clear"
	Item   *ReviewMarkIdentity
}

const (
	ReviewActionMark   = "mark"
	ReviewActionUnmark = "unmark"
	ReviewActionClear  = "clear"
)

func (s *Service) buildReviewsOverview(
	repositoryID, worktreeID, scope, status, reason string,
	liveFiles []ReviewFileState,
) (ReviewsOverview, []ReviewMarkRecord, bool) {
	liveBySlot := make(map[string]ReviewMarkIdentity, len(liveFiles))
	for i := range liveFiles {
		f := liveFiles[i]
		liveBySlot[reviewSlotKey(f.Area, f.Path)] = ReviewMarkIdentity{
			Scope:          f.Scope,
			Area:           f.Area,
			Path:           f.Path,
			OldPath:        f.OldPath,
			BaseMode:       f.BaseMode,
			BaseIdentity:   f.BaseIdentity,
			TargetMode:     f.TargetMode,
			TargetIdentity: f.TargetIdentity,
		}
	}

	s.collabMu.Lock()
	allReviews := append([]ReviewMarkRecord(nil), s.collabReviews...)
	s.collabMu.Unlock()

	validBySlot := make(map[string]time.Time)
	nextReviews := make([]ReviewMarkRecord, 0, len(allReviews))
	pruned := false
	for _, rec := range allReviews {
		if rec.RepositoryID != repositoryID || rec.WorktreeID != worktreeID || rec.Scope != scope {
			nextReviews = append(nextReviews, rec)
			continue
		}
		slot := reviewSlotKey(rec.Area, rec.Path)
		liveID, exists := liveBySlot[slot]
		recID := ReviewMarkIdentity{
			Scope:          rec.Scope,
			Area:           rec.Area,
			Path:           rec.Path,
			OldPath:        rec.OldPath,
			BaseMode:       rec.BaseMode,
			BaseIdentity:   rec.BaseIdentity,
			TargetMode:     rec.TargetMode,
			TargetIdentity: rec.TargetIdentity,
		}
		if !exists || !reviewIdentityMatches(liveID, recID) {
			pruned = true
			continue
		}
		t := rec.UpdatedAt
		validBySlot[slot] = t
		nextReviews = append(nextReviews, rec)
	}

	reviewedCount := 0
	outFiles := make([]ReviewFileState, len(liveFiles))
	for i, f := range liveFiles {
		slot := reviewSlotKey(f.Area, f.Path)
		if t, ok := validBySlot[slot]; ok {
			tCopy := t
			f.Reviewed = true
			f.ReviewedAt = &tCopy
			reviewedCount++
		}
		outFiles[i] = f
	}

	return ReviewsOverview{
		Scope:         scope,
		Status:        status,
		Reason:        reason,
		TotalFiles:    len(outFiles),
		ReviewedFiles: reviewedCount,
		Files:         outFiles,
		Applied:       true,
	}, nextReviews, pruned
}

// GetReviews inspects live worktree changes for scope ("uncommitted" or "task"),
// automatically invalidates any stored marks whose file content or mode changed,
// and returns the live content-bound review state.
func (s *Service) GetReviews(ctx context.Context, repositoryID, worktreeID, scope string) (ReviewsOverview, error) {
	if scope == "" {
		scope = ReviewScopeUncommitted
	}
	if !validReviewScope(scope) {
		return ReviewsOverview{}, problemf(ErrInvalid, "不支持的审阅范围: %s", scope)
	}
	if err := s.requireWorkbenchCollab(); err != nil {
		return ReviewsOverview{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return ReviewsOverview{}, err
	}

	var liveFiles []ReviewFileState
	status := TaskScopeReady
	reason := ""
	if scope == ReviewScopeUncommitted {
		if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
			return ReviewsOverview{}, err
		}
		liveFiles, err = s.collectUncommittedReviewFiles(ctx, loc)
		s.releaseWorktreeWrite(loc.Path)
	} else {
		liveFiles, status, reason, err = s.collectTaskReviewFiles(ctx, repositoryID, worktreeID, loc)
	}
	if err != nil {
		return ReviewsOverview{}, err
	}

	overview, _, pruned := s.buildReviewsOverview(repositoryID, worktreeID, scope, status, reason, liveFiles)
	if pruned {
		s.pruneStaleReviewsBestEffort(repositoryID, worktreeID, scope, liveFiles)
	}
	return overview, nil
}

func (s *Service) pruneStaleReviewsBestEffort(repositoryID, worktreeID, scope string, liveFiles []ReviewFileState) {
	s.collabSaveMu.Lock()
	defer s.collabSaveMu.Unlock()
	s.collabMu.Lock()
	if !s.collabAvailable {
		s.collabMu.Unlock()
		return
	}
	drafts := append([]commitDraftRecord(nil), s.collabDrafts...)
	s.collabMu.Unlock()
	_, nextReviews, stillPruned := s.buildReviewsOverview(repositoryID, worktreeID, scope, TaskScopeReady, "", liveFiles)
	if !stillPruned {
		return
	}
	if err := s.commitWorkbenchStateLocked(drafts, nextReviews); err != nil {
		slog.Warn("prune stale git review marks failed", "repository", repositoryID, "worktree", worktreeID, "error", err)
	}
}

// PutReviews atomically updates server-side review progress for worktreeID.
// When Action=="mark", the submitted BaseIdentity/TargetIdentity MUST match the
// live server-observed file content and mode; stale/offline marks for files that
// have since been modified are rejected without marking the newer file reviewed.
func (s *Service) PutReviews(ctx context.Context, repositoryID, worktreeID string, req PutReviewsRequest) (ReviewsOverview, error) {
	if !validReviewScope(req.Scope) {
		return ReviewsOverview{}, problemf(ErrInvalid, "不支持的审阅范围: %s", req.Scope)
	}
	if req.Action != ReviewActionMark && req.Action != ReviewActionUnmark && req.Action != ReviewActionClear {
		return ReviewsOverview{}, problemf(ErrInvalid, "不支持的审阅进度操作: %s", req.Action)
	}
	if req.Action != ReviewActionClear {
		if req.Item == nil {
			return ReviewsOverview{}, problemf(ErrInvalid, "缺少要更新的审阅文件标识")
		}
		if req.Item.Scope == "" {
			req.Item.Scope = req.Scope
		}
		if req.Item.Scope != req.Scope || !validReviewAreaForScope(req.Scope, req.Item.Area) {
			return ReviewsOverview{}, problemf(ErrInvalid, "审阅条目范围与分区不匹配")
		}
		cleanPath, pErr := validateRepoRelPath(req.Item.Path)
		if pErr != nil {
			return ReviewsOverview{}, pErr
		}
		req.Item.Path = cleanPath
		if req.Item.OldPath != "" {
			cleanOld, oErr := validateRepoRelPath(req.Item.OldPath)
			if oErr != nil {
				return ReviewsOverview{}, oErr
			}
			req.Item.OldPath = cleanOld
		}
	}
	if err := s.requireWorkbenchCollab(); err != nil {
		return ReviewsOverview{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return ReviewsOverview{}, err
	}

	var liveFiles []ReviewFileState
	status := TaskScopeReady
	reason := ""
	if req.Scope == ReviewScopeUncommitted {
		if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
			return ReviewsOverview{}, err
		}
		liveFiles, err = s.collectUncommittedReviewFiles(ctx, loc)
		s.releaseWorktreeWrite(loc.Path)
	} else {
		liveFiles, status, reason, err = s.collectTaskReviewFiles(ctx, repositoryID, worktreeID, loc)
	}
	if err != nil {
		return ReviewsOverview{}, err
	}

	s.collabSaveMu.Lock()
	defer s.collabSaveMu.Unlock()
	if err := s.requireWorkbenchCollab(); err != nil {
		return ReviewsOverview{}, err
	}

	_, validReviews, _ := s.buildReviewsOverview(repositoryID, worktreeID, req.Scope, status, reason, liveFiles)

	applied := true
	staleRejected := false
	rejectedReason := ""

	switch req.Action {
	case ReviewActionClear:
		filtered := make([]ReviewMarkRecord, 0, len(validReviews))
		for _, rec := range validReviews {
			if rec.RepositoryID == repositoryID && rec.WorktreeID == worktreeID && rec.Scope == req.Scope {
				continue
			}
			filtered = append(filtered, rec)
		}
		validReviews = filtered

	case ReviewActionUnmark:
		filtered := make([]ReviewMarkRecord, 0, len(validReviews))
		for _, rec := range validReviews {
			if rec.RepositoryID == repositoryID && rec.WorktreeID == worktreeID &&
				rec.Scope == req.Scope && rec.Area == req.Item.Area && rec.Path == req.Item.Path {
				continue
			}
			filtered = append(filtered, rec)
		}
		validReviews = filtered

	case ReviewActionMark:
		var matchedLive *ReviewFileState
		for i := range liveFiles {
			f := liveFiles[i]
			liveID := ReviewMarkIdentity{
				Scope:          f.Scope,
				Area:           f.Area,
				Path:           f.Path,
				OldPath:        f.OldPath,
				BaseMode:       f.BaseMode,
				BaseIdentity:   f.BaseIdentity,
				TargetMode:     f.TargetMode,
				TargetIdentity: f.TargetIdentity,
			}
			if reviewIdentityMatches(liveID, *req.Item) {
				copyFile := f
				matchedLive = &copyFile
				break
			}
		}
		if matchedLive == nil || matchedLive.Status == "U" {
			applied = false
			staleRejected = true
			if matchedLive != nil && matchedLive.Status == "U" {
				rejectedReason = "合并冲突未解决的文件不能标记为已审阅"
			} else {
				rejectedReason = "文件内容或模式已在服务端更新，过期审阅标记已被拒绝，请核对最新差异"
			}
		} else {
			filtered := make([]ReviewMarkRecord, 0, len(validReviews)+1)
			for _, rec := range validReviews {
				if rec.RepositoryID == repositoryID && rec.WorktreeID == worktreeID &&
					rec.Scope == req.Scope && rec.Area == matchedLive.Area && rec.Path == matchedLive.Path {
					continue
				}
				filtered = append(filtered, rec)
			}
			filtered = append(filtered, ReviewMarkRecord{
				RepositoryID:   repositoryID,
				WorktreeID:     worktreeID,
				Scope:          req.Scope,
				Area:           matchedLive.Area,
				Path:           matchedLive.Path,
				OldPath:        matchedLive.OldPath,
				BaseMode:       matchedLive.BaseMode,
				BaseIdentity:   matchedLive.BaseIdentity,
				TargetMode:     matchedLive.TargetMode,
				TargetIdentity: matchedLive.TargetIdentity,
				UpdatedAt:      time.Now().UTC(),
			})
			validReviews = filtered
		}
	}

	s.collabMu.Lock()
	drafts := append([]commitDraftRecord(nil), s.collabDrafts...)
	s.collabMu.Unlock()
	if err := s.commitWorkbenchStateLocked(drafts, validReviews); err != nil {
		return ReviewsOverview{}, err
	}

	overview, _, _ := s.buildReviewsOverview(repositoryID, worktreeID, req.Scope, status, reason, liveFiles)
	overview.Applied = applied
	overview.StaleRejected = staleRejected
	overview.RejectedReason = rejectedReason
	return overview, nil
}
