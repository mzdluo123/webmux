package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RepositoryLocation describes a registered repository's verified Git identity
// without reading per-worktree status or creating any session.
type RepositoryLocation struct {
	RepositoryID string
	Name         string
	CommonDir    string
	AnchorPath   string
	Bare         bool
	Available    bool
	Reason       string
}

// WorktreeLocation describes one verified worktree of a registered repository.
type WorktreeLocation struct {
	RepositoryID string
	WorktreeID   string
	Path         string
	CommonDir    string
	Kind         string
	Available    bool
	Reason       string
}

// LocateRepository verifies that repositoryID is registered and resolves its
// shared Git directory identity without requiring any session.
func (s *Service) LocateRepository(ctx context.Context, repositoryID string) (RepositoryLocation, error) {
	if repositoryID == "" {
		return RepositoryLocation{}, problemf(ErrInvalid, "请选择已登记的仓库")
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return RepositoryLocation{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	if err := s.verifyIdentity(ctx, r); err != nil {
		return RepositoryLocation{}, err
	}
	e := r.entry
	loc := RepositoryLocation{
		RepositoryID: e.ID,
		Name:         e.DisplayName,
		CommonDir:    e.CommonDir,
		AnchorPath:   e.AnchorPath,
		Available:    true,
	}
	data, err := s.git(ctx, e.AnchorPath, "worktree", "list", "--porcelain", "-z")
	if err == nil {
		if trees, parseErr := parseWorktrees(data, e.ID); parseErr == nil && len(trees) == 1 && trees[0].Kind == "bare" {
			loc.Bare = true
		}
	}
	return loc, nil
}

// LocateWorktree verifies that worktreeID belongs to registered repositoryID
// and resolves its canonical root path, kind (main/linked/bare) and commonDir.
func (s *Service) LocateWorktree(ctx context.Context, repositoryID, worktreeID string) (WorktreeLocation, error) {
	if repositoryID == "" || worktreeID == "" {
		return WorktreeLocation{}, problemf(ErrInvalid, "请选择已登记的仓库与工作树")
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return WorktreeLocation{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	if err := s.verifyIdentity(ctx, r); err != nil {
		return WorktreeLocation{}, err
	}
	e := r.entry
	data, err := s.git(ctx, e.AnchorPath, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		if ctx.Err() != nil {
			return WorktreeLocation{}, ctx.Err()
		}
		return WorktreeLocation{}, problemf(ErrUnavailable, "%s", err.Error())
	}
	trees, err := parseWorktreesRaw(data, e.ID)
	if err != nil {
		return WorktreeLocation{}, problemf(ErrUnavailable, "%s", err.Error())
	}
	var target *Worktree
	for i := range trees {
		if trees[i].ID == worktreeID {
			target = &trees[i]
			break
		}
	}
	if target == nil {
		return WorktreeLocation{}, problemf(ErrNotFound, "工作树不存在或不属于该仓库，请刷新")
	}
	loc := WorktreeLocation{
		RepositoryID: e.ID,
		WorktreeID:   target.ID,
		Path:         target.Path,
		CommonDir:    e.CommonDir,
		Kind:         target.Kind,
	}
	if target.Kind == "bare" {
		loc.Available = true
		loc.Reason = "当前仓库为 bare 仓库，仅开放只读提交历史、引用与历史对象比较"
		return loc, nil
	}
	info, statErr := os.Lstat(target.Path)
	if statErr != nil || !info.IsDir() {
		loc.Reason = fmt.Sprintf("工作树目录不可用或已被替换: %v", statErr)
		return loc, nil
	}
	resolved, evalErr := filepath.EvalSymlinks(target.Path)
	if evalErr != nil || resolved != target.Path {
		loc.Reason = "工作树路径包含符号链接或已被替换"
		return loc, nil
	}
	actual, checkErr := s.common(ctx, target.Path)
	if checkErr != nil || actual != e.CommonDir {
		loc.Reason = fmt.Sprintf("工作树 Git 身份不一致: %v", checkErr)
		return loc, nil
	}
	loc.Available = true
	return loc, nil
}

// parseWorktreesRaw preserves each worktree's registered Git path without
// following a newly replaced leaf symlink so LocateWorktree can detect and
// report directory-replacement or symlink forgery as a worktree conflict.
func parseWorktreesRaw(data []byte, repository string) ([]Worktree, error) {
	if len(data) == 0 || data[len(data)-1] != 0 {
		return nil, errors.New("worktree 输出不是 NUL 格式")
	}
	out := make([]Worktree, 0)
	var current *Worktree
	for _, raw := range bytes.Split(data, []byte{0}) {
		field := string(raw)
		if field == "" {
			current = nil
			continue
		}
		key, value, _ := strings.Cut(field, " ")
		if key == "worktree" {
			if !filepath.IsAbs(value) {
				return nil, errors.New("工作树路径不是绝对路径（来自 Git）")
			}
			cleanPath := filepath.Clean(value)
			parent, err := canonicalTarget(filepath.Dir(cleanPath))
			if err == nil {
				cleanPath = filepath.Join(parent, filepath.Base(cleanPath))
			}
			kind := "linked"
			if len(out) == 0 {
				kind = "main"
			}
			out = append(out, Worktree{ID: worktreeID(repository, cleanPath), RepositoryID: repository, Path: cleanPath, Kind: kind})
			current = &out[len(out)-1]
			continue
		}
		if current == nil {
			return nil, errors.New("工作树记录缺少路径（来自 Git）")
		}
		if key == "bare" {
			current.Kind = "bare"
		}
	}
	return out, nil
}
