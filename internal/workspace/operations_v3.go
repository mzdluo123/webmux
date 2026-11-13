package workspace

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func (s *Service) updateOperationProgress(id string, update func(*GitOperation)) error {
	s.opsSaveMu.Lock()
	defer s.opsSaveMu.Unlock()
	s.opsMu.Lock()
	if !s.opsAvailable {
		errText := s.opsError
		s.opsMu.Unlock()
		if errText != "" {
			return problemf(ErrUnavailable, "%s", errText)
		}
		return nil
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
		return nil
	}
	update(&next[idx])
	next[idx].SanitizedOutput = sanitizeOperationOutput(next[idx].SanitizedOutput)
	return s.commitOperationsLocked(next)
}

func (s *Service) reconcileV3Operation(ctx context.Context, r *repositoryState, op *GitOperation) {
	switch op.Kind {
	case GitOpDiscard:
		s.reconcileDiscardOperation(op)
	case GitOpSmartCheckout:
		s.reconcileSmartCheckoutOperation(ctx, r, op)
	default:
		op.Status = GitOpUncertain
		op.Summary = "服务重启时操作状态未知，请刷新核对"
	}
}

func (s *Service) reconcileDiscardOperation(op *GitOperation) {
	op.Status = GitOpUncertain
	if op.RecoveryOID != "" {
		if op.RecoveryCommand == "" {
			op.RecoveryCommand = fmt.Sprintf("git restore --source=%s --worktree -- .", op.RecoveryOID)
		}
		op.Summary = fmt.Sprintf("服务重启时丢弃修改未完成最终确认；原工作区内容已保存在恢复快照 %s 中", shortOID(op.RecoveryOID))
		op.ReviewReasons = []string{op.Summary}
		return
	}
	op.Summary = "服务重启时丢弃修改在落盘前中断，请刷新检查工作区文件状态"
}

func (s *Service) reconcileSmartCheckoutOperation(ctx context.Context, r *repositoryState, op *GitOperation) {
	anchor := r.entry.AnchorPath
	stashOID := firstNonEmpty(op.StashOID, op.Preview.StashOID)
	if !isObjectID(stashOID) {
		if pinned, err := s.resolveCommitReadOnly(ctx, anchor, smartCheckoutRefPrefix+op.ID); err == nil && isObjectID(pinned) {
			stashOID = pinned
		}
	}
	if !isObjectID(stashOID) {
		stashOID = s.findSmartCheckoutStashOID(ctx, anchor, op.ID, "")
	}
	if isObjectID(stashOID) {
		op.StashOID = stashOID
		op.Preview.StashOID = stashOID
		op.RecoveryOID = stashOID
		op.RecoveryCommand = "git stash apply " + stashOID
	}

	wtPath := op.Preview.WorktreePath
	wtReachable := false
	if wtPath != "" {
		if info, err := os.Stat(wtPath); err == nil && info.IsDir() {
			wtReachable = true
		}
	}
	if !wtReachable || op.WorktreeID == "" {
		op.Status = GitOpUncertain
		if isObjectID(stashOID) {
			op.Summary = fmt.Sprintf("服务重启时 Smart Checkout 被中断；受控贮藏 %s 已完整保留，可执行 %s 恢复", shortOID(stashOID), op.RecoveryCommand)
		} else {
			op.Summary = "服务重启时 Smart Checkout 被中断且未生成受控贮藏，请刷新检查工作树状态"
		}
		return
	}

	conflicted, _ := s.listConflictedFilesLocked(ctx, wtPath)
	if len(conflicted) > 0 {
		if isObjectID(stashOID) {
			s.writeUnstashRefBestEffort(ctx, wtPath, op.WorktreeID, stashOID)
		}
		op.Status = GitOpNeedsReview
		op.Phase = SmartCheckoutPhaseConflict
		op.FailureKind = SmartCheckoutPhaseConflict
		op.Summary = fmt.Sprintf("服务重启对账检测到 %d 个未解决的 Smart Checkout 冲突文件；贮藏 %s 已保留，请在冲突解决器中处理", len(conflicted), shortOID(stashOID))
		op.ReviewReasons = []string{op.Summary}
		return
	}

	branch, detached, _ := s.symbolicRefHead(ctx, wtPath)
	headOID, _ := s.resolveCommitReadOnly(ctx, wtPath, "HEAD")
	onTarget := headOID != "" && headOID == op.Preview.TargetOID &&
		((op.Preview.Detached && detached) || (!op.Preview.Detached && !detached && branch == op.Preview.Target))
	onOrigin := headOID != "" && headOID == op.Preview.HeadOID &&
		(op.Preview.Branch == "" || (!detached && branch == op.Preview.Branch))

	if !isObjectID(stashOID) {
		if onOrigin {
			op.Status = GitOpFailed
			op.Phase = SmartCheckoutPhaseStashing
			op.FailureKind = "interrupted"
			op.Summary = "服务在创建受控贮藏完成前重启，原分支与工作区未切换"
			return
		}
		op.Status = GitOpUncertain
		op.Summary = "服务重启时 Smart Checkout 状态不确定，请刷新检查分支与贮藏列表"
		return
	}

	stashRef := s.findStashRefByOID(ctx, anchor, stashOID)
	pinnedOID, _ := s.resolveCommitReadOnly(ctx, anchor, smartCheckoutRefPrefix+op.ID)
	if onTarget && op.Phase == SmartCheckoutPhaseUnstashing && stashRef == "" && pinnedOID == "" {
		op.Status = GitOpSucceeded
		op.Phase = SmartCheckoutPhaseCompleted
		op.ActualOID = headOID
		op.RecoveryOID = ""
		op.RecoveryCommand = ""
		op.Summary = "服务重启对账确认 Smart Checkout 已完成切换并恢复修改"
		return
	}

	op.Status = GitOpNeedsReview
	op.FailureKind = "interrupted_with_stash"
	switch {
	case onTarget:
		op.Phase = SmartCheckoutPhaseUnstashing
		op.ActualOID = headOID
		op.Summary = fmt.Sprintf("服务重启时工作树已切换至目标位置，受控贮藏 %s（%s）完整保留，请在贮藏视图或终端确认应用状态", shortOID(stashOID), firstNonEmpty(stashRef, "安全引用"))
	case onOrigin:
		op.Phase = SmartCheckoutPhaseSwitching
		op.Summary = fmt.Sprintf("服务在分支切换前重启，工作树仍在原分支；未提交修改保存在受控贮藏 %s 中，可一键恢复", shortOID(stashOID))
	default:
		op.Summary = fmt.Sprintf("服务重启时 Smart Checkout 中断；修改完整保存在受控贮藏 %s 中（%s）", shortOID(stashOID), op.RecoveryCommand)
	}
	op.ReviewReasons = []string{op.Summary}
}

