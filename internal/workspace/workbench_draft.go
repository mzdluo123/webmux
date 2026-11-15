package workspace

import (
	"context"
	"log/slog"
	"time"
	"unicode/utf8"
)

// PutCommitDraftRequest saves a worktree commit draft with optimistic concurrency.
type PutCommitDraftRequest struct {
	Subject      string
	Body         string
	BaseRevision int
}

// DraftConflictError reports an optimistic revision mismatch along with the
// server's latest stored CommitDraft so the caller can display both versions.
type DraftConflictError struct {
	Current CommitDraft
	Message string
}

func (e *DraftConflictError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "提交草稿已在其他设备更新，请确认保留版本"
}

func (e *DraftConflictError) Unwrap() error {
	return ErrConflict
}

func recordToCommitDraft(repositoryID, worktreeID string, rec *commitDraftRecord) CommitDraft {
	if rec == nil {
		return CommitDraft{
			RepositoryID: repositoryID,
			WorktreeID:   worktreeID,
			Subject:      "",
			Body:         "",
			Revision:     0,
		}
	}
	t := rec.UpdatedAt
	return CommitDraft{
		RepositoryID: rec.RepositoryID,
		WorktreeID:   rec.WorktreeID,
		Subject:      rec.Subject,
		Body:         rec.Body,
		Revision:     rec.Revision,
		UpdatedAt:    &t,
	}
}

// GetCommitDraft returns the persisted commit draft of worktreeID, or an empty
// revision-0 draft when none exists.
func (s *Service) GetCommitDraft(ctx context.Context, repositoryID, worktreeID string) (CommitDraft, error) {
	if err := s.requireWorkbenchCollab(); err != nil {
		return CommitDraft{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	if _, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID); err != nil {
		return CommitDraft{}, err
	}
	s.collabMu.Lock()
	defer s.collabMu.Unlock()
	for i := range s.collabDrafts {
		if s.collabDrafts[i].RepositoryID == repositoryID && s.collabDrafts[i].WorktreeID == worktreeID {
			copyRec := s.collabDrafts[i]
			return recordToCommitDraft(repositoryID, worktreeID, &copyRec), nil
		}
	}
	return recordToCommitDraft(repositoryID, worktreeID, nil), nil
}

// PutCommitDraft updates worktreeID's commit draft when req.BaseRevision matches
// the server's current revision, returning *DraftConflictError (HTTP 409) when
// another device or tab saved a newer revision first. Draft text is never logged.
func (s *Service) PutCommitDraft(ctx context.Context, repositoryID, worktreeID string, req PutCommitDraftRequest) (CommitDraft, error) {
	if err := s.requireWorkbenchCollab(); err != nil {
		return CommitDraft{}, err
	}
	if req.BaseRevision < 0 {
		return CommitDraft{}, problemf(ErrInvalid, "baseRevision 不能为负数")
	}
	if !utf8.ValidString(req.Subject) || !utf8.ValidString(req.Body) {
		return CommitDraft{}, problemf(ErrInvalid, "提交草稿包含无效 UTF-8 字符")
	}
	if len(req.Subject)+len(req.Body) > MaxCommitDraftBytes {
		return CommitDraft{}, problemf(ErrTooLarge, "提交草稿超过 %d KiB 长度上限", MaxCommitDraftBytes>>10)
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	if _, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID); err != nil {
		return CommitDraft{}, err
	}

	s.collabSaveMu.Lock()
	defer s.collabSaveMu.Unlock()
	if err := s.requireWorkbenchCollab(); err != nil {
		return CommitDraft{}, err
	}

	s.collabMu.Lock()
	var current *commitDraftRecord
	nextDrafts := make([]commitDraftRecord, 0, len(s.collabDrafts)+1)
	for i := range s.collabDrafts {
		item := s.collabDrafts[i]
		if item.RepositoryID == repositoryID && item.WorktreeID == worktreeID {
			copyRec := item
			current = &copyRec
			continue
		}
		nextDrafts = append(nextDrafts, item)
	}
	nextReviews := append([]ReviewMarkRecord(nil), s.collabReviews...)
	s.collabMu.Unlock()

	currentRev := 0
	if current != nil {
		currentRev = current.Revision
	}
	if req.BaseRevision != currentRev {
		return CommitDraft{}, &DraftConflictError{
			Current: recordToCommitDraft(repositoryID, worktreeID, current),
			Message: "提交说明草稿已在其他设备或标签页更新，请选择保留本机内容、采用远端版本或合并两份内容",
		}
	}

	now := time.Now().UTC()
	updated := commitDraftRecord{
		RepositoryID: repositoryID,
		WorktreeID:   worktreeID,
		Subject:      req.Subject,
		Body:         req.Body,
		Revision:     currentRev + 1,
		UpdatedAt:    now,
	}
	nextDrafts = append(nextDrafts, updated)
	if err := s.commitWorkbenchStateLocked(nextDrafts, nextReviews); err != nil {
		return CommitDraft{}, err
	}
	return recordToCommitDraft(repositoryID, worktreeID, &updated), nil
}

// clearWorktreeCommitDraftBestEffort clears the subject and body of a worktree's
// stored commit draft (while bumping its revision so open tabs observe the post-commit
// reset) ONLY after a commit is confirmed created. Never called on preview expiry,
// commit failure, uncertain commit, or push failure.
func (s *Service) clearWorktreeCommitDraftBestEffort(repositoryID, worktreeID string) {
	if repositoryID == "" || worktreeID == "" {
		return
	}
	s.collabSaveMu.Lock()
	defer s.collabSaveMu.Unlock()
	s.collabMu.Lock()
	if !s.collabAvailable {
		s.collabMu.Unlock()
		return
	}
	var current *commitDraftRecord
	nextDrafts := make([]commitDraftRecord, 0, len(s.collabDrafts))
	for i := range s.collabDrafts {
		item := s.collabDrafts[i]
		if item.RepositoryID == repositoryID && item.WorktreeID == worktreeID {
			copyRec := item
			current = &copyRec
			continue
		}
		nextDrafts = append(nextDrafts, item)
	}
	if current == nil || (current.Subject == "" && current.Body == "") {
		s.collabMu.Unlock()
		return
	}
	nextReviews := append([]ReviewMarkRecord(nil), s.collabReviews...)
	s.collabMu.Unlock()

	cleared := commitDraftRecord{
		RepositoryID: repositoryID,
		WorktreeID:   worktreeID,
		Subject:      "",
		Body:         "",
		Revision:     current.Revision + 1,
		UpdatedAt:    time.Now().UTC(),
	}
	nextDrafts = append(nextDrafts, cleared)
	if err := s.commitWorkbenchStateLocked(nextDrafts, nextReviews); err != nil {
		slog.Warn("clear worktree commit draft after commit failed", "repository", repositoryID, "worktree", worktreeID, "error", err)
	}
}
