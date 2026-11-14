package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ExecuteSmartCheckout executes the durable 3-phase tracked-stash Smart Checkout:
//  1. `stashing`: create uniquely tagged controlled stash and persist `stashCommitOid`
//  2. `switching`: switch worktree to target branch (auto-rollback stash on failure)
//  3. `unstashing`: apply stash on target branch; drop temporary stash ONLY if 100%
//     clean, otherwise preserve `stashCommitOid` and enter 3-way conflict resolution.
func (s *Service) ExecuteSmartCheckout(ctx context.Context, req ExecuteSmartCheckoutRequest) (SmartCheckoutResult, error) {
	reqHash := hashOperationRequest(
		GitOpSmartCheckout,
		req.RepositoryID,
		req.WorktreeID,
		req.ConfirmationID,
		fmtBool(req.AcknowledgeSessions),
	)
	return runIdempotentOperation(
		ctx, s, req.RepositoryID, req.WorktreeID, GitOpSmartCheckout, reqHash,
		func(op GitOperation) (SmartCheckoutResult, error) {
			return s.replaySmartCheckoutOperation(ctx, req.RepositoryID, req.WorktreeID, op)
		},
		func(opID string) (SmartCheckoutResult, error) {
			return s.executeSmartCheckoutOnce(ctx, req, opID, reqHash)
		},
	)
}

