package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// PreviewStashCreate inspects the target worktree and creates a short-lived confirmation for stash push.
func (s *Service) PreviewStashCreate(ctx context.Context, req StashCreatePreviewRequest) (StashCreatePreview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return StashCreatePreview{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return StashCreatePreview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return StashCreatePreview{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return StashCreatePreview{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	state, err := s.buildStashCreateStateLocked(ctx, loc, req)
	if err != nil {
		return StashCreatePreview{}, err
	}
	out := StashCreatePreview{
		RepositoryID:     req.RepositoryID,
		WorktreeID:       req.WorktreeID,
		WorktreePath:     state.worktreePath,
		CurrentBranch:    state.currentBranch,
		Detached:         state.detached,
		HeadOID:          state.headOID,
		HeadSubject:      state.headSubject,
		Message:          state.message,
		IncludeUntracked: state.includeUntracked,
		KeepIndex:        state.keepIndex,
		StagedCount:      state.stagedCount,
		UnstagedCount:    state.unstagedCount,
		UntrackedCount:   state.untrackedCount,
		AffectedPaths:    state.affectedPaths,
		Sessions:         state.sessions,
		Blockers:         state.blockers,
		Warnings:         state.warnings,
	}
	if len(state.blockers) > 0 {
		return out, nil
	}
	id, expires, err := s.confirm(confirmation{
		purpose:      purposeStashCreate,
		repositoryID: req.RepositoryID,
		worktreeID:   req.WorktreeID,
		stashCreate:  &state,
	})
	if err != nil {
		return StashCreatePreview{}, err
	}
	out.ConfirmationID = id
	out.ExpiresAt = &expires
	return out, nil
}

func splitUnstagedAndUntracked(entries []StagingEntry) (unstaged, untracked []StagingEntry) {
	for _, entry := range entries {
		if entry.Status == "?" {
			untracked = append(untracked, entry)
		} else {
			unstaged = append(unstaged, entry)
		}
	}
	return unstaged, untracked
}

func (s *Service) buildStashCreateStateLocked(ctx context.Context, loc WorktreeLocation, req StashCreatePreviewRequest) (stashCreateState, error) {
	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return stashCreateState{}, err
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return stashCreateState{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}
	branch, detached, err := s.symbolicRefHead(ctx, loc.Path)
	if err != nil {
		return stashCreateState{}, err
	}
	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")
	headSubject := s.readCommitSubjectBestEffort(ctx, loc.Path, headOID)

	overview, err := s.inspectStagingLocked(ctx, loc)
	if err != nil {
		return stashCreateState{}, err
	}
	unstagedList, untrackedList := splitUnstagedAndUntracked(overview.Unstaged)
	sessions := s.targetSessions(loc.Path)
	blockers := make([]Issue, 0)
	warnings := make([]Issue, 0)

	if lErr := checkWorktreeIndexLock(gitDir); lErr != nil {
		blockers = append(blockers, Issue{Code: "locked", Message: lErr.Error()})
	}
	if headOID == "" {
		blockers = append(blockers, Issue{
			Code:    "unborn_head",
			Message: "当前分支尚无初始提交（unborn HEAD），Git 无法创建贮藏",
		})
	}
	if op := detectOperation(loc.Path); op != nil {
		blockers = append(blockers, Issue{
			Code:    "operation_in_progress",
			Message: fmt.Sprintf("工作树正在进行未完成的 %s 操作，请先完成或中止后再创建贮藏", *op),
		})
	}
	if len(overview.Conflicted) > 0 {
		blockers = append(blockers, Issue{
			Code:    "conflicted",
			Message: fmt.Sprintf("工作树存在 %d 个未解决冲突文件，请先解决冲突后再创建贮藏", len(overview.Conflicted)),
		})
	}

	pathSet := make(map[string]bool)
	for _, f := range overview.Staged {
		pathSet[f.Path] = true
	}
	for _, f := range unstagedList {
		pathSet[f.Path] = true
	}
	if req.IncludeUntracked {
		for _, f := range untrackedList {
			pathSet[f.Path] = true
		}
	}
	affected := make([]string, 0, len(pathSet))
	for p := range pathSet {
		affected = append(affected, p)
	}
	sort.Strings(affected)

	if len(overview.Staged)+len(unstagedList) == 0 && (!req.IncludeUntracked || len(untrackedList) == 0) {
		msg := "当前工作树没有可贮藏的已跟踪修改"
		if len(untrackedList) > 0 && !req.IncludeUntracked {
			msg = fmt.Sprintf("当前仅有 %d 个未跟踪文件，勾选“包含未跟踪文件”后可创建贮藏", len(untrackedList))
		}
		blockers = append(blockers, Issue{Code: "nothing_to_stash", Message: msg})
	}
	if !req.IncludeUntracked && len(untrackedList) > 0 {
		warnings = append(warnings, Issue{
			Code:    "untracked_excluded",
			Message: fmt.Sprintf("未勾选“包含未跟踪文件”，当前 %d 个未跟踪文件将保留在工作树中不进入贮藏", len(untrackedList)),
		})
	}
	if req.KeepIndex && len(overview.Staged) > 0 {
		warnings = append(warnings, Issue{
			Code:    "keep_index",
			Message: fmt.Sprintf("已启用保留暂存区（--keep-index）：%d 项已暂存变更会写入贮藏并同时保留在当前暂存区与工作树中", len(overview.Staged)),
		})
	}
	if len(sessions) > 0 {
		warnings = append(warnings, Issue{
			Code:    "active_sessions",
			Message: fmt.Sprintf("该工作树有 %d 个活跃终端或代理会话，贮藏会立即清理工作区文件", len(sessions)),
		})
	}

	return stashCreateState{
		commonDir:        loc.CommonDir,
		worktreePath:     loc.Path,
		currentBranch:    branch,
		detached:         detached,
		headOID:          headOID,
		headSubject:      headSubject,
		message:          strings.TrimSpace(req.Message),
		includeUntracked: req.IncludeUntracked,
		keepIndex:        req.KeepIndex,
		stagedCount:      len(overview.Staged),
		unstagedCount:    len(unstagedList),
		untrackedCount:   len(untrackedList),
		affectedPaths:    affected,
		sessions:         sessions,
		blockers:         blockers,
		warnings:         warnings,
	}, nil
}

// ExecuteStashCreate runs a confirmed `git stash push` on the target worktree.
func (s *Service) ExecuteStashCreate(ctx context.Context, req ExecuteStashCreateRequest) (StashCreateResult, error) {
	reqHash := hashOperationRequest(GitOpStashPush, req.RepositoryID, req.WorktreeID, req.ConfirmationID, fmtBool(req.AcknowledgeSessions))
	return runIdempotentOperation(
		ctx, s, req.RepositoryID, req.ClientRequestID, GitOpStashPush, reqHash,
		func(op GitOperation) (StashCreateResult, error) {
			return s.replayStashCreateOperation(ctx, req.RepositoryID, req.WorktreeID, op)
		},
		func(opID string) (StashCreateResult, error) {
			return s.executeStashCreateOnce(ctx, req, opID, reqHash)
		},
	)
}

func (s *Service) executeStashCreateOnce(ctx context.Context, req ExecuteStashCreateRequest, opID, reqHash string) (StashCreateResult, error) {
	if req.ConfirmationID == "" {
		return StashCreateResult{}, problemf(ErrInvalid, "缺少贮藏预览确认凭证")
	}
	if s.gitPath == "" {
		return StashCreateResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	c, ok := s.takeConfirmation(req.ConfirmationID, purposeStashCreate, req.RepositoryID, req.WorktreeID)
	if !ok || c.stashCreate == nil {
		return StashCreateResult{}, problemf(ErrStale, "贮藏预览确认已过期或失效，请重新预览")
	}
	confirmed := *c.stashCreate

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return StashCreateResult{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return StashCreateResult{}, err
	}
	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return StashCreateResult{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(wCtx, loc.Path); err != nil {
		return StashCreateResult{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	current, err := s.buildStashCreateStateLocked(wCtx, loc, StashCreatePreviewRequest{
		RepositoryID:     req.RepositoryID,
		WorktreeID:       req.WorktreeID,
		Message:          confirmed.message,
		IncludeUntracked: confirmed.includeUntracked,
		KeepIndex:        confirmed.keepIndex,
	})
	if err != nil {
		return StashCreateResult{}, err
	}
	if len(current.blockers) > 0 {
		return StashCreateResult{}, problemf(ErrBlocked, "创建贮藏被阻止：%s；请重新预览", issueMessages(current.blockers))
	}
	if current.commonDir != confirmed.commonDir ||
		current.worktreePath != confirmed.worktreePath ||
		current.currentBranch != confirmed.currentBranch ||
		current.headOID != confirmed.headOID ||
		!sameSessions(confirmed.sessions, current.sessions) {
		return StashCreateResult{}, problemf(ErrStale, "工作树分支、HEAD 提交或活跃会话已变化，请重新预览")
	}
	if len(current.sessions) > 0 && !req.AcknowledgeSessions {
		return StashCreateResult{}, problemf(ErrBlocked, "存在活跃会话，必须确认已知晓风险后才能创建贮藏")
	}

	beforeTop, _ := s.resolveCommitReadOnly(wCtx, r.entry.AnchorPath, "refs/stash")
	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    req.RepositoryID,
		WorktreeID:      req.WorktreeID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpStashPush,
		Status:          GitOpRunning,
		ConfirmationID:  req.ConfirmationID,
		StartedAt:       time.Now().UTC(),
		Summary:         "正在创建贮藏",
		Preview: GitOperationPreview{
			WorktreePath: loc.Path,
			Branch:       confirmed.currentBranch,
			HeadOID:      confirmed.headOID,
			StashOID:     beforeTop,
		},
	}); err != nil {
		return StashCreateResult{}, err
	}

	args := []string{"stash", "push", "--quiet"}
	if confirmed.includeUntracked {
		args = append(args, "--include-untracked")
	}
	if confirmed.keepIndex {
		args = append(args, "--keep-index")
	}
	if confirmed.message != "" {
		args = append(args, "-m", confirmed.message)
	}
	run := s.runNonInteractiveMutatingGit(ctx, loc.Path, "webmux-stash-push:"+opID, args...)

	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	afterTree := s.refreshWorktreeLocked(refreshCtx, r, req.WorktreeID)

	if run.err != nil {
		retErr := classifyGitMutateError("stash push", run)
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = retErr.Error()
			op.SanitizedOutput = run.stderr
		})
		return StashCreateResult{}, retErr
	}

	stashes, listErr := s.listStashesLocked(refreshCtx, r.entry.AnchorPath)
	if listErr != nil || len(stashes) == 0 || stashes[0].OID == beforeTop {
		retErr := problemf(ErrBlocked, "当前工作树没有可贮藏的本地更改")
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = "blocked"
			op.Summary = retErr.Error()
			op.SanitizedOutput = run.stderr
		})
		return StashCreateResult{}, retErr
	}

	created := stashes[0]
	summary := fmt.Sprintf("已创建贮藏 %s (%s)", created.StashRef, created.ShortOID)
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.ActualOID = created.OID
		op.Subject = created.Summary
		op.Summary = summary
		op.SanitizedOutput = run.stderr
	})
	return StashCreateResult{
		OperationID: opID,
		Entry:       created,
		Worktree:    afterTree,
	}, nil
}

