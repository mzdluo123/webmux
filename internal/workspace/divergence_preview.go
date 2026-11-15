package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rainchan/webmux/internal/session"
)

// PreviewMerge previews merging a local or remote-tracking branch into the
// target worktree's currently checked-out branch.
func (s *Service) PreviewMerge(ctx context.Context, req DivergencePreviewRequest) (DivergencePreview, error) {
	return s.previewDivergence(ctx, GitOpMerge, purposeMerge, req)
}

// PreviewRebase previews rebasing the target worktree's currently checked-out
// branch onto a selected local or remote-tracking branch.
func (s *Service) PreviewRebase(ctx context.Context, req DivergencePreviewRequest) (DivergencePreview, error) {
	return s.previewDivergence(ctx, GitOpRebase, purposeRebase, req)
}

// PreviewCherryPick previews applying one selected commit onto the target
// worktree's currently checked-out branch.
func (s *Service) PreviewCherryPick(ctx context.Context, req DivergencePreviewRequest) (DivergencePreview, error) {
	return s.previewDivergence(ctx, GitOpCherryPick, purposeCherryPick, req)
}

// PreviewRevert previews creating a revert commit for one selected commit on
// the target worktree's currently checked-out branch.
func (s *Service) PreviewRevert(ctx context.Context, req DivergencePreviewRequest) (DivergencePreview, error) {
	return s.previewDivergence(ctx, GitOpRevert, purposeRevert, req)
}

