package workspace

import (
	"bytes"
	"context"
	"strings"
	"unicode/utf8"
)

// InspectStagingFile captures the exact identity and stageable hunks for one file.
func (s *Service) InspectStagingFile(ctx context.Context, repositoryID, worktreeID string, area StageArea, relPath string, oldPaths ...string) (StagingFileDiff, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	cleanPath, err := validateRepoRelPath(relPath)
	if err != nil {
		return StagingFileDiff{}, err
	}
	cleanOldPath := ""
	if len(oldPaths) > 0 && oldPaths[0] != "" {
		cleanOldPath, err = validateRepoRelPath(oldPaths[0])
		if err != nil {
			return StagingFileDiff{}, err
		}
	}
	if area != StageAreaUnstaged && area != StageAreaStaged {
		return StagingFileDiff{}, problemf(ErrInvalid, "不支持的暂存区域: %s", area)
	}
	loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return StagingFileDiff{}, err
	}
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return StagingFileDiff{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")
	diff, err := s.inspectFileLocked(ctx, loc, headOID, area, cleanPath, cleanOldPath, "")
	if err != nil {
		return StagingFileDiff{}, err
	}
	if !diff.Conflicted && diff.Identity.BaseMode == "" && diff.Identity.TargetMode == "" && diff.Identity.OldBaseMode == "" {
		return StagingFileDiff{}, problemf(ErrNotFound, "指定路径不存在可检视的变更")
	}
	return diff, nil
}

type indexStageEntry struct {
	mode       string
	oid        string
	conflicted bool
}

func (s *Service) readIndexEntry(ctx context.Context, cwd, relPath string) (indexStageEntry, error) {
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "ls-files", "--stage", "-z", "--", relPath)
	if err != nil {
		return indexStageEntry{}, err
	}
	if len(data) == 0 {
		return indexStageEntry{}, nil
	}
	var out indexStageEntry
	for _, rec := range bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		meta, p, ok := strings.Cut(string(rec), "\t")
		if !ok || p != relPath {
			continue
		}
		parts := strings.Fields(meta)
		if len(parts) != 3 {
			continue
		}
		if parts[2] != "0" {
			out.conflicted = true
			continue
		}
		out.mode = parts[0]
		out.oid = parts[1]
	}
	return out, nil
}

func (s *Service) readHeadTreeEntry(ctx context.Context, cwd, headOID, relPath string) (mode, oid string, err error) {
	if headOID == "" {
		return "", "", nil
	}
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "ls-tree", "-z", headOID, "--", relPath)
	if err != nil || len(data) == 0 {
		return "", "", err
	}
	for _, rec := range bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0}) {
		meta, p, ok := strings.Cut(string(rec), "\t")
		if !ok || p != relPath {
			continue
		}
		parts := strings.Fields(meta)
		if len(parts) == 3 {
			return parts[0], parts[2], nil
		}
	}
	return "", "", nil
}

