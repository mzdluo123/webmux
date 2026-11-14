package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/rainchan/webmux/internal/session"
)

// PreviewSmartCheckout builds a two-step preview for 3-phase tracked-stash
// Smart Checkout (`stashing` -> `switching` -> `unstashing`).
func (s *Service) PreviewSmartCheckout(ctx context.Context, req SmartCheckoutPreviewRequest) (SmartCheckoutPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return SmartCheckoutPreview{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return SmartCheckoutPreview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return SmartCheckoutPreview{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return SmartCheckoutPreview{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	includeUntracked := true
	if req.IncludeUntracked != nil {
		includeUntracked = *req.IncludeUntracked
	}

	state, err := s.buildSmartCheckoutStateLocked(ctx, r, loc, req.WorktreeID, req.Target, includeUntracked)
	if err != nil {
		return SmartCheckoutPreview{}, err
	}

	preview := SmartCheckoutPreview{
		RepositoryID:     req.RepositoryID,
		WorktreeID:       req.WorktreeID,
		Worktree:         s.withLastCommit(ctx, r, state.tree),
		Target:           state.plan.info,
		IncludeUntracked: state.includeUntracked,
		StagedCount:      state.stagedCount,
		UnstagedCount:    state.unstagedCount,
		UntrackedCount:   state.untrackedCount,
		DirtyPaths:       state.dirtyPaths,
		HasActiveAgent:   state.hasActiveAgent,
		Sessions:         state.sessions,
		Blockers:         state.blockers,
		Warnings:         state.warnings,
	}
	if len(state.blockers) > 0 {
		return preview, nil
	}

	id, expires, err := s.confirm(confirmation{
		purpose:       purposeSmartCheckout,
		repositoryID:  req.RepositoryID,
		worktreeID:    req.WorktreeID,
		smartCheckout: &state,
	})
	if err != nil {
		return SmartCheckoutPreview{}, err
	}
	preview.ConfirmationID = id
	preview.ExpiresAt = &expires
	return preview, nil
}

func (s *Service) buildSmartCheckoutStateLocked(
	ctx context.Context,
	r *repositoryState,
	loc WorktreeLocation,
	worktreeID string,
	target SwitchTarget,
	includeUntracked bool,
) (smartCheckoutState, error) {
	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return smartCheckoutState{}, err
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return smartCheckoutState{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}

	selected, listed, err := s.currentTreesListed(ctx, r, []string{worktreeID})
	if err != nil {
		return smartCheckoutState{}, err
	}
	tree := selected[0]
	plan, err := s.resolveSwitch(ctx, r.entry.AnchorPath, target)
	if err != nil {
		return smartCheckoutState{}, err
	}

	sessions := s.runningSessions(tree.Path)
	if sessions == nil {
		sessions = []session.Info{}
	}
	hasAgent := false
	for _, sess := range sessions {
		if s.hybrids != nil && s.hybrids.Metadata(sess.ID) != nil {
			hasAgent = true
			break
		}
	}

	overview, err := s.inspectStagingLocked(ctx, loc)
	if err != nil {
		return smartCheckoutState{}, err
	}
	stagedCount := len(overview.Staged)
	unstagedCount := 0
	untrackedCount := 0
	dirtySet := make(map[string]bool)
	var dirtyPaths []string
	hash := sha256.New()

	recordIdentity := func(e StagingEntry) {
		_, _ = fmt.Fprintf(
			hash, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00\n",
			e.Area, e.Status, e.Path,
			e.Identity.BaseMode, e.Identity.BaseOID,
			e.Identity.TargetMode, e.Identity.TargetSHA256+e.Identity.TargetOID,
		)
		if !dirtySet[e.Path] {
			dirtySet[e.Path] = true
			dirtyPaths = append(dirtyPaths, e.Path)
		}
	}
	for _, e := range overview.Staged {
		recordIdentity(e)
	}
	for _, e := range overview.Unstaged {
		if e.Status == "?" {
			untrackedCount++
			if includeUntracked {
				recordIdentity(e)
			} else {
				_, _ = fmt.Fprintf(hash, "untracked-skip\x00%s\x00%s\n", e.Path, e.Identity.TargetSHA256)
			}
		} else {
			unstagedCount++
			recordIdentity(e)
		}
	}
	slices.Sort(dirtyPaths)
	if dirtyPaths == nil {
		dirtyPaths = []string{}
	}

	blockers := smartCheckoutBlockers(tree, listed, plan, gitDir, overview, stagedCount, unstagedCount, untrackedCount, includeUntracked)
	warnings := smartCheckoutWarnings(tree, sessions, hasAgent, stagedCount, unstagedCount, untrackedCount, includeUntracked)

	return smartCheckoutState{
		commonDir:        loc.CommonDir,
		worktreeID:       worktreeID,
		tree:             tree,
		plan:             plan,
		includeUntracked: includeUntracked,
		stagedCount:      stagedCount,
		unstagedCount:    unstagedCount,
		untrackedCount:   untrackedCount,
		dirtyPaths:       dirtyPaths,
		dirtyDigest:      hex.EncodeToString(hash.Sum(nil)),
		hasActiveAgent:   hasAgent,
		sessions:         sessions,
		blockers:         blockers,
		warnings:         warnings,
	}, ctx.Err()
}

func smartCheckoutBlockers(
	tree Worktree,
	listed []Worktree,
	plan switchPlan,
	gitDir string,
	overview StagingOverview,
	stagedCount, unstagedCount, untrackedCount int,
	includeUntracked bool,
) []Issue {
	out := []Issue{}
	if tree.Kind == "bare" || !tree.Available || tree.Prunable {
		return append(out, Issue{Code: issueUnsupported, Message: "bare 仓库、不可用或待清理的工作树不支持 Smart Checkout"})
	}
	if tree.Error != "" || tree.Changes == nil {
		return append(out, Issue{Code: issueUnverified, Message: "工作树状态无法验证: " + tree.Error})
	}
	if lErr := checkWorktreeIndexLock(gitDir); lErr != nil {
		out = append(out, Issue{Code: issueLocked, Message: lErr.Error()})
	}
	if tree.Operation != nil {
		out = append(out, Issue{Code: issueOperation, Message: fmt.Sprintf("正在进行 %s 操作，请先完成或中止后再切换分支", *tree.Operation)})
	}
	if len(overview.Conflicted) > 0 || tree.Changes.Conflicted > 0 {
		out = append(out, Issue{Code: "conflicted", Message: "存在未解决的合并冲突，请先在冲突解决器中处理"})
	}
	if overview.Unborn || tree.Head == nil {
		out = append(out, Issue{Code: "unborn", Message: "当前分支尚无初始提交（unborn HEAD），无法创建受控贮藏"})
	}

	stashableCount := stagedCount + unstagedCount
	if includeUntracked {
		stashableCount += untrackedCount
	}
	if stashableCount == 0 {
		out = append(out, Issue{Code: "clean", Message: "当前没有需要打包携带的未提交修改，可直接执行普通分支切换"})
	}

	switch plan.target.Kind {
	case SwitchLocal:
		branch := *plan.info.Branch
		if sameText(tree.Branch, plan.info.Branch) {
			out = append(out, Issue{Code: issueSame, Message: "工作树已在分支 " + branch + " 上"})
			break
		}
		for _, other := range listed {
			if other.ID != tree.ID && sameText(other.Branch, plan.info.Branch) {
				out = append(out, Issue{Code: issueOccupied, Message: "分支 " + branch + " 已被另一个工作树检出", Path: other.Path})
				break
			}
		}
	case SwitchRemote:
		if plan.exists {
			out = append(out, Issue{Code: issueExists, Message: "本地分支 " + *plan.info.Branch + " 已存在，请改为切换到该本地分支"})
		}
	case SwitchNew:
		if plan.exists {
			out = append(out, Issue{Code: issueExists, Message: "分支 " + *plan.info.Branch + " 已存在，请换一个名称或切换到该分支"})
		}
	case SwitchDetach:
		if tree.Detached && tree.Head != nil && *tree.Head == plan.info.Commit {
			out = append(out, Issue{Code: issueSame, Message: "工作树已分离在该提交上"})
		}
	}
	return out
}

func smartCheckoutWarnings(
	tree Worktree,
	sessions []session.Info,
	hasAgent bool,
	stagedCount, unstagedCount, untrackedCount int,
	includeUntracked bool,
) []Issue {
	out := []Issue{
		{
			Code: "smart_checkout_phases",
			Message: fmt.Sprintf(
				"Smart Checkout 将按三阶段执行：1. 创建受控贮藏（已暂存 %d、未暂存 %d、未跟踪 %d）→ 2. 切换分支 → 3. 自动恢复修改；若切换失败将自动回滚，若与目标分支冲突将保留贮藏并进入三方冲突解决器",
				stagedCount, unstagedCount, ternaryInt(includeUntracked, untrackedCount, 0),
			),
		},
	}
	if !includeUntracked && untrackedCount > 0 {
		out = append(out, Issue{
			Code:    issueUntracked,
			Message: fmt.Sprintf("未勾选携带 %d 个未跟踪文件；若目标分支跟踪同名路径，第 2 步切换会失败并自动恢复已跟踪修改", untrackedCount),
		})
	}
	if _, err := os.Lstat(filepath.Join(tree.Path, ".gitmodules")); err == nil {
		out = append(out, Issue{Code: issueSubmodules, Message: "存在子模块；切换不会自动更新子模块"})
	}
	if len(sessions) > 0 {
		msg := fmt.Sprintf("工作树中有 %d 个运行中的会话；建议优先「改为在新工作树打开」，原地 Smart Checkout 需确认会话知悉", len(sessions))
		if hasAgent {
			msg = fmt.Sprintf("工作树中有运行中的 CLI Agent 会话（共 %d 个会话），原地切换可能干扰正在读写文件的 Agent；强烈建议「改为在新工作树打开」", len(sessions))
		}
		out = append(out, Issue{Code: issueSessions, Message: msg})
	}
	return out
}

func ternaryInt(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
}
