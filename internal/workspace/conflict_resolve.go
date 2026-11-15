package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/rainchan/webmux/internal/worktreefs"
)

const (
	ResolutionText        = "text"
	ResolutionChooseLeft  = "choose_left"
	ResolutionChooseRight = "choose_right"
	ResolutionDelete      = "delete"
)

// SaveConflictResult writes the user's merged text to the worktree file on disk
// after verifying the unmerged index stages and on-disk SHA-256 snapshot,
// WITHOUT staging or clearing stages 1..3 in .git/index.
func (s *Service) SaveConflictResult(ctx context.Context, req SaveConflictResultRequest) (SaveConflictResultResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	cleanPath, err := validateRepoRelPath(req.Identity.Path)
	if err != nil {
		return SaveConflictResultResponse{}, err
	}
	if len(req.ResultText) > maxHunkFileBytes {
		return SaveConflictResultResponse{}, problemf(ErrTooLarge, "合并结果超过 2 MiB 上限")
	}

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return SaveConflictResultResponse{}, err
	}
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return SaveConflictResultResponse{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return SaveConflictResultResponse{}, err
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return SaveConflictResultResponse{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}
	if err := checkWorktreeIndexLock(gitDir); err != nil {
		return SaveConflictResultResponse{}, err
	}

	current, err := s.inspectConflictFileLocked(ctx, req.RepositoryID, req.WorktreeID, loc, cleanPath)
	if err != nil {
		return SaveConflictResultResponse{}, err
	}
	if !current.TextMergeable {
		return SaveConflictResultResponse{}, problemf(ErrConflict, "文件 %s 不支持文本保存结果: %s", cleanPath, current.DegradeReason)
	}
	if current.Identity != req.Identity {
		return SaveConflictResultResponse{}, problemf(ErrStale, "文件 %s 的冲突阶段或磁盘内容已被外部修改，请重新载入后再保存", cleanPath)
	}

	execBit := current.Identity.WorktreeMode == "100755" ||
		current.Left.Mode == "100755" ||
		current.Right.Mode == "100755"
	newSum, err := worktreefs.WriteRegular(
		loc.Path,
		cleanPath,
		[]byte(req.ResultText),
		execBit,
		true,
		req.Identity.WorktreeSHA256,
		maxWorkspaceGitOutput,
	)
	if err != nil {
		return SaveConflictResultResponse{}, mapWorktreeFSError(cleanPath, err)
	}

	nextID := current.Identity
	if execBit {
		nextID.WorktreeMode = "100755"
	} else {
		nextID.WorktreeMode = "100644"
	}
	nextID.WorktreeSHA256 = newSum
	return SaveConflictResultResponse{
		Path:     cleanPath,
		Identity: nextID,
	}, nil
}

// ResolveConflict marks a conflicted path as resolved in the real Git index
// (clearing stages 1..3 -> stage 0 or removing on delete) after verifying
// snapshot identity and keeping the worktree file synchronized with the index.
func (s *Service) ResolveConflict(ctx context.Context, req ResolveConflictRequest) (ResolveConflictResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	cleanPath, err := validateRepoRelPath(req.Identity.Path)
	if err != nil {
		return ResolveConflictResponse{}, err
	}
	switch req.Resolution {
	case ResolutionText, ResolutionChooseLeft, ResolutionChooseRight, ResolutionDelete:
	default:
		return ResolveConflictResponse{}, problemf(ErrInvalid, "不支持的冲突解决方式: %s", req.Resolution)
	}
	if req.Resolution == ResolutionText && len(req.ResultText) > maxHunkFileBytes {
		return ResolveConflictResponse{}, problemf(ErrTooLarge, "合并结果超过 2 MiB 上限")
	}

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return ResolveConflictResponse{}, err
	}
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return ResolveConflictResponse{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return ResolveConflictResponse{}, err
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return ResolveConflictResponse{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}
	if err := checkWorktreeIndexLock(gitDir); err != nil {
		return ResolveConflictResponse{}, err
	}

	current, err := s.inspectConflictFileLocked(ctx, req.RepositoryID, req.WorktreeID, loc, cleanPath)
	if err != nil {
		return ResolveConflictResponse{}, err
	}
	if current.Identity != req.Identity {
		return ResolveConflictResponse{}, problemf(ErrStale, "文件 %s 的冲突阶段或磁盘内容已被外部修改，请重新核对后再标记已解决", cleanPath)
	}

	switch req.Resolution {
	case ResolutionText:
		if !current.TextMergeable {
			return ResolveConflictResponse{}, problemf(ErrConflict, "文件 %s 不支持文本合并解决: %s", cleanPath, current.DegradeReason)
		}
		if err := s.resolveConflictWithTextLocked(ctx, loc.Path, cleanPath, current, req.ResultText); err != nil {
			return ResolveConflictResponse{}, err
		}
	case ResolutionChooseLeft:
		if err := s.resolveConflictWithSideLocked(ctx, loc.Path, cleanPath, current, current.Left); err != nil {
			return ResolveConflictResponse{}, err
		}
	case ResolutionChooseRight:
		if err := s.resolveConflictWithSideLocked(ctx, loc.Path, cleanPath, current, current.Right); err != nil {
			return ResolveConflictResponse{}, err
		}
	case ResolutionDelete:
		if err := s.resolveConflictDeleteLocked(ctx, loc.Path, cleanPath, current); err != nil {
			return ResolveConflictResponse{}, err
		}
	}

	inProg, err := s.inspectInProgressLocked(ctx, loc)
	if err != nil {
		return ResolveConflictResponse{}, err
	}
	return ResolveConflictResponse{
		Path:       cleanPath,
		InProgress: inProg,
	}, nil
}

