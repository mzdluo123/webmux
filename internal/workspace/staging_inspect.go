package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// InspectStagingFile captures the exact identity and stageable hunks for one file.
func (s *Service) InspectStagingFile(ctx context.Context, repositoryID, worktreeID string, area StageArea, relPath string) (StagingFileDiff, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	cleanPath, err := validateRepoRelPath(relPath)
	if err != nil {
		return StagingFileDiff{}, err
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
	return s.inspectFileLocked(ctx, loc, headOID, area, cleanPath, "", "")
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

	var baseBytes, targetBytes, wtRaw []byte
	id := StageFileIdentity{Area: area, Path: relPath, OldPath: oldPath}

	if area == StageAreaUnstaged {
		id.BaseOID = idxEntry.oid
		id.BaseMode = idxEntry.mode
		wtMode, wtSum, raw, wtErr := s.readWorktreeFile(ctx, loc.Path, relPath, idxEntry.mode)
		if wtErr != nil {
			return StagingFileDiff{}, wtErr
		}
		id.TargetMode = wtMode
		id.TargetSHA256 = wtSum
		wtRaw = raw
		if id.BaseOID != "" && isRegularGitMode(id.BaseMode) {
			baseBytes, err = ReadGit(ctx, loc.Path, maxWorkspaceGitOutput, "cat-file", "blob", id.BaseOID)
			if err != nil {
				return StagingFileDiff{}, err
			}
		}
		targetBytes = raw
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
		if id.BaseOID != "" && isRegularGitMode(id.BaseMode) {
			baseBytes, err = ReadGit(ctx, loc.Path, maxWorkspaceGitOutput, "cat-file", "blob", id.BaseOID)
			if err != nil {
				return StagingFileDiff{}, err
			}
		}
		if id.TargetOID != "" && isRegularGitMode(id.TargetMode) {
			targetBytes, err = ReadGit(ctx, loc.Path, maxWorkspaceGitOutput, "cat-file", "blob", id.TargetOID)
			if err != nil {
				return StagingFileDiff{}, err
			}
		}
	}

	out := StagingFileDiff{
		Identity:    id,
		Hunks:       []StagingHunk{},
		baseBytes:   baseBytes,
		targetBytes: targetBytes,
		worktreeRaw: wtRaw,
	}

	if reason, bin := s.degradeHunkReason(ctx, loc.Path, relPath, oldPath, statusHint, id, baseBytes, targetBytes); reason != "" {
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

func (s *Service) readWorktreeFile(ctx context.Context, rootPath, relPath, indexMode string) (mode, sum string, raw []byte, err error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return "", "", nil, problemf(ErrUnavailable, "无法打开工作树目录: %v", err)
	}
	defer func() { _ = root.Close() }()

	fsRel := filepath.FromSlash(relPath)
	info, err := root.Lstat(fsRel)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", nil, nil
	}
	if err != nil {
		return "", "", nil, problemf(ErrConflict, "无法读取工作区路径 %s: %v", relPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, lErr := root.Readlink(fsRel)
		if lErr != nil {
			return "", "", nil, problemf(ErrConflict, "无法读取符号链接 %s: %v", relPath, lErr)
		}
		b := []byte(target)
		digest := sha256.Sum256(b)
		return "120000", hex.EncodeToString(digest[:]), b, nil
	}
	if info.IsDir() {
		if indexMode == "160000" {
			subOID, _ := s.resolveCommitReadOnly(ctx, filepath.Join(rootPath, fsRel), "HEAD")
			return "160000", subOID, nil, nil
		}
		return "", "", nil, problemf(ErrInvalid, "路径 %s 是目录而非文件", relPath)
	}
	if !info.Mode().IsRegular() {
		return "", "", nil, problemf(ErrInvalid, "路径 %s 不是普通文件", relPath)
	}
	f, err := root.Open(fsRel)
	if err != nil {
		return "", "", nil, problemf(ErrConflict, "无法读取文件 %s: %v", relPath, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxWorkspaceGitOutput+1))
	if err != nil {
		return "", "", nil, problemf(ErrInternal, "读取文件 %s 失败: %v", relPath, err)
	}
	if len(data) > maxWorkspaceGitOutput {
		return "", "", nil, problemf(ErrTooLarge, "文件 %s 超过大小限制", relPath)
	}
	gitMode := "100644"
	if info.Mode()&0o111 != 0 {
		gitMode = "100755"
	}
	digest := sha256.Sum256(data)
	return gitMode, hex.EncodeToString(digest[:]), data, nil
}

func (s *Service) degradeHunkReason(ctx context.Context, cwd, relPath, oldPath, statusHint string, id StageFileIdentity, baseBytes, targetBytes []byte) (string, bool) {
	if oldPath != "" || strings.HasPrefix(statusHint, "R") || strings.HasPrefix(statusHint, "C") {
		return "重命名或复制条目仅支持整文件操作", false
	}
	if (id.BaseMode != "" && !isRegularGitMode(id.BaseMode)) || (id.TargetMode != "" && !isRegularGitMode(id.TargetMode)) {
		return "符号链接或子模块仅支持整文件操作", false
	}
	if id.Area == StageAreaUnstaged && id.TargetMode == "" {
		return "已删除文件仅支持整文件暂存", false
	}
	if id.Area == StageAreaStaged && id.TargetMode == "" {
		return "已暂存删除条目仅支持整文件取消暂存", false
	}
	if s.hasTransformAttributes(ctx, cwd, relPath) {
		return "配置了 filter 或 working-tree-encoding 属性，仅支持整文件操作", false
	}
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
