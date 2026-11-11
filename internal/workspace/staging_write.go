package workspace

import (
	"bytes"
	"context"
	"os"
	"strings"
)

func verifyWorktreeSameOnDisk(loc WorktreeLocation) error {
	rootStat, err := os.Stat(loc.Path)
	if err != nil || !rootStat.IsDir() {
		return problemf(ErrStale, "工作树目录不可访问或已被移除")
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}
	if gitStat, err := os.Stat(gitDir); err != nil || !gitStat.IsDir() {
		return problemf(ErrStale, "工作树 Git 目录不可访问")
	}
	commonStat, err := os.Stat(loc.CommonDir)
	if err != nil || !commonStat.IsDir() {
		return problemf(ErrStale, "仓库 commonDir 不可访问")
	}
	return nil
}

// Stage writes whole-file (empty HunkIDs) or selected text hunks from the worktree
// into the real Git index after verifying the captured content snapshot under the
// per-worktree write gate.
func (s *Service) Stage(ctx context.Context, req StageRequest) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	if req.Identity.Area != StageAreaUnstaged {
		return problemf(ErrInvalid, "Stage 仅接受未暂存快照身份")
	}
	cleanPath, err := validateRepoRelPath(req.Identity.Path)
	if err != nil {
		return err
	}
	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return err
	}
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return err
	}
	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")
	current, err := s.inspectFileLocked(ctx, loc, headOID, StageAreaUnstaged, cleanPath, "", "")
	if err != nil {
		return err
	}
	if current.Conflicted {
		return problemf(ErrConflicted, "文件 %s 存在未解决的合并冲突，请进入冲突处理", cleanPath)
	}
	if current.Identity != req.Identity {
		return problemf(ErrStale, "文件 %s 的工作区或索引内容已变化，快照已过期，请刷新后重试", cleanPath)
	}

	if len(req.HunkIDs) == 0 {
		return s.stageWholeFileLocked(ctx, loc.Path, cleanPath, current)
	}
	if !current.HunkStageAvailable {
		return problemf(ErrConflict, "文件 %s 不支持逐块暂存: %s", cleanPath, current.DegradeReason)
	}
	keep, err := validateRequestedHunks(current.Hunks, req.HunkIDs)
	if err != nil {
		return err
	}
	nextBytes := spliceHunkSubset(current.baseBytes, current.targetBytes, keep)
	blobOID, err := s.writeBlobWithPathFilter(ctx, loc.Path, cleanPath, nextBytes)
	if err != nil {
		return err
	}
	mode := current.Identity.TargetMode
	if !isRegularGitMode(mode) {
		mode = current.Identity.BaseMode
	}
	if !isRegularGitMode(mode) {
		mode = "100644"
	}
	_, _, err = s.gitInputOutput(ctx, loc.Path, nil, nil, "update-index", "--add", "--cacheinfo", mode+","+blobOID+","+cleanPath)
	return err
}

func (s *Service) stageWholeFileLocked(ctx context.Context, cwd, cleanPath string, current StagingFileDiff) error {
	switch current.Identity.TargetMode {
	case "":
		_, _, err := s.gitInputOutput(ctx, cwd, nil, nil, "update-index", "--force-remove", "--", cleanPath)
		return err
	case "160000":
		if !isObjectID(current.Identity.TargetSHA256) {
			return problemf(ErrConflict, "子模块 %s 当前 HEAD 无效，无法暂存", cleanPath)
		}
		_, _, err := s.gitInputOutput(ctx, cwd, nil, nil, "update-index", "--add", "--cacheinfo", "160000,"+current.Identity.TargetSHA256+","+cleanPath)
		return err
	case "120000":
		out, _, err := s.gitInputOutput(ctx, cwd, bytes.NewReader(current.worktreeRaw), nil, "hash-object", "-w", "--no-filters", "--stdin")
		if err != nil {
			return err
		}
		blobOID := strings.TrimSpace(string(out))
		_, _, err = s.gitInputOutput(ctx, cwd, nil, nil, "update-index", "--add", "--cacheinfo", "120000,"+blobOID+","+cleanPath)
		return err
	default:
		blobOID, err := s.writeBlobWithPathFilter(ctx, cwd, cleanPath, current.worktreeRaw)
		if err != nil {
			return err
		}
		_, _, err = s.gitInputOutput(ctx, cwd, nil, nil, "update-index", "--add", "--cacheinfo", current.Identity.TargetMode+","+blobOID+","+cleanPath)
		return err
	}
}

