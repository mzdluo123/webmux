package codeview

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func (s *Service) resolver() (TargetResolver, error) {
	s.mu.Lock()
	r := s.workspace
	s.mu.Unlock()
	if r == nil {
		return nil, problemf(ErrUnavailable, "仓库定位服务未启用")
	}
	return r, nil
}

func (s *Service) bindRepository(ctx context.Context, key scopeKey) (*binding, error) {
	resolver, err := s.resolver()
	if err != nil {
		return nil, err
	}
	if key.owner == "" || strings.ContainsRune(key.owner, '\x00') {
		return nil, problemf(ErrInvalid, "无效仓库标识")
	}
	loc, err := resolver.LocateRepository(ctx, key.owner)
	if err != nil {
		return nil, problemf(errorKind(err), "%s", err.Error())
	}
	if !loc.Available {
		return nil, problemf(ErrUnavailable, "%s", loc.Reason)
	}
	s.mu.Lock()
	b := s.roots[key]
	if b != nil {
		b.timeUsed = time.Now()
	}
	s.mu.Unlock()
	if b != nil {
		return b, s.validate(ctx, b)
	}
	commonDir, err := filepath.EvalSymlinks(loc.CommonDir)
	if err != nil || commonDir != loc.CommonDir {
		return nil, problemf(ErrConflict, "仓库共享目录路径不一致")
	}
	commonInfo, err := os.Stat(commonDir)
	if err != nil || !commonInfo.IsDir() {
		return nil, problemf(ErrUnavailable, "仓库共享目录不可访问")
	}
	kind := "repository"
	reason := "未选定可用工作树，当前处于只读仓库上下文（仅可查看提交历史、分支引用与历史对象比较，工作区浏览与暂存不可用）"
	if loc.Bare {
		kind = "bare"
		reason = "当前仓库为 bare 仓库，没有工作区或索引，仅开放只读提交历史、引用与历史对象比较"
	}
	b = &binding{
		Context: Context{
			ID:     random(),
			Root:   "",
			Kind:   kind,
			Browse: false,
			Diff:   false,
			Ignore: false,
			Write:  false,
			Reason: reason,
		},
		commonDir:      commonDir,
		commonIdentity: commonInfo,
		timeUsed:       time.Now(),
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if old := s.roots[key]; old != nil {
		old.timeUsed = time.Now()
		b = old
	} else {
		s.roots[key] = b
	}
	s.mu.Unlock()
	return b, s.validate(ctx, b)
}

func (s *Service) bindWorktree(ctx context.Context, key scopeKey) (*binding, error) {
	resolver, err := s.resolver()
	if err != nil {
		return nil, err
	}
	repoID, wtID, ok := strings.Cut(key.owner, "\x00")
	if !ok || repoID == "" || wtID == "" {
		return nil, problemf(ErrInvalid, "无效仓库或工作树标识")
	}
	loc, err := resolver.LocateWorktree(ctx, repoID, wtID)
	if err != nil {
		return nil, problemf(errorKind(err), "%s", err.Error())
	}
	if !loc.Available {
		return nil, problemf(ErrConflict, "%s", loc.Reason)
	}
	s.mu.Lock()
	b := s.roots[key]
	if b != nil {
		b.timeUsed = time.Now()
	}
	s.mu.Unlock()
	if b != nil {
		return b, s.validate(ctx, b)
	}
	if loc.Kind == "bare" {
		commonDir, evalErr := filepath.EvalSymlinks(loc.CommonDir)
		if evalErr != nil || commonDir != loc.CommonDir {
			return nil, problemf(ErrConflict, "仓库共享目录路径不一致")
		}
		commonInfo, statErr := os.Stat(commonDir)
		if statErr != nil || !commonInfo.IsDir() {
			return nil, problemf(ErrUnavailable, "仓库共享目录不可访问")
		}
		b = &binding{
			Context: Context{
				ID:     random(),
				Root:   "",
				Kind:   "bare",
				Browse: false,
				Diff:   false,
				Ignore: false,
				Write:  false,
				Reason: loc.Reason,
			},
			commonDir:      commonDir,
			commonIdentity: commonInfo,
			timeUsed:       time.Now(),
		}
	} else {
		info, statErr := os.Lstat(loc.Path)
		if statErr != nil || !info.IsDir() {
			return nil, problemf(ErrConflict, "工作树目录不存在或不是常规目录")
		}
		resolved, evalErr := filepath.EvalSymlinks(loc.Path)
		if evalErr != nil || resolved != loc.Path {
			return nil, problemf(ErrConflict, "工作树根路径存在符号链接伪造或已改变")
		}
		top, topErr := git(ctx, resolved, "rev-parse", "--show-toplevel")
		if topErr != nil {
			return nil, problemf(ErrConflict, "无法核实工作树 Git 根目录")
		}
		actualTop, topEvalErr := filepath.EvalSymlinks(strings.TrimSuffix(string(top), "\n"))
		if topEvalErr != nil || actualTop != resolved {
			return nil, problemf(ErrConflict, "工作树根目录与 Git 拓扑不一致")
		}
		var gitDirs []string
		for _, arg := range []string{"--absolute-git-dir", "--git-common-dir"} {
			out, gitErr := git(ctx, resolved, "rev-parse", "--path-format=absolute", arg)
			if gitErr != nil {
				return nil, problemf(ErrConflict, "无法读取工作树 Git 管理目录")
			}
			p, pErr := filepath.EvalSymlinks(strings.TrimSuffix(string(out), "\n"))
			if pErr != nil {
				return nil, problemf(ErrConflict, "工作树 Git 管理目录不可解析")
			}
			gitDirs = append(gitDirs, p)
		}
		if gitDirs[1] != loc.CommonDir {
			return nil, problemf(ErrConflict, "工作树不属于目标仓库的共享 Git 目录")
		}
		commonInfo, commonErr := os.Stat(gitDirs[1])
		if commonErr != nil || !commonInfo.IsDir() {
			return nil, problemf(ErrConflict, "仓库共享目录不可访问")
		}
		gitInfo, gitErr := os.Stat(gitDirs[0])
		if gitErr != nil {
			return nil, problemf(ErrConflict, "工作树 Git 目录不可访问")
		}
		b = &binding{
			Context: Context{
				ID:     random(),
				Root:   resolved,
				Kind:   "worktree",
				Browse: true,
				Diff:   true,
				Ignore: true,
				Write:  writable(resolved),
			},
			identity:       info,
			gitDirs:        gitDirs,
			gitIdentity:    gitInfo,
			commonIdentity: commonInfo,
			commonDir:      gitDirs[1],
			timeUsed:       time.Now(),
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if old := s.roots[key]; old != nil {
		old.timeUsed = time.Now()
		b = old
	} else {
		s.roots[key] = b
	}
	s.mu.Unlock()
	return b, s.validate(ctx, b)
}

func withTargetContext[T any](ctx context.Context, s *Service, t Target, cid string, fn func(scopeKey, *binding) (T, error)) (T, error) {
	key := targetScope(t)
	return withScope(ctx, s, key, func(b *binding) (T, error) {
		if b.ID != cid {
			var zero T
			return zero, problemf(ErrConflict, "浏览上下文已改变")
		}
		return fn(key, b)
	})
}

// RepositoryContext binds a session-independent read-only repository context
// verified against the workspace registry.
func (s *Service) RepositoryContext(ctx context.Context, repositoryID string) (Context, error) {
	if repositoryID == "" {
		return Context{}, problemf(ErrInvalid, "请选择已登记的仓库")
	}
	return withScope(ctx, s, targetScope(Target{RepositoryID: repositoryID}), func(b *binding) (Context, error) {
		return b.Context, nil
	})
}

// WorktreeContext binds a session-independent worktree context verified
// against the registered repository and worktree identity.
func (s *Service) WorktreeContext(ctx context.Context, repositoryID, worktreeID string) (Context, error) {
	if repositoryID == "" || worktreeID == "" {
		return Context{}, problemf(ErrInvalid, "请选择已登记的仓库与工作树")
	}
	return withScope(ctx, s, targetScope(Target{RepositoryID: repositoryID, WorktreeID: worktreeID}), func(b *binding) (Context, error) {
		return b.Context, nil
	})
}

// TargetTree lists a directory in a session-independent worktree context.
func (s *Service) TargetTree(ctx context.Context, t Target, cid, nid string, include bool, cursor string) (Listing, error) {
	return withTargetContext(ctx, s, t, cid, func(_ scopeKey, b *binding) (Listing, error) {
		return treeInBinding(ctx, cid, b, nid, include, cursor)
	})
}

// TargetSearch searches files in a session-independent worktree context.
func (s *Service) TargetSearch(ctx context.Context, t Target, cid, q string, include bool) (Listing, error) {
	return withTargetContext(ctx, s, t, cid, func(_ scopeKey, b *binding) (Listing, error) {
		return searchInBinding(ctx, b, q, include)
	})
}

// TargetChanges lists working tree and index changes in a session-independent
// worktree context.
func (s *Service) TargetChanges(ctx context.Context, t Target, cid string, queries ...ChangeQuery) (Changes, error) {
	return withTargetContext(ctx, s, t, cid, func(_ scopeKey, b *binding) (Changes, error) {
		var q ChangeQuery
		if len(queries) > 0 {
			q = queries[0]
		}
		return changesInBinding(ctx, cid, b, q)
	})
}

// TargetFile reads a worktree file snapshot in a session-independent worktree context.
func (s *Service) TargetFile(ctx context.Context, t Target, cid, nid string) (File, error) {
	return s.TargetFileSource(ctx, t, cid, nid, "worktree")
}

// TargetFileSource reads a file from worktree, index or head in a
// session-independent worktree context.
func (s *Service) TargetFileSource(ctx context.Context, t Target, cid, nid, source string) (File, error) {
	return withTargetContext(ctx, s, t, cid, func(key scopeKey, b *binding) (File, error) {
		return s.fileFromSource(ctx, key, cid, b, nid, source)
	})
}

// TargetDiff captures a frozen diff snapshot for one change entry in a
// session-independent worktree context.
func (s *Service) TargetDiff(ctx context.Context, t Target, cid, changeID string) (Diff, error) {
	return withTargetContext(ctx, s, t, cid, func(key scopeKey, b *binding) (Diff, error) {
		return s.diffInBinding(ctx, key, cid, b, changeID)
	})
}

// TargetSnapshotFile reads one side of a snapshot belonging to t.
func (s *Service) TargetSnapshotFile(ctx context.Context, t Target, sid, side string) (File, error) {
	key := targetScope(t)
	return withScope(ctx, s, key, func(*binding) (File, error) {
		return s.snapshotSideFile(key, sid, side)
	})
}

// TargetCheck checks snapshots, directories and change version in t.
func (s *Service) TargetCheck(ctx context.Context, t Target, req CheckRequest) (Check, error) {
	if len(req.Snapshots) > 40 || len(req.Directories) > 30 {
		return Check{}, problemf(ErrInvalid, "检查数量超过限制")
	}
	return withTargetContext(ctx, s, t, req.Context, func(key scopeKey, b *binding) (Check, error) {
		return s.checkInBinding(ctx, key, b, req)
	})
}

// TargetRaw opens a verified download transfer for a snapshot owned by t.
func (s *Service) TargetRaw(ctx context.Context, t Target, sid, side string) (*Transfer, error) {
	key := targetScope(t)
	return holdTransfer(s, func() (*Transfer, error) {
		return withScope(ctx, s, key, func(b *binding) (*Transfer, error) {
			return s.openRawInBinding(ctx, key, b, sid, side)
		})
	}, func(tr *Transfer, done func()) { tr.release = done })
}

// TargetNodeRaw opens a verified download transfer for a regular file in t.
func (s *Service) TargetNodeRaw(ctx context.Context, t Target, cid, nid string) (*Transfer, error) {
	return holdTransfer(s, func() (*Transfer, error) {
		return withTargetContext(ctx, s, t, cid, func(_ scopeKey, b *binding) (*Transfer, error) {
			if !b.Browse {
				return nil, problemf(ErrConflict, "当前上下文没有可用工作树文件目录")
			}
			p, e := nodePath(nid)
			if e != nil {
				return nil, e
			}
			f, e := b.open(p)
			if e != nil {
				return nil, e
			}
			info, e := f.Stat()
			switch {
			case e != nil:
				_ = f.Close()
				return nil, e
			case !info.Mode().IsRegular():
				_ = f.Close()
				return nil, problemf(ErrInvalid, "目录请下载为 ZIP")
			case info.Size() > MaxDownloadBytes:
				_ = f.Close()
				return nil, problemf(ErrTooLarge, "超过 100 MiB 下载上限")
			}
			return &Transfer{Name: path.Base(p), Size: info.Size(), binding: b, path: p, file: f, identity: fileIdentity(info)}, nil
		})
	}, func(tr *Transfer, done func()) { tr.release = done })
}