func (s *Service) refreshWorktreeLocked(ctx context.Context, r *repositoryState, worktreeID string) Worktree {
	afterTree, err := s.currentTree(ctx, r, worktreeID)
	if err == nil {
		afterTree = s.withLastCommit(ctx, r, afterTree)
	}
	s.readRepository(ctx, r)
	return afterTree
}

func classifyGitMutateError(action string, run gitExit) error {
	if isGitLockBusy(run.stderr) {
		return problemf(ErrLocked, "仓库正被其他进程占用（Git 锁冲突），未删除任何锁文件，请稍后重试")
	}
	var launch *exec.Error
	if errors.As(run.err, &launch) || errors.Is(run.err, os.ErrNotExist) || errors.Is(run.err, os.ErrPermission) {
		return problemf(ErrUnavailable, "Git 无法执行: %v", run.err)
	}
	if run.ctxErr != nil {
		return problemf(ErrTimeout, "Git %s 执行超时: %v", action, run.ctxErr)
	}
	if paths, more := overwrittenPaths(run.stderr); len(paths) > 0 {
		msg := "以下本地或未跟踪文件会被覆盖，Git 已拒绝执行以保护工作区数据: " + strings.Join(paths, ", ")
		if more > 0 {
			msg += fmt.Sprintf(" 等另外 %d 个", more)
		}
		return problemf(ErrConflict, "%s", msg)
	}
	return problemf(ErrConflict, "Git %s 执行失败: %s", action, firstGitLine(redactLine(run.stderr)))
}