// Unstage restores whole-file (empty HunkIDs) or selected text hunks in the real
// Git index back toward HEAD (or removes newly added entries when HEAD is unborn
// or lacks the file) without ever modifying working tree files on disk.
func (s *Service) Unstage(ctx context.Context, req StageRequest) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	if req.Identity.Area != StageAreaStaged {
		return problemf(ErrInvalid, "Unstage 仅接受已暂存快照身份")
	}
	cleanPath, err := validateRepoRelPath(req.Identity.Path)
	if err != nil {
		return err
	}
	cleanOldPath := ""
	if req.Identity.OldPath != "" {
		cleanOldPath, err = validateRepoRelPath(req.Identity.OldPath)
		if err != nil {
			return err
		}
	}

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return err
	}
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return err
	}
	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")
	current, err := s.inspectFileLocked(ctx, loc, headOID, StageAreaStaged, cleanPath, cleanOldPath, "")
	if err != nil {
		return err
	}
	if current.Conflicted {
		return problemf(ErrConflicted, "文件 %s 存在未解决的合并冲突，请进入冲突处理", cleanPath)
	}
	if current.Identity != req.Identity {
		return problemf(ErrStale, "文件 %s 的索引或 HEAD 已变化，快照已过期，请刷新后重试", cleanPath)
	}

	if len(req.HunkIDs) == 0 {
		return s.unstageWholeFileLocked(ctx, loc.Path, cleanPath, cleanOldPath, current.Identity)
	}
	if !current.HunkStageAvailable {
		return problemf(ErrConflict, "文件 %s 不支持逐块取消暂存: %s", cleanPath, current.DegradeReason)
	}
	unstageSet, err := validateRequestedHunks(current.Hunks, req.HunkIDs)
	if err != nil {
		return err
	}
	keep := make(map[string]bool, len(current.Hunks))
	for _, h := range current.Hunks {
		if !unstageSet[h.ID] {
			keep[h.ID] = true
		}
	}
	if len(keep) == 0 && current.Identity.BaseOID == "" {
		_, _, err = s.gitInputOutput(ctx, loc.Path, nil, nil, "update-index", "--force-remove", "--", cleanPath)
		return err
	}
	nextBytes := spliceHunkSubset(current.baseBytes, current.targetBytes, keep)
	out, _, err := s.gitInputOutput(ctx, loc.Path, bytes.NewReader(nextBytes), nil, "hash-object", "-w", "--no-filters", "--stdin")
	if err != nil {
		return err
	}
	blobOID := strings.TrimSpace(string(out))
	mode := current.Identity.TargetMode
	if len(keep) == 0 && isRegularGitMode(current.Identity.BaseMode) {
		mode = current.Identity.BaseMode
	}
	if !isRegularGitMode(mode) {
		mode = "100644"
	}
	_, _, err = s.gitInputOutput(ctx, loc.Path, nil, nil, "update-index", "--add", "--cacheinfo", mode+","+blobOID+","+cleanPath)
	return err
}

func (s *Service) unstageWholeFileLocked(ctx context.Context, cwd, cleanPath, cleanOldPath string, id StageFileIdentity) error {
	if cleanOldPath != "" && id.OldBaseOID != "" && id.OldBaseMode != "" {
		if _, _, err := s.gitInputOutput(ctx, cwd, nil, nil, "update-index", "--add", "--cacheinfo", id.OldBaseMode+","+id.OldBaseOID+","+cleanOldPath); err != nil {
			return err
		}
	}
	if id.BaseOID == "" || id.BaseMode == "" {
		_, _, err := s.gitInputOutput(ctx, cwd, nil, nil, "update-index", "--force-remove", "--", cleanPath)
		return err
	}
	_, _, err := s.gitInputOutput(ctx, cwd, nil, nil, "update-index", "--add", "--cacheinfo", id.BaseMode+","+id.BaseOID+","+cleanPath)
	return err
}

func (s *Service) writeBlobWithPathFilter(ctx context.Context, cwd, cleanPath string, data []byte) (string, error) {
	out, _, err := s.gitInputOutput(ctx, cwd, bytes.NewReader(data), nil, "hash-object", "-w", "--path="+cleanPath, "--stdin")
	if err != nil {
		return "", err
	}
	oid := strings.TrimSpace(string(out))
	if !isObjectID(oid) {
		return "", problemf(ErrInternal, "Git hash-object 返回了无效对象 ID")
	}
	return oid, nil
}

func validateRequestedHunks(available []StagingHunk, requested []string) (map[string]bool, error) {
	known := make(map[string]bool, len(available))
	for _, h := range available {
		known[h.ID] = true
	}
	selected := make(map[string]bool, len(requested))
	for _, id := range requested {
		if id == "" {
			return nil, problemf(ErrInvalid, "代码块标识不能为空")
		}
		if !known[id] {
			return nil, problemf(ErrStale, "代码块标识 %s 不存在于当前快照，请刷新后重试", id)
		}
		selected[id] = true
	}
	return selected, nil
}