func (s *Service) inspectFileLocked(ctx context.Context, loc WorktreeLocation, headOID string, area StageArea, relPath, oldPath, statusHint string) (StagingFileDiff, error) {
	idxEntry, err := s.readIndexEntry(ctx, loc.Path, relPath)
	if err != nil {
		return StagingFileDiff{}, err
	}
	if idxEntry.conflicted {
		return StagingFileDiff{
			Identity:           StageFileIdentity{Area: area, Path: relPath},
			Conflicted:         true,
			HunkStageAvailable: false,
			DegradeReason:      "文件存在未解决的合并冲突，请先处理冲突",
			Hunks:              []StagingHunk{},
		}, nil
	}

	id := StageFileIdentity{Area: area, Path: relPath, OldPath: oldPath}
	var wtScan worktreeFileScan

	if area == StageAreaUnstaged {
		id.BaseOID = idxEntry.oid
		id.BaseMode = idxEntry.mode
		wtScan, err = s.scanWorktreeFile(ctx, loc.Path, relPath, idxEntry.mode)
		if err != nil {
			return StagingFileDiff{}, err
		}
		id.TargetMode = wtScan.mode
		id.TargetSHA256 = wtScan.sum
	} else {
		if oldPath != "" {
			oldMode, oldOID, hErr := s.readHeadTreeEntry(ctx, loc.Path, headOID, oldPath)
			if hErr != nil {
				return StagingFileDiff{}, hErr
			}
			id.OldBaseMode = oldMode
			id.OldBaseOID = oldOID
		}
		hMode, hOID, hErr := s.readHeadTreeEntry(ctx, loc.Path, headOID, relPath)
		if hErr != nil {
			return StagingFileDiff{}, hErr
		}
		id.BaseMode = hMode
		id.BaseOID = hOID
		id.TargetMode = idxEntry.mode
		id.TargetOID = idxEntry.oid
	}

	out := StagingFileDiff{
		Identity:          id,
		Hunks:             []StagingHunk{},
		worktreeRaw:       wtScan.raw,
		worktreeRawLoaded: wtScan.rawLoaded,
		worktreeSize:      wtScan.size,
		untrackedRepo:     wtScan.untrackedRepo,
		replacedByDir:     wtScan.replacedByDir,
	}

	if reason := s.degradeStructuralReason(ctx, loc.Path, relPath, oldPath, statusHint, id, wtScan); reason != "" {
		out.HunkStageAvailable = false
		out.DegradeReason = reason
		return out, nil
	}

	var baseBytes, targetBytes []byte
	var baseLoaded bool
	exceedsHunk := wtScan.exceedsHunk

	if area == StageAreaUnstaged {
		targetBytes = wtScan.raw
		if !exceedsHunk && id.BaseOID != "" && isRegularGitMode(id.BaseMode) {
			var baseExceeds bool
			baseBytes, baseLoaded, baseExceeds, err = s.readGitBlobForHunk(ctx, loc.Path, id.BaseOID)
			if err != nil {
				return StagingFileDiff{}, err
			}
			exceedsHunk = exceedsHunk || baseExceeds
		}
	} else {
		if id.BaseOID != "" && isRegularGitMode(id.BaseMode) {
			var baseExceeds bool
			baseBytes, baseLoaded, baseExceeds, err = s.readGitBlobForHunk(ctx, loc.Path, id.BaseOID)
			if err != nil {
				return StagingFileDiff{}, err
			}
			exceedsHunk = exceedsHunk || baseExceeds
		}
		if !exceedsHunk && id.TargetOID != "" && isRegularGitMode(id.TargetMode) {
			var targetExceeds bool
			targetBytes, _, targetExceeds, err = s.readGitBlobForHunk(ctx, loc.Path, id.TargetOID)
			if err != nil {
				return StagingFileDiff{}, err
			}
			exceedsHunk = exceedsHunk || targetExceeds
		}
	}

	out.baseBytes = baseBytes
	out.baseBytesLoaded = baseLoaded
	out.targetBytes = targetBytes

	if exceedsHunk {
		out.HunkStageAvailable = false
		out.DegradeReason = "文件超过 2 MiB 文本分块上限，仅支持整文件操作"
		return out, nil
	}

	if reason, bin := degradeContentReason(baseBytes, targetBytes); reason != "" {
		out.Binary = bin
		out.HunkStageAvailable = false
		out.DegradeReason = reason
		return out, nil
	}

	if area == StageAreaUnstaged && len(targetBytes) > 0 && bytes.Contains(targetBytes, []byte("\r\n")) && !bytes.Contains(baseBytes, []byte("\r\n")) {
		if s.cleanNormalizesToLF(ctx, loc.Path, relPath, targetBytes) {
			targetBytes = bytes.ReplaceAll(targetBytes, []byte("\r\n"), []byte("\n"))
			out.targetBytes = targetBytes
		}
	}

	hunks, _ := buildStagingHunks(baseBytes, targetBytes)
	if len(hunks) == 0 {
		if id.BaseMode != id.TargetMode {
			out.DegradeReason = "纯文件模式变更仅支持整文件操作"
		} else {
			out.DegradeReason = "文件无逐块文本差异，仅支持整文件操作"
		}
		return out, nil
	}
	out.HunkStageAvailable = true
	out.Hunks = hunks
	return out, nil
}