func (s *Service) resolveConflictWithTextLocked(ctx context.Context, cwd, cleanPath string, current ConflictFileDetail, resultText string) error {
	content := []byte(resultText)
	wantDigest := sha256.Sum256(content)
	wantSum := hex.EncodeToString(wantDigest[:])

	execBit := current.Identity.WorktreeMode == "100755" ||
		current.Left.Mode == "100755" ||
		current.Right.Mode == "100755"
	mode := "100644"
	if execBit {
		mode = "100755"
	}

	if current.Identity.WorktreeSHA256 != wantSum || current.Identity.WorktreeMode != mode {
		if _, err := worktreefs.WriteRegular(
			cwd,
			cleanPath,
			content,
			execBit,
			true,
			current.Identity.WorktreeSHA256,
			maxWorkspaceGitOutput,
		); err != nil {
			return mapWorktreeFSError(cleanPath, err)
		}
	}

	blobOID, err := s.writeBlobWithPathFilter(ctx, cwd, cleanPath, content)
	if err != nil {
		return err
	}
	return s.stageZeroClearUnmergedLocked(ctx, cwd, cleanPath, mode, blobOID)
}

func (s *Service) resolveConflictWithSideLocked(ctx context.Context, cwd, cleanPath string, current ConflictFileDetail, chosen ConflictSide) error {
	if !chosen.Present || chosen.Mode == "" || chosen.BlobOID == "" {
		return problemf(ErrInvalid, "所选一侧不包含文件版本，若要移除文件请选择删除文件")
	}
	switch chosen.Mode {
	case "160000":
		return s.stageZeroClearUnmergedLocked(ctx, cwd, cleanPath, "160000", chosen.BlobOID)
	case "120000":
		if err := s.stageZeroClearUnmergedLocked(ctx, cwd, cleanPath, "120000", chosen.BlobOID); err != nil {
			return err
		}
		_, _, err := s.gitInputOutput(ctx, cwd, nil, nil, "checkout-index", "-f", "--", cleanPath)
		return err
	default:
		raw, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "cat-file", "blob", chosen.BlobOID)
		if err != nil {
			return err
		}
		execBit := chosen.Mode == "100755"
		if _, wErr := worktreefs.WriteRegular(
			cwd,
			cleanPath,
			raw,
			execBit,
			true,
			current.Identity.WorktreeSHA256,
			maxWorkspaceGitOutput,
		); wErr != nil {
			return mapWorktreeFSError(cleanPath, wErr)
		}
		mode := "100644"
		if execBit {
			mode = "100755"
		}
		return s.stageZeroClearUnmergedLocked(ctx, cwd, cleanPath, mode, chosen.BlobOID)
	}
}

func (s *Service) resolveConflictDeleteLocked(ctx context.Context, cwd, cleanPath string, current ConflictFileDetail) error {
	if current.Left.Mode == "160000" || current.Right.Mode == "160000" {
		return problemf(ErrConflict, "子模块条目删除请在终端处理")
	}
	if err := worktreefs.RemoveEntry(cwd, cleanPath); err != nil {
		return mapWorktreeFSError(cleanPath, err)
	}
	_, _, err := s.gitInputOutput(ctx, cwd, nil, nil, "update-index", "--force-remove", "--", cleanPath)
	return err
}

func (s *Service) stageZeroClearUnmergedLocked(ctx context.Context, cwd, cleanPath, mode, blobOID string) error {
	zeroOID := strings.Repeat("0", len(blobOID))
	if len(zeroOID) != 40 && len(zeroOID) != 64 {
		zeroOID = strings.Repeat("0", 40)
	}
	var payload bytes.Buffer
	// Feeding mode 0 first removes all unmerged stages (1..3) for cleanPath;
	// feeding stage 0 next writes the resolved entry atomically in one index transaction.
	payload.WriteString("0 " + zeroOID + "\t" + cleanPath)
	payload.WriteByte(0)
	payload.WriteString(mode + " " + blobOID + " 0\t" + cleanPath)
	payload.WriteByte(0)
	_, _, err := s.gitInputOutput(ctx, cwd, &payload, nil, "update-index", "-z", "--index-info")
	return err
}

func mapWorktreeFSError(cleanPath string, err error) error {
	switch {
	case errors.Is(err, worktreefs.ErrStale):
		return problemf(ErrStale, "文件 %s 在磁盘上已被外部进程修改，请刷新后重试", cleanPath)
	case errors.Is(err, worktreefs.ErrSymlink):
		return problemf(ErrConflict, "路径 %s 涉及符号链接或嵌套仓库边界，拒绝直接覆盖", cleanPath)
	case errors.Is(err, worktreefs.ErrInvalid):
		return problemf(ErrInvalid, "文件路径无效: %s", cleanPath)
	default:
		return problemf(ErrInternal, "写入工作区文件 %s 失败: %v", cleanPath, err)
	}
}
