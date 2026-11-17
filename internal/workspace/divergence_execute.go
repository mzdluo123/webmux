package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ExecuteMerge executes a previously previewed branch merge on the target worktree.
func (s *Service) ExecuteMerge(ctx context.Context, req ExecuteDivergenceRequest) (DivergenceResult, error) {
	return s.executeDivergence(ctx, GitOpMerge, purposeMerge, req)
}

// ExecuteRebase executes a previously previewed branch rebase on the target worktree.
func (s *Service) ExecuteRebase(ctx context.Context, req ExecuteDivergenceRequest) (DivergenceResult, error) {
	return s.executeDivergence(ctx, GitOpRebase, purposeRebase, req)
}

// ExecuteCherryPick executes a previously previewed cherry-pick on the target worktree.
func (s *Service) ExecuteCherryPick(ctx context.Context, req ExecuteDivergenceRequest) (DivergenceResult, error) {
	return s.executeDivergence(ctx, GitOpCherryPick, purposeCherryPick, req)
}

// ExecuteRevert executes a previously previewed revert on the target worktree.
func (s *Service) ExecuteRevert(ctx context.Context, req ExecuteDivergenceRequest) (DivergenceResult, error) {
	return s.executeDivergence(ctx, GitOpRevert, purposeRevert, req)
}

func (s *Service) executeDivergence(ctx context.Context, kind string, p purpose, req ExecuteDivergenceRequest) (DivergenceResult, error) {
	reqHash := hashOperationRequest(kind, req.RepositoryID, req.WorktreeID, req.ConfirmationID, fmtBool(req.AcknowledgeSessions))
	return runIdempotentOperation(
		ctx, s, req.RepositoryID, req.ClientRequestID, kind, reqHash,
		func(op GitOperation) (DivergenceResult, error) {
			return s.replayDivergenceOperation(ctx, req.RepositoryID, req.WorktreeID, op)
		},
		func(opID string) (DivergenceResult, error) {
			return s.executeDivergenceOnce(ctx, kind, p, req, opID, reqHash)
		},
	)
}

func (s *Service) executeDivergenceOnce(ctx context.Context, kind string, p purpose, req ExecuteDivergenceRequest, opID, reqHash string) (DivergenceResult, error) {
	if req.ConfirmationID == "" {
		return DivergenceResult{}, problemf(ErrInvalid, "缺少预览确认凭证")
	}
	if s.gitPath == "" {
		return DivergenceResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}

	c, ok := s.takeConfirmation(req.ConfirmationID, p, req.RepositoryID, req.WorktreeID)
	if !ok || c.divergence == nil || c.divergence.kind != kind {
		return DivergenceResult{}, problemf(ErrStale, "操作预览确认已过期或失效，请重新预览")
	}
	confirmed := *c.divergence

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

	current, err := s.buildDivergencePreviewState(checkCtx, r, loc, kind, p, DivergencePreviewRequest{
		RepositoryID: req.RepositoryID,
		WorktreeID:   req.WorktreeID,
		RefKind:      confirmed.targetKind,
		RefName:      confirmed.targetRef,
		CommitOID:    confirmed.targetOID,
	})
	if err != nil {
		return DivergenceResult{}, err
	}
	if len(current.blockers) > 0 {
		return DivergenceResult{}, problemf(ErrBlocked, "操作被最新状态阻止：%s；请重新预览", issueMessages(current.blockers))
	}
	if current.commonDir != confirmed.commonDir ||
		current.worktreePath != confirmed.worktreePath ||
		current.currentBranch != confirmed.currentBranch ||
		current.headOID != confirmed.headOID ||
		current.targetOID != confirmed.targetOID ||
		!sameSessions(confirmed.sessions, current.sessions) {
		return DivergenceResult{}, problemf(ErrStale, "工作树分支、HEAD 提交、目标提交或活跃会话已变化，请重新预览")
	}
	if len(current.sessions) > 0 && !req.AcknowledgeSessions {
		return DivergenceResult{}, problemf(ErrInvalid, "工作树中存在活跃会话，请勾选确认会话影响后再执行")
	}

	recoveryRef := s.saveRecoveryRefBestEffort(checkCtx, loc.Path, opID, confirmed.headOID)

	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    req.RepositoryID,
		WorktreeID:      req.WorktreeID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            kind,
		Status:          GitOpRunning,
		ConfirmationID:  req.ConfirmationID,
		StartedAt:       time.Now().UTC(),
		Summary:         confirmed.directionSummary,
		Preview: GitOperationPreview{
			WorktreePath: loc.Path,
			Branch:       confirmed.currentBranch,
			HeadOID:      confirmed.headOID,
			Target:       confirmed.targetRef,
			TargetOID:    confirmed.targetOID,
		},
	}); err != nil {
		return DivergenceResult{}, err
	}
	if shouldAbortBeforeGit(ctx, opID) {
		return DivergenceResult{}, errSimulatedCrash
	}

	args := divergenceGitArgs(kind, confirmed)
	reflogTag := fmt.Sprintf("webmux-%s:%s", kind, opID)
	run := s.runNonInteractiveMutatingGit(ctx, loc.Path, reflogTag, args...)
	if shouldAbortAfterGit(ctx, opID) {
		return DivergenceResult{}, errSimulatedCrash
	}

	return s.evaluateDivergenceRun(r, loc, req.WorktreeID, opID, kind, recoveryRef, confirmed.directionSummary, run)
}