func (s *Service) executeSmartCheckoutOnce(ctx context.Context, req ExecuteSmartCheckoutRequest, opID, reqHash string) (SmartCheckoutResult, error) {
	if req.ConfirmationID == "" {
		return SmartCheckoutResult{}, problemf(ErrInvalid, "缺少 Smart Checkout 确认凭证")
	}
	if s.gitPath == "" {
		return SmartCheckoutResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	c, ok := s.takeConfirmation(req.ConfirmationID, purposeSmartCheckout, req.RepositoryID, req.WorktreeID)
	if !ok || c.smartCheckout == nil {
		return SmartCheckoutResult{}, problemf(ErrStale, "Smart Checkout 预览已过期或失效，请重新预览")
	}
	confirmed := *c.smartCheckout

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return SmartCheckoutResult{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return SmartCheckoutResult{}, err
	}

	checkCtx, checkCancel := context.WithTimeout(ctx, writeTimeout)
	defer checkCancel()
	if err := acquire(checkCtx, r); err != nil {
		return SmartCheckoutResult{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(checkCtx, loc.Path); err != nil {
		return SmartCheckoutResult{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	current, err := s.buildSmartCheckoutStateLocked(checkCtx, r, loc, req.WorktreeID, confirmed.plan.target, confirmed.includeUntracked)
	if err != nil {
		if errorKind(err) == ErrInvalid {
			return SmartCheckoutResult{}, problemf(ErrStale, "%v；切换目标已变化，请重新预览", err)
		}
		return SmartCheckoutResult{}, err
	}
	if len(current.blockers) > 0 {
		return SmartCheckoutResult{}, problemf(ErrBlocked, "Smart Checkout 被当前状态阻止：%s；请重新预览", issueMessages(current.blockers))
	}
	if current.commonDir != confirmed.commonDir ||
		current.tree.Path != confirmed.tree.Path ||
		current.tree.Detached != confirmed.tree.Detached ||
		!sameText(current.tree.Branch, confirmed.tree.Branch) ||
		!sameText(current.tree.Head, confirmed.tree.Head) ||
		current.plan.info.Commit != confirmed.plan.info.Commit ||
		current.dirtyDigest != confirmed.dirtyDigest ||
		!sameSessions(confirmed.sessions, current.sessions) {
		return SmartCheckoutResult{}, problemf(ErrStale, "工作树修改内容、当前分支、目标提交或活跃会话已在预览后变化，请重新预览")
	}
	if len(current.sessions) > 0 && !req.AcknowledgeSessions {
		return SmartCheckoutResult{}, problemf(ErrBlocked, "工作树中存在活跃会话，请展开原地切换选项并勾选确认会话影响后再执行 Smart Checkout")
	}

	fromLabel := firstNonEmpty(derefStr(current.tree.Branch), shortOID(derefStr(current.tree.Head)))
	toLabel := firstNonEmpty(derefStr(current.plan.info.Branch), shortOID(current.plan.info.Commit))
	hooks := getSmartCheckoutHooks(ctx)

	// ---- Phase 1: stashing ----
	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    req.RepositoryID,
		WorktreeID:      req.WorktreeID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpSmartCheckout,
		Status:          GitOpRunning,
		Phase:           SmartCheckoutPhaseStashing,
		ConfirmationID:  req.ConfirmationID,
		StartedAt:       time.Now().UTC(),
		Summary:         fmt.Sprintf("Smart Checkout (1/3)：正在为 %s → %s 创建受控贮藏", fromLabel, toLabel),
		Preview: GitOperationPreview{
			WorktreePath: loc.Path,
			Branch:       derefStr(current.tree.Branch),
			HeadOID:      derefStr(current.tree.Head),
			Target:       derefStr(current.plan.info.Branch),
			TargetOID:    current.plan.info.Commit,
			Detached:     current.plan.target.Kind == SwitchDetach,
		},
	}); err != nil {
		return SmartCheckoutResult{}, err
	}
	if shouldAbortBeforeGit(ctx, opID) {
		return SmartCheckoutResult{}, errSimulatedCrash
	}

	execCtx, execCancel := context.WithTimeout(context.WithoutCancel(ctx), switchTimeout)
	defer execCancel()

	beforeStashTop, _ := s.resolveCommitReadOnly(execCtx, r.entry.AnchorPath, "refs/stash")
	stashMsg := fmt.Sprintf("webmux-smart-checkout:%s (%s -> %s)", opID, fromLabel, toLabel)
	stashArgs := []string{"stash", "push", "-m", stashMsg}
	if current.includeUntracked && current.untrackedCount > 0 {
		stashArgs = append(stashArgs, "--include-untracked")
	}
	stashRun := s.runNonInteractiveMutatingGit(execCtx, loc.Path, "webmux-smart-checkout-stash:"+opID, stashArgs...)
	if stashRun.err != nil {
		retErr := classifyGitMutateError("stash push", stashRun)
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.Phase = SmartCheckoutPhaseStashing
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = retErr.Error()
			op.SanitizedOutput = stashRun.stderr
		})
		return SmartCheckoutResult{}, retErr
	}

	stashOID := s.findSmartCheckoutStashOID(execCtx, r.entry.AnchorPath, opID, beforeStashTop)
	if !isObjectID(stashOID) {
		retErr := problemf(ErrConflict, "未能确认受控贮藏提交 ID，已停止切换")
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.Phase = SmartCheckoutPhaseStashing
			op.FailureKind = "conflict"
			op.Summary = retErr.Error()
		})
		return SmartCheckoutResult{}, retErr
	}

	s.pinSmartCheckoutRefBestEffort(execCtx, loc.Path, opID, stashOID)
	recoveryCmd := "git stash apply " + stashOID

	// Persist exact stashCommitOid BEFORE touching HEAD!
	if err := s.updateOperationProgress(opID, func(op *GitOperation) {
		op.Phase = SmartCheckoutPhaseSwitching
		op.StashOID = stashOID
		op.RecoveryOID = stashOID
		op.RecoveryCommand = recoveryCmd
		op.Preview.StashOID = stashOID
		op.Summary = fmt.Sprintf("Smart Checkout (2/3)：已保存贮藏 %s，正在切换至 %s", shortOID(stashOID), toLabel)
	}); err != nil {
		return SmartCheckoutResult{}, err
	}
	if hooks.afterStash != nil {
		if hErr := hooks.afterStash(opID, stashOID); hErr != nil {
			return SmartCheckoutResult{}, hErr
		}
	}

	// ---- Phase 2: switching ----
	var switchStderr string
	var switchErr error
	if hooks.failSwitch != nil {
		switchErr = hooks.failSwitch(opID, stashOID)
		switchStderr = switchErr.Error()
	} else {
		_, switchStderr, switchErr = s.gitOutput(execCtx, loc.Path, switchArgs(current.plan)...)
	}
	if switchErr != nil {
		return s.rollbackSmartCheckoutOnSwitchFailure(execCtx, r, loc, req.WorktreeID, opID, current, stashOID, toLabel, switchErr, switchStderr)
	}
	if hooks.afterSwitch != nil {
		if hErr := hooks.afterSwitch(opID, stashOID); hErr != nil {
			return SmartCheckoutResult{}, hErr
		}
	}

	// ---- Phase 3: unstashing ----
	if err := s.updateOperationProgress(opID, func(op *GitOperation) {
		op.Phase = SmartCheckoutPhaseUnstashing
		op.ActualOID = current.plan.info.Commit
		op.Summary = fmt.Sprintf("Smart Checkout (3/3)：已切换至 %s，正在自动恢复贮藏 %s", toLabel, shortOID(stashOID))
	}); err != nil {
		return SmartCheckoutResult{}, err
	}
	if hooks.beforeUnstash != nil {
		if hErr := hooks.beforeUnstash(opID, stashOID); hErr != nil {
			return SmartCheckoutResult{}, hErr
		}
	}

	applyRun := s.applyControlledStashLocked(execCtx, loc.Path, opID, stashOID, current.stagedCount > 0)
	if shouldAbortAfterGit(ctx, opID) {
		return SmartCheckoutResult{}, errSimulatedCrash
	}

	return s.finalizeSmartCheckoutUnstash(execCtx, r, loc, req.WorktreeID, opID, stashOID, toLabel, applyRun, hooks.afterUnstash)
}

