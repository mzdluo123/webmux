package workspace

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// PreviewStashApply inspects the target worktree before applying or popping a stash entry.
func (s *Service) PreviewStashApply(ctx context.Context, req StashApplyPreviewRequest) (StashApplyPreview, error) {
	if req.Mode != "apply" && req.Mode != "pop" {
		return StashApplyPreview{}, problemf(ErrInvalid, "mode 必须为 apply 或 pop")
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return StashApplyPreview{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return StashApplyPreview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return StashApplyPreview{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return StashApplyPreview{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	state, err := s.buildStashApplyStateLocked(ctx, r.entry.AnchorPath, loc, req)
	if err != nil {
		return StashApplyPreview{}, err
	}
	out := StashApplyPreview{
		RepositoryID:     req.RepositoryID,
		WorktreeID:       req.WorktreeID,
		WorktreePath:     state.worktreePath,
		Mode:             state.mode,
		Entry:            state.entry,
		ReinstateIndex:   state.reinstateIndex,
		CurrentBranch:    state.currentBranch,
		Detached:         state.detached,
		HeadOID:          state.headOID,
		HeadSubject:      state.headSubject,
		StagedCount:      state.stagedCount,
		UnstagedCount:    state.unstagedCount,
		UntrackedCount:   state.untrackedCount,
		OverlappingPaths: state.overlappingPaths,
		Sessions:         state.sessions,
		Blockers:         state.blockers,
		Warnings:         state.warnings,
	}
	if len(state.blockers) > 0 {
		return out, nil
	}
	p := purposeStashApply
	if req.Mode == "pop" {
		p = purposeStashPop
	}
	id, expires, err := s.confirm(confirmation{
		purpose:      p,
		repositoryID: req.RepositoryID,
		worktreeID:   req.WorktreeID,
		stashApply:   &state,
	})
	if err != nil {
		return StashApplyPreview{}, err
	}
	out.ConfirmationID = id
	out.ExpiresAt = &expires
	return out, nil
}

func (s *Service) buildStashApplyStateLocked(ctx context.Context, anchor string, loc WorktreeLocation, req StashApplyPreviewRequest) (stashApplyState, error) {
	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return stashApplyState{}, err
	}
	entry, err := s.resolveStashSlotLocked(ctx, anchor, req.StashRef, req.StashOID)
	if err != nil {
		return stashApplyState{}, err
	}
	detail, err := s.inspectStashLocked(ctx, anchor, entry.OID)
	if err != nil {
		return stashApplyState{}, err
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return stashApplyState{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}
	branch, detached, err := s.symbolicRefHead(ctx, loc.Path)
	if err != nil {
		return stashApplyState{}, err
	}
	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")
	headSubject := s.readCommitSubjectBestEffort(ctx, loc.Path, headOID)
	overview, err := s.inspectStagingLocked(ctx, loc)
	if err != nil {
		return stashApplyState{}, err
	}
	unstagedList, untrackedList := splitUnstagedAndUntracked(overview.Unstaged)
	sessions := s.targetSessions(loc.Path)
	blockers := make([]Issue, 0)
	warnings := make([]Issue, 0)

	if lErr := checkWorktreeIndexLock(gitDir); lErr != nil {
		blockers = append(blockers, Issue{Code: "locked", Message: lErr.Error()})
	}
	if headOID == "" {
		blockers = append(blockers, Issue{Code: "unborn_head", Message: "当前分支尚无初始提交（unborn HEAD），无法应用贮藏"})
	}
	if op := detectOperation(loc.Path); op != nil {
		blockers = append(blockers, Issue{Code: "operation_in_progress", Message: fmt.Sprintf("工作树正在进行未完成的 %s 操作，请先完成或中止", *op)})
	}
	if len(overview.Conflicted) > 0 {
		blockers = append(blockers, Issue{Code: "conflicted", Message: fmt.Sprintf("工作树存在 %d 个未解决冲突文件，请先解决冲突后再应用贮藏", len(overview.Conflicted))})
	}
	if req.ReinstateIndex && len(overview.Staged) > 0 {
		blockers = append(blockers, Issue{Code: "staged_with_index", Message: "当前暂存区已有变更，恢复暂存状态（--index）会被 Git 拒绝，请先清空暂存区或关闭恢复暂存选项"})
	}

	dirtyPaths := make(map[string]bool)
	untrackedPaths := make(map[string]bool)
	for _, f := range overview.Staged {
		dirtyPaths[f.Path] = true
	}
	for _, f := range unstagedList {
		dirtyPaths[f.Path] = true
	}
	for _, f := range untrackedList {
		dirtyPaths[f.Path] = true
		untrackedPaths[f.Path] = true
	}

	overlapSet := make(map[string]bool)
	untrackedCollision := make([]string, 0)
	for _, f := range detail.Files {
		if dirtyPaths[f.NewPath] {
			overlapSet[f.NewPath] = true
		}
		if f.Untracked && untrackedPaths[f.NewPath] {
			untrackedCollision = append(untrackedCollision, f.NewPath)
		}
	}
	if len(untrackedCollision) > 0 {
		sort.Strings(untrackedCollision)
		blockers = append(blockers, Issue{
			Code:    "untracked_collision",
			Message: "贮藏中的未跟踪文件与工作树现有未跟踪文件重名，Git 会拒绝覆盖: " + strings.Join(untrackedCollision, ", "),
		})
	}

	overlaps := make([]string, 0, len(overlapSet))
	for p := range overlapSet {
		overlaps = append(overlaps, p)
	}
	sort.Strings(overlaps)

	if len(overlaps) > 0 {
		warnings = append(warnings, Issue{
			Code:    "overlapping_changes",
			Message: fmt.Sprintf("目标工作树有 %d 个本地修改文件与该贮藏重叠，应用时可能产生冲突或被 Git 拒绝覆盖", len(overlaps)),
		})
	} else if len(overview.Staged)+len(unstagedList) > 0 {
		warnings = append(warnings, Issue{
			Code:    "dirty_worktree",
			Message: fmt.Sprintf("目标工作树当前包含 %d 项本地修改，若发生冲突将保留原贮藏条目不丢弃", len(overview.Staged)+len(unstagedList)),
		})
	}
	if req.Mode == "pop" {
		warnings = append(warnings, Issue{
			Code:    "safe_two_step_pop",
			Message: "安全两步 Pop：仅当贮藏干净无冲突地应用成功后才会自动移除该贮藏条目；一旦发生冲突或错误将原样保留贮藏",
		})
	}
	if len(sessions) > 0 {
		warnings = append(warnings, Issue{
			Code:    "active_sessions",
			Message: fmt.Sprintf("该工作树有 %d 个活跃终端或代理会话，应用贮藏会修改工作区文件", len(sessions)),
		})
	}

	return stashApplyState{
		commonDir:        loc.CommonDir,
		worktreePath:     loc.Path,
		mode:             req.Mode,
		stashRef:         entry.StashRef,
		stashOID:         entry.OID,
		entry:            entry,
		reinstateIndex:   req.ReinstateIndex,
		currentBranch:    branch,
		detached:         detached,
		headOID:          headOID,
		headSubject:      headSubject,
		stagedCount:      len(overview.Staged),
		unstagedCount:    len(unstagedList),
		untrackedCount:   len(untrackedList),
		overlappingPaths: overlaps,
		sessions:         sessions,
		blockers:         blockers,
		warnings:         warnings,
	}, nil
}

// ExecuteStashApply applies a confirmed stash entry without dropping it.
func (s *Service) ExecuteStashApply(ctx context.Context, req ExecuteStashApplyRequest) (StashApplyResult, error) {
	return s.executeStashApplyOrPop(ctx, GitOpStashApply, purposeStashApply, false, req)
}

// ExecuteStashPop executes a safe two-step Pop: applies the stash first and drops
// the stash entry ONLY when the working tree and index apply cleanly with zero conflicts.
func (s *Service) ExecuteStashPop(ctx context.Context, req ExecuteStashApplyRequest) (StashApplyResult, error) {
	return s.executeStashApplyOrPop(ctx, GitOpStashPop, purposeStashPop, true, req)
}

func (s *Service) executeStashApplyOrPop(ctx context.Context, kind string, p purpose, dropOnClean bool, req ExecuteStashApplyRequest) (StashApplyResult, error) {
	reqHash := hashOperationRequest(kind, req.RepositoryID, req.WorktreeID, req.ConfirmationID, fmtBool(req.AcknowledgeSessions))
	return runIdempotentOperation(
		ctx, s, req.RepositoryID, req.ClientRequestID, kind, reqHash,
		func(op GitOperation) (StashApplyResult, error) {
			return s.replayStashApplyOperation(ctx, req.RepositoryID, req.WorktreeID, op)
		},
		func(opID string) (StashApplyResult, error) {
			return s.executeStashApplyOnce(ctx, kind, p, dropOnClean, req, opID, reqHash)
		},
	)
}

func (s *Service) executeStashApplyOnce(ctx context.Context, kind string, p purpose, dropOnClean bool, req ExecuteStashApplyRequest, opID, reqHash string) (StashApplyResult, error) {
	if req.ConfirmationID == "" {
		return StashApplyResult{}, problemf(ErrInvalid, "缺少贮藏应用确认凭证")
	}
	if s.gitPath == "" {
		return StashApplyResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	c, ok := s.takeConfirmation(req.ConfirmationID, p, req.RepositoryID, req.WorktreeID)
	if !ok || c.stashApply == nil {
		return StashApplyResult{}, problemf(ErrStale, "贮藏预览确认已过期或失效，请重新预览")
	}
	confirmed := *c.stashApply

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return StashApplyResult{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return StashApplyResult{}, err
	}
	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return StashApplyResult{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(wCtx, loc.Path); err != nil {
		return StashApplyResult{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	current, err := s.buildStashApplyStateLocked(wCtx, r.entry.AnchorPath, loc, StashApplyPreviewRequest{
		RepositoryID:   req.RepositoryID,
		WorktreeID:     req.WorktreeID,
		Mode:           confirmed.mode,
		StashRef:       confirmed.stashRef,
		StashOID:       confirmed.stashOID,
		ReinstateIndex: confirmed.reinstateIndex,
	})
	if err != nil {
		return StashApplyResult{}, err
	}
	if len(current.blockers) > 0 {
		return StashApplyResult{}, problemf(ErrBlocked, "应用贮藏被阻止：%s；请重新预览", issueMessages(current.blockers))
	}
	if current.commonDir != confirmed.commonDir ||
		current.worktreePath != confirmed.worktreePath ||
		current.currentBranch != confirmed.currentBranch ||
		current.headOID != confirmed.headOID ||
		!sameSessions(confirmed.sessions, current.sessions) {
		return StashApplyResult{}, problemf(ErrStale, "工作树分支、HEAD 提交或活跃会话已变化，请重新预览")
	}
	if len(current.sessions) > 0 && !req.AcknowledgeSessions {
		return StashApplyResult{}, problemf(ErrBlocked, "存在活跃会话，必须确认已知晓风险后才能应用贮藏")
	}

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
		Summary:         fmt.Sprintf("正在应用贮藏 %s", confirmed.stashRef),
		Preview: GitOperationPreview{
			WorktreePath: loc.Path,
			Branch:       confirmed.currentBranch,
			HeadOID:      confirmed.headOID,
			StashRef:     confirmed.stashRef,
			StashOID:     confirmed.stashOID,
		},
	}); err != nil {
		return StashApplyResult{}, err
	}

	applyArgs := []string{"stash", "apply", "--quiet"}
	if confirmed.reinstateIndex {
		applyArgs = append(applyArgs, "--index")
	}
	applyArgs = append(applyArgs, "--end-of-options", confirmed.stashOID)
	run := s.runNonInteractiveMutatingGit(ctx, loc.Path, "webmux-stash-apply:"+opID, applyArgs...)

	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	conflicted, _ := s.listConflictedFilesLocked(refreshCtx, loc.Path)

	// Conflict halt: NEVER drop the stash entry! Record unstash source ref so
	// GitConflictView displays accurate stage 2 / stage 3 labels.
	if len(conflicted) > 0 {
		s.writeUnstashRefBestEffort(refreshCtx, loc.Path, loc.WorktreeID, confirmed.stashOID)
		inProg, _ := s.inspectInProgressLocked(refreshCtx, loc)
		afterTree := s.refreshWorktreeLocked(refreshCtx, r, req.WorktreeID)
		summary := fmt.Sprintf("应用贮藏 %s 遇到 %d 个冲突文件，原贮藏条目 %s 已完整保留，请在冲突视图中解决", confirmed.stashRef, len(conflicted), shortOID(confirmed.stashOID))
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpNeedsReview
			op.FailureKind = "conflicted"
			op.ActualOID = confirmed.stashOID
			op.Summary = summary
			op.ReviewReasons = []string{summary}
			op.SanitizedOutput = run.stderr
		})
		return StashApplyResult{
			OperationID:    opID,
			Kind:           kind,
			Outcome:        StashApplyConflicted,
			StashRef:       confirmed.stashRef,
			StashOID:       confirmed.stashOID,
			StashDropped:   false,
			StashPreserved: true,
			Summary:        summary,
			InProgress:     inProg,
			Worktree:       afterTree,
		}, nil
	}

	if run.err != nil {
		s.readRepository(refreshCtx, r)
		retErr := classifyGitMutateError("stash apply", run)
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = retErr.Error()
			op.SanitizedOutput = run.stderr
		})
		return StashApplyResult{}, retErr
	}

	s.clearUnstashRefBestEffort(refreshCtx, loc.Path, loc.WorktreeID)
	dropped := false
	preserved := true
	outcome := StashApplyCompleted
	ledgerStatus := GitOpSucceeded
	summary := fmt.Sprintf("已应用贮藏 %s (%s)，贮藏条目保留", confirmed.stashRef, shortOID(confirmed.stashOID))
	var reviewReasons []string

	if dropOnClean {
		slot, slotErr := s.resolveStashSlotLocked(refreshCtx, r.entry.AnchorPath, confirmed.stashRef, confirmed.stashOID)
		if slotErr != nil {
			outcome = StashApplyNeedsReview
			ledgerStatus = GitOpNeedsReview
			summary = fmt.Sprintf("贮藏 %s 的变更已干净应用，但贮藏列表发生偏移未自动删除条目，请手动确认", shortOID(confirmed.stashOID))
			reviewReasons = []string{summary}
		} else {
			dropRun := s.runNonInteractiveMutatingGit(ctx, r.entry.AnchorPath, "webmux-stash-drop:"+opID, "stash", "drop", "--quiet", "refs/"+slot.StashRef)
			if dropRun.err != nil {
				outcome = StashApplyNeedsReview
				ledgerStatus = GitOpNeedsReview
				summary = fmt.Sprintf("贮藏 %s 的变更已干净应用，但移除原贮藏条目失败，已安全保留条目", slot.StashRef)
				reviewReasons = []string{summary}
			} else {
				dropped = true
				preserved = false
				summary = fmt.Sprintf("已弹出并移除贮藏 %s (%s)", slot.StashRef, shortOID(confirmed.stashOID))
			}
		}
	}

	inProg, _ := s.inspectInProgressLocked(refreshCtx, loc)
	afterTree := s.refreshWorktreeLocked(refreshCtx, r, req.WorktreeID)

	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = ledgerStatus
		op.ActualOID = confirmed.stashOID
		op.Summary = summary
		op.ReviewReasons = reviewReasons
		op.SanitizedOutput = run.stderr
	})
	return StashApplyResult{
		OperationID:    opID,
		Kind:           kind,
		Outcome:        outcome,
		StashRef:       confirmed.stashRef,
		StashOID:       confirmed.stashOID,
		StashDropped:   dropped,
		StashPreserved: preserved,
		Summary:        summary,
		InProgress:     inProg,
		Worktree:       afterTree,
	}, nil
}
