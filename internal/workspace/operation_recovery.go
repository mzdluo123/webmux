package workspace

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/session"
)

func normalizeSequencerKind(kind string) string {
	switch kind {
	case "merge":
		return "merge"
	case "rebase":
		return "rebase"
	case "cherry-pick", "cherry_pick":
		return "cherry-pick"
	case "revert":
		return "revert"
	default:
		return ""
	}
}

// ContinueOperation resumes an active merge, rebase, cherry-pick, or revert
// ONLY after verifying under the per-worktree gate that 0 unmerged entries remain.
func (s *Service) ContinueOperation(ctx context.Context, req ContinueOperationRequest) (DivergenceResult, error) {
	normKind := normalizeSequencerKind(req.Kind)
	if normKind == "" {
		return DivergenceResult{}, problemf(ErrInvalid, "不支持继续的操作类型: %s", req.Kind)
	}
	reqHash := hashOperationRequest(GitOpContinue, req.RepositoryID, req.WorktreeID, normKind)
	return runIdempotentOperation(
		ctx, s, req.RepositoryID, req.ClientRequestID, GitOpContinue, reqHash,
		func(op GitOperation) (DivergenceResult, error) {
			return s.replayDivergenceOperation(ctx, req.RepositoryID, req.WorktreeID, op)
		},
		func(opID string) (DivergenceResult, error) {
			return s.continueOperationOnce(ctx, req, normKind, opID, reqHash)
		},
	)
}

