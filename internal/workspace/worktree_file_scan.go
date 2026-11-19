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
	"strconv"
	"strings"
)

type worktreeFileScan struct {
	mode          string
	sum           string
	raw           []byte
	rawLoaded     bool
	size          int64
	exceedsHunk   bool
	untrackedRepo bool
	replacedByDir bool
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr contextReader) Read(p []byte) (int, error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, err
	}
	return cr.r.Read(p)
}

func wrapFileReadError(relPath string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return problemf(ErrTimeout, "读取文件 %s 取消或超时: %v", relPath, err)
	}
	return problemf(ErrInternal, "读取文件 %s 失败: %v", relPath, err)
}

func (s *Service) scanWorktreeFile(ctx context.Context, rootPath, relPath, indexMode string) (worktreeFileScan, error) {
	if err := ctx.Err(); err != nil {
		return worktreeFileScan{}, wrapFileReadError(relPath, err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return worktreeFileScan{}, problemf(ErrUnavailable, "无法打开工作树目录: %v", err)
	}
	defer func() { _ = root.Close() }()

	fsRel := filepath.FromSlash(relPath)
	info, err := root.Lstat(fsRel)
	if errors.Is(err, os.ErrNotExist) {
		return worktreeFileScan{}, nil
	}
	if err != nil {
		return worktreeFileScan{}, problemf(ErrConflict, "无法读取工作区路径 %s: %v", relPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, lErr := root.Readlink(fsRel)
		if lErr != nil {
			return worktreeFileScan{}, problemf(ErrConflict, "无法读取符号链接 %s: %v", relPath, lErr)
		}
		b := []byte(target)
		digest := sha256.Sum256(b)
		return worktreeFileScan{
			mode:      "120000",
			sum:       hex.EncodeToString(digest[:]),
			raw:       b,
			rawLoaded: true,
			size:      int64(len(b)),
		}, nil
	}
	if info.IsDir() {
		_, dotErr := root.Lstat(filepath.Join(fsRel, ".git"))
		hasDotGit := dotErr == nil
		if indexMode == "160000" || hasDotGit {
			subSum := s.inspectNestedGitHead(ctx, rootPath, filepath.Join(rootPath, fsRel), hasDotGit)
			return worktreeFileScan{
				mode:          "160000",
				sum:           subSum,
				untrackedRepo: indexMode != "160000",
			}, nil
		}
		if indexMode != "" {
			return worktreeFileScan{replacedByDir: true}, nil
		}
		return worktreeFileScan{}, problemf(ErrInvalid, "路径 %s 是目录，仅支持文件路径", relPath)
	}
	if !info.Mode().IsRegular() {
		if indexMode != "" {
			return worktreeFileScan{}, nil
		}
		return worktreeFileScan{}, problemf(ErrInvalid, "路径 %s 为非常规文件", relPath)
	}

	f, err := root.Open(fsRel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return worktreeFileScan{}, nil
		}
		return worktreeFileScan{}, problemf(ErrConflict, "无法读取文件 %s: %v", relPath, err)
	}
	defer func() { _ = f.Close() }()

	fInfo, err := f.Stat()
	if err != nil || !fInfo.Mode().IsRegular() || !os.SameFile(info, fInfo) {
		return worktreeFileScan{}, problemf(ErrConflict, "文件 %s 在读取过程中发生变化", relPath)
	}

	gitMode := "100644"
	if fInfo.Mode()&0o111 != 0 {
		gitMode = "100755"
	}

	cr := contextReader{ctx: ctx, r: f}
	if fInfo.Size() <= maxHunkFileBytes {
		data, rErr := io.ReadAll(io.LimitReader(cr, maxHunkFileBytes+1))
		if rErr != nil {
			return worktreeFileScan{}, wrapFileReadError(relPath, rErr)
		}
		if len(data) <= maxHunkFileBytes {
			digest := sha256.Sum256(data)
			return worktreeFileScan{
				mode:      gitMode,
				sum:       hex.EncodeToString(digest[:]),
				raw:       data,
				rawLoaded: true,
				size:      int64(len(data)),
			}, nil
		}
		hasher := sha256.New()
		_, _ = hasher.Write(data)
		rem, cErr := io.Copy(hasher, cr)
		if cErr != nil {
			return worktreeFileScan{}, wrapFileReadError(relPath, cErr)
		}
		return worktreeFileScan{
			mode:        gitMode,
			sum:         hex.EncodeToString(hasher.Sum(nil)),
			size:        int64(len(data)) + rem,
			exceedsHunk: true,
		}, nil
	}

	hasher := sha256.New()
	total, err := io.Copy(hasher, cr)
	if err != nil {
		return worktreeFileScan{}, wrapFileReadError(relPath, err)
	}
	return worktreeFileScan{
		mode:        gitMode,
		sum:         hex.EncodeToString(hasher.Sum(nil)),
		size:        total,
		exceedsHunk: true,
	}, nil
}

func (s *Service) inspectNestedGitHead(ctx context.Context, parentRoot, subPath string, hasDotGit bool) string {
	if !hasDotGit || s.gitPath == "" {
		return "unborn:uninitialized"
	}
	extra := []string{"GIT_OPTIONAL_LOCKS=0", "GIT_CEILING_DIRECTORIES=" + parentRoot}
	stdout, run := s.runGit(ctx, subPath, extra, "rev-parse", "--verify", "-q", "--end-of-options", "HEAD^{commit}")
	if run.err == nil {
		if oid := strings.TrimSpace(stdout.String()); isObjectID(oid) {
			return oid
		}
	}
	symOut, symRun := s.runGit(ctx, subPath, extra, "symbolic-ref", "-q", "HEAD")
	if symRun.err == nil {
		if ref := strings.TrimSpace(symOut.String()); ref != "" {
			return "unborn:" + ref
		}
	}
	return "unborn:HEAD"
}

func (s *Service) readGitBlobForHunk(ctx context.Context, cwd, oid string) (raw []byte, loaded bool, exceedsHunk bool, err error) {
	sizeOut, err := ReadGit(ctx, cwd, 128, "cat-file", "-s", oid)
	if err != nil {
		return nil, false, false, err
	}
	sz, pErr := strconv.ParseInt(strings.TrimSpace(string(sizeOut)), 10, 64)
	if pErr != nil {
		return nil, false, false, problemf(ErrInternal, "无效 Git 对象大小输出")
	}
	if sz > maxHunkFileBytes {
		return nil, false, true, nil
	}
	raw, err = ReadGit(ctx, cwd, maxHunkFileBytes+1, "cat-file", "blob", oid)
	if err != nil {
		if errors.Is(err, ErrTooLarge) {
			return nil, false, true, nil
		}
		return nil, false, false, err
	}
	if len(raw) > maxHunkFileBytes {
		return nil, false, true, nil
	}
	return raw, true, false, nil
}

func (s *Service) streamWorktreeBlobWrite(
	ctx context.Context,
	rootPath, cleanPath, expectedSHA256 string,
	applyPathFilter bool,
) (string, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return "", problemf(ErrUnavailable, "无法打开工作树目录: %v", err)
	}
	defer func() { _ = root.Close() }()

	fsRel := filepath.FromSlash(cleanPath)
	linfo, err := root.Lstat(fsRel)
	if errors.Is(err, os.ErrNotExist) {
		return "", problemf(ErrStale, "文件 %s 在写入前已被移除，请刷新后重试", cleanPath)
	}
	if err != nil {
		return "", problemf(ErrConflict, "无法读取文件 %s: %v", cleanPath, err)
	}
	if !linfo.Mode().IsRegular() {
		return "", problemf(ErrStale, "文件 %s 类型已在磁盘变化，请刷新后重试", cleanPath)
	}

	f, err := root.Open(fsRel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", problemf(ErrStale, "文件 %s 在写入前已被移除，请刷新后重试", cleanPath)
		}
		return "", problemf(ErrConflict, "无法打开文件 %s: %v", cleanPath, err)
	}
	defer func() { _ = f.Close() }()

	finfo, err := f.Stat()
	if err != nil || !finfo.Mode().IsRegular() || !os.SameFile(linfo, finfo) {
		return "", problemf(ErrStale, "文件 %s 在写入前已在磁盘变化，请刷新后重试", cleanPath)
	}

	hasher := sha256.New()
	cr := contextReader{ctx: ctx, r: f}
	tee := io.TeeReader(cr, hasher)

	args := []string{"hash-object", "-w"}
	if applyPathFilter {
		args = append(args, "--path="+cleanPath)
	} else {
		args = append(args, "--no-filters")
	}
	args = append(args, "--stdin")

	out, _, err := s.gitInputOutput(ctx, rootPath, tee, nil, args...)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(hasher, cr); err != nil {
		return "", wrapFileReadError(cleanPath, err)
	}
	actualSum := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA256 != "" && actualSum != expectedSHA256 {
		return "", problemf(ErrStale, "文件 %s 在写入过程中被并发修改，已拒绝更新索引，请刷新后重试", cleanPath)
	}
	oid := strings.TrimSpace(string(out))
	if !isObjectID(oid) {
		return "", problemf(ErrInternal, "Git hash-object 返回了无效对象 ID")
	}
	return oid, nil
}