func (s *Service) rollbackSmartCheckoutOnSwitchFailure(
	ctx context.Context,
	r *repositoryState,
	loc WorktreeLocation,
	worktreeID, opID string,
	before smartCheckoutState,
	stashOID, toLabel string,
	switchErr error,
	switchStderr string,
) (SmartCheckoutResult, error) {
	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()

	midTree, refreshErr := s.currentTree(refreshCtx, r, worktreeID)
	retErr := switchFailure(switchErr, switchStderr, before.tree, midTree, refreshErr)
	headSame := refreshErr == nil &&
		midTree.Detached == before.tree.Detached &&
		sameText(midTree.Branch, before.tree.Branch) &&
		sameText(midTree.Head, before.tree.Head)

	restored := false
	dropped := false
	recoveryCmd := "git stash apply " + stashOID

	if headSame {
		applyRun := s.applyControlledStashLocked(ctx, loc.Path, opID, stashOID, before.stagedCount > 0)
		conflicted, _ := s.listConflictedFilesLocked(refreshCtx, loc.Path)
		if applyRun.err == nil && len(conflicted) == 0 {
			restored = true
			dropped = s.dropStashByExactOIDLocked(ctx, r.entry.AnchorPath, opID, stashOID)
			if dropped {
				s.clearSmartCheckoutRefBestEffort(ctx, loc.Path, opID)
				recoveryCmd = ""
			}
		}
	}

	inProg, _ := s.inspectInProgressLocked(refreshCtx, loc)
	afterTree := s.refreshWorktreeLocked(refreshCtx, r, worktreeID)
	stashRef := s.findStashRefByOID(refreshCtx, r.entry.AnchorPath, stashOID)

	if restored {
		summary := fmt.Sprintf("切换至 %s 失败（%s），已在原分支自动完整恢复未提交修改", toLabel, retErr.Error())
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.Phase = SmartCheckoutPhaseRolledBack
			op.FailureKind = classifyFailureKind(retErr)
			op.StashOID = stashOID
			op.RecoveryOID = ""
			op.RecoveryCommand = recoveryCmd
			op.Summary = summary
			op.SanitizedOutput = switchStderr
		})
		return SmartCheckoutResult{
			OperationID:     opID,
			Outcome:         SmartCheckoutRolledBack,
			Phase:           SmartCheckoutPhaseRolledBack,
			Summary:         summary,
			ErrorMessage:    retErr.Error(),
			StashOID:        stashOID,
			StashRef:        stashRef,
			StashDropped:    dropped,
			StashPreserved:  !dropped,
			StashRestored:   true,
			RecoveryCommand: recoveryCmd,
			InProgress:      inProg,
			Worktree:        afterTree,
		}, nil
	}

	summary := fmt.Sprintf("切换至 %s 未完成且未能自动还原贮藏，原修改仍安全保存在贮藏 %s 中", toLabel, shortOID(stashOID))
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpNeedsReview
		op.Phase = SmartCheckoutPhaseRolledBack
		op.FailureKind = "rollback_needs_review"
		op.StashOID = stashOID
		op.RecoveryOID = stashOID
		op.RecoveryCommand = recoveryCmd
		op.Summary = summary
		op.ReviewReasons = []string{summary}
		op.SanitizedOutput = switchStderr
	})
	return SmartCheckoutResult{
		OperationID:     opID,
		Outcome:         SmartCheckoutRolledBack,
		Phase:           SmartCheckoutPhaseRolledBack,
		Summary:         summary,
		ErrorMessage:    retErr.Error(),
		StashOID:        stashOID,
		StashRef:        stashRef,
		StashDropped:    false,
		StashPreserved:  true,
		StashRestored:   false,
		RecoveryCommand: recoveryCmd,
		InProgress:      inProg,
		Worktree:        afterTree,
	}, nil
}

