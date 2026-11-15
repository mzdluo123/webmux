package workspace

import (
	"context"
	"fmt"

	"github.com/rainchan/webmux/internal/session"
)

// PreviewDiscard builds a two-step preview for discarding unstaged working tree
// edits (single file, multi-selected files, single hunk, or selected lines) or
// deleting untracked files, without touching staged .git/index entries.
func (s *Service) PreviewDiscard(ctx context.Context, req DiscardPreviewRequest) (DiscardPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return DiscardPreview{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return DiscardPreview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return DiscardPreview{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return DiscardPreview{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	state, err := s.buildDiscardPreviewStateLocked(ctx, loc, req)
	if err != nil {
		return DiscardPreview{}, err
	}

	preview := DiscardPreview{
		RepositoryID:          req.RepositoryID,
		WorktreeID:            req.WorktreeID,
		WorktreePath:          state.worktreePath,
		Scope:                 state.scope,
		TrackedOverwritePaths: state.trackedOverwritePaths,
		UntrackedDeletePaths:  state.untrackedDeletePaths,
		Items:                 state.items,
		HunkCount:             state.hunkCount,
		LineCount:             state.lineCount,
		Summary:               state.summary,
		Sessions:              state.sessions,
		Blockers:              state.blockers,
		Warnings:              state.warnings,
	}
	if len(state.blockers) > 0 {
		return preview, nil
	}
	id, expires, err := s.confirm(confirmation{
		purpose:      purposeDiscard,
		repositoryID: req.RepositoryID,
		worktreeID:   req.WorktreeID,
		discard:      &state,
	})
	if err != nil {
		return DiscardPreview{}, err
	}
	preview.ConfirmationID = id
	preview.ExpiresAt = &expires
	return preview, nil
}

func (s *Service) buildDiscardPreviewStateLocked(ctx context.Context, loc WorktreeLocation, req DiscardPreviewRequest) (discardPreviewState, error) {
	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return discardPreviewState{}, err
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return discardPreviewState{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}

	sessions := s.runningSessions(loc.Path)
	if sessions == nil {
		sessions = []session.Info{}
	}
	blockers := make([]Issue, 0)
	warnings := make([]Issue, 0)
	if lErr := checkWorktreeIndexLock(gitDir); lErr != nil {
		blockers = append(blockers, Issue{Code: "locked", Message: lErr.Error()})
	}

	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")

	state := discardPreviewState{
		commonDir:             loc.CommonDir,
		worktreeID:            loc.WorktreeID,
		worktreePath:          loc.Path,
		scope:                 req.Scope,
		trackedOverwritePaths: []string{},
		untrackedDeletePaths:  []string{},
		items:                 []DiscardPreviewItem{},
		sessions:              sessions,
		blockers:              blockers,
		warnings:              warnings,
	}

	switch req.Scope {
	case DiscardScopeFiles:
		if err := s.populateFileDiscardPreview(ctx, loc, headOID, req, &state); err != nil {
			return discardPreviewState{}, err
		}
	case DiscardScopeHunks:
		if err := s.populateHunkDiscardPreview(ctx, loc, headOID, req, &state); err != nil {
			return discardPreviewState{}, err
		}
	case DiscardScopeLines:
		if err := s.populateLineDiscardPreview(ctx, loc, headOID, req, &state); err != nil {
			return discardPreviewState{}, err
		}
	default:
		return discardPreviewState{}, problemf(ErrInvalid, "丢弃范围必须为 files、hunks 或 lines")
	}

	state.warnings = append(state.warnings, Issue{
		Code:    "destructive_discard",
		Message: "丢弃工作区修改会直接覆盖磁盘文件或删除未跟踪文件（已暂存的 index 条目不受影响）；执行前会自动保存短期恢复快照",
	})
	if len(sessions) > 0 {
		state.warnings = append(state.warnings, Issue{
			Code:    issueSessions,
			Message: fmt.Sprintf("工作树中有 %d 个运行中的会话，请确认无正在写入同文件的 CLI Agent 后再执行", len(sessions)),
		})
	}
	return state, nil
}

func (s *Service) populateFileDiscardPreview(
	ctx context.Context,
	loc WorktreeLocation,
	headOID string,
	req DiscardPreviewRequest,
	state *discardPreviewState,
) error {
	expectedByPath := make(map[string]*StageFileIdentity, len(req.Identities))
	orderedPaths := make([]string, 0, len(req.Paths)+len(req.Identities))
	seen := make(map[string]bool)

	for i := range req.Identities {
		id := req.Identities[i]
		if id.Area != StageAreaUnstaged {
			return problemf(ErrInvalid, "丢弃修改仅接受未暂存或未跟踪快照身份（索引已暂存项请使用取消暂存 Unstage）")
		}
		cleanPath, err := validateRepoRelPath(id.Path)
		if err != nil {
			return err
		}
		id.Path = cleanPath
		if !seen[cleanPath] {
			seen[cleanPath] = true
			orderedPaths = append(orderedPaths, cleanPath)
		}
		copied := id
		expectedByPath[cleanPath] = &copied
	}
	for _, rawPath := range req.Paths {
		cleanPath, err := validateRepoRelPath(rawPath)
		if err != nil {
			return err
		}
		if !seen[cleanPath] {
			seen[cleanPath] = true
			orderedPaths = append(orderedPaths, cleanPath)
		}
	}

	if len(orderedPaths) == 0 {
		return problemf(ErrInvalid, "未选择任何要丢弃修改的文件")
	}
	if len(orderedPaths) > maxDiscardFiles {
		return problemf(ErrInvalid, "单次最多丢弃 %d 个文件", maxDiscardFiles)
	}

	for _, cleanPath := range orderedPaths {
		diff, err := s.inspectFileLocked(ctx, loc, headOID, StageAreaUnstaged, cleanPath, "", "")
		if err != nil {
			return err
		}
		if diff.Conflicted {
			return problemf(ErrConflicted, "文件 %s 存在未解决的合并冲突，请在冲突解决器中处理", cleanPath)
		}
		if exp := expectedByPath[cleanPath]; exp != nil && diff.Identity != *exp {
			return problemf(ErrStale, "文件 %s 的工作区或索引内容已变化，请刷新后重新预览", cleanPath)
		}
		id := diff.Identity
		if id.BaseMode == "" && id.TargetMode == "" {
			return problemf(ErrStale, "文件 %s 当前没有可丢弃的修改，请刷新列表", cleanPath)
		}
		if id.BaseOID != "" && id.BaseMode == id.TargetMode && len(diff.Hunks) == 0 && !diff.Binary && diff.DegradeReason == "文件无逐块文本差异，仅支持整文件操作" {
			return problemf(ErrStale, "文件 %s 当前与索引一致，无需丢弃修改", cleanPath)
		}
		if id.BaseMode == "160000" || id.TargetMode == "160000" {
			state.blockers = append(state.blockers, Issue{
				Code:    "unsupported_submodule",
				Message: fmt.Sprintf("路径 %s 为子模块或嵌套 Git 仓库，不支持通过丢弃修改处理", cleanPath),
			})
			continue
		}

		var action, status string
		switch {
		case id.BaseOID == "" && id.BaseMode == "":
			action = "delete_untracked"
			status = "?"
			state.untrackedDeletePaths = append(state.untrackedDeletePaths, cleanPath)
		case id.TargetMode == "":
			action = "restore_deleted"
			status = "D"
			state.trackedOverwritePaths = append(state.trackedOverwritePaths, cleanPath)
		default:
			action = "overwrite_tracked"
			status = "M"
			state.trackedOverwritePaths = append(state.trackedOverwritePaths, cleanPath)
		}
		state.items = append(state.items, DiscardPreviewItem{
			Path:   cleanPath,
			Status: status,
			Action: action,
		})
		state.targets = append(state.targets, discardTargetSpec{
			identity: id,
			action:   action,
			status:   status,
		})
	}

	state.summary = fmt.Sprintf(
		"丢弃 %d 个文件的未暂存/未跟踪修改（覆盖/恢复已跟踪 %d 个，删除未跟踪 %d 个）",
		len(state.items), len(state.trackedOverwritePaths), len(state.untrackedDeletePaths),
	)
	return nil
}

func (s *Service) populateHunkDiscardPreview(
	ctx context.Context,
	loc WorktreeLocation,
	headOID string,
	req DiscardPreviewRequest,
	state *discardPreviewState,
) error {
	if req.Identity.Area != StageAreaUnstaged {
		return problemf(ErrInvalid, "丢弃代码块仅支持未暂存工作区修改（已暂存项请使用取消暂存 Unstage）")
	}
	cleanPath, err := validateRepoRelPath(req.Identity.Path)
	if err != nil {
		return err
	}
	if len(req.HunkIDs) == 0 {
		return problemf(ErrInvalid, "未指定要丢弃的代码块标识")
	}
	diff, err := s.inspectFileLocked(ctx, loc, headOID, StageAreaUnstaged, cleanPath, "", "")
	if err != nil {
		return err
	}
	if diff.Conflicted {
		return problemf(ErrConflicted, "文件 %s 存在未解决的合并冲突", cleanPath)
	}
	if diff.Identity != req.Identity {
		return problemf(ErrStale, "文件 %s 的工作区或索引内容已变化，请刷新后重试", cleanPath)
	}
	if diff.Identity.BaseOID == "" || !diff.HunkStageAvailable {
		return problemf(ErrConflict, "文件 %s 不支持按代码块丢弃: %s", cleanPath, firstNonEmpty(diff.DegradeReason, "未跟踪文件仅支持整文件删除"))
	}
	if _, err := validateRequestedHunks(diff.Hunks, req.HunkIDs); err != nil {
		return err
	}

	state.hunkCount = len(req.HunkIDs)
	state.trackedOverwritePaths = []string{cleanPath}
	state.items = []DiscardPreviewItem{{
		Path:   cleanPath,
		Status: "M",
		Action: "overwrite_tracked",
	}}
	state.targets = []discardTargetSpec{{
		identity: diff.Identity,
		action:   "partial_text",
		status:   "M",
		hunkIDs:  append([]string(nil), req.HunkIDs...),
	}}
	state.summary = fmt.Sprintf("丢弃文件 %s 中选中的 %d 个未暂存代码块", cleanPath, len(req.HunkIDs))
	return nil
}

func (s *Service) populateLineDiscardPreview(
	ctx context.Context,
	loc WorktreeLocation,
	headOID string,
	req DiscardPreviewRequest,
	state *discardPreviewState,
) error {
	if req.Identity.Area != StageAreaUnstaged {
		return problemf(ErrInvalid, "丢弃选中行仅支持未暂存工作区修改（已暂存项请使用取消暂存 Unstage）")
	}
	cleanPath, err := validateRepoRelPath(req.Identity.Path)
	if err != nil {
		return err
	}
	diff, err := s.inspectFileLocked(ctx, loc, headOID, StageAreaUnstaged, cleanPath, "", "")
	if err != nil {
		return err
	}
	if diff.Conflicted {
		return problemf(ErrConflicted, "文件 %s 存在未解决的合并冲突", cleanPath)
	}
	if diff.Identity != req.Identity {
		return problemf(ErrStale, "文件 %s 的工作区或索引内容已变化，请刷新后重试", cleanPath)
	}
	if diff.Identity.BaseOID == "" || !diff.HunkStageAvailable {
		return problemf(ErrConflict, "文件 %s 不支持逐行丢弃: %s", cleanPath, firstNonEmpty(diff.DegradeReason, "未跟踪文件仅支持整文件删除"))
	}
	selMap, err := validateLineSelections(diff.Hunks, req.LineSelections)
	if err != nil {
		return err
	}
	lineCount := 0
	for _, set := range selMap {
		lineCount += len(set)
	}
	copiedSelections := make([]LineSelection, len(req.LineSelections))
	for i, sel := range req.LineSelections {
		copiedSelections[i] = LineSelection{
			HunkID:      sel.HunkID,
			LineIndices: append([]int(nil), sel.LineIndices...),
		}
	}

	state.lineCount = lineCount
	state.trackedOverwritePaths = []string{cleanPath}
	state.items = []DiscardPreviewItem{{
		Path:   cleanPath,
		Status: "M",
		Action: "overwrite_tracked",
	}}
	state.targets = []discardTargetSpec{{
		identity:       diff.Identity,
		action:         "partial_text",
		status:         "M",
		lineSelections: copiedSelections,
	}}
	state.summary = fmt.Sprintf("丢弃文件 %s 中选中的 %d 行未暂存修改", cleanPath, lineCount)
	return nil
}