func isRegularGitMode(mode string) bool {
	return mode == "100644" || mode == "100755"
}

func (s *Service) degradeStructuralReason(ctx context.Context, cwd, relPath, oldPath, statusHint string, id StageFileIdentity, wtScan worktreeFileScan) string {
	if oldPath != "" || strings.HasPrefix(statusHint, "R") || strings.HasPrefix(statusHint, "C") {
		return "重命名或复制条目仅支持整文件操作"
	}
	if wtScan.untrackedRepo || (id.Area == StageAreaUnstaged && id.TargetMode == "160000" && id.BaseMode != "160000") {
		return "未跟踪嵌套 Git 仓库仅展示状态，请添加为子模块或移除嵌套 .git 目录"
	}
	if (id.BaseMode != "" && !isRegularGitMode(id.BaseMode)) || (id.TargetMode != "" && !isRegularGitMode(id.TargetMode)) {
		return "符号链接或子模块仅支持整文件操作"
	}
	if id.Area == StageAreaUnstaged && id.TargetMode == "" {
		return "已删除文件仅支持整文件暂存"
	}
	if id.Area == StageAreaStaged && id.TargetMode == "" {
		return "已暂存删除条目仅支持整文件取消暂存"
	}
	if s.hasTransformAttributes(ctx, cwd, relPath) {
		return "配置了 filter 或 working-tree-encoding 属性，仅支持整文件操作"
	}
	return ""
}

func degradeContentReason(baseBytes, targetBytes []byte) (string, bool) {
	if bytes.IndexByte(baseBytes, 0) >= 0 || bytes.IndexByte(targetBytes, 0) >= 0 {
		return "二进制文件仅支持整文件操作", true
	}
	if !utf8.Valid(baseBytes) || !utf8.Valid(targetBytes) {
		return "非 UTF-8 或二进制内容仅支持整文件操作", true
	}
	if len(baseBytes) > maxHunkFileBytes || len(targetBytes) > maxHunkFileBytes {
		return "文件超过 2 MiB 文本分块上限，仅支持整文件操作", false
	}
	if bytes.Count(baseBytes, []byte("\n"))+1 > maxHunkFileLines || bytes.Count(targetBytes, []byte("\n"))+1 > maxHunkFileLines {
		return "文件行数超过分块上限，仅支持整文件操作", false
	}
	return "", false
}

func (s *Service) hasTransformAttributes(ctx context.Context, cwd, relPath string) bool {
	out, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "check-attr", "-z", "filter", "working-tree-encoding", "--", relPath)
	if err != nil || len(out) == 0 {
		return false
	}
	fields := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	for i := 0; i+2 < len(fields); i += 3 {
		val := string(fields[i+2])
		if val != "unspecified" && val != "unset" && val != "" {
			return true
		}
	}
	return false
}

func (s *Service) cleanNormalizesToLF(ctx context.Context, cwd, relPath string, raw []byte) bool {
	noFilterOut, _, err1 := s.gitInputOutput(ctx, cwd, bytes.NewReader(raw), []string{"GIT_OPTIONAL_LOCKS=0"}, "hash-object", "--no-filters", "--stdin")
	withPathOut, _, err2 := s.gitInputOutput(ctx, cwd, bytes.NewReader(raw), []string{"GIT_OPTIONAL_LOCKS=0"}, "hash-object", "--path="+relPath, "--stdin")
	if err1 != nil || err2 != nil {
		return false
	}
	return outputLine(noFilterOut) != outputLine(withPathOut)
}