func (s *Service) finalizeSmartCheckoutUnstash(
	ctx context.Context,
	r *repositoryState,
	loc WorktreeLocation,
	worktreeID, opID, stashOID, toLabel string,
	applyRun gitExit,
	afterUnstash func(opID, stashOID string) error,
) (SmartCheckoutResult, error) {
	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()

	conflicted, _ := s.listConflictedFilesLocked(refreshCtx, loc.Path)
	recoveryCmd := "git stash apply " + stashOID

	if len(conflicted) > 0 || applyRun.err != nil {
		// NEVER drop stashOID when apply encounters conflicts or errors!
		if len(conflicted) > 0 {
			s.writeUnstashRefBestEffort(refreshCtx, loc.Path, loc.WorktreeID, stashOID)
		}
		inProg, _ := s.inspectInProgressLocked(refreshCtx, loc)
		afterTree := s.refreshWorktreeLocked(refreshCtx, r, worktreeID)
		stashRef := s.findStashRefByOID(refreshCtx, r.entry.AnchorPath, stashOID)

		var summary, failKind string
		if len(conflicted) > 0 {
			failKind = SmartCheckoutPhaseConflict
			summary = fmt.Sprintf(
				"已切换至 %s，恢复修改时有 %d 个文件与目标分支冲突；受控贮藏 %s 已完整保留，请在三方冲突解决器中处理",
				toLabel, len(conflicted), shortOID(stashOID),
			)
		} else {
			failKind = "unstash_blocked"
			summary = fmt.Sprintf(
				"已切换至 %s，但自动恢复贮藏 %s 受阻（%s）；受控贮藏已完整保留，可处理冲突路径后应用",
				toLabel, shortOID(stashOID), firstGitLine(redactLine(applyRun.stderr)),
			)
		}
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpNeedsReview
			op.Phase = SmartCheckoutPhaseConflict
			op.FailureKind = failKind
			op.StashOID = stashOID
			op.RecoveryOID = stashOID
			op.RecoveryCommand = recoveryCmd
			op.Summary = summary
			op.ReviewReasons = []string{summary}
			op.SanitizedOutput = applyRun.stderr
		})
		return SmartCheckoutResult{
			OperationID:     opID,
			Outcome:         SmartCheckoutConflict,
			Phase:           SmartCheckoutPhaseConflict,
			Summary:         summary,
			StashOID:        stashOID,
			StashRef:        stashRef,
			StashDropped:    false,
			StashPreserved:  true,
			StashRestored:   false,
			RecoveryCommand: recoveryCmd,
			InProgress:      inProg,
			Worktree:        afterTree,
		}, nil
	}

	s.clearUnstashRefBestEffort(refreshCtx, loc.Path, loc.WorktreeID)
	dropped := s.dropStashByExactOIDLocked(ctx, r.entry.AnchorPath, opID, stashOID)
	if dropped {
		s.clearSmartCheckoutRefBestEffort(ctx, loc.Path, opID)
	}
	if afterUnstash != nil {
		if hErr := afterUnstash(opID, stashOID); hErr != nil {
			return SmartCheckoutResult{}, hErr
		}
	}
	inProg, _ := s.inspectInProgressLocked(refreshCtx, loc)
	afterTree := s.refreshWorktreeLocked(refreshCtx, r, worktreeID)

	summary := fmt.Sprintf("Smart Checkout 已完成：已携带本地修改切换至 %s", toLabel)
	remCmd := ""
	if !dropped {
		remCmd = recoveryCmd
	}
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.Phase = SmartCheckoutPhaseCompleted
		op.StashOID = stashOID
		op.RecoveryOID = ""
		op.RecoveryCommand = remCmd
		op.Summary = summary
		op.SanitizedOutput = applyRun.stderr
	})
	return SmartCheckoutResult{
		OperationID:     opID,
		Outcome:         SmartCheckoutCompleted,
		Phase:           SmartCheckoutPhaseCompleted,
		Summary:         summary,
		StashOID:        stashOID,
		StashDropped:    dropped,
		StashPreserved:  !dropped,
		StashRestored:   true,
		RecoveryCommand: remCmd,
		InProgress:      inProg,
		Worktree:        afterTree,
	}, nil
}

