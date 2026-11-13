package workspace

import (
	"context"
	"fmt"
	"time"
)

// PreviewStashDrop creates a confirmation for dropping a single stash entry, including its recovery command.
func (s *Service) PreviewStashDrop(ctx context.Context, req StashDropPreviewRequest) (StashDropPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return StashDropPreview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return StashDropPreview{}, err
	}
	defer release(r)
	if err := s.verifyIdentity(ctx, r); err != nil {
		return StashDropPreview{}, err
	}
	entry, err := s.resolveStashSlotLocked(ctx, r.entry.AnchorPath, req.StashRef, req.StashOID)
	if err != nil {
		return StashDropPreview{}, err
	}
	detail, err := s.inspectStashLocked(ctx, r.entry.AnchorPath, entry.OID)
	if err != nil {
		return StashDropPreview{}, err
	}
	recovery := "git stash apply " + entry.OID
	state := stashDropState{
		stashRef:        entry.StashRef,
		stashOID:        entry.OID,
		entry:           entry,
		fileCount:       len(detail.Files),
		recoveryCommand: recovery,
	}
	id, expires, err := s.confirm(confirmation{
		purpose:      purposeStashDrop,
		repositoryID: req.RepositoryID,
		stashDrop:    &state,
	})
	if err != nil {
		return StashDropPreview{}, err
	}
	warnings := []Issue{{
		Code:    "drop_recovery_hint",
		Message: "删除仅移除单个 refs/stash 引用条目；在 Git GC 回收悬空对象前，可通过下方提交 OID 恢复",
	}}
	return StashDropPreview{
		ConfirmationID:  id,
		ExpiresAt:       &expires,
		RepositoryID:    req.RepositoryID,
		Entry:           entry,
		FileCount:       len(detail.Files),
		RecoveryCommand: recovery,
		Blockers:        []Issue{},
		Warnings:        warnings,
	}, nil
}

// ExecuteStashDrop removes a single confirmed stash entry after re-verifying its reflog slot OID.
func (s *Service) ExecuteStashDrop(ctx context.Context, req ExecuteStashDropRequest) (StashDropResult, error) {
	reqHash := hashOperationRequest(GitOpStashDrop, req.RepositoryID, req.ConfirmationID)
	return runIdempotentOperation(
		ctx, s, req.RepositoryID, req.ClientRequestID, GitOpStashDrop, reqHash,
		func(op GitOperation) (StashDropResult, error) {
			return s.replayStashDropOperation(ctx, req.RepositoryID, op)
		},
		func(opID string) (StashDropResult, error) {
			return s.executeStashDropOnce(ctx, req, opID, reqHash)
		},
	)
}

func (s *Service) executeStashDropOnce(ctx context.Context, req ExecuteStashDropRequest, opID, reqHash string) (StashDropResult, error) {
	if req.ConfirmationID == "" {
		return StashDropResult{}, problemf(ErrInvalid, "缺少删除贮藏确认凭证")
	}
	if s.gitPath == "" {
		return StashDropResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	c, ok := s.takeConfirmation(req.ConfirmationID, purposeStashDrop, req.RepositoryID, "")
	if !ok || c.stashDrop == nil {
		return StashDropResult{}, problemf(ErrStale, "删除贮藏确认已过期或失效，请重新预览")
	}
	confirmed := *c.stashDrop

	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return StashDropResult{}, err
	}
	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return StashDropResult{}, err
	}
	defer release(r)
	if err := s.verifyIdentity(wCtx, r); err != nil {
		return StashDropResult{}, err
	}
	entry, err := s.resolveStashSlotLocked(wCtx, r.entry.AnchorPath, confirmed.stashRef, confirmed.stashOID)
	if err != nil {
		return StashDropResult{}, err
	}

	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    req.RepositoryID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpStashDrop,
		Status:          GitOpRunning,
		ConfirmationID:  req.ConfirmationID,
		StartedAt:       time.Now().UTC(),
		Summary:         fmt.Sprintf("正在删除贮藏 %s", entry.StashRef),
		Preview: GitOperationPreview{
			StashRef: entry.StashRef,
			StashOID: entry.OID,
		},
	}); err != nil {
		return StashDropResult{}, err
	}

	run := s.runNonInteractiveMutatingGit(ctx, r.entry.AnchorPath, "webmux-stash-drop:"+opID, "stash", "drop", "--quiet", "refs/"+entry.StashRef)
	if run.err != nil {
		retErr := classifyGitMutateError("stash drop", run)
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = retErr.Error()
			op.SanitizedOutput = run.stderr
		})
		return StashDropResult{}, retErr
	}

	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	stashes, _ := s.listStashesLocked(refreshCtx, r.entry.AnchorPath)
	if stashes == nil {
		stashes = []StashEntry{}
	}
	summary := fmt.Sprintf("已删除贮藏 %s (%s)", entry.StashRef, entry.ShortOID)
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.ActualOID = entry.OID
		op.Subject = entry.Summary
		op.Summary = summary
		op.SanitizedOutput = run.stderr
	})
	return StashDropResult{
		OperationID:     opID,
		StashRef:        entry.StashRef,
		StashOID:        entry.OID,
		RecoveryCommand: confirmed.recoveryCommand,
		Stashes:         stashes,
	}, nil
}