func divergenceGitArgs(kind string, state divergencePreviewState) []string {
	switch kind {
	case GitOpMerge:
		// Pass targetFullRef when available so MERGE_MSG records the real branch name
		// while still verifying targetOID before execution under the repository + worktree gates.
		arg := state.targetFullRef
		if arg == "" {
			arg = state.targetOID
		}
		return []string{"merge", "--no-edit", "--no-autostash", "--end-of-options", arg}
	case GitOpRebase:
		return []string{"rebase", "--no-autostash", "--end-of-options", state.targetOID}
	case GitOpCherryPick:
		return []string{"cherry-pick", "--no-edit", "--end-of-options", state.targetOID}
	case GitOpRevert:
		return []string{"revert", "--no-edit", "--end-of-options", state.targetOID}
	default:
		return nil
	}
}

func (s *Service) runNonInteractiveMutatingGit(ctx context.Context, cwd, reflogTag string, args ...string) gitExit {
	runCtx, runCancel := context.WithTimeout(context.WithoutCancel(ctx), divergenceTimeout)
	defer runCancel()
	stdout := &boundedOutput{limit: maxWorkspaceGitOutput, cancel: runCancel}
	fullArgs := append([]string{
		"-C", cwd,
		"-c", "core.logAllRefUpdates=always",
		"-c", "user.useConfigOnly=true",
	}, args...)
	env := append(append([]string{}, gitEnv...),
		"GIT_EDITOR=true",
		"GIT_SEQUENCE_EDITOR=true",
		"GIT_REFLOG_ACTION="+reflogTag,
	)
	run := runGitProcess(runCtx, gitProcess{
		path:        s.gitPath,
		args:        fullArgs,
		env:         env,
		stdout:      stdout,
		stderrLimit: maxWorkspaceGitError,
	})
	if outStr := strings.TrimSpace(stdout.String()); outStr != "" {
		if run.stderr != "" {
			run.stderr = outStr + "\n" + run.stderr
		} else {
			run.stderr = outStr
		}
	}
	return run
}

func (s *Service) saveRecoveryRefBestEffort(ctx context.Context, cwd, opID, commitOID string) string {
	if !isObjectID(commitOID) || opID == "" {
		return ""
	}
	ref := "refs/webmux/recovery/" + opID
	if _, _, err := s.gitInputOutput(ctx, cwd, nil, nil, "update-ref", "-m", "webmux recovery anchor", ref, commitOID); err != nil {
		return ""
	}
	return ref
}