func (s *Service) replayDiscardOperation(ctx context.Context, repositoryID, worktreeID string, op GitOperation) (DiscardResult, error) {
	if op.Status == GitOpSucceeded {
		r, err := s.repository(repositoryID)
		if err != nil {
			return DiscardResult{}, err
		}
		readCtx, cancel := context.WithTimeout(ctx, readTimeout)
		defer cancel()
		if err := acquire(readCtx, r); err != nil {
			return DiscardResult{}, err
		}
		defer release(r)
		tree, _ := s.currentTree(readCtx, r, worktreeID)
		ref := ""
		if op.RecoveryOID != "" {
			ref = discardRecoveryPrefix + op.ID
		}
		return DiscardResult{
			OperationID:     op.ID,
			Scope:           op.Preview.Target,
			Summary:         op.Summary,
			RecoveryOID:     op.RecoveryOID,
			RecoveryRef:     ref,
			RecoveryCommand: op.RecoveryCommand,
			DiscardedPaths:  []string{},
			Worktree:        s.withLastCommit(readCtx, r, tree),
		}, nil
	}
	if op.Status == GitOpUncertain {
		return DiscardResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return DiscardResult{}, replayFailureError(op)
}

func (s *Service) replaySmartCheckoutOperation(ctx context.Context, repositoryID, worktreeID string, op GitOperation) (SmartCheckoutResult, error) {
	if op.Status == GitOpUncertain {
		return SmartCheckoutResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	if op.Status == GitOpFailed && op.Phase != SmartCheckoutPhaseRolledBack {
		return SmartCheckoutResult{}, replayFailureError(op)
	}
	loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return SmartCheckoutResult{}, err
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return SmartCheckoutResult{}, err
	}
	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	if err := acquire(readCtx, r); err != nil {
		return SmartCheckoutResult{}, err
	}
	defer release(r)

	inProg, _ := s.inspectInProgressLocked(readCtx, loc)
	tree, _ := s.currentTree(readCtx, r, worktreeID)
	stashOID := firstNonEmpty(op.StashOID, op.Preview.StashOID)
	stashRef := s.findStashRefByOID(readCtx, r.entry.AnchorPath, stashOID)

	outcome := SmartCheckoutCompleted
	switch {
	case op.Phase == SmartCheckoutPhaseRolledBack:
		outcome = SmartCheckoutRolledBack
	case op.Status == GitOpNeedsReview || strings.Contains(op.Phase, "conflict"):
		outcome = SmartCheckoutConflict
	}
	dropped := stashRef == "" && op.RecoveryCommand == ""
	return SmartCheckoutResult{
		OperationID:     op.ID,
		Outcome:         outcome,
		Phase:           firstNonEmpty(op.Phase, outcome),
		Summary:         op.Summary,
		StashOID:        stashOID,
		StashRef:        stashRef,
		StashDropped:    dropped,
		StashPreserved:  !dropped,
		StashRestored:   outcome == SmartCheckoutCompleted || (outcome == SmartCheckoutRolledBack && op.RecoveryCommand == ""),
		RecoveryCommand: op.RecoveryCommand,
		InProgress:      inProg,
		Worktree:        s.withLastCommit(readCtx, r, tree),
	}, nil
}