func (s *Service) hashWorktreeRegularReadOnly(
	ctx context.Context,
	rootPath, relPath string,
	scan worktreeFileScan,
) (string, error) {
	extra := []string{"GIT_OPTIONAL_LOCKS=0"}
	if scan.rawLoaded {
		out, _, err := s.gitInputOutput(ctx, rootPath, bytes.NewReader(scan.raw), extra, "hash-object", "--no-filters", "--stdin")
		if err != nil {
			return "", err
		}
		return outputLine(out), nil
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return "", problemf(ErrUnavailable, "无法打开工作树目录: %v", err)
	}
	defer func() { _ = root.Close() }()

	fsRel := filepath.FromSlash(relPath)
	linfo, err := root.Lstat(fsRel)
	if err != nil || !linfo.Mode().IsRegular() {
		return "", problemf(ErrConflict, "无法读取文件 %s", relPath)
	}
	f, err := root.Open(fsRel)
	if err != nil {
		return "", problemf(ErrConflict, "无法打开文件 %s: %v", relPath, err)
	}
	defer func() { _ = f.Close() }()
	finfo, err := f.Stat()
	if err != nil || !finfo.Mode().IsRegular() || !os.SameFile(linfo, finfo) {
		return "", problemf(ErrConflict, "文件 %s 在读取过程中发生变化", relPath)
	}

	out, _, err := s.gitInputOutput(ctx, rootPath, contextReader{ctx: ctx, r: f}, extra, "hash-object", "--no-filters", "--stdin")
	if err != nil {
		return "", err
	}
	return outputLine(out), nil
}