func (s *Service) evaluateDivergenceRun(
	r *repositoryState,
	loc WorktreeLocation,
	worktreeID, opID, kind, recoveryRef, baseSummary string,
	run gitExit,
) (DivergenceResult, error) {
	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()

	inProg, inErr := s.inspectInProgressLocked(refreshCtx, loc)
	afterTree, treeErr := s.currentTree(refreshCtx, r, worktreeID)
	if treeErr == nil {
		afterTree = s.withLastCommit(refreshCtx, r, afterTree)
	}
	s.readRepository(refreshCtx, r)

	if run.err != nil {
		if isGitLockBusy(run.stderr) {
			retErr := problemf(ErrLocked, "仓库正被其他进程占用（Git 锁冲突），未删除任何锁文件，请稍后重试")
			s.finalizeOperation(opID, func(op *GitOperation) {
				op.Status = GitOpFailed
				op.FailureKind = "locked"
				op.Summary = retErr.Error()
				op.SanitizedOutput = run.stderr
			})
			return DivergenceResult{}, retErr
		}
		var launch *exec.Error
		if errors.As(run.err, &launch) || errors.Is(run.err, os.ErrNotExist) || errors.Is(run.err, os.ErrPermission) {
			retErr := problemf(ErrUnavailable, "Git 无法执行: %v", run.err)
			s.finalizeOperation(opID, func(op *GitOperation) {
				op.Status = GitOpFailed
				op.FailureKind = "unavailable"
				op.Summary = retErr.Error()
				op.SanitizedOutput = run.stderr
			})
			return DivergenceResult{}, retErr
		}
		if run.ctxErr != nil {
			retErr := problemf(ErrTimeout, "Git %s 执行超时: %v", kind, run.ctxErr)
			s.finalizeOperation(opID, func(op *GitOperation) {
				op.Status = GitOpUncertain
				op.FailureKind = "timeout"
				op.Summary = retErr.Error()
				op.SanitizedOutput = run.stderr
			})
			return DivergenceResult{}, retErr
		}

		// Conflict halt: Git stopped cleanly with operation marker / unmerged entries!
		if inErr == nil && (inProg.Active || inProg.ConflictedCount > 0) {
			summary := fmt.Sprintf("%s 遇到 %d 个冲突文件，请使用三方冲突解决器处理后继续或中止", baseSummary, inProg.ConflictedCount)
			s.finalizeOperation(opID, func(op *GitOperation) {
				op.Status = GitOpNeedsReview
				op.FailureKind = "conflicted"
				op.Summary = summary
				op.ActualOID = inProg.HeadOID
				op.SanitizedOutput = run.stderr
				op.ReviewReasons = []string{summary}
			})
			return DivergenceResult{
				OperationID: opID,
				Kind:        kind,
				Outcome:     DivergenceOutcomeConflicted,
				Summary:     summary,
				HeadOID:     inProg.HeadOID,
				Subject:     inProg.HeadSubject,
				RecoveryRef: recoveryRef,
				InProgress:  inProg,
				Worktree:    afterTree,
			}, nil
		}

		if paths, more := overwrittenPaths(run.stderr); len(paths) > 0 {
			msg := "以下未跟踪或工作区文件会被覆盖，操作已拒绝执行: " + strings.Join(paths, ", ")
			if more > 0 {
				msg += fmt.Sprintf(" 等另外 %d 个", more)
			}
			retErr := problemf(ErrConflict, "%s", msg)
			s.finalizeOperation(opID, func(op *GitOperation) {
				op.Status = GitOpFailed
				op.FailureKind = "conflict"
				op.Summary = retErr.Error()
				op.SanitizedOutput = run.stderr
			})
			return DivergenceResult{}, retErr
		}

		retErr := problemf(ErrHookFailed, "Git %s 执行失败: %s", kind, firstGitLine(redactLine(run.stderr)))
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = "hook_failed"
			op.Summary = retErr.Error()
			op.SanitizedOutput = run.stderr
		})
		return DivergenceResult{}, retErr
	}

	headOID := inProg.HeadOID
	subject := inProg.HeadSubject
	summary := fmt.Sprintf("已完成：%s", baseSummary)
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.ActualOID = headOID
		op.Subject = subject
		op.Summary = summary
		op.SanitizedOutput = run.stderr
	})
	return DivergenceResult{
		OperationID: opID,
		Kind:        kind,
		Outcome:     DivergenceOutcomeCompleted,
		Summary:     summary,
		HeadOID:     headOID,
		Subject:     subject,
		RecoveryRef: recoveryRef,
		InProgress:  inProg,
		Worktree:    afterTree,
	}, nil
}
