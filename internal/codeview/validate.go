package codeview

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) validate(ctx context.Context, b *binding) error {
	if b.Kind == "repository" || b.Kind == "bare" {
		if b.commonDir == "" {
			return problemf(ErrConflict, "仓库目录身份缺失")
		}
		commonInfo, err := os.Stat(b.commonDir)
		if err != nil || !commonInfo.IsDir() || !os.SameFile(commonInfo, b.commonIdentity) {
			return problemf(ErrConflict, "仓库 common directory 已改变或替换")
		}
		common, e := git(ctx, b.commonDir, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if e != nil {
			return e
		}
		commonPath, e := filepath.EvalSymlinks(strings.TrimSuffix(string(common), "\n"))
		if e != nil || commonPath != b.commonDir {
			return problemf(ErrConflict, "仓库 common directory 已改变")
		}
		return nil
	}
	stat, err := os.Lstat(b.Root)
	if err != nil || !stat.IsDir() || !os.SameFile(b.identity, stat) {
		return problemf(ErrConflict, "浏览根身份已失效")
	}
	if b.Diff {
		top, e := git(ctx, b.Root, "rev-parse", "--show-toplevel")
		if e != nil {
			return e
		}
		actual, e := filepath.EvalSymlinks(strings.TrimSuffix(string(top), "\n"))
		if e != nil || actual != b.Root {
			return problemf(ErrConflict, "工作树根来源已改变")
		}
		common, e := git(ctx, b.Root, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if e != nil {
			return e
		}
		commonPath, e := filepath.EvalSymlinks(strings.TrimSuffix(string(common), "\n"))
		if e != nil || commonPath != b.gitDirs[1] {
			return problemf(ErrConflict, "仓库 common directory 已改变")
		}
		commonInfo, e := os.Stat(commonPath)
		if e != nil || !os.SameFile(commonInfo, b.commonIdentity) {
			return problemf(ErrConflict, "仓库 common directory 已替换")
		}
		out, e := git(ctx, b.Root, "rev-parse", "--absolute-git-dir")
		if e != nil {
			return e
		}
		p, e := filepath.EvalSymlinks(strings.TrimSuffix(string(out), "\n"))
		if e != nil || p != b.gitDirs[0] {
			return problemf(ErrConflict, "Git 来源身份已改变")
		}
		stat, e = os.Stat(p)
		if e != nil || !os.SameFile(stat, b.gitIdentity) {
			return problemf(ErrConflict, "Git 管理目录已替换")
		}
	}
	return nil
}