func (s *Service) previewDivergence(ctx context.Context, kind string, p purpose, req DivergencePreviewRequest) (DivergencePreview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return DivergencePreview{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return DivergencePreview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return DivergencePreview{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return DivergencePreview{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	state, err := s.buildDivergencePreviewState(ctx, r, loc, kind, p, req)
	if err != nil {
		return DivergencePreview{}, err
	}

	preview := DivergencePreview{
		Kind:             kind,
		RepositoryID:     req.RepositoryID,
		WorktreeID:       req.WorktreeID,
		WorktreePath:     state.worktreePath,
		DirectionSummary: state.directionSummary,
		RecoveryHint:     state.recoveryHint,
		CurrentBranch:    state.currentBranch,
		HeadOID:          state.headOID,
		HeadSubject:      state.headSubject,
		TargetRef:        state.targetRef,
		TargetKind:       state.targetKind,
		TargetOID:        state.targetOID,
		TargetSubject:    state.targetSubject,
		MergeBaseOID:     state.mergeBaseOID,
		FastForward:      state.fastForward,
		AlreadyUpToDate:  state.alreadyUpToDate,
		Commits:          state.commits,
		Sessions:         state.sessions,
		Blockers:         state.blockers,
		Warnings:         state.warnings,
	}
	if len(state.blockers) > 0 {
		return preview, nil
	}
	id, expires, err := s.confirm(confirmation{
		purpose:      p,
		repositoryID: req.RepositoryID,
		worktreeID:   req.WorktreeID,
		divergence:   &state,
	})
	if err != nil {
		return DivergencePreview{}, err
	}
	preview.ConfirmationID = id
	preview.ExpiresAt = &expires
	return preview, nil
}

func (s *Service) buildDivergencePreviewState(
	ctx context.Context,
	r *repositoryState,
	loc WorktreeLocation,
	kind string,
	p purpose,
	req DivergencePreviewRequest,
) (divergencePreviewState, error) {
	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return divergencePreviewState{}, err
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return divergencePreviewState{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}

	branch, detached, err := s.symbolicRefHead(ctx, loc.Path)
	if err != nil {
		return divergencePreviewState{}, err
	}
	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")
	headSubject := s.readCommitSubjectBestEffort(ctx, loc.Path, headOID)
	sessions := s.runningSessions(loc.Path)
	if sessions == nil {
		sessions = []session.Info{}
	}

	state := divergencePreviewState{
		kind:          kind,
		purpose:       p,
		commonDir:     loc.CommonDir,
		worktreeID:    loc.WorktreeID,
		worktreePath:  loc.Path,
		currentBranch: branch,
		headOID:       headOID,
		headSubject:   headSubject,
		commits:       CommitList{Items: []Commit{}},
		sessions:      sessions,
		blockers:      []Issue{},
		warnings:      []Issue{},
	}

	if err := s.resolveDivergenceTarget(ctx, r.entry.AnchorPath, kind, req, &state); err != nil {
		return divergencePreviewState{}, err
	}

	if err := checkWorktreeIndexLock(gitDir); err != nil {
		state.blockers = append(state.blockers, Issue{Code: "locked", Message: err.Error()})
	}
	if op := detectOperation(loc.Path); op != nil {
		state.blockers = append(state.blockers, Issue{
			Code:    "operation_in_progress",
			Message: fmt.Sprintf("工作树存在未完成的 %s 操作，请先继续完成或中止后再执行新操作", *op),
		})
	}
	if detached || branch == "" {
		state.blockers = append(state.blockers, Issue{
			Code:    "detached_head",
			Message: "当前工作树处于游离 HEAD 状态，请先切换到明确本地分支",
		})
	}
	if headOID == "" {
		state.blockers = append(state.blockers, Issue{
			Code:    "unborn_head",
			Message: "当前分支尚无初始提交，无法执行此操作",
		})
	}

	ident, iErr := s.readCommitIdentity(ctx, loc.Path)
	if iErr == nil && (strings.TrimSpace(ident.AuthorName) == "" || strings.TrimSpace(ident.AuthorEmail) == "" ||
		strings.TrimSpace(ident.CommitterName) == "" || strings.TrimSpace(ident.CommitterEmail) == "") {
		state.blockers = append(state.blockers, Issue{
			Code:    "missing_identity",
			Message: "缺少 Git user.name 或 user.email 配置，无法生成合并或变基提交",
		})
	}

	statusData, err := ReadGit(ctx, loc.Path, maxWorkspaceGitOutput, "status", "--porcelain=v2", "-z", "--untracked-files=normal")
	if err != nil {
		return divergencePreviewState{}, err
	}
	st, err := parseStatus(statusData)
	if err != nil {
		return divergencePreviewState{}, problemf(ErrInternal, "%v", err)
	}
	if st.changes.Conflicted > 0 {
		state.blockers = append(state.blockers, Issue{
			Code:    "conflicted",
			Message: fmt.Sprintf("工作树存在 %d 个未解决的合并冲突，请先解决冲突", st.changes.Conflicted),
		})
	}
	if st.changes.Staged > 0 || st.changes.Unstaged > 0 {
		state.blockers = append(state.blockers, Issue{
			Code:    "dirty_worktree",
			Message: fmt.Sprintf("工作树有 %d 项已暂存或未暂存修改，请先提交或清理后再执行（不启用隐式 autostash）", st.changes.Staged+st.changes.Unstaged),
		})
	}
	if st.changes.Untracked > 0 {
		state.warnings = append(state.warnings, Issue{
			Code:    issueUntracked,
			Message: fmt.Sprintf("存在 %d 个未跟踪文件；若与传入提交中的同名路径冲突，Git 将拒绝覆盖并保持安全失败", st.changes.Untracked),
		})
	}
	if len(sessions) > 0 {
		state.warnings = append(state.warnings, Issue{
			Code:    issueSessions,
			Message: fmt.Sprintf("工作树中有 %d 个运行中的会话，执行后它们会立即看到更新或冲突后的文件", len(sessions)),
		})
	}

	if err := s.computeDivergenceGraph(ctx, r.entry.AnchorPath, loc.Path, &state); err != nil {
		return divergencePreviewState{}, err
	}
	return state, nil
}

func (s *Service) resolveDivergenceTarget(ctx context.Context, anchor, kind string, req DivergencePreviewRequest, state *divergencePreviewState) error {
	switch kind {
	case GitOpMerge, GitOpRebase:
		refName := strings.TrimSpace(req.RefName)
		switch req.RefKind {
		case "local":
			if err := s.validBranch(ctx, anchor, refName); err != nil {
				return err
			}
			fullRef := "refs/heads/" + refName
			oid, err := s.resolveCommit(ctx, anchor, fullRef)
			if err != nil {
				return err
			}
			if oid == "" {
				return problemf(ErrNotFound, "本地分支不存在: %s", refName)
			}
			state.targetKind = "local"
			state.targetRef = refName
			state.targetFullRef = fullRef
			state.targetOID = oid
		case "remote":
			remote, rest, _ := strings.Cut(refName, "/")
			if !plainRevision(refName) || remote == "" || rest == "" {
				return problemf(ErrInvalid, "远端跟踪分支名无效")
			}
			fullRef := "refs/remotes/" + refName
			if _, err := s.git(ctx, anchor, "check-ref-format", fullRef); err != nil {
				return problemf(ErrInvalid, "远端跟踪分支名无效: %s", refName)
			}
			oid, err := s.resolveCommit(ctx, anchor, fullRef)
			if err != nil {
				return err
			}
			if oid == "" {
				return problemf(ErrNotFound, "远端跟踪分支不存在: %s", refName)
			}
			state.targetKind = "remote"
			state.targetRef = refName
			state.targetFullRef = fullRef
			state.targetOID = oid
		default:
			return problemf(ErrInvalid, "分支引用类型必须为 local 或 remote")
		}
		state.targetSubject = s.readCommitSubjectBestEffort(ctx, anchor, state.targetOID)

	case GitOpCherryPick, GitOpRevert:
		oid := strings.TrimSpace(req.CommitOID)
		if !isObjectID(oid) {
			return problemf(ErrInvalid, "必须提供完整的提交哈希 (commitOid)")
		}
		resolved, err := s.resolveCommit(ctx, anchor, oid)
		if err != nil {
			return err
		}
		if resolved == "" {
			return problemf(ErrNotFound, "指定的提交不存在: %s", shortOID(oid))
		}
		_, parents, subject, metaErr := s.readCommitTreeAndParents(ctx, anchor, resolved)
		if metaErr != nil {
			return metaErr
		}
		state.targetKind = "commit"
		state.targetRef = shortOID(resolved)
		state.targetFullRef = resolved
		state.targetOID = resolved
		state.targetSubject = subject
		if len(parents) > 1 {
			state.blockers = append(state.blockers, Issue{
				Code:    "merge_commit",
				Message: "该提交包含多个父节点（合并提交），需要指定主线父节点，请在终端执行",
			})
		}
		if len(parents) == 0 && kind == GitOpRevert {
			state.blockers = append(state.blockers, Issue{
				Code:    "root_commit",
				Message: "根提交没有父提交，无法通过常规 revert 还原",
			})
		}
		commits, descErr := s.describeCommits(ctx, anchor, []string{resolved})
		if descErr == nil && len(commits) > 0 {
			state.commits = CommitList{Total: 1, Items: commits}
		}
	default:
		return problemf(ErrInvalid, "不支持的操作类型: %s", kind)
	}
	return nil
}

func (s *Service) computeDivergenceGraph(ctx context.Context, anchor, wtPath string, state *divergencePreviewState) error {
	wtBase := filepath.Base(wtPath)
	branchLabel := firstNonEmpty(state.currentBranch, "HEAD")

	if state.headOID != "" && state.targetOID != "" {
		mbOut, err := ReadGit(ctx, anchor, 128, "merge-base", state.headOID, state.targetOID)
		if err == nil {
			if mb := strings.TrimSpace(string(mbOut)); isObjectID(mb) {
				state.mergeBaseOID = mb
			}
		}
	}

	switch state.kind {
	case GitOpMerge:
		if state.targetKind == "local" && state.targetRef == state.currentBranch {
			state.blockers = append(state.blockers, Issue{
				Code:    issueSame,
				Message: "不能将当前工作树已检出的分支合入其自身",
			})
		}
		if state.headOID != "" && state.targetOID != "" {
			incoming, err := s.listCommits(ctx, anchor, state.headOID+".."+state.targetOID)
			if err != nil {
				return err
			}
			state.commits = incoming
			if incoming.Total == 0 {
				state.alreadyUpToDate = true
				state.blockers = append(state.blockers, Issue{
					Code:    "already_up_to_date",
					Message: fmt.Sprintf("当前分支 %s 已包含 %s 的全部提交，无需合并", branchLabel, state.targetRef),
				})
			} else if state.mergeBaseOID == state.headOID {
				state.fastForward = true
			}
		}
		state.directionSummary = fmt.Sprintf(
			"将 %s（%s）的 %d 个提交合入工作树 %s 的分支 %s（%s）",
			state.targetRef, shortOID(state.targetOID), state.commits.Total, wtBase, branchLabel, shortOID(state.headOID),
		)
		state.recoveryHint = fmt.Sprintf(
			"若发生冲突，将进入三方冲突解决器逐步处理；也可随时预览并执行 Abort 恢复到合并前起点 %s。",
			shortOID(state.headOID),
		)

	case GitOpRebase:
		if state.targetKind == "local" && state.targetRef == state.currentBranch {
			state.blockers = append(state.blockers, Issue{
				Code:    issueSame,
				Message: "不能将当前分支变基到其自身",
			})
		}
		if state.headOID != "" && state.targetOID != "" {
			replayed, err := s.listCommits(ctx, anchor, state.targetOID+".."+state.headOID)
			if err != nil {
				return err
			}
			state.commits = replayed
			if state.headOID == state.targetOID || (replayed.Total == 0 && state.mergeBaseOID == state.targetOID) {
				state.alreadyUpToDate = true
				state.blockers = append(state.blockers, Issue{
					Code:    "already_up_to_date",
					Message: fmt.Sprintf("当前分支 %s 已位于 %s 之上，无需变基", branchLabel, state.targetRef),
				})
			} else if replayed.Total == 0 && state.mergeBaseOID == state.headOID {
				state.fastForward = true
			}
		}
		state.directionSummary = fmt.Sprintf(
			"将工作树 %s 的分支 %s（%s）上的 %d 个提交变基到 %s（%s）之上",
			wtBase, branchLabel, shortOID(state.headOID), state.commits.Total, state.targetRef, shortOID(state.targetOID),
		)
		state.recoveryHint = fmt.Sprintf(
			"变基会重写本地尚未包含在目标基底中的提交哈希；若发生冲突可逐个提交在三方冲突解决器中解决后 Continue，或执行 Abort 精确回退到起点 %s。",
			shortOID(state.headOID),
		)

	case GitOpCherryPick:
		state.directionSummary = fmt.Sprintf(
			"将提交 %s（%s）遴选应用到工作树 %s 的分支 %s（%s）",
			shortOID(state.targetOID), state.targetSubject, wtBase, branchLabel, shortOID(state.headOID),
		)
		state.recoveryHint = fmt.Sprintf(
			"若发生冲突，可在三方冲突解决器中解决后点击 Continue 生成新提交，或点击 Abort 恢复到起点 %s。",
			shortOID(state.headOID),
		)

	case GitOpRevert:
		state.directionSummary = fmt.Sprintf(
			"在工作树 %s 的分支 %s（%s）上生成新提交，用于还原提交 %s（%s）",
			wtBase, branchLabel, shortOID(state.headOID), shortOID(state.targetOID), state.targetSubject,
		)
		state.recoveryHint = fmt.Sprintf(
			"还原操作通过创建反向提交保留历史；若发生冲突可在三方冲突解决器中处理后 Continue，或点击 Abort 恢复到起点 %s。",
			shortOID(state.headOID),
		)
	}
	return nil
}
