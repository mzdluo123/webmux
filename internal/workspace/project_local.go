package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rainchan/webmux/internal/pathutil"
)

// ProjectDirectories belongs to workspace and does not require the files feature.
func (s *Service) ProjectDirectories(prefix string) ([]string, error) {
	out, err := pathutil.CompleteDirectories(prefix, s.defaultCwd)
	if err != nil {
		return nil, fmt.Errorf("%w: 浏览项目目录: %w", ErrInvalid, err)
	}
	return out, nil
}

// OpenProject explicitly discovers and registers Git when available, while
// ordinary directories remain usable without Git. Git trust/permission errors
// must not be silently presented as an ordinary non-Git project.
func (s *Service) OpenProject(ctx context.Context, path string) (Project, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	if s.ProjectStorageError() != nil {
		return Project{}, false, problemf(ErrUnavailable, "项目清单不可用")
	}
	if path == "" {
		return Project{}, false, problemf(ErrInvalid, "请选择项目目录")
	}
	dir, err := canonical(path)
	if err != nil {
		return Project{}, false, fmt.Errorf("%w: 项目目录: %w", ErrInvalid, err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return Project{}, false, fmt.Errorf("%w: 项目目录: %w", ErrInvalid, err)
	}
	if !info.IsDir() {
		return Project{}, false, problemf(ErrInvalid, "项目路径必须是文件夹")
	}
	projects, err := s.ListProjects()
	if err != nil {
		return Project{}, false, err
	}
	for _, p := range projects {
		if p.Path == dir {
			return p, false, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return Project{}, false, err
	}
	if s.gitPath != "" {
		_, stderr, err := s.gitOutput(ctx, dir, "rev-parse", "--git-common-dir")
		if err == nil {
			return s.openGitProject(ctx, dir)
		}
		if !strings.Contains(stderr, "not a git repository") {
			return Project{}, false, err
		}
	}
	return s.RegisterDirectoryProject(dir)
}

// CreateProject never overwrites an existing target. On later failure it keeps
// the new directory so users can register it again without losing their files.
func (s *Service) CreateProject(ctx context.Context, parent, name string, initializeGit bool) (Project, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	if err := s.ProjectStorageError(); err != nil {
		return Project{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	_, err := pathutil.CleanName(name)
	if err != nil {
		return Project{}, problemf(ErrInvalid, "项目名称必须是一个文件夹名称")
	}
	if parent == "" {
		return Project{}, problemf(ErrInvalid, "请选择父目录")
	}
	base, err := canonical(parent)
	if err != nil {
		return Project{}, fmt.Errorf("%w: 父目录: %w", ErrInvalid, err)
	}
	if initializeGit && s.gitPath == "" {
		return Project{}, problemf(ErrUnavailable, "Git 不可用，无法初始化仓库")
	}
	if err := ctx.Err(); err != nil {
		return Project{}, err
	}
	target := filepath.Join(base, name)
	if err := s.createProjectDirectory(target); err != nil {
		if os.IsExist(err) {
			return Project{}, problemf(ErrConflict, "目标目录已存在，请选择本地文件夹或修改名称")
		}
		if os.IsPermission(err) {
			return Project{}, fmt.Errorf("%w: 服务用户无权创建项目目录: %w", ErrUnavailable, err)
		}
		return Project{}, fmt.Errorf("%w: 创建项目目录: %w", ErrInternal, err)
	}
	partial := Project{Path: target, Name: name}
	if initializeGit {
		if _, err := s.git(ctx, target, "init", "--initial-branch=main", "."); err != nil {
			return partial, err
		}
		p, _, err := s.openGitProject(ctx, target)
		if err != nil {
			return partial, err
		}
		return p, nil
	}
	p, _, err := s.RegisterDirectoryProject(target)
	if err != nil {
		return partial, err
	}
	return p, nil
}

func (s *Service) createProjectDirectory(path string) error {
	if s.mkdirProject != nil {
		return s.mkdirProject(path)
	}
	return os.Mkdir(path, 0755)
}

func (s *Service) openGitProject(ctx context.Context, dir string) (Project, bool, error) {
	repo, created, err := s.registerRepository(ctx, dir)
	if err != nil {
		return Project{}, false, err
	}
	s.reassociate(ctx)
	projects, err := s.ListProjects()
	if err != nil {
		return Project{}, false, err
	}
	for _, p := range projects {
		if p.RepositoryID == repo.entry.ID {
			return p, created, nil
		}
	}
	return Project{}, false, problemf(ErrUnavailable, "仓库已登记但项目暂不可读取，请刷新")
}
