package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/session"
)

// BranchRenamePreviewRequest previews renaming a local branch.
type BranchRenamePreviewRequest struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

// BranchRenamePreview presents impacted checked-out worktrees, sessions, and upstream migration.
type BranchRenamePreview struct {
	ConfirmationID    string         `json:"confirmationId,omitempty"`
	ExpiresAt         *time.Time     `json:"expiresAt,omitempty"`
	RepositoryID      string         `json:"repositoryId"`
	OldName           string         `json:"oldName"`
	NewName           string         `json:"newName"`
	HeadOID           string         `json:"headOid"`
	Subject           string         `json:"subject"`
	Upstream          string         `json:"upstream,omitempty"`
	AffectedWorktrees []Worktree     `json:"affectedWorktrees"`
	Sessions          []session.Info `json:"sessions"`
	Blockers          []Issue        `json:"blockers"`
	Warnings          []Issue        `json:"warnings"`
}

type branchRenameState struct {
	oldName           string
	newName           string
	headOID           string
	subject           string
	upstream          string
	affectedWorktrees []Worktree
	sessions          []session.Info
	blockers          []Issue
	warnings          []Issue
}

// RenameBranchRequest executes a confirmed local branch rename.
type RenameBranchRequest struct {
	ConfirmationID      string `json:"confirmationId"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
}

// BranchMutationResult returns the updated repository and branch catalog after a branch mutation.
type BranchMutationResult struct {
	OperationID string     `json:"operationId,omitempty"`
	Branch      string     `json:"branch"`
	Upstream    string     `json:"upstream,omitempty"`
	Repository  Repository `json:"repository"`
	Branches    Branches   `json:"branches"`
}

// BranchUpstreamRequest sets or unsets the upstream tracking branch for a local branch.
type BranchUpstreamRequest struct {
	Branch          string `json:"branch"`
	Upstream        string `json:"upstream,omitempty"` // e.g. "origin/main", or empty to unset
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// PreviewBranchRename inspects a local branch rename before execution.
func (s *Service) PreviewBranchRename(ctx context.Context, repositoryID string, req BranchRenamePreviewRequest) (BranchRenamePreview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	r, err := s.repository(repositoryID)
	if err != nil {
		return BranchRenamePreview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return BranchRenamePreview{}, err
	}
	defer release(r)
	state, err := s.buildBranchRenameStateLocked(ctx, r, req.OldName, req.NewName)
	if err != nil {
		return BranchRenamePreview{}, err
	}
	out := BranchRenamePreview{
		RepositoryID:      repositoryID,
		OldName:           state.oldName,
		NewName:           state.newName,
		HeadOID:           state.headOID,
		Subject:           state.subject,
		Upstream:          state.upstream,
		AffectedWorktrees: state.affectedWorktrees,
		Sessions:          state.sessions,
		Blockers:          state.blockers,
		Warnings:          state.warnings,
	}
	if len(state.blockers) > 0 {
		return out, nil
	}
	id, expires, err := s.confirm(confirmation{
		purpose:      purposeBranchRename,
		repositoryID: repositoryID,
		branchRename: &state,
	})
	if err != nil {
		return BranchRenamePreview{}, err
	}
	out.ConfirmationID = id
	out.ExpiresAt = &expires
	return out, nil
}

func (s *Service) buildBranchRenameStateLocked(ctx context.Context, r *repositoryState, rawOld, rawNew string) (branchRenameState, error) {
	oldName := strings.TrimSpace(rawOld)
	newName := strings.TrimSpace(rawNew)
	if oldName == "" {
		return branchRenameState{}, problemf(ErrInvalid, "原分支名称不能为空")
	}
	if newName == "" {
		return branchRenameState{}, problemf(ErrInvalid, "新分支名称不能为空")
	}
	repo, err := s.current(ctx, r)
	if err != nil {
		return branchRenameState{}, err
	}
	anchor := r.entry.AnchorPath
	if err := s.validBranch(ctx, anchor, oldName); err != nil {
		return branchRenameState{}, err
	}
	headOID, err := s.resolveCommit(ctx, anchor, "refs/heads/"+oldName)
	if err != nil {
		return branchRenameState{}, problemf(ErrNotFound, "本地分支 %s 不存在", oldName)
	}
	subject := s.readCommitSubjectBestEffort(ctx, anchor, headOID)

	blockers := make([]Issue, 0)
	warnings := make([]Issue, 0)

	if err := s.validBranch(ctx, anchor, newName); err != nil {
		blockers = append(blockers, Issue{Code: "invalid_name", Message: err.Error()})
	} else if oldName == newName {
		blockers = append(blockers, Issue{Code: "same_name", Message: "新分支名称与当前分支名称相同"})
	} else if existingOID, eErr := s.resolveCommit(ctx, anchor, "refs/heads/"+newName); eErr == nil && existingOID != "" {
		blockers = append(blockers, Issue{
			Code:    "branch_exists",
			Message: fmt.Sprintf("本地分支 %s 已存在（指向 %s），拒绝覆盖现有分支", newName, shortOID(existingOID)),
		})
	}

	var upstream string
	rem, _, _ := s.configValue(ctx, anchor, "branch."+oldName+".remote")
	mergeRef, _, _ := s.configValue(ctx, anchor, "branch."+oldName+".merge")
	if rem != "" && strings.HasPrefix(mergeRef, "refs/heads/") {
		upstream = rem + "/" + strings.TrimPrefix(mergeRef, "refs/heads/")
	}

	affected := make([]Worktree, 0)
	sessions := make([]session.Info, 0)
	seenSess := make(map[string]bool)
	for _, tree := range repo.Worktrees {
		if tree.Branch != nil && *tree.Branch == oldName {
			affected = append(affected, tree)
			if tree.Operation != nil {
				blockers = append(blockers, Issue{
					Code:    "worktree_operation",
					Message: fmt.Sprintf("检出该分支的工作树 %s 正在进行 %s 操作，请先完成或中止后再重命名", tree.Path, *tree.Operation),
				})
			}
			for _, sess := range s.targetSessions(tree.Path) {
				if !seenSess[sess.ID] {
					seenSess[sess.ID] = true
					sessions = append(sessions, sess)
				}
			}
		}
	}

	if len(affected) > 0 {
		warnings = append(warnings, Issue{
			Code:    "checked_out_worktree",
			Message: fmt.Sprintf("将同步更新 %d 个已检出该分支的工作树 HEAD 引用指向 %s", len(affected), newName),
		})
	}
	if upstream != "" {
		warnings = append(warnings, Issue{
			Code:    "upstream_migrated",
			Message: fmt.Sprintf("分支上游跟踪配置（%s）将随本地分支重命名保留，不会重命名远端分支", upstream),
		})
	}
	if len(sessions) > 0 {
		warnings = append(warnings, Issue{
			Code:    "active_sessions",
			Message: fmt.Sprintf("关联工作树中存在 %d 个活跃终端或代理会话，它们的当前分支名将立即切换为 %s", len(sessions), newName),
		})
	}

	return branchRenameState{
		oldName:           oldName,
		newName:           newName,
		headOID:           headOID,
		subject:           subject,
		upstream:          upstream,
		affectedWorktrees: affected,
		sessions:          sessions,
		blockers:          blockers,
		warnings:          warnings,
	}, nil
}

// RenameBranch executes a confirmed local branch rename (`git branch -m`).
func (s *Service) RenameBranch(ctx context.Context, repositoryID string, req RenameBranchRequest) (BranchMutationResult, error) {
	reqHash := hashOperationRequest(GitOpBranchRename, repositoryID, req.ConfirmationID, fmtBool(req.AcknowledgeSessions))
	return runIdempotentOperation(
		ctx, s, repositoryID, req.ClientRequestID, GitOpBranchRename, reqHash,
		func(op GitOperation) (BranchMutationResult, error) {
			return s.replayBranchMutationOperation(ctx, repositoryID, op)
		},
		func(opID string) (BranchMutationResult, error) {
			return s.renameBranchOnce(ctx, repositoryID, req, opID, reqHash)
		},
	)
}

func (s *Service) renameBranchOnce(ctx context.Context, repositoryID string, req RenameBranchRequest, opID, reqHash string) (BranchMutationResult, error) {
	if req.ConfirmationID == "" {
		return BranchMutationResult{}, problemf(ErrInvalid, "缺少重命名分支确认凭证")
	}
	if s.gitPath == "" {
		return BranchMutationResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	c, ok := s.takeConfirmation(req.ConfirmationID, purposeBranchRename, repositoryID, "")
	if !ok || c.branchRename == nil {
		return BranchMutationResult{}, problemf(ErrStale, "重命名分支确认已过期或失效，请重新预览")
	}
	confirmed := *c.branchRename

	r, err := s.repository(repositoryID)
	if err != nil {
		return BranchMutationResult{}, err
	}
	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return BranchMutationResult{}, err
	}
	defer release(r)

	current, err := s.buildBranchRenameStateLocked(wCtx, r, confirmed.oldName, confirmed.newName)
	if err != nil {
		return BranchMutationResult{}, err
	}
	if len(current.blockers) > 0 {
		return BranchMutationResult{}, problemf(ErrBlocked, "重命名分支被阻止：%s；请重新预览", issueMessages(current.blockers))
	}
	if current.headOID != confirmed.headOID || !sameSessions(confirmed.sessions, current.sessions) {
		return BranchMutationResult{}, problemf(ErrStale, "分支提交或活跃会话已变化，请重新预览")
	}
	if len(current.sessions) > 0 && !req.AcknowledgeSessions {
		return BranchMutationResult{}, problemf(ErrBlocked, "存在活跃会话，必须确认已知晓影响后才能重命名分支")
	}

	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    repositoryID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpBranchRename,
		Status:          GitOpRunning,
		ConfirmationID:  req.ConfirmationID,
		StartedAt:       time.Now().UTC(),
		Summary:         fmt.Sprintf("正在重命名分支 %s -> %s", confirmed.oldName, confirmed.newName),
		Preview: GitOperationPreview{
			Branch:  confirmed.oldName,
			HeadOID: confirmed.headOID,
			Target:  confirmed.newName,
		},
	}); err != nil {
		return BranchMutationResult{}, err
	}

	runCwd := r.entry.AnchorPath
	if len(current.affectedWorktrees) > 0 && current.affectedWorktrees[0].Path != "" {
		runCwd = current.affectedWorktrees[0].Path
	}
	run := s.runNonInteractiveMutatingGit(ctx, runCwd, "webmux-branch-rename:"+opID, "branch", "-m", "--end-of-options", confirmed.oldName, confirmed.newName)
	if run.err != nil {
		retErr := classifyGitMutateError("branch -m", run)
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = retErr.Error()
			op.SanitizedOutput = run.stderr
		})
		return BranchMutationResult{}, retErr
	}

	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	repo := s.readRepository(refreshCtx, r)
	branches, bErr := s.branches(refreshCtx, r, repo)
	if bErr != nil {
		branches = Branches{Branches: []Branch{}, Bases: []Base{}, Tags: []Tag{}}
	}
	summary := fmt.Sprintf("已将本地分支 %s 重命名为 %s", confirmed.oldName, confirmed.newName)
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.ActualOID = confirmed.headOID
		op.Subject = confirmed.subject
		op.Summary = summary
		op.SanitizedOutput = run.stderr
	})
	return BranchMutationResult{
		OperationID: opID,
		Branch:      confirmed.newName,
		Upstream:    confirmed.upstream,
		Repository:  repo,
		Branches:    branches,
	}, nil
}

// SetBranchUpstream binds, changes, or unsets the upstream tracking branch of a local branch.
func (s *Service) SetBranchUpstream(ctx context.Context, repositoryID string, req BranchUpstreamRequest) (BranchMutationResult, error) {
	branch := strings.TrimSpace(req.Branch)
	upstream := strings.TrimSpace(req.Upstream)
	reqHash := hashOperationRequest(GitOpBranchUpstream, repositoryID, branch, upstream)
	return runIdempotentOperation(
		ctx, s, repositoryID, req.ClientRequestID, GitOpBranchUpstream, reqHash,
		func(op GitOperation) (BranchMutationResult, error) {
			return s.replayBranchMutationOperation(ctx, repositoryID, op)
		},
		func(opID string) (BranchMutationResult, error) {
			return s.setBranchUpstreamOnce(ctx, repositoryID, branch, upstream, req.ClientRequestID, opID, reqHash)
		},
	)
}

func (s *Service) setBranchUpstreamOnce(ctx context.Context, repositoryID, branch, upstream, clientRequestID, opID, reqHash string) (BranchMutationResult, error) {
	if branch == "" {
		return BranchMutationResult{}, problemf(ErrInvalid, "分支名称不能为空")
	}
	if s.gitPath == "" {
		return BranchMutationResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return BranchMutationResult{}, err
	}
	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return BranchMutationResult{}, err
	}
	defer release(r)
	if err := s.verifyIdentity(wCtx, r); err != nil {
		return BranchMutationResult{}, err
	}
	anchor := r.entry.AnchorPath
	if err := s.validBranch(wCtx, anchor, branch); err != nil {
		return BranchMutationResult{}, err
	}
	headOID, err := s.resolveCommit(wCtx, anchor, "refs/heads/"+branch)
	if err != nil {
		return BranchMutationResult{}, problemf(ErrNotFound, "本地分支 %s 不存在", branch)
	}

	var targetOID string
	if upstream != "" {
		if strings.HasPrefix(upstream, "-") || strings.ContainsAny(upstream, " \t\r\n\x00") || !strings.Contains(upstream, "/") {
			return BranchMutationResult{}, problemf(ErrInvalid, "上游跟踪分支格式无效，应为 <remote>/<branch>（例如 origin/main）")
		}
		if _, err := s.git(wCtx, anchor, "check-ref-format", "refs/remotes/"+upstream); err != nil {
			return BranchMutationResult{}, problemf(ErrInvalid, "上游跟踪分支引用格式无效")
		}
		targetOID, err = s.resolveCommit(wCtx, anchor, "refs/remotes/"+upstream)
		if err != nil {
			return BranchMutationResult{}, problemf(ErrNotFound, "远端跟踪分支 %s 不存在，请先获取远程（Fetch）后再设置", upstream)
		}
	}

	startSummary := fmt.Sprintf("正在取消分支 %s 的上游跟踪", branch)
	if upstream != "" {
		startSummary = fmt.Sprintf("正在将分支 %s 的上游跟踪设为 %s", branch, upstream)
	}
	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    repositoryID,
		ClientRequestID: clientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpBranchUpstream,
		Status:          GitOpRunning,
		StartedAt:       time.Now().UTC(),
		Summary:         startSummary,
		Preview: GitOperationPreview{
			Branch:    branch,
			HeadOID:   headOID,
			Target:    upstream,
			TargetOID: targetOID,
		},
	}); err != nil {
		return BranchMutationResult{}, err
	}

	var run gitExit
	if upstream != "" {
		run = s.runNonInteractiveMutatingGit(ctx, anchor, "webmux-upstream:"+opID, "branch", "--set-upstream-to=refs/remotes/"+upstream, "--", branch)
	} else {
		rem, _, _ := s.configValue(wCtx, anchor, "branch."+branch+".remote")
		mergeRef, _, _ := s.configValue(wCtx, anchor, "branch."+branch+".merge")
		if rem != "" || mergeRef != "" {
			run = s.runNonInteractiveMutatingGit(ctx, anchor, "webmux-upstream:"+opID, "branch", "--unset-upstream", "--", branch)
		}
	}
	if run.err != nil {
		retErr := classifyGitMutateError("branch upstream", run)
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = retErr.Error()
			op.SanitizedOutput = run.stderr
		})
		return BranchMutationResult{}, retErr
	}

	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	repo := s.readRepository(refreshCtx, r)
	branches, bErr := s.branches(refreshCtx, r, repo)
	if bErr != nil {
		branches = Branches{Branches: []Branch{}, Bases: []Base{}, Tags: []Tag{}}
	}
	summary := fmt.Sprintf("已取消分支 %s 的上游跟踪", branch)
	if upstream != "" {
		summary = fmt.Sprintf("已将分支 %s 的上游跟踪设为 %s", branch, upstream)
	}
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.ActualOID = headOID
		op.Summary = summary
		op.SanitizedOutput = run.stderr
	})
	return BranchMutationResult{
		OperationID: opID,
		Branch:      branch,
		Upstream:    upstream,
		Repository:  repo,
		Branches:    branches,
	}, nil
}