func (s *Service) applyControlledStashLocked(ctx context.Context, cwd, opID, stashOID string, tryIndex bool) gitExit {
	if tryIndex {
		runIdx := s.runNonInteractiveMutatingGit(ctx, cwd, "webmux-smart-checkout-apply:"+opID, "stash", "apply", "--quiet", "--index", "--end-of-options", stashOID)
		if runIdx.err == nil {
			return runIdx
		}
		conflicted, _ := s.listConflictedFilesLocked(ctx, cwd)
		if len(conflicted) > 0 {
			return runIdx
		}
	}
	return s.runNonInteractiveMutatingGit(ctx, cwd, "webmux-smart-checkout-apply:"+opID, "stash", "apply", "--quiet", "--end-of-options", stashOID)
}

func (s *Service) findSmartCheckoutStashOID(ctx context.Context, anchor, opID, beforeTop string) string {
	tag := "webmux-smart-checkout:" + opID
	if stashes, err := s.listStashesLocked(ctx, anchor); err == nil {
		for _, item := range stashes {
			if strings.Contains(item.Summary, tag) {
				return item.OID
			}
		}
	}
	afterTop, _ := s.resolveCommitReadOnly(ctx, anchor, "refs/stash")
	if isObjectID(afterTop) && afterTop != beforeTop {
		return afterTop
	}
	return ""
}

func (s *Service) findStashRefByOID(ctx context.Context, anchor, stashOID string) string {
	if !isObjectID(stashOID) {
		return ""
	}
	stashes, err := s.listStashesLocked(ctx, anchor)
	if err != nil {
		return ""
	}
	for _, entry := range stashes {
		if entry.OID == stashOID {
			return entry.StashRef
		}
	}
	return ""
}

func (s *Service) dropStashByExactOIDLocked(ctx context.Context, anchor, opID, stashOID string) bool {
	ref := s.findStashRefByOID(ctx, anchor, stashOID)
	if ref == "" {
		return false
	}
	slot, err := s.resolveStashSlotLocked(ctx, anchor, ref, stashOID)
	if err != nil {
		return false
	}
	run := s.runNonInteractiveMutatingGit(ctx, anchor, "webmux-smart-checkout-drop:"+opID, "stash", "drop", "--quiet", "refs/"+slot.StashRef)
	return run.err == nil
}

func (s *Service) pinSmartCheckoutRefBestEffort(ctx context.Context, cwd, opID, stashOID string) {
	if opID == "" || !isObjectID(stashOID) || s.gitPath == "" {
		return
	}
	_, _ = s.git(ctx, cwd, "update-ref", "-m", "webmux smart checkout stash", smartCheckoutRefPrefix+opID, stashOID)
}

func (s *Service) clearSmartCheckoutRefBestEffort(ctx context.Context, cwd, opID string) {
	if opID == "" || s.gitPath == "" {
		return
	}
	_, _ = s.git(ctx, cwd, "update-ref", "-d", smartCheckoutRefPrefix+opID)
}