func (s *Service) continueOperationOnce(ctx context.Context, req ContinueOperationRequest, normKind, opID, reqHash string) (DivergenceResult, error) {
	if s.gitPath == "" {
		return DivergenceResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return DivergenceResult{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return DivergenceResult{}, err
	}

	checkCtx, checkCancel := context.WithTimeout(ctx, writeTimeout)
	defer checkCancel()
	if err := acquire(checkCtx, r); err != nil {
		return DivergenceResult{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(checkCtx, loc.Path); err != nil {
		return DivergenceResult{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return DivergenceResult{}, err
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return DivergenceResult{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}
	if err := checkWorktreeIndexLock(gitDir); err != nil {
		return DivergenceResult{}, err
	}

	inProg, err := s.inspectInProgressLocked(checkCtx, loc)
	if err != nil {
		return DivergenceResult{}, err
	}
	if !inProg.Active || inProg.Kind != normKind {
		return DivergenceResult{}, problemf(ErrStale, "工作树当前未处于进行中的 %s 操作，请刷新状态", normKind)
	}
	if inProg.ConflictedCount > 0 {
		return DivergenceResult{}, problemf(ErrConflicted, "仍有 %d 个未解决的合并冲突，必须全部标记已解决后才能继续", inProg.ConflictedCount)
	}
	if !inProg.CanContinue {
		return DivergenceResult{}, problemf(ErrBlocked, "当前操作尚不能继续: %s", issueMessages(inProg.ContinueBlockers))
	}

	summary := fmt.Sprintf("继续完成 %s 操作", normKind)
	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    req.RepositoryID,
		WorktreeID:      req.WorktreeID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpContinue,
		Status:          GitOpRunning,
		StartedAt:       time.Now().UTC(),
		Summary:         summary,
		Preview: GitOperationPreview{
			WorktreePath: loc.Path,
			Branch:       inProg.CurrentBranch,
			HeadOID:      inProg.HeadOID,
			Target:       inProg.TargetLabel,
			TargetOID:    inProg.TargetOID,
		},
	}); err != nil {
		return DivergenceResult{}, err
	}
	if shouldAbortBeforeGit(ctx, opID) {
		return DivergenceResult{}, errSimulatedCrash
	}

	reflogTag := fmt.Sprintf("webmux-continue:%s", opID)
	run := s.runContinueGit(ctx, loc.Path, normKind, reflogTag)
	if shouldAbortAfterGit(ctx, opID) {
		return DivergenceResult{}, errSimulatedCrash
	}

	return s.evaluateDivergenceRun(r, loc, req.WorktreeID, opID, GitOpContinue, "", summary, run)
}

func (s *Service) runContinueGit(ctx context.Context, cwd, normKind, reflogTag string) gitExit {
	switch normKind {
	case "merge":
		return s.runNonInteractiveMutatingGit(ctx, cwd, reflogTag, "commit", "--no-edit")
	case "rebase":
		return s.runNonInteractiveMutatingGit(ctx, cwd, reflogTag, "rebase", "--continue")
	case "cherry-pick":
		return s.runNonInteractiveMutatingGit(ctx, cwd, reflogTag, "cherry-pick", "--continue", "--no-edit")
	case "revert":
		return s.runNonInteractiveMutatingGit(ctx, cwd, reflogTag, "revert", "--continue", "--no-edit")
	default:
		return s.runNonInteractiveMutatingGit(ctx, cwd, reflogTag, normKind, "--continue")
	}
}

// PreviewAbortOperation builds an explicit, honest abort preview naming the exact
// restore commit (ORIG_HEAD / orig-head) and every dirty/conflicted path whose
// conflict edits will be discarded.
func (s *Service) PreviewAbortOperation(ctx context.Context, repositoryID, worktreeID string) (AbortOperationPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return AbortOperationPreview{}, err
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return AbortOperationPreview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return AbortOperationPreview{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return AbortOperationPreview{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	state, err := s.buildAbortPreviewState(ctx, loc)
	if err != nil {
		return AbortOperationPreview{}, err
	}

	preview := AbortOperationPreview{
		RepositoryID:     repositoryID,
		WorktreeID:       worktreeID,
		WorktreePath:     state.worktreePath,
		Kind:             state.kind,
		CurrentBranch:    state.currentBranch,
		HeadOID:          state.headOID,
		RestoreOID:       state.restoreOID,
		RestoreSubject:   state.restoreSubject,
		DirectionSummary: state.directionSummary,
		AffectedPaths:    state.affectedPaths,
		Sessions:         state.sessions,
		Blockers:         state.blockers,
		Warnings:         state.warnings,
	}
	if len(state.blockers) > 0 {
		return preview, nil
	}
	id, expires, err := s.confirm(confirmation{
		purpose:      purposeAbortOp,
		repositoryID: repositoryID,
		worktreeID:   worktreeID,
		abortPreview: &state,
	})
	if err != nil {
		return AbortOperationPreview{}, err
	}
	preview.ConfirmationID = id
	preview.ExpiresAt = &expires
	return preview, nil
}

func (s *Service) buildAbortPreviewState(ctx context.Context, loc WorktreeLocation) (abortPreviewState, error) {
	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return abortPreviewState{}, err
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return abortPreviewState{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}
	inProg, err := s.inspectInProgressLocked(ctx, loc)
	if err != nil {
		return abortPreviewState{}, err
	}
	if !inProg.Active || !inProg.CanAbort {
		return abortPreviewState{}, problemf(ErrNotFound, "当前工作树没有可中止的进行中操作")
	}

	sessions := s.runningSessions(loc.Path)
	if sessions == nil {
		sessions = []session.Info{}
	}
	affected := s.collectDirtyAndConflictedPaths(ctx, loc.Path)

	restoreOID := firstNonEmpty(inProg.OrigHeadOID, inProg.HeadOID)
	restoreSubject := inProg.OrigHeadSubject
	if restoreSubject == "" && restoreOID == inProg.HeadOID {
		restoreSubject = inProg.HeadSubject
	}

	blockers := make([]Issue, 0)
	warnings := make([]Issue, 0)
	if err := checkWorktreeIndexLock(gitDir); err != nil {
		blockers = append(blockers, Issue{Code: "locked", Message: err.Error()})
	}
	if restoreOID == "" {
		blockers = append(blockers, Issue{
			Code:    "missing_orig_head",
			Message: "未检测到 ORIG_HEAD 起点记录，无法自动验证中止恢复位置，请在终端手动核对",
		})
	}
	if len(affected) > 0 {
		warnings = append(warnings, Issue{
			Code:    "discard_conflict_edits",
			Message: fmt.Sprintf("中止操作将丢弃 %d 个受影响文件中的冲突解决草稿与暂存改动", len(affected)),
		})
	}
	if len(sessions) > 0 {
		warnings = append(warnings, Issue{
			Code:    issueSessions,
			Message: fmt.Sprintf("工作树中有 %d 个运行中的会话，它们会看到回退后的文件状态", len(sessions)),
		})
	}

	branchLabel := firstNonEmpty(inProg.CurrentBranch, "HEAD")
	summary := fmt.Sprintf(
		"中止工作树 %s 上的 %s 操作，并将分支 %s 恢复到操作前起点 %s",
		filepath.Base(loc.Path), inProg.Kind, branchLabel, shortOID(restoreOID),
	)

	return abortPreviewState{
		commonDir:        loc.CommonDir,
		worktreeID:       loc.WorktreeID,
		worktreePath:     loc.Path,
		kind:             inProg.Kind,
		currentBranch:    inProg.CurrentBranch,
		headOID:          inProg.HeadOID,
		restoreOID:       restoreOID,
		restoreSubject:   restoreSubject,
		directionSummary: summary,
		affectedPaths:    affected,
		sessions:         sessions,
		blockers:         blockers,
		warnings:         warnings,
	}, nil
}

func (s *Service) collectDirtyAndConflictedPaths(ctx context.Context, cwd string) []string {
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "status", "--porcelain=v2", "-z", "--untracked-files=no")
	if err != nil || len(data) == 0 {
		return []string{}
	}
	paths := make([]string, 0)
	seen := make(map[string]bool)
	records := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	for i := 0; i < len(records); i++ {
		rec := string(records[i])
		if len(rec) < 3 {
			continue
		}
		var relPath string
		switch rec[0] {
		case 'u':
			fields := strings.SplitN(rec, " ", 11)
			if len(fields) == 11 {
				relPath = fields[10]
			}
		case '1':
			fields := strings.SplitN(rec, " ", 9)
			if len(fields) == 9 {
				relPath = fields[8]
			}
		case '2':
			fields := strings.SplitN(rec, " ", 10)
			if len(fields) == 10 {
				relPath = fields[9]
			}
			if i+1 < len(records) {
				i++
			}
		}
		if relPath != "" && !seen[relPath] {
			seen[relPath] = true
			paths = append(paths, relPath)
		}
	}
	return paths
}

// AbortOperation aborts an in-progress merge, rebase, cherry-pick, or revert
// after verifying the preview confirmation and restoring HEAD to restoreOID.
func (s *Service) AbortOperation(ctx context.Context, req ExecuteAbortOperationRequest) (DivergenceResult, error) {
	reqHash := hashOperationRequest(GitOpAbort, req.RepositoryID, req.WorktreeID, req.ConfirmationID, fmtBool(req.AcknowledgeSessions))
	return runIdempotentOperation(
		ctx, s, req.RepositoryID, req.ClientRequestID, GitOpAbort, reqHash,
		func(op GitOperation) (DivergenceResult, error) {
			return s.replayDivergenceOperation(ctx, req.RepositoryID, req.WorktreeID, op)
		},
		func(opID string) (DivergenceResult, error) {
			return s.abortOperationOnce(ctx, req, opID, reqHash)
		},
	)
}

func (s *Service) abortOperationOnce(ctx context.Context, req ExecuteAbortOperationRequest, opID, reqHash string) (DivergenceResult, error) {
	if req.ConfirmationID == "" {
		return DivergenceResult{}, problemf(ErrInvalid, "缺少中止确认凭证")
	}
	if s.gitPath == "" {
		return DivergenceResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	c, ok := s.takeConfirmation(req.ConfirmationID, purposeAbortOp, req.RepositoryID, req.WorktreeID)
	if !ok || c.abortPreview == nil {
		return DivergenceResult{}, problemf(ErrStale, "中止预览已过期或失效，请重新预览")
	}
	confirmed := *c.abortPreview

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return DivergenceResult{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return DivergenceResult{}, err
	}

	checkCtx, checkCancel := context.WithTimeout(ctx, writeTimeout)
	defer checkCancel()
	if err := acquire(checkCtx, r); err != nil {
		return DivergenceResult{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(checkCtx, loc.Path); err != nil {
		return DivergenceResult{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	current, err := s.buildAbortPreviewState(checkCtx, loc)
	if err != nil {
		return DivergenceResult{}, err
	}
	if len(current.blockers) > 0 {
		return DivergenceResult{}, problemf(ErrBlocked, "中止操作被阻止: %s", issueMessages(current.blockers))
	}
	if current.commonDir != confirmed.commonDir ||
		current.worktreePath != confirmed.worktreePath ||
		current.kind != confirmed.kind ||
		current.restoreOID != confirmed.restoreOID ||
		!sameSessions(confirmed.sessions, current.sessions) {
		return DivergenceResult{}, problemf(ErrStale, "进行中操作状态或活跃会话已变化，请重新预览中止影响")
	}
	if len(current.sessions) > 0 && !req.AcknowledgeSessions {
		return DivergenceResult{}, problemf(ErrInvalid, "工作树中存在活跃会话，请确认会话影响后再中止")
	}

	recoveryRef := s.saveRecoveryRefBestEffort(checkCtx, loc.Path, opID, current.headOID)

	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    req.RepositoryID,
		WorktreeID:      req.WorktreeID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpAbort,
		Status:          GitOpRunning,
		ConfirmationID:  req.ConfirmationID,
		StartedAt:       time.Now().UTC(),
		Summary:         confirmed.directionSummary,
		Preview: GitOperationPreview{
			WorktreePath: loc.Path,
			Branch:       confirmed.currentBranch,
			HeadOID:      confirmed.headOID,
			TargetOID:    confirmed.restoreOID,
		},
	}); err != nil {
		return DivergenceResult{}, err
	}
	if shouldAbortBeforeGit(ctx, opID) {
		return DivergenceResult{}, errSimulatedCrash
	}

	reflogTag := fmt.Sprintf("webmux-abort:%s", opID)
	run := s.runNonInteractiveMutatingGit(ctx, loc.Path, reflogTag, confirmed.kind, "--abort")
	if shouldAbortAfterGit(ctx, opID) {
		return DivergenceResult{}, errSimulatedCrash
	}

	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	inProg, _ := s.inspectInProgressLocked(refreshCtx, loc)
	afterTree, treeErr := s.currentTree(refreshCtx, r, req.WorktreeID)
	if treeErr == nil {
		afterTree = s.withLastCommit(refreshCtx, r, afterTree)
	}
	s.readRepository(refreshCtx, r)

	if run.err != nil {
		retErr := problemf(ErrConflict, "Git %s --abort 执行失败: %s", confirmed.kind, firstGitLine(redactLine(run.stderr)))
		if isGitLockBusy(run.stderr) {
			retErr = problemf(ErrLocked, "仓库正被其他进程占用（Git 锁冲突），请稍后重试")
		}
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = retErr.Error()
			op.SanitizedOutput = run.stderr
		})
		return DivergenceResult{}, retErr
	}

	if inProg.Active || (confirmed.restoreOID != "" && inProg.HeadOID != confirmed.restoreOID) {
		summary := fmt.Sprintf("已执行 %s --abort，但实际 HEAD（%s）与预期恢复点（%s）不一致，请人工核对", confirmed.kind, shortOID(inProg.HeadOID), shortOID(confirmed.restoreOID))
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpNeedsReview
			op.ActualOID = inProg.HeadOID
			op.Summary = summary
			op.ReviewReasons = []string{summary}
			op.SanitizedOutput = run.stderr
		})
		return DivergenceResult{
			OperationID: opID,
			Kind:        GitOpAbort,
			Outcome:     DivergenceOutcomeAborted,
			Summary:     summary,
			HeadOID:     inProg.HeadOID,
			Subject:     inProg.HeadSubject,
			RecoveryRef: recoveryRef,
			InProgress:  inProg,
			Worktree:    afterTree,
		}, nil
	}

	summary := fmt.Sprintf("已中止 %s 操作并恢复到 %s", confirmed.kind, shortOID(inProg.HeadOID))
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.ActualOID = inProg.HeadOID
		op.Subject = inProg.HeadSubject
		op.Summary = summary
		op.SanitizedOutput = run.stderr
	})
	return DivergenceResult{
		OperationID: opID,
		Kind:        GitOpAbort,
		Outcome:     DivergenceOutcomeAborted,
		Summary:     summary,
		HeadOID:     inProg.HeadOID,
		Subject:     inProg.HeadSubject,
		RecoveryRef: recoveryRef,
		InProgress:  inProg,
		Worktree:    afterTree,
	}, nil
}
